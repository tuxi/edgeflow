package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/exchange"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	model2 "github.com/nntaoli-project/goex/v2/model"
)

// 定义支持的排序字段常量
const (
	SortByVolume      = "volume_24h"   // 成交量（默认）
	SortByPriceChange = "price_change" // 24小时价格涨跌幅
	SortByPrice       = "price"        // 最新价格
)

// 交易数据结构体
type TradingItem struct {
	Coin   entity.CryptoInstrument `json:"coin"`
	Ticker TickerData              `json:"ticker"`
}

// 将定时排序币种中耗时的字符串转换移出排序循环，作为一个结构体
type SortableItem struct {
	ID           string
	VolumeFloat  float64
	PriceFloat   float64
	ChangeFloat  float64 // 如果 Change24h 已经是 float，则跳过转换
	OriginalItem TradingItem
}

// 币种更新结构体
type BaseInstrumentUpdate struct {
	NewInstruments      []string // 新上架的 InstID 列表
	DelistedInstruments []string // 已下架的 InstID 列表
}

type InstrumentFetcher interface {
	// 只需要返回所有 USDT 交易对的基础数据
	GetAllActiveUSDTInstruments(ctx context.Context, exID int64) ([]entity.CryptoInstrument, error)
	// 用于更新币种状态的方法
	UpdateInstrumentStatus(ctx context.Context, exid int64, instIDs []string, status string) error
}

// 历史价格结构体
type PricePoint struct {
	Timestamp int64   // 时间戳（毫秒）
	Price     float64 // 价格
}

// 行情服务，负责整合数据、排序和缓存结构
// 整合 Kafka 生产者
type MarketDataService struct {
	// 锁用于保护所有共享内存数据，确保并发安全
	mu sync.RWMutex

	// 基础数据：所有交易对的 CoinItem
	baseCoins map[string]entity.CryptoInstrument

	// 统一的内存存储：所有活跃的交易对数据
	tradingItems map[string]TradingItem

	// 排序缓存：按成交量排序的 InstID 列表
	SortedInstIDs []string

	// 依赖服务
	tickerClient      *OKXTickerService     // 实时数据源
	instrumentFetcher InstrumentFetcher     // 基础数据源
	producer          kafka.ProducerService // Kafka 生产者服务

	// 控制定时排序的通道
	stopSortCh chan struct{}

	// 当前生效的排序字段
	currentSortField string

	ex         exchange.Exchange
	signalRepo dao.SignalDao // DB 接口

	// AlertService 接口
	alertService AlertPublisher

	// 历史价格队列 (InstID -> []PricePoint)
	// 这是一个临界资源，必须在 mu 锁保护下访问
	priceHistory map[string][]PricePoint
}

func NewMarketDataService(ticker *OKXTickerService, instrumentFetcher InstrumentFetcher, ex exchange.Exchange, SignalRepo dao.SignalDao, producer kafka.ProducerService, alertService AlertPublisher) *MarketDataService {
	m := &MarketDataService{
		baseCoins:         make(map[string]entity.CryptoInstrument),
		tradingItems:      make(map[string]TradingItem),
		SortedInstIDs:     make([]string, 0),
		tickerClient:      ticker,
		instrumentFetcher: instrumentFetcher,
		stopSortCh:        make(chan struct{}),
		currentSortField:  SortByVolume, // 默认按成交量排序
		ex:                ex,
		signalRepo:        SignalRepo,
		producer:          producer,
		alertService:      alertService,
		priceHistory:      make(map[string][]PricePoint),
	}
	// 启动 MarketService 的核心 Worker
	go m.startDataWorkers()
	// 每次 收到连接成功信号时，执行一次订阅恢复
	go m.runTickerResubscriptionLoop()

	go func() {
		time.Sleep(5)
		ticker.Run()
	}()
	return m
}

