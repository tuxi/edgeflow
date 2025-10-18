package market

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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

// MarketHandler 负责客户端连接管理和数据分发
type MarketHandler struct {
	marketService *service.MarketDataService
	candleClient  *service.OKXCandleService // 实时k线数据源
	mu            sync.Mutex                // 这里使用Mutx （只需要在写操作时保护client的更新）

	// 仅维护所有活跃的连接
	// 存储*ClientConn 集合快照，使用atomic.Value 保证读取时无损，这就是使用Copy-onWrite(CoW)模式减少对公共资源的锁竞争和持有时间
	clients atomic.Value // 存储 map[*ClientConn]struct{}

	upgrader websocket.Upgrader
}

func NewMarketHandler(ms *service.MarketDataService, candleClient *service.OKXCandleService) *MarketHandler {
	h := &MarketHandler{
		marketService: ms,
		candleClient:  candleClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	// 首次初始化clients map
	h.clients.Store(make(map[*ClientConn]struct{}))

	// ⚠️ 核心：启动协程监听 MarketDataService 的排序结果通道
	go h.listenForSortedIDs()
	// 启动实时价格推送
	go h.listenForPriceUpdates()
	// 启动币种上新下架推送
	go h.listenForInstrumentUpdates()
	// 启动 K 线实时推送
	// 这与 listenForPriceUpdates 互不干扰，完全隔离
	go h.listenForCandleUpdates()
	// 启动订阅消息的错误
	go h.listenForSubscriptionErrors()
	return h
}

// listenForInstrumentUpdates 监听币种上下架通知并广播给所有客户端
func (h *MarketHandler) listenForInstrumentUpdates() {
	updateCh := h.marketService.GetInstrumentUpdateChannel()

	for update := range updateCh {

		// 1. 构造 JSON 消息
		message := map[string]interface{}{
			"action": "instrument_update", // 客户端识别的 action
			"data":   update,              // {NewInstruments: [...], DelistedInstruments: [...]}
		}
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Error marshalling instrument update: %v", err)
			continue
		}

		// 无锁获取当前clients的快照，原本的map需要使用h.mu.RLock()
		currentClients, ok := h.clients.Load().(map[*ClientConn]struct{})
		if !ok {
			return
		}
		// 2. 广播给所有客户端
		for client := range currentClients {
			select {
			case client.Send <- data:
			default:
				// 队列满则丢弃
			}
		}
	}
}

// 实时价格推送
func (h *MarketHandler) listenForPriceUpdates() {
	// 监听来自 MarketDataService 的实时 Ticker 更新
	priceUpdatesCh := h.marketService.GetPriceUpdateChannel()

	for ticker := range priceUpdatesCh {

		// 1. 构造 JSON 消息：只推送一个 Ticker 的数据
		message := map[string]interface{}{
			"action": "price_update",
			"data":   ticker, // 客户端只需要根据 InstID 快速更新 UI
		}
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Error marshalling price update: %v", err)
			continue
		}

		// 无锁获取当前clients的快照
		currentClients, ok := h.clients.Load().(map[*ClientConn]struct{})
		if !ok {
			return
		}

		// 2. 广播给所有活跃的客户端
		// ⚠️ 注意：价格更新通常需要全量广播，因为所有客户端都需要它。

		for client := range currentClients {
			select {
			case client.Send <- data:
			default:
				// 队列满则丢弃，保证主循环不阻塞
			}
		}
	}
}

// listenForSortedIDs 监听 MarketDataService 推送的排序 ID 列表，并广播给所有客户端
func (h *MarketHandler) listenForSortedIDs() {
	// 假设 MarketDataService 提供了这个通道，它在排序发生变化时发送最新的 [InstID]
	sortedIDsCh := h.marketService.GetSortedIDsChannel()

	for newSortedIDs := range sortedIDsCh {

		// 1. 构造 JSON 消息 (Action: "update_sort", Data: [IDs])
		message := map[string]interface{}{
			"action": "update_sort",
			"data":   newSortedIDs,
		}
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Error marshalling sorted IDs: %v", err)
			continue
		}

		// 无所获取当前clients的快照
		currentClients, ok := h.clients.Load().(map[*ClientConn]struct{})
		if !ok {
			return
		}

		// 2. 广播给所有活跃的客户端

		for client := range currentClients {
			select {
			case client.Send <- data:
			default:
				// 队列满则丢弃
			}
		}
	}
}

