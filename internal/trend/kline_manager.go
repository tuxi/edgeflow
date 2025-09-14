package trend

import (
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	model2 "github.com/nntaoli-project/goex/v2/model"
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
	return &KlineManager{
		cache:   make(map[string]map[model2.KlinePeriod][]model.Kline),
		mu:      sync.RWMutex{},
		ex:      ex,
		symbols: symbols,
	}
}

// 启动调度：独立于 15min 信号
func (tm *KlineManager) RunScheduled(onUpdate func()) {
	// --- 第一次启动时先拉取k线数据铺底 ---
	log.Println("初始化趋势数据...")
	// 拉取所有k线作为基础数据
	tm.updateKAllLines()

	// 初始化完成后，更新一次趋势
	onUpdate()

	go tm.run(onUpdate)
}

func (tm *KlineManager) run(onUpdate func()) {

	log.Println("[trend.KlineManager 启动]，等待下一次时间对齐拉取k线")
	// 等待到下一个对齐点（比如30m整点）
	tm.waitForNextAlignment()

	// 在开启定时器前，先拉取一次所有k线
	tm.updateKAllLines()

	// 定时器开始前，先执行一次update
	onUpdate()

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	log.Println("[trend.KlineManager 启动]，独立调度趋势K线")

	for {
		select {
		case t := <-ticker.C:
			log.Printf("[Trend] 调度触发: %s", t.Format("15:04:05"))

			// 15m对齐
			//if t.Minute()%5 == 0 {
			//	tm.updateKlines(tm.symbols, model2.Kline_15min)
			//}
			if t.Minute()%15 == 0 {
				tm.updateKlines(tm.symbols, model2.Kline_15min)
			}

			// 30m对齐
			if t.Minute()%30 == 0 {
				tm.updateKlines(tm.symbols, model2.Kline_30min)
			}

			// 1h整点
			if t.Minute() == 0 {
				tm.updateKlines(tm.symbols, model2.Kline_1h)
			}

			// 4h整点
			if t.Hour()%4 == 0 && t.Minute() == 0 {
				tm.updateKlines(tm.symbols, model2.Kline_4h)
			}

			// 每次调度完，更新趋势
			onUpdate()
		}
	}
}

func (tm *KlineManager) updateKAllLines() {
	tm.updateKlines(tm.symbols, model2.Kline_15min)
	tm.updateKlines(tm.symbols, model2.Kline_30min)
	tm.updateKlines(tm.symbols, model2.Kline_4h)
	tm.updateKlines(tm.symbols, model2.Kline_1h)
}

// 等待第一个对齐点（比如整30m）
func (tm *KlineManager) waitForNextAlignment() {
	now := time.Now()
	next := now.Truncate(15 * time.Minute).Add(15 * time.Minute)
	// 再加30秒确保交易所数据更新完成
	waitUntil := next.Add(30 * time.Second)
	sleep := time.Until(waitUntil)
	log.Printf("[Trend] 等待到 %s 开始调度", next.Format("15:04:05"))
	time.Sleep(sleep)
}

// 拉取并存储K线
func (tm *KlineManager) updateKlines(symbols []string, timeframe model2.KlinePeriod) error {
	for _, symbol := range symbols {
		kLines, err := tm.ex.GetKlineRecords(symbol, timeframe, 210, 0, model.OrderTradeSwap, true)
		if err != nil {
			log.Printf("[TrendManager] fetch %v kline error for %s: %v", timeframe, symbol, err)
			return err
		}
		tm.mu.Lock()
		if _, ok := tm.cache[symbol]; !ok {
			m := make(map[model2.KlinePeriod][]model.Kline)
			tm.cache[symbol] = m
		}
		tm.cache[symbol][timeframe] = kLines
		tm.mu.Unlock()
		log.Printf("[Trend] %s 更新 %s K线，条数=%d", symbol, timeframe, len(kLines))
	}
	return nil
}

// 获取缓存
func (km *KlineManager) Get(symbol string, period model2.KlinePeriod) ([]model.Kline, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	lines, ok := km.cache[symbol][period]
	return lines, ok
}
