package ticker

import (
	"context"
	"edgeflow/internal/service"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 客户端请求的消息格式
type ClientMessage struct {
	Action  string   `json:"action"`  // subscribe | unsubscribe
	Symbols []string `json:"symbols"` // ["BTC-USDT", "ETH-USDT"]
}

type ClientConn struct {
	Conn    *websocket.Conn
	Send    chan []byte // 异步发送通道
	Symbols map[string]struct{}
}

type Handler struct {
	service *service.OKXTickerService
	mu      sync.RWMutex
	// 每个币种对应的订阅客户端集合
	symbolSubscribers map[string]map[*ClientConn]struct{}
	// 每个连接订阅的币种
	clientSymbols map[*ClientConn]map[string]struct{}
	upgrader      websocket.Upgrader
}

func NewHandler(s *service.OKXTickerService) *Handler {
	h := &Handler{
		service:           s,
		symbolSubscribers: make(map[string]map[*ClientConn]struct{}),
		clientSymbols:     make(map[*ClientConn]map[string]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域
		},
	}
	//  启动 OKX 重连监听协程
	go h.handleOKXReconnect()
	return h
}

// 处理okx的重连业务
func (h *Handler) handleOKXReconnect() {
	for range h.service.ReconnectCh {
		// 1. 获取所有活跃的币种列表
		activeSymbols := h.getAllActiveSymbols()

		// 2. 调用 OKXTickerService 的重订阅方法
		// 注意：这里我们使用 ResubscribeAll，而不是 SubscribeSymbols，
		// 以确保 OKX 服务端重置了状态并全量订阅。
		err := h.service.ResubscribeAll(activeSymbols)

		if err != nil {
			// 错误处理：记录日志，如果失败可能需要进一步断开连接，等待下一次重连
			log.Printf("Failed to resubscribe active symbols after OKX reconnect: %v", err)
		}
	}
}

// 计算当前所有客户端正在订阅的币种列表
func (h *Handler) getAllActiveSymbols() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 遍历 symbolSubscribers，收集所有键
	symbols := make([]string, 0, len(h.symbolSubscribers))
	for sym, subscribers := range h.symbolSubscribers {
		// 严格来说，应该检查是否有活动的订阅者 (len(subscribers) > 0)
		// 但如果symbolSubscribers 只有在有订阅者时才存在，则无需检查。
		if len(subscribers) > 0 {
			symbols = append(symbols, sym)
		}
	}
	return symbols
}

// 用来接收客户端的 WebSocket 消息
func (h *Handler) ServeWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	client := &ClientConn{
		Conn:    conn,
		Send:    make(chan []byte, 100),
		Symbols: make(map[string]struct{}),
	}
	// 加入管理 map
	h.mu.Lock()
	h.clientSymbols[client] = client.Symbols
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		// 1. 移除该连接订阅的币种
		// 1. 移除该连接订阅的币种
		if symbols, ok := h.clientSymbols[client]; ok {
			for s := range symbols {
				delete(h.symbolSubscribers[s], client)
				// 如果没有任何客户端订阅该币种，真正向 OKX 退订
				if len(h.symbolSubscribers[s]) == 0 {
					h.service.UnsubscribeSymbols(context.Background(), []string{s})
					delete(h.symbolSubscribers, s)
				}
			}
			delete(h.clientSymbols, client)
		}

		conn.Close()
	}()

	// 不断从 Send channel 取消息，然后写入 WebSockek
	go client.writePump()
	// 循环读取客户端发来的消息，要求阻塞线程
	client.readPump(h)
}

// 发送最新行情给订阅的连接
func (h *Handler) BroadcastPrices() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.RLock()
		for client, symbolsMap := range h.clientSymbols {
			// 转成 slice 方便服务端获取行情
			symbolsSlice := make([]string, 0, len(symbolsMap))
			for s := range symbolsMap {
				symbolsSlice = append(symbolsSlice, s)
			}

			if len(symbolsSlice) == 0 {
				continue
			}

			// 从服务端获取行情
			prices, err := h.service.GetPrices(context.Background(), symbolsSlice)
			if err != nil {
				log.Println("GetPrices error:", err)
				continue
			}

			// 转成 JSON 发给客户端
			data, _ := json.Marshal(prices)

			select {
			case client.Send <- data: // 异步发送行情给客户端
			default:
				// 队列满就丢掉或记录
			}
		}
		h.mu.RUnlock()
	}
}

func (h *Handler) TickerGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		symbol := c.Query("symbol") // GET /price?symbol=BTC-USDT
		if symbol == "" {
			c.JSON(400, gin.H{"error": "symbol is required"})
			return
		}

		priceData, err := h.service.GetPrice(c, symbol)
		if err != nil {
			c.JSON(404, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"data": priceData})
	}
}

func (h *Handler) TickersGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		symbols := c.QueryArray("symbol") // GET /prices?symbol=BTC-USDT&symbol=ETH-USDT
		if len(symbols) == 0 {
			c.JSON(400, gin.H{"error": "at least one symbol is required"})
			return
		}

		pricesMap, err := h.service.GetPrices(c, symbols)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"data": pricesMap})
	}
}

func (c *ClientConn) writePump() {
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Println("write error:", err)
			break
		}
	}
}

