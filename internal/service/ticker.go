package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"strings"
	"sync"
	"time"
)

// 更新实时价格的服务

// TickerData 封装单个币种的实时行情数据
type TickerData struct {
	InstId    string  `json:"inst_id"`     // 币种符号，例如 BTC-USDT
	LastPrice string  `json:"last_price"`  // 最新成交价格
	Vol24h    string  `json:"vol_24h"`     // 24小时成交量单位币
	VolCcy24h string  `json:"vol_ccy_24h"` // 24小时成交量
	High24h   string  `json:"high_24h"`    // 24小时最高价
	Low24h    string  `json:"low_24h"`     // 24小时最低价
	Open24h   string  `json:"open_24h"`    // 24小时开盘价格
	Change24h float64 `json:"change_24h"`  // 24小时涨跌幅（%）
	AskPx     string  `json:"ask_px"`      // 卖一价（最低的卖单价）
	AskSz     string  `json:"ask_sz"`      // 卖一量
	BidPx     string  `json:"bid_px"`      // 买一价（最高的买单价）
	BidSz     string  `json:"bid_sz"`      // 买一量
	Ts        float64 `json:"ts"`          // 时间戳
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
	GetPrices(ctx context.Context, symbols []string) (map[string]TickerData, error)

	// Close 关闭行情服务连接（例如 WebSocket）
	Close() error
}

// OKXTickerService 基于 OKX WebSocket 的实现
type OKXTickerService struct {
	sync.RWMutex
	conn *websocket.Conn
	// 记录 OKX 连接上实际已发送 subscribe 消息的币种集合
	subscribed map[string]struct{}
	prices     map[string]TickerData
	url        string
	closeCh    chan struct{}

	lastRequest time.Time
	// 新增：系统必须保持订阅的币种列表
	defaultSymbols []string
	// 连接成功建立后，会向此通道发送一个信号
	connectionReady chan struct{}
	// 收到行情变化的通道
	tickersCh chan map[string]TickerData
}

// NewOKXTickerService 创建实例并连接 OKX WebSocket
func NewOKXTickerService(defaultSymbols []string) *OKXTickerService {
	url := "wss://ws.okx.com:8443/ws/v5/public"
	for i, coin := range defaultSymbols {
		parts := strings.Split(coin, "/")
		if len(parts) == 1 { // 防止BTC-USDT-SWAP
			parts = strings.Split(coin, "-")
		}
		if len(parts) == 1 {
			// 如果只有一部分，拼接上-USDT
			symbol := fmt.Sprintf("%v-USDT", parts[0])
			defaultSymbols[i] = symbol
		}
	}
	s := &OKXTickerService{
		conn:            nil,
		subscribed:      make(map[string]struct{}),
		prices:          make(map[string]TickerData),
		tickersCh:       make(chan map[string]TickerData, 100), // 设置一个合理的缓冲区大小（例如 100），防止数据堆积阻塞上游
		url:             url,
		closeCh:         make(chan struct{}),
		defaultSymbols:  defaultSymbols,
		connectionReady: make(chan struct{}), //非缓冲冲到
	}

	go s.run()

	return s
}

// GetTickerChannel 供下游服务（如 MarketDataService）获取并监听 Ticker 数据流
func (s *OKXTickerService) GetTickerChannel() <-chan map[string]TickerData {
	return s.tickersCh
}

// startPingLoop 在每次新连接建立后调用
func (s *OKXTickerService) startPingLoop(conn *websocket.Conn) {
	// 间隔时间应该小于 OKX 的超时时间 (例如 30s，我们设置为 15s)
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	// Ping 消息内容，例如 OKX 的格式
	//pingMsg := map[string]string{"op": "ping"}

	for {
		select {
		case <-ticker.C:
			// 收到定时信号，发送 Ping
			s.RLock()
			// 确保使用传入的连接对象，而不是可能已被替换的 s.conn
			err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
			s.RUnlock()

			if err != nil {
				// 如果写入失败 (连接可能已断开)，记录日志并退出此协程
				log.Printf("Ping failed on current connection: %v. Stopping ping loop.", err)
				return
			}

		// 关键：监听 s.closeCh 或其他退出信号
		case <-s.closeCh:
			// 收到服务关闭信号，优雅退出
			return
		}
	}
}