func (m *MarketDataService) startDataWorkers() {
	// 1. 启动定时排序 Worker
	go m.startSortingScheduler()

	// 2. 监听 TickerService 的实时数据更新（OKX的原始数据流）
	tickerUpdates := m.tickerClient.GetTickerChannel()

	// 3. 🚀 引入定时窗口：强制每 50 毫秒才处理一次 Ticker 批次
	const processInterval = 50 * time.Millisecond
	ticker := time.NewTicker(processInterval)
	defer ticker.Stop() // 确保退出时停止定时器

	// 4. 用于缓存 OKX 推送的最新 Ticker 数据。
	// 使用 map 来保证每个 InstID 都是最新的。
	// 注意：这个 map 需要在 Goroutine 之间安全共享，或者像这里一样只在主循环中访问。
	latestTickerUpdates := make(map[string]TickerData)

	// 使用锁来保护 latestTickerUpdates，尽管本例中只在主循环中访问，但在复杂的场景中是必需的。
	// 假设在当前设计中，只有这个 Goroutine 写入 latestTickerUpdates，其他 Goroutine 仅读取

	for {
		select {
		case newUpdate := <-tickerUpdates:
			// 收到新数据：立即更新缓存中的最新值
			// newUpdate 是一个 map[string]TickerData
			for instID, ticker := range newUpdate {
				latestTickerUpdates[instID] = ticker
			}

		case <-ticker.C:
			// 🚀 定时器触发：强制处理并发送缓存中的最新批次
			if len(latestTickerUpdates) > 0 {
				// 1. 复制要处理的数据
				dataToSend := make(map[string]TickerData, len(latestTickerUpdates))
				for k, v := range latestTickerUpdates {
					dataToSend[k] = v
				}

				// 2. 清空缓存，准备接收下一个窗口的数据
				// 保持 map 的底层内存分配，只清除内容，以提高效率
				// for k := range latestTickerUpdates { delete(latestTickerUpdates, k) }
				// 或者直接创建一个新 map (内存开销更大，但更安全)
				latestTickerUpdates = make(map[string]TickerData)

				// 3. 将最新的全量批次交给处理函数
				// m.updateRealTimeData 内部会进行组合、锁操作和批量 Kafka 写入
				m.updateRealTimeData(dataToSend)
			}

		case <-m.stopSortCh:
			return
		}
	}
}

// updateRealTimeData 处理实时 Ticker 更新和数据组合
func (m *MarketDataService) updateRealTimeData(tickerMap map[string]TickerData) {

	// 收集所有需要发送给下游（Handler）的 Ticker
	tickersToForward := make([]TickerData, 0, len(tickerMap))

	// --- 1. 临界区操作：更新内存数据 ---
	m.mu.Lock()

	for instID, ticker := range tickerMap {
		currentPrice, err := strconv.ParseFloat(ticker.LastPrice, 64)
		if err != nil {
			// 如果价格转换失败，记录错误并跳过此币种的提醒检查
			log.Printf("WARN: 价格转换失败，跳过提醒检查。InstID: %s, Price: %s, Error: %v",
				instID, ticker.LastPrice, err)
			currentPrice = 0
		}

		// 更新价格历史
		newPricePoint := PricePoint{
			Timestamp: ticker.Ts, // 使用 Ticker 中的时间戳
			Price:     currentPrice,
		}

		// 获取当前币种的历史记录
		history := m.priceHistory[instID]
		// 追加新的价格点
		history = append(history, newPricePoint)

		// 清理旧数据 (只保留过去 N 分钟，例如 6分钟)
		maxAge := time.Now().Add(-6 * time.Minute).UnixMilli()

		// 找到第一个比 maxAge 新的价格点索引
		startIndex := 0
		for i, pp := range history {
			if pp.Timestamp >= maxAge {
				startIndex = i
				break
			}
		}
		// 截断旧数据
		history = history[startIndex:]
		m.priceHistory[instID] = history

		// A. 尝试更新已存在的 TradingItem
		if item, ok := m.tradingItems[instID]; ok {
			lastPrice, _ := strconv.ParseFloat(item.Ticker.LastPrice, 64)
			// 直接更新 Ticker 数据
			item.Ticker = ticker
			m.tradingItems[instID] = item

			if currentPrice > 0 {
				m.CheckAndTriggerAlerts(instID, currentPrice, lastPrice)
			}

			// 将此 Ticker 加入转发列表
			tickersToForward = append(tickersToForward, ticker)
			continue
		}

		// B. 新数据：尝试组合
		if coin, ok := m.baseCoins[instID]; ok {
			// 成功组合：基础数据 + 实时数据
			m.tradingItems[instID] = TradingItem{
				Coin:   coin,
				Ticker: ticker,
			}

			// 检查并触发提醒
			if currentPrice > 0 {
				m.CheckAndTriggerAlerts(instID, currentPrice, 0)
			}

			// 将此 Ticker 加入转发列表
			tickersToForward = append(tickersToForward, ticker)
			continue
		}

		// 如果 baseCoins 中没有，则该 Ticker 被忽略，不加入转发列表
	}

	m.mu.Unlock() // 立即释放锁！
	// --- 临界区结束 ---

	if len(tickersToForward) == 0 {
		return
	}

	var tickers []*pb.TickerUpdate
	// --- 2. 非临界区操作：kafka转发 ---
	for _, ticker := range tickersToForward {
		ticperUpdate := &pb.TickerUpdate{
			InstId:     ticker.InstId,
			LastPrice:  ticker.LastPrice,
			Vol_24H:    ticker.Vol24h,
			VolCcy_24H: ticker.VolCcy24h,
			High_24H:   ticker.High24h,
			Low_24H:    ticker.Low24h,
			Open_24H:   ticker.Open24h,
			Change_24H: ticker.Change24h,
			AskPx:      ticker.AskPx,
			AskSz:      ticker.AskSz,
			BidPx:      ticker.BidPx,
			BidSz:      ticker.BidSz,
			Ts:         ticker.Ts,
		}
		tickers = append(tickers, ticperUpdate)
	}

	protoMsg := &kafka.Message{
		Key: "TICKER_BATCH",
		Data: &pb.WebSocketMessage{
			Type: "TICKER_UPDATE",
			Payload: &pb.WebSocketMessage_TickerBatch{&pb.TickerBatch{
				Tickers: tickers,
			}},
		},
	}

	go func(message *kafka.Message) {
		if message == nil {
			return
		}
		// 在这个单 Goroutine 中执行阻塞的 m.producer.Produce(ctx, topic, messages...)
		// 将Kafka 写入超时时间设置为 2 秒，防止超时
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel() // 确保context 及时释放
		// 序列化并写入kafka
		topic := kafka.TopicTicker // Ticker 高频主题
		if err := m.producer.Produce(ctx, topic, *message); err != nil {
			// 记录错误，但不阻塞主循环
			log.Printf("ERROR: MarketDataService topic=%s 生产者批量写入Ticker数据到kafka失败: %v", topic, err)
		}
	}(protoMsg)
}

