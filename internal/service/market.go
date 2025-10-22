package service

import (
	"context"
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/internal/service/signal/repository"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"errors"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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
	signalRepo repository.SignalRepository // DB 接口
}

func NewMarketDataService(ticker *OKXTickerService, instrumentFetcher InstrumentFetcher, ex exchange.Exchange, SignalRepo repository.SignalRepository, producer kafka.ProducerService) *MarketDataService {
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
	}
	// 启动 MarketService 的核心 Worker
	go m.startDataWorkers()
	// 每次 收到连接成功信号时，执行一次订阅恢复
	go m.runTickerResubscriptionLoop()
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

		// A. 尝试更新已存在的 TradingItem
		if item, ok := m.tradingItems[instID]; ok {
			// 直接更新 Ticker 数据
			item.Ticker = ticker
			m.tradingItems[instID] = item
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
		tickers = append(tickers, &pb.TickerUpdate{
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
		})
	}

	// 映射到Protobuf 结构体
	payload := &pb.WebSocketMessage_TickerBatch{TickerBatch: &pb.TickerBatch{
		Tickers: tickers,
	}}
	protoMsg := &pb.WebSocketMessage{
		Type:    "TICKER_UPDATE",
		Payload: payload,
	}

	// 仅启动一个 Goroutine 来处理整个批次的 I/O
	// 将 I/O 阻塞操作（Kafka 写入） 放入独立的Goroutine
	go func(protoMsg *pb.WebSocketMessage) {
		// 将Kafka 写入超时时间设置为 2 秒，防止超时
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel() // 确保context 及时释放
		// 序列化并写入kafka
		key := []byte("TICKER_BATCH_KEY")
		topic := "marketdata_ticker" // Ticker 高频主题
		if err := m.producer.Produce(ctx, topic, key, protoMsg); err != nil {
			// 记录错误，但不阻塞主循环
			log.Printf("ERROR: topic=%s 生产者批量写入Ticker数据到kafka失败: %v", topic, err)
		}
	}(protoMsg) // 传递ticker, protoMsg 副本

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
			//
			key := []byte("GLOBAL_COIN_SORT") // 使用固定Key确保所有排序更新有序
			// 使用marketdata_system主题
			topic := "marketdata_system"
			if err := m.producer.Produce(ctx, topic, key, protoMsg); err != nil {
				log.Printf("ERROR: topic=%s 生产者写入kafka币种id排序数据失败: %v", topic, err)
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
	log.Println("Waiting for OKX Ticker Service connection...")
	if err := m.tickerClient.WaitForConnectionReady(ctx); err != nil {
		return fmt.Errorf("failed to wait for OKX WS connection: %w", err)
	}
	log.Println("OKX WS connection ready. Proceeding with full subscription.")

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
				log.Printf("TickerService reconnected. Executing resubscription for %d symbols.", len(symbolsToResubscribe))

				// 执行重新订阅
				// 忽略 Context，因为这是后台的恢复操作
				err := m.tickerClient.SubscribeSymbols(context.Background(), symbolsToResubscribe)
				if err != nil {
					log.Printf("ERROR: Failed to resubscribe symbols after reconnect: %v", err)
					// 此时可以加入错误处理或指数退避机制
				}
			}
			// 循环继续，等待下一次连接断开重连
		}
	}
}

