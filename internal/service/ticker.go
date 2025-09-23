package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"sync"
	"time"
)

// 更新实时价格的服务

// TickerData 封装单个币种的实时行情数据
type TickerData struct {
	InstId    string  `json:"inst_id"`     // 币种符号，例如 BTC-USDT
	LastPrice float64 `json:"last_price"`  // 最新成交价格
	Vol24h    float64 `json:"vol_24h"`     // 24小时成交量单位币
	VolCcy24h float64 `json:"vol_ccy_24h"` // 24小时成交量
	High24h   float64 `json:"high_24h"`    // 24小时最高价
	Low24h    float64 `json:"low_24h"`     // 24小时最低价
	Open24h   float64 `json:"open_24h"`    // 24小时开盘价格
	Change24h float64 `json:"change_24h"`  // 24小时涨跌幅（%）
	AskPx     float64 `json:"ask_px"`      // 卖一价（最低的卖单价）
	AskSz     float64 `json:"ask_sz"`      // 卖一量
	BidPx     float64 `json:"bid_px"`      // 买一价（最高的买单价）
	BidSz     float64 `json:"bid_sz"`      // 买一量
}

// TickerService 定义行情服务接口
type TickerService interface {
	// SubscribeSymbols 订阅一个或多个币种的实时行情
	SubscribeSymbols(ctx context.Context, symbols []string) error

	// UnsubscribeSymbols 取消订阅某些币种
	UnsubscribeSymbols(ctx context.Context, symbols []string) error

	// GetPrice 获取某个币种的最新行情数据
	GetPrice(ctx context.Context, symbol string) (*TickerData, error)

	// GetPrices 获取多个币种的最新行情数据
	GetPrices(ctx context.Context, symbols []string) ([]*TickerData, error)

	// Close 关闭行情服务连接（例如 WebSocket）
	Close() error
}

// OKXTickerService 基于 OKX WebSocket 的实现
type OKXTickerService struct {
	sync.RWMutex
	conn        *websocket.Conn
	subscribed  map[string]struct{}
	prices      map[string]*TickerData
	url         string
	closeCh     chan struct{}
	lastRequest time.Time
}

// NewOKXTickerService 创建实例并连接 OKX WebSocket
func NewOKXTickerService() (*OKXTickerService, error) {
	url := "wss://ws.okx.com:8443/ws/v5/public"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	s := &OKXTickerService{
		conn:       conn,
		subscribed: make(map[string]struct{}),
		prices:     make(map[string]*TickerData),
		url:        url,
		closeCh:    make(chan struct{}),
	}

	err = s.SubscribeSymbols(context.Background(), []string{"BTC-USDT"})
	if err != nil {
		log.Println(err)
	}
	go s.readLoop()
	return s, nil
}

func (c *OKXTickerService) readLoop() {
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			c.reconnect() // 出错时尝试重连
			return
		}

		c.handleMessage(msg)
	}
}

func (client *OKXTickerService) sendMessage(message interface{}) error {
	// Ensure at least 50ms between requests
	timeSinceLastRequest := time.Since(client.lastRequest)
	if timeSinceLastRequest < 50*time.Millisecond {
		time.Sleep(50*time.Millisecond - timeSinceLastRequest)
	}
	client.lastRequest = time.Now()

	return client.conn.WriteJSON(message)
}

