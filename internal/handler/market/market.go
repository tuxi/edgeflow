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
	clients atomic.Value // 存储 map[string]*ClientConn

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
	h.clients.Store(map[string]*ClientConn{})

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

// ServeWS 仅处理连接建立和断开
func (h *MarketHandler) ServeWS(c *gin.Context) {

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

	newClient := &ClientConn{
		ClientID:            clientID,
		Conn:                conn,
		Send:                make(chan []byte, 100),
		CandleSubscriptions: make(map[string]struct{}),
	}

	// 收集需要恢复的订阅列表
	var subscriptionsToRestore []string

	// 2. 覆盖旧连接 (原子操作)
	var oldClient *ClientConn

	h.mu.Lock()
	{
		oldClients := h.clients.Load().(map[string]*ClientConn)

		// 检查是否有旧连接，如果有，先保存旧连接，以便关闭
		if existingClient, found := oldClients[clientID]; found {
			oldClient = existingClient
			log.Printf("ClientID %s reconnected. Replacing old connection.", clientID)

			// --- 状态迁移核心逻辑 ---
			// 1. 获取旧连接的 CandleSubscriptions 状态
			oldClient.mu.Lock() // 锁住旧连接的本地状态
			// 复制订阅状态到新连接
			for subKey := range oldClient.CandleSubscriptions {
				newClient.CandleSubscriptions[subKey] = struct{}{}
				subscriptionsToRestore = append(subscriptionsToRestore, subKey) // 收集 key
			}
			log.Printf("ClientID %s: Migrated %d subscriptions to new connection.", clientID, len(newClient.CandleSubscriptions))

			// ⚠️ 关键点：对于外部资源 (h.candleClient)，不需要在此时调用 UnsubscribeCandle，
			// 因为订阅是基于 (symbol, period) 的引用计数，只要有一个活跃连接订阅了，
			// h.candleClient 就不会真正取消订阅。
			// 我们只需要在旧连接的 defer 中阻止它执行不必要的 UnsubscribeCandle 即可。
			// -----------------------

			// 标记旧连接已被替换，阻止其 defer 逻辑执行 Unsubscribe
			oldClient.replaced = true
			// 💡 可以在此清空 oldClient.CandleSubscriptions，以节省内存和确保安全
			oldClient.CandleSubscriptions = nil
			oldClient.mu.Unlock()
			log.Printf("ClientID %s: Migrated %d subscriptions to new connection.",
				clientID, len(newClient.CandleSubscriptions))
		}

		// 创建新副本并替换/添加
		newClients := make(map[string]*ClientConn, len(oldClients))
		for k, v := range oldClients {
			newClients[k] = v
		}

		newClients[clientID] = newClient // 替换或添加
		h.clients.Store(newClients)
	}
	h.mu.Unlock()

	// 3. 异步清理旧连接
	// 立即关闭旧连接，使其 readPump/writePump 退出，defer 逻辑触发
	if oldClient != nil {
		// 先关闭底层连接，关闭后会触发旧 client 的 defer 逻辑
		go oldClient.Close() // 推荐异步关闭，避免阻塞 ServeWS
		log.Printf("Closed old connection for ClientID %s.", clientID)
	}

	// 连接成功后，立即发送当前的 SortedInstIDs 状态，客户端不需要获取就主动推送一次
	go h.sendInitialSortData(newClient)

	// 4. 异步恢复外部订阅 (新连接特有的步骤)
	// 必须异步执行，以避免阻塞 ServeWS 主线程
	if len(subscriptionsToRestore) > 0 {
		go h.restoreCandleSubscriptions(newClient, subscriptionsToRestore)
	}

	defer func() {

		// 4. 清理当前新连接（在连接断开时）
		h.mu.Lock()
		{
			oldClients := h.clients.Load().(map[string]*ClientConn)
			// 只有当要移除的 client 仍然是当前 ClientID 对应的 *ClientConn 时才移除
			if currentClient, exists := oldClients[clientID]; exists && currentClient == newClient {
				newClients := make(map[string]*ClientConn, len(oldClients))
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

		conn.Close()

		newClient.mu.Lock()
		// 检查是否已被替换：如果被替换，则跳过 Unsubscribe 逻辑
		if newClient.replaced {
			log.Printf("ClientID %s: Skip external unsubscribe (replaced by new connection).", newClient.ClientID)
			newClient.mu.Unlock()
			return
		}
		// 处理客户端所有未取消的k线订阅
		for subKey := range newClient.CandleSubscriptions {
			// 找到对应的symbol和period
			symbol, period, ok := newClient.GetInstIdByCandleKey(subKey)
			if ok {
				// 取消订阅
				err := h.candleClient.UnsubscribeCandle(context.Background(), symbol, period)
				if err != nil {
					log.Printf("WARNING: Cleanup unsubscribe failed for %s: %v", subKey, err)
				} else {

				}
			}
		}
		newClient.mu.Unlock()

		// 确保资源关闭
		newClient.Close()
	}()

	// 启动协程
	go newClient.writePump() // 不断从 Send channel 取消息，然后写入 webscoekt
	// 循环读取客户端发来的消息，要求阻塞线程
	// ⚠️这里会阻塞serverWs方法，直到客户端断开连接，断开后会进入defer 清理
	newClient.readPump(h)
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
		currentClients, ok := h.clients.Load().(map[string]*ClientConn)
		if !ok {
			return
		}
		// 2. 广播给所有客户端
		for _, client := range currentClients {
			client.safeSend(data)
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
		currentClients, ok := h.clients.Load().(map[string]*ClientConn)
		if !ok {
			return
		}

		// 2. 广播给所有活跃的客户端
		// ⚠️ 注意：价格更新通常需要全量广播，因为所有客户端都需要它。

		for _, client := range currentClients {
			client.safeSend(data)
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
		currentClients, ok := h.clients.Load().(map[string]*ClientConn)
		if !ok {
			return
		}

		// 2. 广播给所有活跃的客户端

		for _, client := range currentClients {
			// 使用safeSend 替代select/default 避免写入已关闭的通道panic
			client.safeSend(data)
		}
	}
}

// 监听新的k线数据，并定向推送给需要的客户端
func (h *MarketHandler) listenForCandleUpdates() {
	candleCh := h.candleClient.GetCandleChannel()

	for kline := range candleCh {
		// 无锁获取clients的快照
		currentClients, ok := h.clients.Load().(map[string]*ClientConn)
		if !ok {
			return
		}
		// kline: map[string]model2.Kline (Key: "BTC-USDT-15m")
		// 迭代所有客户端，需要加读锁
		for _, client := range currentClients {

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
					client.safeSend(data)
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
				client.safeSend(jsonData)
			}
		}
	}
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
	client.safeSend(data)
}

func (h *MarketHandler) restoreCandleSubscriptions(conn *ClientConn, subscribes []string) {
	for _, subKey := range subscribes {
		symbol, period, ok := conn.GetInstIdByCandleKey(subKey)
		if ok {
			h.handleSubscribeCandle(conn, symbol, period)
		}
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
		client.safeSend(data)
	}

}

// 客户端主动取消订阅时调用
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
