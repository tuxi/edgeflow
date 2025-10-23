package service

import (
	"context"
	model2 "edgeflow/internal/model"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// CandleService 定义 K 线服务接口
type CandleService interface {
	// SubscribeCandle 订阅指定币种和周期的 K 线数据。
	// period 应为 OKX 标准，如 "1m", "15m", "1H", "1D" 等。
	SubscribeCandle(ctx context.Context, symbol string, period string) error

	// UnsubscribeCandle 取消订阅指定币种和周期的 K 线数据。
	UnsubscribeCandle(ctx context.Context, symbol string, period string) error

	// Close 关闭 K 线服务连接
	Close() error

	// WaitForConnectionReady 同步等待连接建立
	WaitForConnectionReady(ctx context.Context) error
}

// OKXCandleService 基于 OKX WebSocket 的 K 线实现
// 只有在客户端首次订阅即才连接okx service
type OKXCandleService struct {
	sync.RWMutex
	conn *websocket.Conn
	// 记录 OKX 连接上实际已发送 subscribe 消息的 K 线集合
	// 全局 K 线订阅计数器
	// Key: "BTC-USDT-15m"
	// Value: 订阅该频道的客户端数量
	subscribed map[model2.SubscriptionKey]int
	url        string
	closeCh    chan struct{}

	lastRequest time.Time

	// 连接成功建立后，会向此通道发送一个信号 (事件流)
	connectionNotifier chan struct{}

	// Kafka Producer 依赖
	producer kafka.ProducerService

	// 用于同步等待“第一次连接成功”的通道 (同步信号）
	readyCond *sync.Cond // 条件变量

	// 用于向 MarketHandler 异步通知订阅错误的通道
	errorCh chan model2.ClientError

	// 原子布尔值或互斥锁，用于控制 run 协程的生命周期
	// 我们使用一个布尔值配合 RWMutex
	isRunning bool
}

// NewOKXCandleService 创建实例并连接 OKX WebSocket
func NewOKXCandleService(producer kafka.ProducerService) *OKXCandleService {
	url := "wss://ws.okx.com:8443/ws/v5/business"

	s := &OKXCandleService{
		conn:               nil,
		subscribed:         make(map[model2.SubscriptionKey]int),
		producer:           producer,
		url:                url,
		closeCh:            make(chan struct{}),
		connectionNotifier: make(chan struct{}),
		errorCh:            make(chan model2.ClientError, 10),
	}

	s.readyCond = sync.NewCond(&s.RWMutex) // 条件变量绑定到 RWMutex

	// 本服务将 OKXCandleService 从 “启动即连接” 模式改为 “首次订阅即连接” 模式
	// 所以不需要启动时就连接，只有需要时才连接
	//go s.run() // 启动连接/重连主循环

	return s
}

// --- 连接和重连逻辑 (与 TickerService 类似，确保独立性) ---
func (s *OKXCandleService) startPingLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.RLock()
			err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
			s.RUnlock()

			if err != nil {
				log.Printf("OKXCandleService ping 失败: %v. 停止 ping loop.", err)
				return
			}

		case <-s.closeCh:
			return
		}
	}
}

// 恢复订阅所有之前已订阅的 K 线
func (s *OKXCandleService) resubscribeAll() (int, error) {
	s.RLock()
	keys := make([]model2.SubscriptionKey, 0, len(s.subscribed))
	for key := range s.subscribed {
		keys = append(keys, key)
	}
	s.RUnlock()

	if len(keys) == 0 {
		return 0, nil
	}

	// 重建订阅请求
	args := []map[string]string{}
	for _, key := range keys {
		args = append(args, map[string]string{
			"channel": "candle" + key.Period,
			"instId":  key.Symbol,
		})
	}

	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	// 发送批量订阅
	return len(keys), s.writeMessageInternal(subMsg)
}