// startSortingScheduler 定时执行排序和缓存
func (m *MarketDataService) startSortingScheduler() {
	// 定时器，例如每 1.5 秒执行一次排序
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performSortAndCache()
		case <-m.stopSortCh:
			return
		}
	}
}

// performSortAndCache 执行排序，并更新缓存（需要在后台线程调用）
func (m *MarketDataService) performSortAndCache() {
	m.mu.RLock()
	// 1. 转换为可排序切片并预处理浮点数
	sortableItems := make([]SortableItem, 0, len(m.tradingItems))
	for _, item := range m.tradingItems {
		// 🚀 核心优化：只转换一次
		vol, _ := strconv.ParseFloat(item.Ticker.VolCcy24h, 64)
		price, _ := strconv.ParseFloat(item.Ticker.LastPrice, 64)

		sortableItems = append(sortableItems, SortableItem{
			ID:          item.Coin.InstrumentID,
			VolumeFloat: vol,
			PriceFloat:  price,
			// 假设 Change24h 已经是 float 或直接从 item.Ticker 中获取
			OriginalItem: item,
		})
	}
	m.mu.RUnlock()

	// 2. 排序 (Sort.Slice 现在使用预计算的 float64，速度极快)
	sort.Slice(sortableItems, func(i, j int) bool {
		a := sortableItems[i]
		b := sortableItems[j]

		switch m.currentSortField {
		case SortByVolume:
			// 默认：按成交量降序 (Largest Volume first)
			return a.VolumeFloat > b.VolumeFloat
		case SortByPriceChange:
			// 按涨跌幅降序 (Highest Price Change first)
			return a.ChangeFloat > b.ChangeFloat

		case SortByPrice:
			// 按价格降序 (Highest Price first)
			return a.PriceFloat > b.PriceFloat

		default:
			// 默认回退到 Volume 排序
			return a.VolumeFloat > b.VolumeFloat
		}
	})

	// 2. 生成新的 ID 列表
	newSortedIDs := make([]string, len(sortableItems))
	for i, item := range sortableItems {
		newSortedIDs[i] = item.OriginalItem.Coin.InstrumentID
	}

	var protoMsg *pb.WebSocketMessage // 声明在外部
	var shouldPush = false            // 标记是否需要推送

	// 缓存结果 需要用写锁
	m.mu.Lock()
	if !slicesEqual(m.SortedInstIDs, newSortedIDs) {
		// 只有排序结果发生变化时才更新缓存
		m.SortedInstIDs = newSortedIDs
		// 标记需要推送
		shouldPush = true
	}

	// 只需要在锁内生成需要发送的 Protobuf 消息，不需要执行发送 I/O
	// 将新的价格排序构造成Protobuf消息
	payload := &pb.SortUpdate{
		SortBy:        m.currentSortField,
		SortedInstIds: newSortedIDs,
	}
	protoMsg = &pb.WebSocketMessage{
		Type:    "SORT_UPDATE",
		Payload: &pb.WebSocketMessage_SortUpdate{SortUpdate: payload},
	}
	m.mu.Unlock()

	// 3. 缓存结果（需要写锁）
	m.mu.Lock()
	defer m.mu.Unlock()

	// 在锁外异步发送Kafka消息
	if shouldPush {
		// 必须使用 Goroutine异步发送Kafka消息
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// 写入kafka
			// 排序更新是稍低频事件，可以和订阅数量共用一个topic，或者使用一个新的低频主题

			// 使用marketdata_system主题
			topic := kafka.TopicSystem
			message := kafka.Message{
				Key:  "GLOBAL_COIN_SORT", // 使用固定Key确保所有排序更新有序
				Data: protoMsg,
			}
			if err := m.producer.Produce(ctx, topic, message); err != nil {
				log.Printf("ERROR: MarketDataService topic=%s 生产者写入kafka币种id排序数据失败: %v", topic, err)
			}
		}()
	}
}