// 监听新的k线数据，并定向推送给需要的客户端
func (h *MarketHandler) listenForCandleUpdates() {
	candleCh := h.candleClient.GetCandleChannel()

	for kline := range candleCh {
		// 无锁获取clients的快照
		currentClients, ok := h.clients.Load().(map[*ClientConn]struct{})
		if !ok {
			return
		}
		// kline: map[string]model2.Kline (Key: "BTC-USDT-15m")
		// 迭代所有客户端，需要加读锁
		for client := range currentClients {
			// 迭代收到的k线数据
			for subKey, klineData := range kline { // subKey 是 "BTC-USDT-15m"
				// 过滤订阅了这条数据的客户端
				if _, subscribed := client.CandleSubscriptions[subKey]; subscribed {
					// 构造消息
					message := map[string]interface{}{
						"action": "candle_update",
						"data":   klineData,
					}
					data, _ := json.Marshal(message)
					// 定向推送
					select {
					case client.Send <- data:
					default:
						// 队列满则丢弃
					}
				}
			}
		}
	}

}

// MarketHandler
func (h *MarketHandler) listenForSubscriptionErrors() {
	// 获取错误通道
	errorCh := h.candleClient.GetErrorChannel()

	for subErr := range errorCh {

		// 1. 构造一个错误消息给客户端
		// 这个错误通常只通知给**发起请求的客户端**。
		// 由于这里是广播，我们假设您可能希望通知所有客户端，或仅记录日志。

		jsonData, err := json.Marshal(subErr)
		if err != nil {
			log.Printf("Error marshalling subscription error: %v", err)
			continue
		}

		period := subErr.Data["period"]
		symbol := subErr.Data["symbol"]

		// 无锁获取clients的快照
		currentLients, ok := h.clients.Load().(map[*ClientConn]struct{})
		if !ok {
			return
		}

		// 2. 广播给所有客户端（如果您不知道哪个客户端发起的请求）
		// 如果您的业务要求只通知发起请求的客户端，您需要在订阅时记录 clientID/connID

		for client := range currentLients {
			if client.isSubscribedCandle(symbol, period) {
				// 这里可以加入 client 过滤逻辑，例如：
				// if client.isSubscribed(subErr.Symbol, subErr.Period) { ... }
				select {
				case client.Send <- jsonData:
				default:
					// 队列满则丢弃
				}
			}
		}
	}
}

// ServeWS 仅处理连接建立和断开
func (h *MarketHandler) ServeWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	client := &ClientConn{
		Conn:                conn,
		Send:                make(chan []byte, 100),
		Symbols:             make(map[string]struct{}), // 保持 ClientConn 结构不变，但 Symbols 不再用于订阅 OKX
		CandleSubscriptions: make(map[string]struct{}),
	}

	// 加入管理 map
	h.mu.Lock()
	// 获取旧的clients map副本
	oldClients := h.clients.Load().(map[*ClientConn]struct{})
	// 创建一个新的clients map 副本
	newClients := make(map[*ClientConn]struct{}, len(oldClients)+1)
	// 拷贝旧数据
	for k, v := range oldClients {
		newClients[k] = v
	}
	// 添加新的client
	newClients[client] = struct{}{}
	// 原子性替换
	h.clients.Store(newClients)
	h.mu.Unlock()

	// 连接成功后，立即发送当前的 SortedInstIDs 状态，客户端不需要获取就主动推送一次
	go h.sendInitialSortData(client)

	defer func() {
		// 清理连接
		h.mu.Lock()
		oldClients := h.clients.Load().(map[*ClientConn]struct{})
		newClients := make(map[*ClientConn]struct{}, len(oldClients)+1)
		// 拷贝旧的数据，并移除client
		for k, v := range oldClients {
			if k != client {
				newClients[k] = v
			}
		}
		h.clients.Store(newClients)
		h.mu.Unlock()
		conn.Close()

		client.mu.Lock()
		// 处理客户端所有未取消的k线订阅
		for subKey := range client.CandleSubscriptions {
			// 找到对应的symbol和period
			parts := strings.Split(subKey, "-")
			if len(parts) >= 3 {
				symbol := parts[0] + "-" + parts[1]
				period := parts[2]
				// 取消订阅
				err := h.candleClient.UnsubscribeCandle(context.Background(), symbol, period)
				if err != nil {
					log.Printf("WARNING: Cleanup unsubscribe failed for %s: %v", subKey, err)
				} else {

				}
			}
		}
		client.mu.Unlock()
	}()

	// 不断从 Send channel 取消息，然后写入 webscoekt
	go client.writePump()
	// 循环读取客户端发来的消息，要求阻塞线程
	client.readPump(h)
}