// 启动连接，只有在需要时才建立连接，没有订阅时会不会产生连接
func (s *OKXCandleService) run() {
	log.Println("OKXCandleService 连接运行循环run loop开始.")
	// 退出后设置 isRunning = false
	defer func() {
		s.Lock()
		s.isRunning = false
		s.Unlock()
		log.Println("OKXCandleService 连接运行循环run loop结束")
	}()

	for {

		// 懒加载连接：按需连接
		// 由于 run() 是由订阅触发启动的，所以它不应再无限重连。
		// 当前版本若连接断开会自动重连，但如果这时没有订阅，应直接退出。
		s.RLock()
		if len(s.subscribed) == 0 {
			s.RUnlock()
			log.Println("OKXCandleService: 没有活跃的订阅，退出run 循环.")
			s.Lock()
			s.isRunning = false
			s.Unlock()
			return // 没有订阅直接退出运行循环，而不是continue
		}
		s.RUnlock()

		// 尝试连接
		conn, _, err := websocket.DefaultDialer.Dial(s.url, nil)
		if err != nil {
			log.Println("OKXCandleService 连接失败 2s后重试:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// 连接成功后
		s.Lock()
		s.conn = conn
		// 唤醒所有等待者
		s.readyCond.Broadcast()

		select {
		case s.connectionNotifier <- struct{}{}:
			log.Println("OKXCandleService 连接已经建立，发出通知.")
		default:
		}

		// 关键：连接成功，重置 closeCh 并重新创建 readyCh/closeCh
		if s.closeCh != nil {
			// 关闭旧的 closeCh，通知旧的 Ping/Listen 协程退出
			close(s.closeCh)
		}
		s.closeCh = make(chan struct{})

		// 注意：这里没有清空 s.subscribed，因为 resubscribeAll 会依赖它来恢复订阅
		s.Unlock()

		// 恢复所有旧的 订阅
		resubCount, err := s.resubscribeAll()
		if err != nil {
			log.Printf("OKXCandleService 重新连接后，恢复已有订阅失败: %v. 即将重试..\n", err)
			_ = s.conn.Close()
			continue
		} else {
			if resubCount > 0 {
				log.Printf("OKXCandleService 重新连接后，成功恢复了%v 条订阅\n", resubCount)
			}
		}

		// 启动 Ping 循环
		go s.startPingLoop(conn)
		s.runListen(conn) // 阻塞直到连接断开

		// 连接断开后，重置状态
		s.Lock()
		s.conn = nil
		s.Unlock()

		log.Println("OKXCandleService 连接断开，即将在2秒后运行连run 循环")
		time.Sleep(2 * time.Second)
	}
}

func (s *OKXCandleService) runListen(conn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Candle WebSocket ReadMessage failed: %v", err)
			return // 退出，触发 run() 重连
		}
		s.handleMessage(message)
	}
}

// 内部方法，负责限速，假设外部已加锁 s.Lock() 或 s.RLock()
func (s *OKXCandleService) writeMessageInternal(message interface{}) error {
	timeSinceLastRequest := time.Since(s.lastRequest)
	if timeSinceLastRequest < 50*time.Millisecond {
		time.Sleep(50*time.Millisecond - timeSinceLastRequest)
	}
	s.lastRequest = time.Now()

	if s.conn == nil {
		return errors.New("当前ws连接不存在，请先建立连接")
	}
	return s.conn.WriteJSON(message)
}

// --- 外部接口实现 ---

func (s *OKXCandleService) GetErrorChannel() <-chan model2.ClientError {
	return s.errorCh
}

// 等待连接完成的方法
func (s *OKXCandleService) WaitForConnectionReady(ctx context.Context) error {
	// 使用 Cond 必须用 Lock，不能用 RLock
	s.Lock()
	defer s.Unlock()

	for s.conn == nil {
		done := make(chan struct{})
		go func() {
			// 阻塞等待
			s.readyCond.Wait() // 释放锁并阻塞，被 Broadcast 后重新获取锁
			close(done)
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
		}
	}
	return nil
}