// 辅助函数，用于比较两个 string slice 是否相等
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 加载所有基础数据 (仅在启动时调用一次)
func (m *MarketDataService) InitializeBaseInstruments(ctx context.Context, exID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// ⚠️ 核心修正：在订阅之前，同步等待 TickerService 连接成功
	log.Println("MarketDataService 正在等待OKX Ticker服务连接...")
	if err := m.tickerClient.WaitForConnectionReady(ctx); err != nil {
		return fmt.Errorf("failed to wait for OKX WS connection: %w", err)
	}
	log.Println("MarketDataService OKX WS连接就绪。继续进行完全订阅。")

	// 1. 获取所有基础数据
	coins, err := m.instrumentFetcher.GetAllActiveUSDTInstruments(ctx, exID)
	if err != nil {
		return err
	}

	// 2. 无条件存储到 m.baseCoins (全量覆盖)
	newCoinsMap := make(map[string]entity.CryptoInstrument, len(coins))
	var symbols []string
	for _, coin := range coins {
		newCoinsMap[coin.InstrumentID] = coin
		symbols = append(symbols, coin.InstrumentID)
	}
	m.baseCoins = newCoinsMap

	// 3. 启动 TickerService 的全量订阅
	// TickerService 会将整个列表发送给 OKX
	m.tickerClient.SubscribeSymbols(ctx, symbols)

	// 4. 不发送任何客户端通知 (因为客户端还未建立连接或 List 未初始化)

	return nil
}

// 永久运行，监听 TickerService 的连接事件
func (m *MarketDataService) runTickerResubscriptionLoop() {
	// 监听 TickerClient 的连接事件通道
	connectionEvents := m.tickerClient.ConnectionEvents()

	for {
		select {
		case <-connectionEvents: // 阻塞等待连接就绪信号
			// 收到连接就绪信号！现在我们必须恢复订阅。
			// 因为这是重连，所以 baseCoins 中应该已经有数据了。
			m.mu.RLock() // 只读锁保护 baseCoins 的读取

			var symbolsToResubscribe []string
			for symbol := range m.baseCoins {
				symbolsToResubscribe = append(symbolsToResubscribe, symbol)
			}

			m.mu.RUnlock() // 释放锁

			if len(symbolsToResubscribe) > 0 {
				log.Printf("MarketDataService 已重新连接。正在对%d个symbol执行重新订阅。", len(symbolsToResubscribe))

				// 执行重新订阅
				// 忽略 Context，因为这是后台的恢复操作
				err := m.tickerClient.SubscribeSymbols(context.Background(), symbolsToResubscribe)
				if err != nil {
					log.Printf("ERROR: MarketDataService 重新连接后重新订阅符号失败: %v", err)
					// 此时可以加入错误处理或指数退避机制
				}
			}
			// 循环继续，等待下一次连接断开重连
		}
	}
}