// readPump 读取客户端消息
func (c *ClientConn) readPump(h *Handler) {
	defer func() {
		log.Println("ClientConn client disconnected")
	}()
	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			break
		}

		var clientMsg subscribeMessage

		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Println("invalid message:", err)
			continue
		}

		switch clientMsg.Action {
		case "subscribe":
			h.handleOnSubscribe(c, &clientMsg)
		case "unsubscribe":
			h.handleOnUnsubscribe(c, &clientMsg)
		}
	}
}

// 收到客户单取消订阅的处理
func (h *Handler) handleOnUnsubscribe(c *ClientConn, clientMsg *subscribeMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var newlyUnsubscribedFromOKX []string // 存储本次操作中，计数归零的币种

	// 1. 遍历所有请求取消订阅的币种
	for _, sym := range clientMsg.Symbols {
		// 确保该币种和客户端存在订阅关系
		if _, ok := h.symbolSubscribers[sym]; !ok {
			continue // 如果全局或客户端连接中没有这个订阅，跳过
		}

		// 核心：减少计数（通过删除 map key 来实现计数-1）
		delete(h.symbolSubscribers[sym], c)

		// 清理 clientSymbols 中的对应关系
		if h.clientSymbols[c] != nil {
			delete(h.clientSymbols[c], sym)
		}

		// 检查是否计数归零
		if len(h.symbolSubscribers[sym]) == 0 {
			// 这是该币种的最后一个订阅者退出，需要通知 OKXService 退订
			newlyUnsubscribedFromOKX = append(newlyUnsubscribedFromOKX, sym)

			// ❗ 注意：这里不删除 h.symbolSubscribers[sym]，等待 OKXService 成功返回
		}
	}

	// 2. 检查是否有需要向 OKXService 退订的币种
	if len(newlyUnsubscribedFromOKX) == 0 {
		return
	}

	// 3. 一次性通知 OKXService 退订所有计数归零的币种
	err := h.service.UnsubscribeSymbols(context.Background(), newlyUnsubscribedFromOKX)

	// 4. 根据 OKXService 的结果，更新本地状态
	if err != nil {
		// 关键：如果 OKXService 退订失败，我们不能清理本地状态！
		// 状态保持不变：len(h.symbolSubscribers[sym]) 仍然是 0，但该键仍存在。
		// 这表示：OKX 仍在推送数据，但 Handler 知道客户端已全部退出。
		// （在下一次重连时，如果该键仍存在，Handler 会尝试再次退订）。
		log.Printf("ERROR: Failed to unsubscribe from OKX WS for symbols %v: %v. Local state maintained.", newlyUnsubscribedFromOKX, err)
		return
	}

	// 5. 成功退订：清理本地状态
	for _, sym := range newlyUnsubscribedFromOKX {
		// 只有在 OKXService 成功退订后，才删除该币种在全局订阅中的记录
		delete(h.symbolSubscribers, sym)
	}

	// 可选：清理 clientSymbols 中空 map (如果需要)
	if len(h.clientSymbols[c]) == 0 {
		delete(h.clientSymbols, c)
	}
}

// 收到客户端订阅的处理
func (h *Handler) handleOnSubscribe(c *ClientConn, clientMsg *subscribeMessage) {

	h.mu.Lock()
	defer h.mu.Unlock()

	var newlySubscribedByClients []string // 存储本次操作中，被首次客户端订阅的币种

	// 1. 遍历所有请求的币种，更新本地状态，并收集新增项
	for _, sym := range clientMsg.Symbols {
		// 检查该币种在全局是否有订阅者
		if _, ok := h.symbolSubscribers[sym]; !ok {
			// 这是该币种的第一个订阅者，需要向 OKXService 发送订阅请求
			h.symbolSubscribers[sym] = make(map[*ClientConn]struct{})
			newlySubscribedByClients = append(newlySubscribedByClients, sym)
		}

		// 无论是否首次，都记录该客户端的订阅需求
		h.symbolSubscribers[sym][c] = struct{}{}
		h.clientSymbols[c][sym] = struct{}{} // 记录客户端-币种关系
	}

	// 2. 检查是否有新增币种需要通知 OKXService
	if len(newlySubscribedByClients) == 0 {
		return // 没有新增订阅，直接返回
	}

	// 3. 一次性通知 OKXService 订阅所有新增币种
	// OKXService 内部会接收这个列表，并最终发送 subscribe 消息
	err := h.service.SubscribeSymbols(context.Background(), newlySubscribedByClients)
	if err != nil {
		// **关键：如果 OKX 订阅失败，必须回滚本地状态！**
		log.Printf("ERROR: Failed to subscribe to OKX WS for symbols %v: %v. Rolling back local state.", newlySubscribedByClients, err)

		// 回滚：清理本次新增的订阅状态
		for _, sym := range newlySubscribedByClients {
			// 移除该客户端的订阅
			delete(h.symbolSubscribers[sym], c)
			// 如果移除后集合为空，则清理 map
			if len(h.symbolSubscribers[sym]) == 0 {
				delete(h.symbolSubscribers, sym)
			}
			delete(h.clientSymbols[c], sym)
		}
	}
	// 如果成功，本地状态保持不变（已更新），不需要进一步操作
}

type subscribeMessage struct {
	Action  string   `json:"action"`  // subscribe | unsubscribe
	Symbols []string `json:"symbols"` // 币种列表
}