// 订阅初始币种 订阅默认列表中的所有币种
func (s *OKXTickerService) subscribeDefaults() error {
	// 订阅默认列表，无需过滤，因为这是首次订阅
	err := s.sendSubscribe(s.defaultSymbols)
	if err == nil {
		// 订阅成功后，更新本地状态
		s.Lock()
		for _, sym := range s.defaultSymbols {
			s.subscribed[sym] = struct{}{}
		}
		s.Unlock()
	}
	return err
}

// 发送消息的入口： 所有的发送消息都要从这里
// 内部方法，必须在持有 s.Lock() 或 s.RLock() 时调用。
// 它负责限速，但不再负责 Mutex 加锁。
func (s *OKXTickerService) writeMessageInternal(message interface{}) error {
	// 假设外部已加锁 s.Lock()

	// 确保至少 50ms 之间有间隔（限速逻辑需要保护）
	timeSinceLastRequest := time.Since(s.lastRequest)
	if timeSinceLastRequest < 50*time.Millisecond {
		time.Sleep(50*time.Millisecond - timeSinceLastRequest)
	}
	s.lastRequest = time.Now()

	// conn.WriteJSON 也应在锁保护下，但由于 Mutex 已经在外部持有，这里直接使用
	if s.conn == nil {
		return errors.New("当前ws连接不存在，请先建立连接")
	}
	return s.conn.WriteJSON(message)
}

func (s *OKXTickerService) run() {
	// 持续运行的主循环，负责连接和重连，永远不要退出
	for {
		// 1. 尝试建立连接
		conn, _, err := websocket.DefaultDialer.Dial(s.url, nil)
		if err != nil {
			log.Println("OKXTickerService connection failed, retrying in 2s:", err)
			time.Sleep(2 * time.Second)
			continue // 继续下一次循环，重试连接
		}

		// 2. 连接成功：更新状态
		s.Lock()
		s.conn = conn

		// ⚠️ 核心：通知等待者，连接已就绪
		// 使用 select 是为了防止重复发送导致 panic (如果 run 被多次调用)
		select {
		case s.connectionReady <- struct{}{}:
			log.Println("OKX WebSocket connection established and ready.")
		default:
			// 已经是 ready 状态，忽略
		}

		// 关键：重置订阅状态，因为这是一个新连接
		s.subscribed = make(map[string]struct{})

		// 每次连接成功，都要重置closeCh，以确保旧的Ping/Listen协程能够退出
		// 并且新的 closeCh 可以用于控制新的协程
		if s.closeCh != nil {
			close(s.closeCh) // 关闭旧的closeCh，通知旧协程退出
		}
		// 创建新的协程
		s.closeCh = make(chan struct{})

		s.Unlock()

		log.Println("OKXTickerService connection established/reconnected to OKX WebSocket")

		// 初始/恢复订阅默认币种
		err = s.subscribeDefaults()
		if err != nil {
			log.Printf("Failed to subscribe default symbols after connect: %v. Retrying connection.", err)
			_ = s.conn.Close() // 订阅失败，关闭连接以触发下一次重试
			continue
		}

		// 通知 Handler 进行动态订阅恢复（仅在重连时需要，但首次连接发送信号也无妨）
		select {
		case s.connectionReady <- struct{}{}:
			// 通知 Handler 计算并重新订阅客户端需要的币种
		default:
			// 防止阻塞
		}

		// 启动心跳协程
		go s.startPingLoop(conn)

		// 启动数据读取循环
		// 这个 runListen 协程会阻塞直到连接断开
		s.runListen(conn) // <-- 启动读取协程，等待连接断开

		// 6. runListen 退出（连接断开），循环继续，开始下一次重连尝试
		log.Println("OKXTickerService lost connection. Restarting reconnect loop...")
		time.Sleep(2 * time.Second) // 避免高频重连
	}
}