// MarketHandler.sendInitialSortData 负责在连接建立时发送当前状态
func (h *MarketHandler) sendInitialSortData(client *ClientConn) {

	// 1. 从 MarketDataService 获取当前的排序 ID 列表
	currentIDs := h.marketService.GetSortedIDsl()

	// 2. 构造消息
	message := map[string]interface{}{
		"action": "update_sort",
		"data":   currentIDs,
	}
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshalling initial sort data: %v", err)
		return
	}

	// 3. 发送给新的客户端
	select {
	case client.Send <- data:
		log.Println("Sent initial sorted IDs to new client.")
	default:
		log.Println("Client send channel full during initial send.")
	}
}

// MarketHandler.handleChangeSort 示例
func (h *MarketHandler) handleChangeSort(c *ClientConn, sortBy string) {
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

// 收到订阅k线行情的消息
func (h *MarketHandler) handleSubscribeCandle(client *ClientConn, symbol string, period string) {
	subKey := fmt.Sprintf("%s-%s", symbol, period) // e.g., "BTC-USDT-15m"
	// 管理客户端订阅状态，必须加锁
	client.mu.Lock()
	client.CandleSubscriptions[subKey] = struct{}{}
	client.mu.Unlock()

	// SubscribeCandle内部会检查是否有其他客户端已订阅了该频道
	err := h.candleClient.SubscribeCandle(context.Background(), symbol, period)
	if err != nil {
		log.Printf("Failed to subscribe %s to OKX: %v", subKey, err)
		// 订阅失败，回滚客户端状态（可选）
		client.mu.Lock()
		delete(client.CandleSubscriptions, subKey)
		client.mu.Unlock()

		//  构造错误消息
		errMsg := fmt.Sprintf("Subscription to %s failed: %v", subKey, err)
		clientErr := model.NewClientError("subscribe_candle", "400", errMsg, map[string]string{
			"symbol": symbol,
			"period": period,
		})

		data, marshalErr := json.Marshal(clientErr)
		if marshalErr != nil {
			log.Printf("Error marshalling internal error message: %v", marshalErr)
			return
		}

		// 定向发送错误给发起请求的客户端
		select {
		case client.Send <- data:
			log.Printf("Sent subscription failure notification to client.")
		default:
			log.Println("Client send channel full during error notification.")
		}

	}

}

func (h *MarketHandler) handleUnsubscribeCandle(client *ClientConn, symbol string, period string) error {
	subKey := fmt.Sprintf("%s-%s", symbol, period)
	client.mu.Lock()
	defer client.mu.Unlock()
	if _, exists := client.CandleSubscriptions[subKey]; !exists {
		return nil
	}

	// 从客户端本地状态中移除
	delete(client.CandleSubscriptions, subKey)

	err := h.candleClient.UnsubscribeCandle(context.Background(), symbol, period)
	if err != nil {
		// 发送错误消息给客户端
	}
	return err
}

// handleGetPage 收到处理客户端的分页请求
func (h *MarketHandler) handleGetPage(c *ClientConn, page, limit int) {
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

	// 2. 封装并发送给客户端
	message := map[string]interface{}{
		"action": "paged_data",
		"data":   pagedData,
	}
	data, _ := json.Marshal(message)

	select {
	case c.Send <- data:
	default:
		log.Println("Client send channel full, dropping paged data.")
	}
}

func (h *MarketHandler) SortedInstIDsGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentIDs := h.marketService.GetSortedIDsl()

		response.JSON(ctx, nil, currentIDs)
	}
}

func (m *MarketHandler) GetDetail() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.MarketDetailReq
		if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := m.marketService.GetDetailByID(ctx, req)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}
