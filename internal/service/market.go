package service

import (
	"context"
	"edgeflow/internal/model/entity"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
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
type MarketDataService struct {
	// 锁用于保护所有共享内存数据，确保并发安全
	mu sync.RWMutex

	// 基础数据：所有交易对的 CoinItem
	baseCoins map[string]entity.CryptoInstrument

	// 统一的内存存储：所有活跃的交易对数据
	TradingItems map[string]TradingItem

	// 排序缓存：按成交量排序的 InstID 列表
	SortedInstIDs []string

	// 依赖服务
	tickerClient      *OKXTickerService // 实时数据源
	instrumentFetcher InstrumentFetcher // 基础数据源

	// 控制定时排序的通道
	stopSortCh      chan struct{}
	sortedInstIDsCh chan []string
	// 实时价格更新通道
	priceUpdateCh chan TickerData

	// 当前生效的排序字段
	currentSortField string

	// 用于通知handler 基础数据结构变化的通道instrumentUpdateCh
	instrumentUpdateCh chan BaseInstrumentUpdate
}

func NewMarketDataService(ticker *OKXTickerService, instrumentFetcher InstrumentFetcher) *MarketDataService {
	m := &MarketDataService{
		baseCoins:          make(map[string]entity.CryptoInstrument),
		TradingItems:       make(map[string]TradingItem),
		SortedInstIDs:      make([]string, 0),
		tickerClient:       ticker,
		instrumentFetcher:  instrumentFetcher,
		stopSortCh:         make(chan struct{}),
		sortedInstIDsCh:    make(chan []string, 100),
		priceUpdateCh:      make(chan TickerData, 500),
		currentSortField:   SortByVolume,                        // 默认按成交量排序
		instrumentUpdateCh: make(chan BaseInstrumentUpdate, 10), // 缓冲区小，因为频率低
	}
	// 启动 MarketService 的核心 Worker
	go m.startDataWorkers()
	return m
}

// StartDataWorkers 启动 K线获取和排序定时器，并监听 TickerService 的更新
func (m *MarketDataService) startDataWorkers() {
	// 1. 启动定时排序 Worker
	go m.startSortingScheduler()

	// 2. 监听 TickerService 的实时数据更新（假设 TickerService 有一个 TickerData 管道）
	tickerUpdates := m.tickerClient.GetTickerChannel()

	for {
		select {
		case ticker := <-tickerUpdates:
			m.updateRealTimeData(ticker) // 处理实时数据整合
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
		if item, ok := m.TradingItems[instID]; ok {
			// 直接更新 Ticker 数据
			item.Ticker = ticker
			m.TradingItems[instID] = item

			// 将此 Ticker 加入转发列表
			tickersToForward = append(tickersToForward, ticker)
			continue
		}

		// B. 新数据：尝试组合
		if coin, ok := m.baseCoins[instID]; ok {
			// 成功组合：基础数据 + 实时数据
			m.TradingItems[instID] = TradingItem{
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

	// --- 2. 非临界区操作：通道转发 ---

	// 循环发送所有收集到的 Ticker 数据给下游 Handler
	for _, ticker := range tickersToForward {
		select {
		case m.priceUpdateCh <- ticker:
			// 成功转发
		default:
			// 通道满，丢弃本次 Ticker，不阻塞！
			// log.Println("WARN: Price update channel full, dropping Ticker for", ticker.InstID)
		}
	}

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
	// 将所有 TradingItem 转换为一个切片 (Slice)
	items := make([]TradingItem, 0, len(m.TradingItems))
	for _, item := range m.TradingItems {
		items = append(items, item)
	}
	m.mu.RUnlock()

	// 1. 根据 currentField 选择排序算法
	sort.Slice(items, func(i, j int) bool {
		a := items[i].Ticker
		b := items[j].Ticker

		switch m.currentSortField {
		case SortByVolume:
			// 默认：按成交量降序 (Largest Volume first)
			v1, _ := strconv.ParseFloat(a.VolCcy24h, 64)
			v2, _ := strconv.ParseFloat(b.VolCcy24h, 64)
			return v1 > v2

		case SortByPriceChange:
			// 按涨跌幅降序 (Highest Price Change first)
			return a.Change24h > b.Change24h

		case SortByPrice:
			// 按价格降序 (Highest Price first)
			p1, _ := strconv.ParseFloat(a.LastPrice, 64)
			p2, _ := strconv.ParseFloat(b.LastPrice, 64)
			return p1 > p2

		default:
			// 默认回退到 Volume 排序
			v1, _ := strconv.ParseFloat(a.VolCcy24h, 64)
			v2, _ := strconv.ParseFloat(b.VolCcy24h, 64)
			return v1 > v2
		}
	})

	// 1. 核心排序操作（在后台线程完成）
	sort.Slice(items, func(i, j int) bool {
		// 按 Volume 降序排序
		volCcy24h1, _ := strconv.ParseFloat(items[i].Ticker.VolCcy24h, 64)
		volCcy24h2, _ := strconv.ParseFloat(items[j].Ticker.VolCcy24h, 64)
		return volCcy24h1 > volCcy24h2
	})

	// 2. 生成新的 ID 列表
	newSortedIDs := make([]string, len(items))
	for i, item := range items {
		newSortedIDs[i] = item.Coin.InstrumentID
	}

	// 3. 缓存结果（需要写锁）
	m.mu.Lock()
	defer m.mu.Unlock()

	// 差异推送的核心逻辑：只有排序结果发生变化时才更新缓存（避免不必要的推送）
	if !slicesEqual(m.SortedInstIDs, newSortedIDs) {
		m.SortedInstIDs = newSortedIDs
		// ⚠️ 此处应触发 WebSocket 推送给客户端 (例如发送一个新的 [String] 列表)
		select {
		case m.sortedInstIDsCh <- newSortedIDs:
			log.Printf("交易对ids排序发生变化")
		default:
			// 通道满了，丢弃本次变化
		}
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

		update := BaseInstrumentUpdate{
			NewInstruments:      toSubscribe,
			DelistedInstruments: toUnsubscribe,
		}

		select {
		case m.instrumentUpdateCh <- update:
			log.Println("Instrument structure update sent to handler.")
		default:
			log.Println("WARN: Instrument update channel full, structure update skipped.")
		}
	}

	// 6.清理 TradingItems：移除 delisted 的数据
	for _, instID := range toUnsubscribe {
		delete(m.TradingItems, instID)
		log.Printf("Cleaned up delisted instrument %s from TradingItems.", instID)
	}

	return nil

}

func (m *MarketDataService) GetSortedIDsChannel() <-chan []string {
	return m.sortedInstIDsCh
}

func (m *MarketDataService) GetSortedIDsl() []string {
	return m.SortedInstIDs
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

	// 遍历当前页的 ID 列表，并从 TradingItems 字典中快速查找数据
	for _, instID := range pagedIDs {
		if item, ok := m.TradingItems[instID]; ok {
			if item.Coin.ID == 0 {
				log.Printf("error")
			}
			// ⚠️ 注意：这里返回的是 TradingItem 的值类型副本
			results = append(results, item)
		} else {
			// 理论上不应该发生：如果 ID 在 SortedInstIDs 中，它就应该在 TradingItems 中。
			log.Printf("WARN: InstID %s found in SortedInstIDs cache but not in TradingItems map.", instID)
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

func (m *MarketDataService) GetPriceUpdateChannel() <-chan TickerData {
	return m.priceUpdateCh
}

func (m *MarketDataService) GetInstrumentUpdateChannel() <-chan BaseInstrumentUpdate {
	return m.instrumentUpdateCh
}