func (s *OKXTickerService) reconnect() {

	// 关闭旧连接
	_ = s.conn.Close()

	// 循环尝试重连
	for {
		conn, _, err := websocket.DefaultDialer.Dial(s.url, nil)
		if err != nil {
			log.Println("OKXTickerService reconnect failed, retrying in 2s:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		s.Lock()
		s.conn = conn
		s.Unlock()

		log.Println("OKXTickerService reconnected to OKX WebSocket")

		// 重新订阅之前的币种
		s.subscribeAll()
		break
	}
}

func (s *OKXTickerService) subscribeAll() {
	symbols := make([]string, 0, len(s.subscribed))
	for sym := range s.subscribed {
		symbols = append(symbols, sym)
	}
	if len(symbols) > 0 {
		_ = s.SubscribeSymbols(context.Background(), symbols)
	}
}

// handleMessage 处理 OKX 推送消息
func (s *OKXTickerService) handleMessage(msg []byte) {
	var raw map[string]interface{}
	if err := json.Unmarshal(msg, &raw); err != nil {
		log.Println("OKXTickerService json unmarshal error:", err)
		return
	}

	if evt, ok := raw["event"].(string); ok { // {"event":"subscribe","arg":{"channel":"tickers","instId":"BTC-USDT"},"connId":"6b422e60"}
		switch evt {
		case "ping":
			// OKX 发了心跳请求，回 pong
			pong := map[string]string{"event": "pong"}
			data, _ := json.Marshal(pong)
			if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("write pong error: %v", err)
			}
			log.Println("Received ping, sent pong")
			return

		case "error":
			log.Printf("Error from OKX: %v", raw)
			return
		}
	}

	arg, ok := raw["arg"].(map[string]interface{})
	if !ok {
		return
	}

	channel, ok := arg["channel"].(string)
	if !ok || channel != "tickers" {
		return
	}

	dataArr, ok := raw["data"].([]interface{})
	if !ok {
		return
	}

	s.Lock()
	defer s.Unlock()
	for _, d := range dataArr {
		item := d.(map[string]interface{})
		instId := item["instId"].(string)

		lastPrice := parseFloat(item["last"])      // 最新成交价格
		vol24h := parseFloat(item["vol24h"])       // 24小时成交量（以交易标的计，比如 BTC）
		volCcy24h := parseFloat(item["volCcy24h"]) // 24小时成交量（以计价货币计，比如 USDT）
		high24h := parseFloat(item["high24h"])     // 24小时最高价
		low24h := parseFloat(item["low24h"])       // 24小时最低价
		open24h := parseFloat(item["open24h"])     // 24小时开盘价
		askPx := parseFloat(item["askPx"])         // 卖一价（最低的卖单价）
		askSz := parseFloat(item["askSz"])         // 卖一量
		bidPx := parseFloat(item["bidPx"])         // 买一价（最高的买单价）
		bidSz := parseFloat(item["bidSz"])         // 买一量

		change24h := 0.0
		if open24h != 0 {
			change24h = (lastPrice - open24h) / open24h * 100
		}

		s.prices[instId] = &TickerData{
			InstId:    instId,
			LastPrice: lastPrice,
			Vol24h:    vol24h,
			VolCcy24h: volCcy24h,
			High24h:   high24h,
			Low24h:    low24h,
			Open24h:   open24h,
			Change24h: change24h,
			AskPx:     askPx,
			AskSz:     askSz,
			BidPx:     bidPx,
			BidSz:     bidSz,
		}
	}
}

// parseFloat 辅助解析 float
func parseFloat(v interface{}) float64 {
	switch t := v.(type) {
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	case float64:
		return t
	default:
		return 0
	}
}

// SubscribeSymbols 批量订阅
func (s *OKXTickerService) SubscribeSymbols(ctx context.Context, symbols []string) error {

	s.Lock()
	defer s.Unlock()
	// 只订阅新币种， 过滤掉已经订阅过的
	var toSubscribe []string
	for _, sym := range symbols {
		if _, ok := s.subscribed[sym]; !ok {
			toSubscribe = append(toSubscribe, sym)
			s.subscribed[sym] = struct{}{}
		}
	}
	if toSubscribe == nil || len(toSubscribe) == 0 {
		return nil
	}

	args := []map[string]string{}
	for _, sym := range toSubscribe {
		s.subscribed[sym] = struct{}{}
		args = append(args, map[string]string{
			"channel": "tickers",
			"instId":  sym,
		})
	}

	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}
	return s.conn.WriteJSON(subMsg)
}

// UnsubscribeSymbols 取消订阅
func (s *OKXTickerService) UnsubscribeSymbols(ctx context.Context, symbols []string) error {
	args := []map[string]string{}
	s.Lock()
	for _, sym := range symbols {
		delete(s.subscribed, sym)
		args = append(args, map[string]string{
			"channel": "tickers",
			"instId":  sym,
		})
	}
	s.Unlock()

	unsubMsg := map[string]interface{}{
		"op":   "unsubscribe",
		"args": args,
	}
	return s.conn.WriteJSON(unsubMsg)
}

// GetPrice 获取单个币种行情
func (s *OKXTickerService) GetPrice(ctx context.Context, symbol string) (*TickerData, error) {
	s.RLock()
	defer s.RUnlock()
	data, ok := s.prices[symbol]
	if !ok {
		return nil, fmt.Errorf("price not found for symbol: %s", symbol)
	}
	return data, nil
}

// GetPrices 获取多个币种行情
func (s *OKXTickerService) GetPrices(ctx context.Context, symbols []string) ([]*TickerData, error) {
	s.RLock()
	defer s.RUnlock()
	//result := make(map[string]*TickerData)
	var result []*TickerData
	for _, sym := range symbols {
		if data, ok := s.prices[sym]; ok {
			//result[sym] = data
			result = append(result, data)
		}
	}
	return result, nil
}

// Close 关闭服务
func (s *OKXTickerService) Close() error {
	close(s.closeCh)
	return s.conn.Close()
}