func (m *MarketDataService) UpdateInstruments(delistedInstruments, newInstruments []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 取消订阅下架的币种价格
	m.tickerClient.UnsubscribeSymbols(ctx, delistedInstruments)
	// 订阅新上架的币种价格
	m.tickerClient.SubscribeSymbols(ctx, newInstruments)

	// 清理 tradingItems：移除 delisted 的数据
	for _, instID := range delistedInstruments {
		delete(m.tradingItems, instID)
		log.Println("MarketDataService 从 tradingItems 中移除已下架的币种:", instID)
	}
}

func (m *MarketDataService) GetSortedIDsl() (data []string, sortBy string) {
	return m.SortedInstIDs, m.currentSortField
}

// GetPagedData 从内存中获取排序后的分页数据
func (m *MarketDataService) GetPagedData(page, limit int) ([]TradingItem, error) {
	m.mu.RLock() // 使用读锁保护共享资源
	defer m.mu.RUnlock()

	// 1. 参数验证和索引计算

	if page <= 0 || limit <= 0 {
		return nil, errors.New("page and limit must be positive")
	}

	totalItems := len(m.SortedInstIDs)

	// 计算起始索引和结束索引
	startIndex := (page - 1) * limit
	endIndex := startIndex + limit

	if startIndex >= totalItems {
		// 如果起始索引超出总数，说明该页没有数据
		return []TradingItem{}, nil
	}

	// 确保结束索引不超过总数
	if endIndex > totalItems {
		endIndex = totalItems
	}

	// 2. 核心步骤：根据缓存的 ID 列表进行切片

	// 获取当前页需要的 InstID 列表
	pagedIDs := m.SortedInstIDs[startIndex:endIndex]

	// 3. 数据查找（Lookup）和组装结果

	results := make([]TradingItem, 0, len(pagedIDs))

	// 遍历当前页的 ID 列表，并从 tradingItems 字典中快速查找数据
	for _, instID := range pagedIDs {
		if item, ok := m.tradingItems[instID]; ok {
			if item.Coin.ID == 0 {
				log.Printf("MarketDataService error： item.Coin.ID = 0")
			}
			// ⚠️ 注意：这里返回的是 TradingItem 的值类型副本
			results = append(results, item)
		} else {
			// 理论上不应该发生：如果 ID 在 SortedInstIDs 中，它就应该在 tradingItems 中。
			log.Printf("WARN: MarketDataService 在SortedInstIDs缓存中找到InstID%s，但在tradingItems映射中找不到。", instID)
			// 在生产环境中，可能需要返回一个带占位符的 TradingItem
		}
	}

	return results, nil
}

