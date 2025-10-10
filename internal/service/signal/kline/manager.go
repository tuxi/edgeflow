package kline

import (
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	"edgeflow/pkg/utils"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"go.uber.org/multierr"
	"log"
	"sync"
	"time"
)

type KlineManager struct {
	cache   map[string]map[model2.KlinePeriod][]model.Kline // symbol -> timeframe -> klines
	mu      sync.RWMutex
	ex      exchange.Exchange
	symbols []string
}

func NewKlineManager(ex exchange.Exchange, symbols []string) *KlineManager {
	newSymbols := make([]string, len(symbols))
	for i, symbol := range symbols {
		newSymbols[i] = utils.FormatSymbol(symbol)
	}

	return &KlineManager{
		cache:   make(map[string]map[model2.KlinePeriod][]model.Kline),
		mu:      sync.RWMutex{},
		ex:      ex,
		symbols: newSymbols,
	}
}

// 启动调度：独立于 15min 信号
func (tm *KlineManager) RunScheduled(updateCh chan<- struct{}) {
	// --- 第一次启动时先拉取k线数据铺底 ---
	log.Println("初始化趋势数据...")
	// 获取所有周期的K线数据并缓存
	tm.fetchKAllLines()

	// 熟悉拉取k线数据时，向通道发送通知
	updateCh <- struct{}{}

	// 启动长期循环调度
	go tm.run(updateCh)
}

func (km *KlineManager) run(updateCh chan<- struct{}) {
	log.Println("[trend.KlineManager 启动]，进入精准调度循环")

	for {
		now := time.Now()
		nextAlignment := now.Truncate(15 * time.Minute).Add(15 * time.Minute)
		waitUntil := nextAlignment.Add(30 * time.Second) // 留出30s缓冲时间

		sleepDuration := time.Until(waitUntil)
		if sleepDuration > 0 {
			log.Printf("[KlineManager] 下一次调度时间：%s（等待 %s）", waitUntil.Format("15:04:05"), sleepDuration)
			time.Sleep(sleepDuration)
		}

		// 触发调度和更新
		now = time.Now()

		// 15分钟 k线是每次都拉取的
		km.fetchKlines(km.symbols, model2.Kline_15min)

		if now.Minute()%30 == 0 { // 30分钟
			km.fetchKlines(km.symbols, model2.Kline_30min)
		}
		if now.Minute() == 0 { // 1h整点
			km.fetchKlines(km.symbols, model2.Kline_1h)
		}
		if now.Hour()%4 == 0 && now.Minute() == 0 { // 4小时整点
			km.fetchKlines(km.symbols, model2.Kline_4h)
		}

		// 每次更新完，发送通知
		// 使用select-default 避免通道阻塞
		select {
		case updateCh <- struct{}{}:
		default:
			// 通道已满，忽略本次更新通知，记录日志
		}
	}
}

func (tm *KlineManager) fetchKAllLines() {
	tm.fetchKlines(tm.symbols, model2.Kline_15min)
	tm.fetchKlines(tm.symbols, model2.Kline_30min)
	tm.fetchKlines(tm.symbols, model2.Kline_4h)
	tm.fetchKlines(tm.symbols, model2.Kline_1h)
}

// 拉取并存储K线
func (tm *KlineManager) fetchKlines(symbols []string, timeframe model2.KlinePeriod) error {
	var errs error
	for _, symbol := range symbols {
		kLines, err := tm.ex.GetKlineRecords(symbol, timeframe, 210, 0, model.OrderTradeSwap, true)
		if err != nil {
			log.Printf("[TrendManager] fetch %v kline error for %s: %v", timeframe, symbol, err)
			errs = multierr.Append(errs, err) // 使用 multierror 记录所有错误
			continue
		}
		tm.mu.Lock()
		if _, ok := tm.cache[symbol]; !ok {
			m := make(map[model2.KlinePeriod][]model.Kline)
			tm.cache[symbol] = m
		}
		tm.cache[symbol][timeframe] = kLines
		tm.mu.Unlock()
		//log.Printf("[Trend] %s 更新 %s K线，条数=%d", symbol, timeframe, len(kLines))
	}
	return errs
}

// 获取缓存
func (km *KlineManager) Get(symbol string, period model2.KlinePeriod) ([]model.Kline, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	lines, ok := km.cache[symbol][period]
	return lines, ok
}