func (s *OKXTickerService) runListen(conn *websocket.Conn) {

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// ReadMessage 失败，意味着连接已断开或不可用
			log.Printf("WebSocket ReadMessage failed: %v", err)
			// 退出此协程，让 run() 主循环接管重连逻辑
			return
		}

		// 成功读取到消息，进行处理（解析、推送到 Handler 等）
		s.handleMessage(message)
	}
}

func (s *OKXTickerService) sendSubscribe(symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}

	args := []map[string]string{}
	for _, sym := range symbols {
		args = append(args, map[string]string{
			"channel": "tickers",
			"instId":  sym,
		})
	}

	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}
	// 假设 conn 已经处理好重连
	return s.writeMessageInternal(subMsg)
}

// 同步等待连接建立
func (s *OKXTickerService) WaitForConnectionReady(ctx context.Context) error {
	select {
	case <-s.connectionReady:
		return nil // 信号已收到，连接已就绪
	case <-ctx.Done():
		return ctx.Err() // 超时或 context 被取消
	}
}

// 在连接重置后调用
// activeClientSymbols: 由 Handler 层提供的所有当前有客户端需求的币种
func (s *OKXTickerService) ResubscribeAll(activeClientSymbols []string) error {

	// 1. 合并默认币种和客户端需求的币种，并去重
	symbolSet := make(map[string]struct{})

	// 加入默认币种
	for _, sym := range s.defaultSymbols {
		symbolSet[sym] = struct{}{}
	}

	// 加入客户端动态需求的币种
	for _, sym := range activeClientSymbols {
		symbolSet[sym] = struct{}{}
	}

	// 2. 将集合转换为列表
	symbolsToResubscribe := make([]string, 0, len(symbolSet))
	for sym := range symbolSet {
		symbolsToResubscribe = append(symbolsToResubscribe, sym)
	}

	// 3. 清空并更新本地状态
	s.Lock()
	// 清空旧的订阅状态，因为连接已经失效
	s.subscribed = make(map[string]struct{})
	// 成功发送后，再更新 s.subscribed
	s.Unlock()

	// 4. 发送订阅请求
	err := s.sendSubscribe(symbolsToResubscribe)
	if err == nil {
		s.Lock()
		for _, sym := range symbolsToResubscribe {
			s.subscribed[sym] = struct{}{}
		}
		s.Unlock()
	}
	return err
}

// 批量订阅，供客户端调用
// 这里的 symbols 是 Handler 告诉 OKXService "我需要这些币种" 的列表
func (s *OKXTickerService) SubscribeSymbols(ctx context.Context, symbols []string) error {
	s.Lock()
	defer s.Unlock()

	// 1. 过滤：只订阅那些 OKXService 内部状态还未记录已订阅的币种
	var toSubscribeOKX []string
	for _, sym := range symbols {
		// 检查 s.subscribed (map[string]struct{}) 中是否存在键
		if _, ok := s.subscribed[sym]; !ok {
			toSubscribeOKX = append(toSubscribeOKX, sym)
		}
	}

	if len(toSubscribeOKX) == 0 {
		return nil // 无需订阅新币种
	}

	// 2. 发送订阅请求
	err := s.sendSubscribe(toSubscribeOKX)
	if err != nil {
		// 如果发送失败，无需回滚状态，因为本地状态还未更新
		return err
	}

	// 3. 成功发送后，更新本地状态 (s.subscribed = map[string]struct{})
	for _, sym := range toSubscribeOKX {
		s.subscribed[sym] = struct{}{} // 标记为已发送订阅请求
	}
	return nil
}

