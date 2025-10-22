package market

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

type ClientMessage struct {
	Action  string            `json:"action"` // get_page | change_sort ｜ subscribe_candle ｜ unsubscribe_candle
	Payload map[string]string `json:"payload"`

	/*
		payload中可能包含的字段跟需求有关，目前
		分页参数 (用于 get_page)
		Page  int `json:"page"`
		Limit int `json:"limit"`
		排序字段 (用于 change_sort)
		SortBy string `json:"sort_by"` // 例如 "volume", "price_change"
		查询k线数据
		InstId string `json:"inst_id"`
		time_period
	*/
}

// 订阅键的格式为：<Channel>:<Symbol>:<Period/Detail>
// 例如：
// - K线： "CANDLE:BTC-USDT:15m"
// - 深度： "DEPTH:BTC-USDT:L5" (Level 5)
// - 交易： "TRADE:BTC-USDT:SPOT"
type ClientConn struct {
	ClientID  string // 用于识别客户端
	Conn      *websocket.Conn
	Send      chan []byte // 异步发送通道
	replaced  bool        // 标记该连接是否已被新的重连连接替换
	mu        sync.Mutex
	closeOnce sync.Once

	closed int32 // 0 未关闭，1 已关闭

	// 升级为通用的订阅映射
	Subscriptions map[string]struct{} // Key: 通用订阅键
}

// Close 优雅地关闭连接和相关资源
// 注意：Conn.Close() 会导致 writePump 退出，从而触发 ServeWS 的 defer 逻辑
func (c *ClientConn) Close() {
	c.closeOnce.Do(func() {

		// 标记已关闭 原子存储
		atomic.StoreInt32(&c.closed, 1)

		// 先尝试关闭底层 websocket
		if c.Conn != nil {
			c.Conn.Close()
		}
		// 确保 Send Channel 被关闭，这将最终导致 writePump 退出
		// 理论上，Conn.Close() 触发 writePump 退出后，writePump 应该自己关闭 Send
		// 但为了安全起见，我们在外部控制关闭，并在广播时使用 safeSend
		// 💡 为了解决 panic，我们让 safeSend 来处理写入已关闭通道的 panic，
		// 而这里负责关闭通道。
		defer func() {
			// 捕获 close(c.Send) 时的潜在 panic，如果它已经被关闭
			if r := recover(); r != nil {
				log.Printf("WARNING: ClientConn.close() -- Panic when trying to close client Send channel: %v", r)
			}
		}()

		// 关闭发送 Channel
		// 只有在这里关闭，才能保证 Channel 只关闭一次
		// 并且会通知所有正在等待 c.Send 的 Goroutine 停止
		// 关闭此通道，通知 writePump 退出
		close(c.Send)
	})
}

func (c *ClientConn) writePump() {

	//defer c.Conn.Close()
	for msg := range c.Send {
		// 注意：现在给客户端发送的都是protobuf消息二进制数据，所以不能使用websocket.TextMessage
		if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			log.Println("write error:", err)
			break
		}
	}
}

// readPump 读取客户端消息
func (c *ClientConn) readPump(h *SubscriptionGateway) {

	// 设置读消息超时时间等 (此处省略)

	defer func() {
		log.Println("ClientConn client disconnected")
		// ⚠️ 确保在断开时从 h.clients 移除连接 (参见上一个回答的 ServeWS defer 逻辑)
	}()

	for {
		// 1. 读取原始消息
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			// 客户端断开连接、网络错误等
			log.Println("ClientConn 读取客户端消息:", err)
			break // 退出循环，触发 defer
		}

		var clientMsg ClientMessage

		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Println("invalid message format, skipping:", string(msg))
			continue
		}

		// 2. 根据 Action 处理请求
		switch clientMsg.Action {
		case "get_page", "change_sort":
			// 核心：Subscription Gateway 忽略这些请求，可以返回错误
			log.Printf("WARN: SubGateway received Ticker/Sort request: %s. Use TickerGateway.", clientMsg.Action)

		case "subscribe_candle":
			channel := "CANDLE" // 硬编码为 K线频道
			period := clientMsg.Payload["time_period"]
			symbol := clientMsg.Payload["inst_id"]
			subKey := fmt.Sprintf("%s:%s:%s", channel, symbol, period)
			h.handleSubscribe(c, subKey)
		case "unsubscribe_candle":
			channel := "CANDLE"
			period := clientMsg.Payload["time_period"]
			symbol := clientMsg.Payload["inst_id"]
			subKey := fmt.Sprintf("%s:%s:%s", channel, symbol, period)
			h.removeSubscriptionFromMapByClientID(subKey, c.ClientID)
		default:
			log.Println("Unsupported action received:", clientMsg.Action)
		}

		// ⚠️ 注意：MarketHandler 的 handleGetPage 和 handleChangeSort 内部不应再需要 h.mu.Lock()。
		// 因为它们要么是同步查询 MarketDataService，要么是更新全局配置，不涉及多个goroutine竞争ClientConn map。
		// 因此，此处不再需要 h.mu.Lock() 整个 switch 块。
	}
}

func (c *ClientConn) isClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

// 当前连接是否订阅了k线
func (c *ClientConn) isSubscribedCandle(instId string, period string) bool {
	subKey := fmt.Sprintf("%s-%s", instId, period)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Subscriptions[subKey]; ok {
		return true
	}
	return false
}

func (c *ClientConn) GetInstIdByCandleKey(key string) (instId, period string, ok bool) {
	parts := strings.Split(key, "-")
	if len(parts) >= 3 {
		instId = parts[0] + "-" + parts[1]
		period = parts[2]
		ok = true
	}
	return
}

// safeSend 尝试向客户端通道发送数据，并在通道关闭时安全地捕获 panic。
// 这是一个关键的 panic 防御机制。
func (c *ClientConn) safeSend(data []byte) (sent bool) {
	defer func() {
		// 如果写入已关闭的通道，这里会捕获 panic (runtime error: send on closed channel)
		if r := recover(); r != nil {
			log.Printf("ERROR: Recovered panic during broadcast to ClientID %s. Channel likely closed: %v", c.ClientID, r)
			sent = false
		}
	}()

	if c.isClosed() {
		// 已经关闭通道 丢弃
		return false
	}

	// 非阻塞发送，避免阻塞广播 goroutine
	select {
	case c.Send <- data:
		return true
	default:
		// 队列满则丢弃
		return false
	}
}
