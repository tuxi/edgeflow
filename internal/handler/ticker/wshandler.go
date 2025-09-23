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
	service service.TickerService
	mu      sync.RWMutex
	// 每个币种对应的订阅客户端集合
	symbolSubscribers map[string]map[*ClientConn]struct{}
	// 每个连接订阅的币种
	clientSymbols map[*ClientConn]map[string]struct{}
	upgrader      websocket.Upgrader
}

func NewHandler(s service.TickerService) *Handler {
	return &Handler{
		service:           s,
		symbolSubscribers: make(map[string]map[*ClientConn]struct{}),
		clientSymbols:     make(map[*ClientConn]map[string]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域
		},
	}
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

		var clientMsg struct {
			Action  string   `json:"action"`  // subscribe | unsubscribe
			Symbols []string `json:"symbols"` // 币种列表
		}

		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Println("invalid message:", err)
			continue
		}

		h.mu.Lock()
		switch clientMsg.Action {
		case "subscribe":
			for _, s := range clientMsg.Symbols {
				if _, ok := h.symbolSubscribers[s]; !ok {
					h.symbolSubscribers[s] = make(map[*ClientConn]struct{})
					// 第一次订阅，向 OKX 发请求
					err := h.service.SubscribeSymbols(context.Background(), clientMsg.Symbols)
					if err != nil {
						log.Printf("订阅okx ws失败 : %v\n", err)
					}
				}
				h.symbolSubscribers[s][c] = struct{}{}
				h.clientSymbols[c][s] = struct{}{}
			}
		case "unsubscribe":
			for _, s := range clientMsg.Symbols {
				delete(h.symbolSubscribers[s], c)
				delete(h.clientSymbols[c], s)

				if len(h.symbolSubscribers[s]) == 0 {
					// 没有任何人订阅了，才向 OKX 退订
					h.service.UnsubscribeSymbols(context.Background(), clientMsg.Symbols)
					delete(h.symbolSubscribers, s)
				}
			}
		}
		h.mu.Unlock()
	}
}