// SubscribeCandle 订阅k线
func (s *OKXCandleService) SubscribeCandle(ctx context.Context, symbol string, period string) error {

	defer func() {
		s.RLock()
		for k, v := range s.subscribed {
			log.Printf("OKXCandleService SubscribeCandle 执行订阅后状态 key:%s 次数:%v", k, v)
		}
		s.RUnlock()
	}()

	key := model2.SubscriptionKey{Symbol: symbol, Period: period}
	s.Lock()

	// 1. 检查是否已经订阅
	count, ok := s.subscribed[key]
	if ok {
		// 已经存在订阅，直接return
		s.subscribed[key] = count + 1 // 即使已经订阅，计数器也应该增加
		s.Unlock()
		return nil // 已经订阅
	}

	// 占位，表示正在订阅
	s.subscribed[key] = 0

	// 开始订阅

	// 如果当前服务未运行，则启动 run()
	needStart := !s.isRunning
	if needStart {
		s.isRunning = true
	}

	s.Unlock() // 提前释放锁，以便 run() 协程可以获取锁并建立连接

	// 启动连接管理器
	if needStart {
		go s.run()
	}

	// 新订阅，向okx发送订阅请求
	// 2. 构造订阅消息
	channel := "candle" + period
	args := []map[string]string{
		{"channel": channel, "instId": symbol},
	}
	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	// 因为本服务是按需连接的，所以此时未必已经连接，需要等待连接就绪，然后发送请求
	err := s.WaitForConnectionReady(ctx) // 阻塞等待连接成功

	if err != nil {
		// 如果等待超时或 Context 取消，清理 isRunning 状态并返回错误
		return fmt.Errorf("failed to wait for OKX connection ready: %w", err)
	}

	// 再次检查是否在等待期间被其他协程订阅 (防止竞态条件，但在这个逻辑中影响不大)

	// 最后更新状态
	s.Lock()
	// 发送请求
	err = s.writeMessageInternal(subMsg)
	if err != nil {
		s.Unlock()
		return fmt.Errorf("failed to subscribe to upstream data: %w", err)
	}
	s.subscribed[key] = 1
	s.Unlock()
	log.Printf("✅ Subscribed candle: %s-%s", symbol, period)
	return nil
}

// UnsubscribeCandle 实现
func (s *OKXCandleService) UnsubscribeCandle(ctx context.Context, symbol string, period string) error {
	key := model2.SubscriptionKey{Symbol: symbol, Period: period}

	defer func() {
		s.RLock()
		for k, v := range s.subscribed {
			log.Printf("OKXCandleService UnsubscribeCandle 取消订阅后状态 key:%s 次数:%v", k, v)
		}
		s.RUnlock()
	}()

	s.Lock()
	defer s.Unlock()

	// 1. 检查是否订阅
	currentCount, ok := s.subscribed[key]
	if !ok {
		return nil // 未订阅，无需退订
	}

	// 计数器大于0才可以递减
	if currentCount > 0 {
		s.subscribed[key] = currentCount - 1

		// 判断是否需要向okx发送取消订阅请求
		if currentCount-1 == 0 {
			// 构造退订消息
			channel := "candle" + period
			args := []map[string]string{
				{"channel": channel, "instId": symbol},
			}
			unsubMsg := map[string]interface{}{
				"op":   "unsubscribe",
				"args": args,
			}

			// 发送请求
			err := s.writeMessageInternal(unsubMsg)
			if err != nil {
				return err
			}

			// 取消订阅时，检查是否需要关闭连接
			// 最后一个订阅退订后 延迟关闭连接
			if len(s.subscribed) == 1 {
				go func() {
					time.Sleep(10 * time.Second)
					s.Lock()
					defer s.Unlock()
					if len(s.subscribed) == 0 && s.conn != nil {
						log.Println("OKXCandleService 取消订阅10s后,检查没活跃的订阅，关闭与okx的ws连接.")
						_ = s.conn.Close()
						s.conn = nil
					}
				}()
			}

			// 更新本地状态
			delete(s.subscribed, key)
			log.Printf("OKXCandleService 取消订阅k线: %s-%s", symbol, period)
		}
	}

	return nil
}

