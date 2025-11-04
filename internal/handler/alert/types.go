package alert

import (
	"github.com/gorilla/websocket"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ======================== ClientConn ========================
type AlertClientConn struct {
	ClientID string
	Conn     *websocket.Conn
	Send     chan []byte

	replaced  bool
	mu        sync.Mutex
	closeOnce sync.Once

	// 丢弃统计（可用于强制关闭慢消费者）
	DroppedCount int32
	LastSuccess  int64
}

func (c *AlertClientConn) Close() {
	c.closeOnce.Do(func() {
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
		// close send channel safely
		defer func() {
			if r := recover(); r != nil {
				// already closed
			}
		}()
		close(c.Send)
	})
}

// writePump 负责写入到 websocket （包括 ping）
func (c *AlertClientConn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				// channel closed
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Println("writePump write error:", err)
				return
			}
			atomic.StoreInt64(&c.LastSuccess, time.Now().UnixNano())
			atomic.StoreInt32(&c.DroppedCount, 0)
		case <-ticker.C:
			// send ping
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("writePump ping error:", err)
				return
			}
		}
	}
}

// readPump 读取客户端消息（处理心跳/客户端动作）
func (c *AlertClientConn) readPump(g *AlertGateway) {
	// 设置 Pong handler 和读 deadline
	c.Conn.SetReadLimit(1024 * 1024)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(appData string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			// 断开或 read error
			//log.Println("readPump read error:", err)
			break
		}
		// 简单日志，可扩展为处理客户端命令（订阅/取消订阅/ACK 等）
		log.Printf("recv from %s: %s", c.ClientID, string(msg))
	}
}

// safeSend 非阻塞发送并在通道满时进行计数与保护
func (c *AlertClientConn) safeSend(data []byte) bool {
	defer func() {
		if r := recover(); r != nil {
			// send on closed channel
		}
	}()

	select {
	case c.Send <- data:
		atomic.StoreInt32(&c.DroppedCount, 0)
		return true
	default:
		// channel full -> increase drop count
		cnt := atomic.AddInt32(&c.DroppedCount, 1)
		// 若超过阈值则主动关闭
		if cnt > 200 {
			log.Printf("AlertClientConn: client %s drop > threshold, closing", c.ClientID)
			go c.Close()
		}
		return false
	}
}