// ChangeSortField 更改当前全局排序的规则
func (m *MarketDataService) ChangeSortField(newField string) error {

	// 1. 验证新字段是否支持
	switch newField {
	case SortByVolume, SortByPriceChange, SortByPrice:
		// 支持的字段
	default:
		return errors.New("unsupported sort field: " + newField)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 2. 检查是否需要更新（避免不必要的排序和推送）
	if m.currentSortField == newField {
		return nil // 排序规则未变，直接返回
	}

	// 3. 更新排序字段
	m.currentSortField = newField

	// 4. 强制触发一次立即排序（无需等待定时器）
	// 注意：为了不阻塞主线程，这里通常通过一个 Channel 或 Go routine 触发
	go m.performSortAndCache()

	return nil
}

func (m *MarketDataService) GetPrices() map[string]float64 {
	// 注意m.tradingItems是make创建的，是一个指针，必须从上到下加锁
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.tradingItems
	prices := make(map[string]float64)
	for k, v := range items {
		price, _ := strconv.ParseFloat(v.Ticker.LastPrice, 64)
		prices[k] = price
	}
	return prices
}

func (m *MarketDataService) GetDetailByID(ctx context.Context, req model.MarketDetailReq) (*model.MarketDetail, error) {
	m.mu.Lock()
	coin, ok := m.baseCoins[req.InstrumentID]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("不存在的交易对:%v", req.InstrumentID)
	}

	var detail model.MarketDetail
	detail.InstrumentID = coin.InstrumentID
	detail.PricePrecision = coin.PricePrecision
	tradeType := req.TradeType
	if tradeType == "" {
		tradeType = model.OrderTradeSpot
	}
	kLines, err := m.ex.GetKlineRecords(req.InstrumentID, model2.KlinePeriod(req.TimePeriod), req.Size, req.StartTime, req.EndTime, tradeType, true)
	if err != nil {
		return nil, err
	}
	detail.HistoryKlines = kLines

	if len(kLines) >= 2 {
		startTime := kLines[0].Timestamp
		endTime := kLines[len(kLines)-1].Timestamp
		siganls, err := m.signalRepo.GetSignalsByTimeRange(ctx, req.InstrumentID, startTime, endTime)
		if err == nil {
			detail.HistorySignals = siganls
		}
		if len(siganls) == 0 {
			strs := strings.Split(req.InstrumentID, "-")
			var symbol string
			if len(strs) >= 2 {
				symbol = strs[0] + "/" + strs[1]
			}
			siganls, err := m.signalRepo.GetSignalsByTimeRange(ctx, symbol, startTime, endTime)
			if err == nil {
				detail.HistorySignals = siganls
			}
		}
	}
	return &detail, nil
}

