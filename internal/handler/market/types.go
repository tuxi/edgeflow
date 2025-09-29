package market

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
)

type ClientMessage struct {
	Action string `json:"action"` // get_page | change_sort
	// 分页参数 (用于 get_page)
	Page  int `json:"page"`
	Limit int `json:"limit"`
	// 排序字段 (用于 change_sort)
	SortBy string `json:"sort_by"` // 例如 "volume", "price_change"
}

type ClientConn struct {
	Conn    *websocket.Conn
	Send    chan []byte // 异步发送通道
	Symbols map[string]struct{}
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

		var clientMsg ClientMessage // ⚠️ 使用新的 ClientMessage 结构

		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Println("invalid message format, skipping:", string(msg))
			continue
		}

		// 2. 根据 Action 处理请求
		switch clientMsg.Action {
		case "get_page":
			// 客户端请求某一页的数据 (分页和排序结果)
			// 这一步是同步的，直到数据返回
			h.handleGetPage(c, &clientMsg)

		case "change_sort":
			// 客户端请求改变排序字段 (例如从 Volume 变更为 Price Change)
			h.handleChangeSort(c, clientMsg)

		default:
			log.Println("Unsupported action received:", clientMsg.Action)
		}

		// ⚠️ 注意：MarketHandler 的 handleGetPage 和 handleChangeSort 内部不应再需要 h.mu.Lock()。
		// 因为它们要么是同步查询 MarketDataService，要么是更新全局配置，不涉及多个goroutine竞争ClientConn map。
		// 因此，此处不再需要 h.mu.Lock() 整个 switch 块。
	}
}
