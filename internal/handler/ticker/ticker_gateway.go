package ticker

import (
	"context"
	"edgeflow/internal/service"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

/*
在我们的新架构中，客户端获取分页数据时，必须依赖 MarketDataService 内存中已经计算好的 SortedInstIDs 列表。

客户端获取行情数据：正确的流程设计

客户端在加载行情列表时，应遵循以下 两个独立且并行 的步骤：

步骤 1：获取排序索引和实时更新 (WebSocket)

客户端一进入行情页，就必须建立 WebSocket 连接。

WS 消息 (Push)	数据源	目的
update_sort	MarketDataService.SortedInstIDs	核心： 客户端获取当前全局、按成交量排序的交易对 ID 列表。客户端UI依赖此列表来确定顺序。
price_update	MarketDataService 实时转发	客户端获取所有币种的实时价格进行闪烁更新。
步骤 2：获取分页详细数据 (WebSocket 请求/响应)

一旦客户端知道整体顺序（有了 SortedInstIDs 列表），它就知道当前页面应该展示哪些 InstID（例如，列表的前 50 个 ID）。

客户端随后向服务端发送一个数据请求，要求获取这些 ID 的详细信息。

客户端请求 (WS)	消息体	服务端处理流程
get_page	{"action": "get_page", "page": 1, "limit": 50}	1. 读取 MarketDataService.SortedInstIDs。2. 根据 Page/Limit 切片出当前页的 InstID 列表（例如前 50 个）。3. 从 TradingItems 字典中查找并返回这 50 个完整的 TradingItem。

*/

const cleanupGrace = 10 * time.Second

// TickerGateway 负责实时价格、排序、币种上下架的全量广播
type TickerGateway struct {
	marketService *service.MarketDataService
	consumer      kafka.ConsumerService // Kafka Consumer
	mu            sync.Mutex            // 保护 clients map 写入

	// 仅维护所有活跃的连接 (COW 模式不变)
	clients atomic.Value // 存储 map[string]*ClientConn

	upgrader websocket.Upgrader
	// Ticker Gateway 不再需要 cleanupMap，因为它不管理复杂的订阅状态。
	// 但是，为了实现重连的优雅替换，我们保留它来处理连接的替换逻辑。
	cleanupMap sync.Map
}

func NewTickerGateway(ms *service.MarketDataService, consumer kafka.ConsumerService) *TickerGateway {
	g := &TickerGateway{
		marketService: ms,
		consumer:      consumer,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	g.clients.Store(make(map[string]*TickerClientConn))

	// 核心启动：消费 Kafka Ticker 数据
	go g.listenForTickerUpdates()
	// 核心启动：消费 Kafka System 数据
	go g.listenForSystemUpdates()

	return g
}

// ServeWS 仅处理连接建立和断开
func (h *TickerGateway) ServeWS(c *gin.Context) {

	// 获取clientId
	clientID := c.Query("client_id")
	if clientID == "" {
		// 强制要求客户端提供唯一的ID，否则拒绝连接
		// 或者生成一个临时的UUID作为Client ID
		log.Println("客户单缺少client_id 拒绝连接.")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	newClient := &TickerClientConn{
		ClientID:             clientID,
		Conn:                 conn,
		Send:                 make(chan []byte, 1000),
		LastSuccessfulSendTs: time.Now().UnixNano(), // 将上次成功发送时间初始化为当前时间
	}

	// 收集需要恢复的订阅列表
	var oldClient *TickerClientConn
	var isFromCleanupMap bool

	// 查找旧连接：先查找活跃的map，后查找cleanup map
	// 从活跃连接中查找
	h.mu.Lock()
	oldClients := h.clients.Load().(map[string]*TickerClientConn)
	if existingClient, found := oldClients[clientID]; found {
		oldClient = existingClient
	}
	h.mu.Unlock()

	// 如果活跃连接中没有，则检查 cleanupMap (处理宽限期内的重连)
	if oldClient == nil {
		if conn, loaded := h.cleanupMap.Load(clientID); loaded {
			oldClient = conn.(*TickerClientConn)
			isFromCleanupMap = true
			log.Printf("ClientID %s found in cleanup map. Restoring state from grace period.", clientID)
			// 立即从 cleanupMap 中移除，阻止计时器清理
			h.cleanupMap.Delete(clientID)
		}
	}

	// 执行状态迁移 （前提是找到了旧的连接）
	if oldClient != nil {
		log.Printf("ClientID %s reconnected. Starting state migration.", clientID)

		// 🚨 锁住旧连接的本地状态，执行迁移
		oldClient.mu.Lock()
		// 标记旧连接已被替换，阻止其 defer/cleanup 逻辑执行 Unsubscribe
		oldClient.replaced = true
		oldClient.mu.Unlock()
	}

	// 执行CoW替换新连接 （原子操作）
	h.mu.Lock()
	{
		// 重新加载最新的活跃连接 map
		oldClients = h.clients.Load().(map[string]*TickerClientConn)
		newClients := make(map[string]*TickerClientConn, len(oldClients))

		// 复制旧的 map
		for k, v := range oldClients {
			newClients[k] = v
		}

		// 替换或添加新连接
		newClients[clientID] = newClient
		h.clients.Store(newClients)
	}
	h.mu.Unlock()

	// 异步清理旧连接
	// 立即关闭旧连接，使其 readPump/writePump 退出，defer 逻辑触发
	if oldClient != nil && !isFromCleanupMap {
		// 先关闭底层连接，关闭后会触发旧 client 的 defer 逻辑
		go oldClient.Close() // 推荐异步关闭，避免阻塞 ServeWS
		log.Printf("Closed old connection for ClientID %s.", clientID)
	}

	// 连接成功后，立即发送当前的 SortedInstIDs 状态，客户端不需要获取就主动推送一次
	// 连接成功后，立即发送当前的 SortedInstIDs 状态
	go h.sendInitialSystemState(newClient)
	defer func() {

		// 清理当前新连接（在连接断开时）
		h.mu.Lock()
		{
			oldClients := h.clients.Load().(map[string]*TickerClientConn)
			// 只有当要移除的 client 仍然是当前 ClientID 对应的 *ClientConn 时才移除
			if currentClient, exists := oldClients[clientID]; exists && currentClient == newClient {
				newClients := make(map[string]*TickerClientConn, len(oldClients))
				for k, v := range oldClients {
					if k != clientID { // 按 ClientID 移除
						newClients[k] = v
					}
				}
				h.clients.Store(newClients)
				log.Printf("ClientID %s connection removed from handler.", clientID)
			} else {
				// 如果不相等，说明这个连接已经被一个更新的连接覆盖了，无需从 clients map 中移除
				log.Printf("ClientID %s defer: Connection already replaced, skip map removal.", clientID)
			}
		}
		h.mu.Unlock()

		// 延迟清理逻辑
		// **判断是否已被新连接替换**
		newClient.mu.Lock()
		isReplaced := newClient.replaced // 检查是否是由于重连而断开的
		newClient.mu.Unlock()

		if isReplaced {
			log.Printf("ClientID %s defer: Connection was replaced by a new connection, no cleanup needed.", clientID)
			return
		}

		// 此时，连接是由于超时或客户端主动断开的，但未被替换，需要启动宽限期清理。
		log.Printf("ClientID %s defer: Connection lost. Starting %s cleanup grace period.", clientID, cleanupGrace)

		// 启动宽限期清理
		// 立即从活跃连接 map 中移除后，将其移交给 cleanupMap
		h.cleanupMap.Store(clientID, newClient)

		// 启动一个协程，在宽限期后执行清理
		go func() {
			time.Sleep(cleanupGrace)

			// 1. 检查 cleanupMap 中是否仍存在这个 ClientID
			if conn, loaded := h.cleanupMap.Load(clientID); loaded {
				// 2. 再次检查 conn.replaced 标记 (防止竞态条件)
				clientToCleanup := conn.(*TickerClientConn)
				clientToCleanup.mu.Lock()
				// 仅检查是否被替换，无需执行任何 Unsubscribe
				if !clientToCleanup.replaced {
					log.Printf("ClientID %s: Grace period ended. Final cleanup.", clientID)
				}
				clientToCleanup.mu.Unlock()

				// 3. 无论是清理还是被替换，最终都从 cleanupMap 中移除
				h.cleanupMap.Delete(clientID)
			}
		}()

		// 确保资源关闭
		newClient.Close()
	}()

	// 启动协程
	go newClient.writePump() // 不断从 Send channel 取消息，然后写入 webscoekt
	// 循环读取客户端发来的消息，要求阻塞线程
	// ⚠️这里会阻塞serverWs方法，直到客户端断开连接，断开后会进入defer 清理
	newClient.readPump(h)
}

// 监听ticker价格变化，优先级高
func (g *TickerGateway) listenForTickerUpdates() {
	// Ticker 高频主题
	tickerCh, err := g.consumer.Consume(context.Background(), kafka.TopicTicker, "edgeflow_ticker_gateway_group")
	if err != nil {
		log.Fatalf("未能启动Ticker的kafka消费者： %v", err)
	}
	// 让kafka消费和定时器分开在不同的gotine，防止kafka阻塞定时器发送消息

	for msg := range tickerCh {
		// msg.key 是币种的 symbol
		// 打包成一个消息或者多条广播
		g.broadcast(msg.Value)
	}

}

// 监听其他数据变化，优先级低与Ticker
func (g *TickerGateway) listenForSystemUpdates() {
	// System 为低频主题
	systemCh, err := g.consumer.Consume(context.Background(), kafka.TopicSystem, "edgeflow_ticker_system_group")
	if err != nil {
		log.Fatalf("未能启动System的kafka消费者: %v", err)
	}

	for message := range systemCh {
		key := string(message.Key)
		if key == "INSTRUMENT_CHANGE" {
			var pbMsg pb.WebSocketMessage
			err := proto.Unmarshal(message.Value, &pbMsg)
			if err != nil {
				continue
			}
			update := pbMsg.GetInstrumentStatusUpdate()
			if update != nil {
				g.marketService.UpdateInstruments(update.DelistedInstruments, update.NewInstruments)
				g.broadcast(message.Value)
			}
		} else {
			g.broadcast(message.Value)
		}
	}
}

// 通用广播函数
func (g *TickerGateway) broadcast(data []byte) {
	currentClients, ok := g.clients.Load().(map[string]*TickerClientConn)
	if !ok {
		return
	}

	// 全量广播
	for _, client := range currentClients {
		client.safeSend(data)
	}
}

// MarketHandler.sendInitialSortData 负责在连接建立时发送当前状态
func (h *TickerGateway) sendInitialSystemState(client *TickerClientConn) {

	// 1. 从 MarketDataService 获取当前的排序 ID 列表
	currentIDs, sortBy := h.marketService.GetSortedIDsl()

	// 2. 构造Protobuf 消息并发送给客户端
	payload := &pb.SortUpdate{
		SortBy:        sortBy,
		SortedInstIds: currentIDs,
	}
	protobufMsg := &pb.WebSocketMessage{
		Type: "SORT_UPDATE",
		Payload: &pb.WebSocketMessage_SortUpdate{
			SortUpdate: payload,
		},
	}

	// 将Protobuf 消息结构体转换为[]byte二进制数据
	data, err := proto.Marshal(protobufMsg)
	if err != nil {
		log.Fatalf("Protobuf 序列化消息失败: %v", err)
	}

	// 3. 发送给新的客户端
	client.safeSend(data)
}

// handleGetPage 收到处理客户端的分页请求
func (h *TickerGateway) handleGetPage(c *TickerClientConn, page, limit int) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}

	// 1. 从 MarketDataService 获取分页后的 TradingItem 列表（包含 K线和 Ticker）
	// 假设 GetPagedData(page, limit) 返回 []TradingItem
	pagedData, err := h.marketService.GetPagedData(page, limit)
	if err != nil {
		log.Println("Error getting paged data:", err)
		return
	}

	// 2. 构造为Protobuf并发送给客户端
	var items []*pb.CryptoInstrumentTradingItem
	for _, item := range pagedData {
		var tags []*pb.CryptoTag
		for _, tag := range item.Coin.Tags {
			tags = append(tags, &pb.CryptoTag{
				Id:          uint32(tag.ID),
				Name:        tag.Name,
				Description: tag.Description,
			})
		}
		coin := &pb.CryptoInstrumentMetadata{
			Id:             item.Coin.ID,
			InstrumentId:   item.Coin.InstrumentID,
			ExchangeId:     uint32(item.Coin.ExchangeID),
			BaseCcy:        item.Coin.BaseCcy,
			QuoteCcy:       item.Coin.QuoteCcy,
			NameCn:         item.Coin.NameCN,
			NameEn:         item.Coin.NameEN,
			Status:         item.Coin.Status,
			PricePrecision: item.Coin.PricePrecision,
			QtyPrecision:   item.Coin.QtyPrecision,
			MarketCap:      item.Coin.MarketCap,
			IsContract:     item.Coin.IsContract,
			Tags:           tags,
		}
		ticker := &pb.TickerUpdate{
			InstId:     item.Ticker.InstId,
			LastPrice:  item.Ticker.LastPrice,
			Vol_24H:    item.Ticker.Vol24h,
			VolCcy_24H: item.Ticker.VolCcy24h,
			High_24H:   item.Ticker.High24h,
			Low_24H:    item.Ticker.Low24h,
			Open_24H:   item.Ticker.Open24h,
			Change_24H: item.Ticker.Change24h,
			AskPx:      item.Ticker.AskPx,
			AskSz:      item.Ticker.AskSz,
			BidPx:      item.Ticker.BidPx,
			BidSz:      item.Ticker.BidSz,
			Ts:         item.Ticker.Ts,
		}
		data := &pb.CryptoInstrumentTradingItem{
			InstrumentMetadata: coin,
			TickerUpdate:       ticker,
		}
		items = append(items, data)
	}

	protobufMsg := &pb.WebSocketMessage{ // paged_data
		Type: "inst_Trading_List",
		Payload: &pb.WebSocketMessage_InstrumentTradingList{
			InstrumentTradingList: &pb.CryptoInstrumentTradingArray{Data: items},
		},
	}

	// 将Protobuf 消息结构体转换为[]Byte 二进制数据
	data, err := proto.Marshal(protobufMsg)
	if err != nil {
		log.Fatalf("Protobuf 序列化 失败: %v", err)
	}
	c.safeSend(data)
}

// MarketHandler.handleChangeSort 示例
func (h *TickerGateway) handleChangeSort(c *TickerClientConn, sortBy string) {
	if sortBy == "" {
		log.Println("SortBy field missing in change_sort request.")
		return
	}

	// 1. 调用 MarketDataService 更改全局排序配置
	err := h.marketService.ChangeSortField(sortBy)
	if err != nil {
		log.Printf("Failed to change sort field to %s: %v", sortBy, err)
		// 建议向客户端发送错误通知
		return
	}

	// 2. ⚠️ 后续：MarketDataService 应该在后台重新排序，并
	//    通过通道推送新的 sortedIDList，然后由 listenForSortedIDs 广播给所有客户端。
	//    无需在此处做进一步的推送。

	// 可选：立即返回当前第一页数据

	h.handleGetPage(c, 1, 50)
}