// CheckAndTriggerAlerts 检查并触发给定币种的价格提醒
// 必须在 m.mu.Lock() 保护下调用
func (m *MarketDataService) CheckAndTriggerAlerts(instID string, currentPrice, lastPrice float64) {

	// 1. 检查该币种是否有活跃的提醒
	subs := m.alertService.GetSubscriptionsForInstID(instID)
	if len(subs) == 0 {
		return // 没有订阅
	}

	history := m.priceHistory[instID]

	// 价格重置缓冲区：价格必须远离目标价格 0.5% 才能重置
	// 这是一个关键参数，防止价格在阈值附近震荡导致频繁触发和重置
	const resetBuffer = 0.005

	// 2. 遍历该币种的所有订阅
	for _, sub := range subs {
		// 检查通用价格关口提醒 (BoundaryPrecision > 0.0)
		// 假设 BoundaryPrecision 已经被 mapModelToServiceSubscription 转换为 float64
		if sub.BoundaryPrecision > 0.0 {

			if lastPrice <= 0 {
				continue
			} // 价格无效，跳过

			// 核心参数
			precision := sub.BoundaryPrecision

			// 确保从低到高遍历
			low := math.Min(currentPrice, lastPrice)
			high := math.Max(currentPrice, lastPrice)

			// 1. 计算起始关口和结束关口
			// 示例：precision=0.01。low=0.1689。startBoundary = 0.17
			startBoundary := math.Floor(low/precision)*precision + precision
			endBoundary := math.Floor(high/precision) * precision

			// 修正浮点数误差，确保计算精确
			startBoundary = math.Round(startBoundary/precision) * precision
			endBoundary = math.Round(endBoundary/precision) * precision

			boundary := startBoundary

			// 2. 遍历所有跨越的关口
			for boundary <= endBoundary {

				// 修正浮点数误差
				boundary = math.Round(boundary/precision) * precision

				triggered := false
				alertTitle := ""

				// UP 订阅：上次价格 < 关口 AND 当前价格 >= 关口
				if sub.Direction == "UP" && lastPrice < boundary && currentPrice >= boundary {
					triggered = true
					alertTitle = fmt.Sprintf("%s 向上突破价格关口 $%.*f", instID, m.GetPrecisionDecimals(precision), boundary)
				} else if sub.Direction == "DOWN" && lastPrice > boundary && currentPrice <= boundary {
					// DOWN 订阅：上次价格 > 关口 AND 当前价格 <= 关口
					triggered = true
					alertTitle = fmt.Sprintf("%s 向下突破价格关口 $%.*f", instID, m.GetPrecisionDecimals(precision), boundary)
				}

				if triggered {
					// 3. 🚀 构建 AlertMessage 并调用 PublishToDevice
					alertMsg := &pb.AlertMessage{
						UserId:         sub.UserID,
						SubscriptionId: sub.SubscriptionID,
						Id:             uuid.NewString(), // 唯一消息 ID
						Title:          alertTitle,
						Content: fmt.Sprintf("当前价格已达到 $%.*f，成功突破了 $%.*f 的关口。",
							m.GetPrecisionDecimals(precision),
							currentPrice,
							m.GetPrecisionDecimals(precision),
							boundary),
						Symbol:    instID,
						Level:     pb.AlertLevel_ALERT_LEVEL_INFO, // 通用关口设为 INFO 级别
						AlertType: pb.AlertType_ALERT_TYPE_PRICE,
						Timestamp: time.Now().UnixMilli(),
						Extra: map[string]string{
							"trigger_price":   fmt.Sprintf("%.*f", m.GetPrecisionDecimals(precision), boundary),
							"current_price":   fmt.Sprintf("%.8f", currentPrice), // 记录原始全精度价格
							"precision_level": fmt.Sprintf("%.8f", precision),
						},
					}

					// 4. 异步发布消息 (不需要调用 MarkSubscriptionAsTriggered)
					go m.alertService.PublishBroadcast(alertMsg)

					log.Printf("ALERT: [%s] 触发通用价格关口提醒: %s", instID, alertTitle)
				}

				// 移动到下一个关口
				boundary += precision
			}
		}

		// ----------------------------------------------------
		// 重置检查 (检查已触发的提醒是否可以重新激活)
		// ----------------------------------------------------
		if !sub.IsActive {

			// 只有 TargetPrice > 0 或 ChangePercent > 0 且上次触发价有效才检查重置
			if sub.LastTriggeredPrice <= 0 {
				continue
			}

			shouldReset := false

			// 检查价格突破提醒的重置条件 (TargetPrice > 0)
			if sub.TargetPrice > 0 {
				// UP 提醒 (突破 TargetPrice): 需跌破 TargetPrice 的另一侧缓冲区
				if sub.Direction == "UP" && currentPrice < sub.TargetPrice*(1.0-resetBuffer) {
					shouldReset = true
				} else if sub.Direction == "DOWN" && currentPrice > sub.TargetPrice*(1.0+resetBuffer) {
					// DOWN 提醒 (跌破 TargetPrice): 需涨回 TargetPrice 的另一侧缓冲区
					shouldReset = true
				}
			} else if sub.ChangePercent > 0 {
				// 检查极速提醒的重置条件 (基于上次触发价格的相对重置)
				// 如果是极速提醒，假设价格必须远离上次触发价格至少 1% 才重置
				if math.Abs(currentPrice-sub.LastTriggeredPrice)/sub.LastTriggeredPrice > 0.01 {
					shouldReset = true
				}
			}

			if shouldReset {
				// 🚀 通知 AlertService 重置状态
				m.alertService.MarkSubscriptionAsReset(sub.InstID, sub.SubscriptionID)
			}

			continue // 仍然处于已触发/重置缓冲区内
		}

		// 突破检查
		if sub.TargetPrice > 0 && (sub.Direction == "UP" && currentPrice >= sub.TargetPrice || // 向上突破
			sub.Direction == "DOWN" && currentPrice <= sub.TargetPrice) { // 向下突破
			// 3. 触发提醒
			// 标记订阅为非活跃，防止重复触发
			m.alertService.MarkSubscriptionAsTriggered(sub.InstID, sub.SubscriptionID, currentPrice)

			// 4. 构建 Protobuf 提醒消息
			alertMsg := &pb.AlertMessage{
				UserId:         sub.UserID,
				SubscriptionId: sub.SubscriptionID,
				Id:             uuid.NewString(),
				Title:          fmt.Sprintf("%s 价格提醒", instID),
				Content:        fmt.Sprintf("%s 已达到 ¥%.2f", instID, currentPrice),
				Symbol:         instID,
				Level:          pb.AlertLevel_ALERT_LEVEL_WARNING,
				AlertType:      pb.AlertType_ALERT_TYPE_PRICE,
				Timestamp:      time.Now().UnixMilli(),
				// 附加数据用于 UI 展示
				Extra: map[string]string{
					"trigger_price": fmt.Sprintf("%.2f", sub.TargetPrice),
					"current_price": fmt.Sprintf("%.2f", currentPrice),
				},
			}

			// 5. 🚀 调用 AlertService 异步发送 (写入 Kafka 定向 Topic)
			// 避免在锁内执行耗时操作，但AlertService是同步写入Kafka，需要注意性能
			// 最佳实践是AlertService内部将消息放入Channel并异步写入Kafka
			go m.alertService.PublishBroadcast(alertMsg)
		}

		// 检查极速上涨/下跌 (ChangePercent)
		if sub.ChangePercent > 0 && sub.WindowMinutes > 0 && len(history) > 0 {
			// 1. 确定时间窗口的起点时间戳
			startTime := time.Now().Add(-time.Duration(sub.WindowMinutes) * time.Minute).UnixMilli()

			// 2. 找到窗口内的起始价格点 (最旧的价格)
			// 由于历史记录是有序且已清理，只需从头开始找第一个在窗口内的点
			var startPrice float64 = -1
			for _, pp := range history {
				if pp.Timestamp >= startTime {
					startPrice = pp.Price
					break
				}
			}

			// 如果历史记录不足，无法计算速率，跳过
			if startPrice <= 0 {
				continue
			}

			// 3. 计算实际变化率
			actualChange := (currentPrice - startPrice) / startPrice * 100.0

			// 4. 检查触发条件
			triggered := false
			alertTitle := ""

			// 检查极速上涨
			if sub.Direction == "UP" && actualChange >= sub.ChangePercent {
				triggered = true
				alertTitle = fmt.Sprintf("%s 极速上涨 %s%% 预警", instID, fmt.Sprintf("%.2f", sub.ChangePercent))
			}
			// 检查极速下跌
			if sub.Direction == "DOWN" && actualChange <= -sub.ChangePercent {
				triggered = true
				alertTitle = fmt.Sprintf("%s 极速下跌 %s%% 预警", instID, fmt.Sprintf("%.2f", sub.ChangePercent))
			}

			if triggered {
				// 标记已经触发
				m.alertService.MarkSubscriptionAsTriggered(sub.InstID, sub.SubscriptionID, currentPrice)

				// 构建 Protobuf 提醒消息
				alertMsg := &pb.AlertMessage{
					UserId:         sub.UserID,
					SubscriptionId: sub.SubscriptionID,
					Id:             uuid.NewString(),
					Title:          alertTitle,
					Content:        fmt.Sprintf("%s 在 %d 分钟内变化了 %.2f%%，当前价格 %.2f", instID, sub.WindowMinutes, actualChange, currentPrice),
					Symbol:         instID,
					Level:          pb.AlertLevel_ALERT_LEVEL_CRITICAL,
					AlertType:      pb.AlertType_ALERT_TYPE_PRICE,
					Timestamp:      time.Now().UnixMilli(),
					Extra: map[string]string{
						"change_percent": fmt.Sprintf("%.2f", actualChange),
						"window_minutes": fmt.Sprintf("%d", sub.WindowMinutes),
					},
				}

				// 异步发送
				go m.alertService.PublishBroadcast(alertMsg)
			}
		}
	}
}

// GetPrecisionDecimals 根据粒度（如 0.01）确定格式化所需的有效小数位数（如 2）。
// 这对于正确显示价格关口非常重要。
func (m *MarketDataService) GetPrecisionDecimals(precision float64) int {
	if precision <= 0 {
		return 8 // 安全默认值
	}

	// 1. 处理整数粒度 (1, 10, 100...)
	// 如果 precision >= 1.0，则不需要小数位
	if precision >= 1.0 {
		return 0
	}

	// 2. 处理小数粒度 (0.1, 0.01, 0.001...)
	// 使用 Log10 来找到 10 的幂次，即需要的小数位数。
	// 示例：Log10(0.01) = -2。取绝对值即为 2。

	// ⚠️ 注意：Go 的 float64 运算可能导致微小的误差 (如 0.01 可能变成 0.009999999999999998)
	// 解决方法：
	// a) 先将 precision 取倒数： 1 / 0.01 = 100.0
	val := 1.0 / precision

	// b) 计算 Log10，并四舍五入到最近的整数，避免浮点误差
	decimals := math.Log10(val)

	// c) 确保结果是正整数
	return int(math.Round(decimals))
}