// Close 实现
func (s *OKXCandleService) Close() error {
	s.Lock()
	defer s.Unlock()
	// 关闭 closeCh 停止 runListen 和 startPingLoop
	close(s.closeCh)
	// 关闭连接
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// --- 消息处理逻辑 ---

func (s *OKXCandleService) handleMessage(msg []byte) {
	if pong := string(msg); pong == "pong" {
		return
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(msg, &raw); err != nil {
		log.Println("OKXCandleService：json反序列化 error:", err)
		return
	}

	// 忽略事件消息
	if event, ok := raw["event"].(string); ok {
		if event == "error" {
			// 通知客户端错误
			s.handleErrorEvent(raw)
		}
		return
	}

	arg, ok := raw["arg"].(map[string]interface{})
	if !ok {
		return
	}

	channel, ok := arg["channel"].(string)
	if !ok {
		return
	}

	instId, ok := arg["instId"].(string)
	if !ok {
		return
	}

	// 只有当频道是 candle* 时才处理
	if len(channel) < 6 || channel[:6] != "candle" {
		return
	}

	dataArr, ok := raw["data"].([]interface{})
	if !ok || len(dataArr) == 0 {
		return
	}

	// 提取周期，例如从 "candle15m" 中提取 "15m"
	period := channel[6:]

	s.handleCandles(dataArr, period, instId)
}

func (s *OKXCandleService) handleErrorEvent(raw map[string]interface{}) {
	code, _ := raw["code"].(string)
	errMsg, _ := raw["msg"].(string)

	log.Printf("OKXCandleService [ERROR] OKX Candle Error. Code: %s, Message: %s", code, errMsg)

	// 只有当错误是订阅失败（例如：交易对或频道不存在）时，才需要清理本地状态
	// 常见的订阅失败代码：60011 (Subscription failed)

	if code == "60018" {
		channel, instId, found := parseFailedSubscription(errMsg)

		if found && strings.HasPrefix(channel, "candle") {
			// 提取周期，例如从 "candle1H" 提取 "1H"
			period := strings.TrimPrefix(channel, "candle")

			key := model2.SubscriptionKey{Symbol: instId, Period: period}

			s.Lock()
			if _, exists := s.subscribed[key]; exists {
				// 找到对应的失败订阅，进行清理
				delete(s.subscribed, key)
				log.Printf("OKXCandleService Cleaned failed subscription from state: Symbol=%s, Period=%s", instId, period)
			}

			// 🚀 通知客户端：可选
			// 如果需要通知，您需要定义一个机制将这个错误广播给发起订阅的客户端。
			// 例如：通过额外的 channel 向上游 MarketHandler 发送错误信息。
			// 由于 MarketHandler 的 listenForCandleUpdates 逻辑是广播数据的，
			// 这种情况下，最佳做法通常是在 MarketHandler 接收到客户端订阅请求后，
			// 对 OKXCandleService 的 SubscribeCandle() 方法返回的错误进行处理。
			// 2. 核心修改：发送结构化错误通知给上游 MarketHandler
			errNotification := model2.NewClientError("subscribe_candle", errMsg, "404", map[string]string{
				"symbol": instId,
				"period": period,
			})

			select {
			case s.errorCh <- errNotification:
				// 成功发送错误
			default:
				log.Println("Warning: OKXCandleService error channel buffer full. Dropping error notification.")
			}

			s.Unlock()
		}
	}
}

// 针对逗号分隔格式的错误消息（例如：Wrong URL or channel:candle15m,instId:ETH-USDT doesn't exist）
// 匹配 [cC]hannel:<channel>,[iI]nst[iI]d:<instId>
var failedSubRegexComma = regexp.MustCompile(`[cC]hannel:\s*([^\s,]+),\s*[iI]nst[iI]d:\s*(\S+)`)

// 针对空格分隔的通用格式（例如：Subscription failed: Channel: candle1H InstId: BTC-USDT does not exist）
// 匹配 [cC]hannel:<channel> [iI]nst[iI]d:<instId>
var failedSubRegexSpace = regexp.MustCompile(`[cC]hannel:\s*(\S+)\s*[iI]nst[iI]d:\s*(\S+)`)

// 辅助函数：尝试从错误消息中解析出失败的频道和 InstId
func parseFailedSubscription(errMsg string) (channel string, instId string, success bool) {
	// 尝试匹配 "Channel: <channel> InstId: <instId>"
	// OKX 错误消息示例: "Subscription failed: Channel: candle1H InstId: BTC-USDT does not exist"
	// 1. 尝试匹配逗号分隔的格式 (您的新发现)
	matches := failedSubRegexComma.FindStringSubmatch(errMsg)
	if len(matches) == 3 {
		// matches[1] 是 channel，matches[2] 是 instId
		return matches[1], matches[2], true
	}

	// 2. 尝试匹配空格分隔的格式 (原有的通用格式)
	matches = failedSubRegexSpace.FindStringSubmatch(errMsg)
	if len(matches) == 3 {
		// matches[1] 是 channel，matches[2] 是 instId
		return matches[1], matches[2], true
	}

	// 如果是其他类型的错误，或者格式不匹配，返回失败
	return "", "", false
}

func (s *OKXCandleService) handleCandles(dataArr []interface{}, period string, instId string) {
	var messages []kafka.Message
	for _, d := range dataArr {
		// OKX K线数据的格式是一个数组 [ts, open, high, low, close, vol, volCcy, ...],
		// 并且 instId 在 arg 中，而不是 data 数组中。这里需要根据实际 OKX 数据格式调整
		item := d.([]interface{})

		// 重新检查 OKX 的数据结构，通常是：
		// data: [ ["1677700000000","20000.0","20001.0","19999.0","20000.5","100.0","2000000.0"], ... ]
		// 如果是这种数组格式，我们需要知道 instId 是哪个

		// 由于无法获取到完整的原始 JSON 结构，我们使用一个简化且可能有偏差的解析（你需要根据实际OKX数据调整）
		if len(item) < 7 {
			continue
		}

		timestamp := parseInt64(item[0].(string)) // 时间戳
		open := item[1].(string)
		high := item[2].(string)
		low := item[3].(string)
		closee := item[4].(string)
		vol := item[5].(string) //
		volCcy := item[6].(string)
		confirm := item[8].(string) // 是否已收盘

		// 1. 转换为 Protobuf K线消息
		klinePb := &pb.WsKlineUpdate_KlineData{
			Timestamp: timestamp / 1000,
			Open:      open,
			Close:     closee,
			High:      high,
			Low:       low,
			Vol:       vol,
			VolCcy:    volCcy,
		}

		// 2. 构造 Protobuf CandleUpdate 消息
		candleUpdate := &pb.WsKlineUpdate{
			InstId:     instId,
			TimePeriod: period,
			Confirm:    confirm == "1",
			Data:       klinePb,
		}

		// 3. 构造 Protobuf 通用 WebSocket 消息
		wsMsg := &pb.WebSocketMessage{
			Type: "CANDLE_UPDATE",
			// 包装 Payload
			Payload: &pb.WebSocketMessage_KlineUpdate{
				KlineUpdate: candleUpdate,
			},
		}

		// Key：使用 SubKey 作为 Kafka Key，确保同一 K线的所有更新进入同一分区，保证顺序
		subKey := fmt.Sprintf("CANDLE:%s:%s", instId, period)
		messages = append(messages, kafka.Message{
			Key:  subKey,
			Data: wsMsg,
		})
	}

	// 5. 写入 Kafka
	go func() {
		// 主题：marketdata_subscribe (用于按需订阅和过滤)
		topic := "marketdata_subscribe"
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		if err := s.producer.Produce(ctx, topic, messages...); err != nil {
			log.Printf("OKXCandleService ERROR: topic=%s 生产者批量写入 k线数据 到 kafka失败: %v", topic, err)
		}
	}()
}

func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return v
	}
	return 0
}
