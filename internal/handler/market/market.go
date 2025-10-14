package market

import (
	"edgeflow/internal/service"
	"edgeflow/pkg/response"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
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
	mu            sync.RWMutex

	// 仅维护所有活跃的连接
	clients map[*ClientConn]struct{}

	upgrader websocket.Upgrader
}

func NewMarketHandler(ms *service.MarketDataService) *MarketHandler {
	h := &MarketHandler{
		marketService: ms,
		clients:       make(map[*ClientConn]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	// ⚠️ 核心：启动协程监听 MarketDataService 的排序结果通道
	go h.listenForSortedIDs()
	// 启动实时价格推送
	go h.listenForPriceUpdates()
	// 启动币种上新下架推送
	go h.listenForInstrumentUpdates()
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

		// 2. 广播给所有客户端
		h.mu.RLock()
		for client := range h.clients {
			select {
			case client.Send <- data:
			default:
				// 队列满则丢弃
			}
		}
		h.mu.RUnlock()
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

		// 2. 广播给所有活跃的客户端
		// ⚠️ 注意：价格更新通常需要全量广播，因为所有客户端都需要它。
		h.mu.RLock()
		for client := range h.clients {
			select {
			case client.Send <- data:
			default:
				// 队列满则丢弃，保证主循环不阻塞
			}
		}
		h.mu.RUnlock()
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

		// 2. 广播给所有活跃的客户端
		h.mu.RLock()
		for client := range h.clients {
			select {
			case client.Send <- data:
			default:
				// 队列满则丢弃
			}
		}
		h.mu.RUnlock()
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
		Conn:    conn,
		Send:    make(chan []byte, 100),
		Symbols: make(map[string]struct{}), // 保持 ClientConn 结构不变，但 Symbols 不再用于订阅 OKX
	}

	// 加入管理 map
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	// 连接成功后，立即发送当前的 SortedInstIDs 状态，客户端不需要获取就主动推送一次
	go h.sendInitialSortData(client)

	defer func() {
		// 清理连接
		h.mu.Lock()
		delete(h.clients, client)
		h.mu.Unlock()
		conn.Close()
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
func (h *MarketHandler) handleChangeSort(c *ClientConn, msg ClientMessage) {
	if msg.SortBy == "" {
		log.Println("SortBy field missing in change_sort request.")
		return
	}

	// 1. 调用 MarketDataService 更改全局排序配置
	err := h.marketService.ChangeSortField(msg.SortBy)
	if err != nil {
		log.Printf("Failed to change sort field to %s: %v", msg.SortBy, err)
		// 建议向客户端发送错误通知
		return
	}

	// 2. ⚠️ 后续：MarketDataService 应该在后台重新排序，并
	//    通过通道推送新的 sortedIDList，然后由 listenForSortedIDs 广播给所有客户端。
	//    无需在此处做进一步的推送。

	// 可选：立即返回当前第一页数据

	h.handleGetPage(c, &ClientMessage{Action: "get_page", Page: 1, Limit: 50})
}

// handleGetPage 处理客户端的分页请求
func (h *MarketHandler) handleGetPage(c *ClientConn, clientMsg *ClientMessage) {
	if clientMsg.Page <= 0 {
		clientMsg.Page = 1
	}
	if clientMsg.Limit <= 0 {
		clientMsg.Limit = 50
	}

	// 1. 从 MarketDataService 获取分页后的 TradingItem 列表（包含 K线和 Ticker）
	// 假设 GetPagedData(page, limit) 返回 []TradingItem
	pagedData, err := h.marketService.GetPagedData(clientMsg.Page, clientMsg.Limit)
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