// 批量退订，供 Handler 调用 (Handler 负责客户端计数和过滤)
// 这里的 symbols 是 Handler 确认不再需要，且需要向 OKX 退订的币种列表。
func (s *OKXTickerService) UnsubscribeSymbols(ctx context.Context, symbols []string) error {
	s.Lock()
	defer s.Unlock()

	// 1. 过滤：只对本地状态 (s.subscribed) 中存在的币种发送退订请求
	var toUnsubscribeOKX []string
	args := []map[string]string{}

	for _, sym := range symbols {
		// 检查 OKXService 是否实际订阅了该币种
		if _, ok := s.subscribed[sym]; ok {
			toUnsubscribeOKX = append(toUnsubscribeOKX, sym)
			args = append(args, map[string]string{
				"channel": "tickers",
				"instId":  sym,
			})
		}
	}

	if len(args) == 0 {
		return nil
	}

	// 2. 发送退订消息
	unsubMsg := map[string]interface{}{
		"op":   "unsubscribe",
		"args": args,
	}

	err := s.conn.WriteJSON(unsubMsg)
	if err != nil {
		// 如果发送失败，状态保持不变（本地 s.subscribed 仍然存在，OKX 端也存在）
		return err
	}

	// 3. 成功发送后，更新本地状态
	for _, sym := range toUnsubscribeOKX {
		delete(s.subscribed, sym)
	}
	return nil
}

// GetPrice 获取单个币种行情
func (s *OKXTickerService) GetPrice(ctx context.Context, symbol string) (*TickerData, error) {
	s.RLock()
	defer s.RUnlock()
	data, ok := s.prices[symbol]
	if !ok {
		return nil, fmt.Errorf("price not found for symbol: %s", symbol)
	}
	return &data, nil
}

// GetPrices 获取多个币种行情
func (s *OKXTickerService) GetPrices(ctx context.Context, symbols []string) (map[string]TickerData, error) {
	s.RLock()
	defer s.RUnlock()
	result := make(map[string]TickerData)
	//var result []*TickerData
	for _, sym := range symbols {
		if data, ok := s.prices[sym]; ok {
			result[sym] = data
			//result = append(result, data)
		}
	}
	return result, nil
}

// Close 关闭服务
func (s *OKXTickerService) Close() error {
	close(s.closeCh)
	return s.conn.Close()
}

// handleMessage 处理 OKX 推送消息
func (s *OKXTickerService) handleMessage(msg []byte) {
	if pong := string(msg); pong == "pong" {
		//log.Println("OKXTickerService 接收到 pong")
		return
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(msg, &raw); err != nil {
		log.Println("OKXTickerService json unmarshal error:", err)
		return
	}

	if evt, ok := raw["event"].(string); ok { // {"event":"subscribe","arg":{"channel":"tickers","instId":"BTC-USDT"},"connId":"6b422e60"}
		switch evt {
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
	if !ok {
		return
	}
	dataArr, ok := raw["data"].([]interface{})
	if !ok {
		return
	}
	switch channel {
	case "tickers":
		s.handleTickers(dataArr)
	}

}

func (s *OKXTickerService) handleTickers(dataArr []interface{}) {

	s.Lock()
	defer s.Unlock()

	changedValues := make(map[string]TickerData, len(dataArr))
	for _, d := range dataArr {
		item := d.(map[string]interface{})
		instId := item["instId"].(string)

		lastPrice := parseFloat(item["last"])  // 最新成交价格
		open24h := parseFloat(item["open24h"]) // 24小时开盘价

		change24h := 0.0
		if open24h != 0 {
			change24h = (lastPrice - open24h) / open24h * 100
		}
		ticker := TickerData{
			InstId:    instId,
			LastPrice: item["last"].(string),      // 最新成交价格
			Vol24h:    item["vol24h"].(string),    // 24小时成交量（以交易标的计，比如 BTC）
			VolCcy24h: item["volCcy24h"].(string), // 24小时成交量（以计价货币计，比如 USDT）
			High24h:   item["high24h"].(string),   // 24小时最高价
			Low24h:    item["low24h"].(string),    // 24小时最低价
			Open24h:   item["open24h"].(string),   // 24小时开盘价
			Change24h: change24h,
			AskPx:     item["askPx"].(string), // 卖一价（最低的卖单价）
			AskSz:     item["askSz"].(string), // 卖一量
			BidPx:     item["bidPx"].(string), // 买一价（最高的买单价）
			BidSz:     item["bidSz"].(string), // 买一量
			Ts:        parseFloat(item["ts"]),
		}
		changedValues[instId] = ticker
		// 全部返回string类型，防止精度丢失
		s.prices[instId] = ticker

	}

	if len(dataArr) > 0 {
		// 只发送本次改变的数据
		s.tickersCh <- changedValues
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
