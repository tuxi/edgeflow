package market

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"strconv"
	"strings"
	"sync"
)

type ClientMessage struct {
	Action string `json:"action"` // get_page | change_sort
	// 分页参数 (用于 get_page)
	//Page  int `json:"page"`
	//Limit int `json:"limit"`
	//// 排序字段 (用于 change_sort)
	//SortBy string `json:"sort_by"` // 例如 "volume", "price_change"
	Payload map[string]string `json:"payload"`
}

type ClientConn struct {
	Conn    *websocket.Conn
	Send    chan []byte // 异步发送通道
	Symbols map[string]struct{}

	mu sync.Mutex
	// Key: 订阅标识 (例如 "BTC-USDT-15m")，Value: struct{}
	CandleSubscriptions map[string]struct{}
}

func (c *ClientConn) writePump() {
	defer c.Conn.Close()
	for msg := range c.Send {
		if strings.Contains(string(msg), "subscribe_candle") {
			fmt.Printf("发送subscribe_candle消息")
		}
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Println("write error:", err)
			break
		}
	}
}

// readPump 读取客户端消息
func (c *ClientConn) readPump(h *MarketHandler) {

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
			log.Println("read error:", err)
			break // 退出循环，触发 defer
		}

		var clientMsg ClientMessage

		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Println("invalid message format, skipping:", string(msg))
			continue
		}

		// 2. 根据 Action 处理请求
		switch clientMsg.Action {
		case "get_page":
			// 客户端请求某一页的数据 (分页和排序结果)
			// 这一步是同步的，直到数据返回
			// 分页参数 (用于 get_page)
			pageStr := clientMsg.Payload["page"]
			limitStr := clientMsg.Payload["limit"]
			page, _ := strconv.ParseInt(pageStr, 10, 64)
			limit, _ := strconv.ParseInt(limitStr, 10, 64)
			h.handleGetPage(c, int(page), int(limit))

		case "change_sort":
			// 客户端请求改变排序字段 (例如从 Volume 变更为 Price Change)
			if sortBy, ok := clientMsg.Payload["sort_by"]; ok {
				h.handleChangeSort(c, sortBy)
			}

		case "subscribe_candle":
			period := clientMsg.Payload["time_period"]
			symbol := clientMsg.Payload["inst_id"]
			h.handleSubscribeCandle(c, symbol, period)
		case "unsubscribe_candle":
			period := clientMsg.Payload["time_period"]
			symbol := clientMsg.Payload["inst_id"]
			h.handleUnsubscribeCandle(c, symbol, period)
		default:
			log.Println("Unsupported action received:", clientMsg.Action)
		}

		// ⚠️ 注意：MarketHandler 的 handleGetPage 和 handleChangeSort 内部不应再需要 h.mu.Lock()。
		// 因为它们要么是同步查询 MarketDataService，要么是更新全局配置，不涉及多个goroutine竞争ClientConn map。
		// 因此，此处不再需要 h.mu.Lock() 整个 switch 块。
	}
}

// 当前连接是否订阅了k线
func (c *ClientConn) isSubscribedCandle(instId string, period string) bool {
	subKey := fmt.Sprintf("%s-%s", instId, period)
	if _, ok := c.CandleSubscriptions[subKey]; ok {
		return true
	}
	return false
}