// 定时器每小时调用检查币种的上新和下架
func (m *MarketDataService) PerformPeriodicUpdate(ctx context.Context) error {
	// 获取所有USDT基础交易对
	newCoins, err := m.instrumentFetcher.GetAllActiveUSDTInstruments(ctx, 1)
	if err != nil {
		// 错误处理，可能数据库连接失败等
		return fmt.Errorf("failed to load base instruments from DAO: %w", err)
	}
	if len(newCoins) == 0 {
		return errors.New("暂时没有交易对数据")
	}
	// 追踪需要新增订阅和需要退订的 InstID
	var toSubscribe []string
	var toUnsubscribe []string

	newCoinsMap := make(map[string]entity.CryptoInstrument, len(newCoins))

	// A. 找出新增或变更的币种 (新列表 - 旧列表)
	for _, coin := range newCoins {
		newCoinsMap[coin.InstrumentID] = coin

		// 如果旧列表（m.baseCoins）中没有这个币种，则需要新增订阅
		if _, ok := m.baseCoins[coin.InstrumentID]; !ok {
			toSubscribe = append(toSubscribe, coin.InstrumentID)
		}
		// 如果数据有变更，虽然不常见，但可以根据需要重新订阅
	}

	// B. 找出需要退订的币种 (旧列表 - 新列表)
	for instID := range m.baseCoins {
		// 如果新列表中没有旧列表的币种，则说明它已被下架
		if _, ok := newCoinsMap[instID]; !ok {
			toUnsubscribe = append(toUnsubscribe, instID)
		}
	}

	// 3. 更新内存状态
	m.baseCoins = newCoinsMap // 用新 Map 完全替换旧 Map

	// 4. 通知 TickerService 执行订阅/退订操作

	if len(toUnsubscribe) > 0 {
		// 通知 OKX 退订已下架的币种
		m.tickerClient.UnsubscribeSymbols(ctx, toUnsubscribe)
		log.Printf("Unsubscribed from %d delisted instruments.", len(toUnsubscribe))

		// ⚠️ 通知 DAO：将这些币种的状态更新为 'DELISTED'
		err := m.instrumentFetcher.UpdateInstrumentStatus(ctx, 1, toUnsubscribe, "DELIST")
		if err != nil {
			// 如果数据库更新失败，我们需要决定是否继续。
			// 建议：记录错误，并继续执行，因为内存状态更新更重要，下次重试。
			log.Printf("CRITICAL ERROR: Failed to update instrument status to DELISTED for %v in DB: %v", toUnsubscribe, err)
			// 我们可以选择返回错误，让 Worker 停止直到解决，或者继续（此处选择继续）。
		}
	}

	if len(toSubscribe) > 0 {
		// 通知 OKX 订阅新上架的币种
		m.tickerClient.SubscribeSymbols(ctx, toSubscribe) // 假设 TickerService 有这个方法
		log.Printf("Subscribed to %d newly listed instruments.", len(toSubscribe))
	}

	// 5.通知handler 上新和下架交易对的消息
	// ⚠️ 核心：通知 MarketHandler 客户端需要更新
	if len(toSubscribe) > 0 || len(toUnsubscribe) > 0 {

		// 构造Protobuf消息
		payload := &pb.InstrumentUpdate{
			NewInstruments:      toSubscribe,
			DelistedInstruments: toUnsubscribe,
		}
		protoMsg := &pb.WebSocketMessage{
			Type:    "INSTRUMENT_UPDATE",
			Payload: &pb.WebSocketMessage_InstrumentStatusUpdate{InstrumentStatusUpdate: payload},
		}
		// 写入kafka
		ctx := context.Background()
		topic := "marketdata_system" // 使用低频系统主题
		key := []byte("INSTRUMENT_CHANGE")

		if err := m.producer.Produce(ctx, topic, key, protoMsg); err != nil {
			log.Printf("ERROR: topic=%s 生产者写入Kafka 币种更新数据失败: %v", topic, err)
		}
	}

	// 6.清理 tradingItems：移除 delisted 的数据
	for _, instID := range toUnsubscribe {
		delete(m.tradingItems, instID)
		log.Printf("Cleaned up delisted instrument %s from tradingItems.", instID)
	}

	return nil

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
				log.Printf("error")
			}
			// ⚠️ 注意：这里返回的是 TradingItem 的值类型副本
			results = append(results, item)
		} else {
			// 理论上不应该发生：如果 ID 在 SortedInstIDs 中，它就应该在 tradingItems 中。
			log.Printf("WARN: InstID %s found in SortedInstIDs cache but not in tradingItems map.", instID)
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
