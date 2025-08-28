package trend

import (
	"edgeflow/internal/exchange"
	model2 "edgeflow/internal/model"
	"fmt"
	"log"
	"sync"
	"time"

	talib "github.com/markcheno/go-talib"
	"github.com/nntaoli-project/goex/v2/model"
)

// TrendManager 负责管理多个币种的趋势状态
type Manager struct {
	mu     sync.RWMutex
	states map[string]*TrendState

	ex      exchange.Exchange // OKX 客户端
	symbols []string
	stopCh  chan struct{}
	cfg     TrendCfg
}

func NewManager(ex exchange.Exchange, symbols []string) *Manager {
	return &Manager{
		states:  make(map[string]*TrendState),
		ex:      ex,
		symbols: symbols,
		stopCh:  make(chan struct{}),
		cfg:     DefaultTrendCfg(),
	}
}

// 启动自动更新（定时拉取 K 线）
func (tm *Manager) StartUpdater() {

	// 启动时立即更新一次
	tm._update()

	// 使用定时器，每隔一分钟执行一次
	ticker := time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-ticker.C:
				tm._update()
			case <-tm.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (tm *Manager) _update() {
	for _, sym := range tm.symbols {
		state, err := tm.update(sym)
		if err == nil {
			tm.save(state)
			fmt.Println(state.Description)
		}
		time.Sleep(time.Second * 3)
	}
}

func (tm *Manager) StopUpdater() {
	close(tm.stopCh)
}

func (tm *Manager) update(symbol string) (*TrendState, error) {
	// 拉取 300 根 K 线
	m15Klines, err := tm.ex.GetKlineRecords(symbol, model.Kline_15min, 300, 0, model2.OrderTradeSpot)
	if err != nil {
		log.Printf("[TrendManager] fetch 15min kline error for %s: %v", symbol, err)
		return nil, err
	}
	// 反转为正序
	m15Klines = normalizeCandles(m15Klines, model.Kline_15min, false)

	h1Klines, err := tm.ex.GetKlineRecords(symbol, model.Kline_1h, 300, 0, model2.OrderTradeSpot)
	if err != nil {
		log.Printf("[TrendManager] fetch 1hour kline error for %s: %v", symbol, err)
		return nil, err
	}
	// 反转为正序
	h1Klines = normalizeCandles(h1Klines, model.Kline_1h, false)

	h4Klines, err := tm.ex.GetKlineRecords(symbol, model.Kline_4h, 300, 0, model2.OrderTradeSpot)
	if err != nil {
		log.Printf("[TrendManager] fetch 4hour kline error for %s: %v", symbol, err)
		return nil, err
	}
	// 反转为正序
	h4Klines = normalizeCandles(h4Klines, model.Kline_4h, false)

	//tm.genCSV(symbol, tm.interval, latestFirst)

	// ---- 数据准备 (倒序 -> 正序) ----
	s15 := tm.ScoreForPeriod(m15Klines)
	s1h := tm.ScoreForPeriod(h1Klines)
	s4h := tm.ScoreForPeriod(h4Klines)

	// 加权平均，权重：大周期更重要
	total := 0.5*s4h + 0.3*s1h + 0.2*s15

	closesM15 := make([]float64, len(m15Klines))
	highsM15 := make([]float64, len(m15Klines))
	lowsM15 := make([]float64, len(m15Klines))

	for i, k := range m15Klines {
		closesM15[i] = k.Close
		highsM15[i] = k.High
		lowsM15[i] = k.Low
	}

	last := m15Klines[len(m15Klines)-1]

	// 判定 StrongM15
	adxM15 := talib.Adx(highsM15, lowsM15, closesM15, 14)
	strongM15 := (s15 >= 2.0 && adxM15[len(adxM15)-1] > 30)

	dirStr := "震荡"
	dir := TrendNeutral // 横盘
	if total >= 1.5 {
		dir = TrendUp // 多头
		dirStr = "多头"
	} else if total <= -1.5 {
		dir = TrendDown // 空头
		dirStr = "空头"
	}

	state := TrendState{
		Symbol:    symbol,
		Direction: dir,
		StrongM15: strongM15,
		LastPrice: last.Close,
		Timestamp: last.Timestamp,
	}

	des := fmt.Sprintf("[Trend $:%v 4小时趋势:%v 15分钟强趋势:%v 当前价格:%f]", state.Symbol, dirStr, state.StrongM15, last.Close)
	state.Description = des

	return &state, nil
}

// 计算周期趋势分数 -3 ~ +3
func (tm *Manager) ScoreForPeriod(klines []model2.Kline) float64 {
	if len(klines) < 200 { // 至少200根保证EMA200有效
		return 0
	}

	// 提取收盘价、高、低
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	last := len(closes) - 1
	price := closes[last]

	// === 指标计算 ===
	ema20 := talib.Ema(closes, 20)
	ema50 := talib.Ema(closes, 50)
	ema200 := talib.Ema(closes, 200)
	adx := talib.Adx(highs, lows, closes, 14)
	upper, middle, lower := talib.BBands(closes, 20, 2, 2, 0)

	// 最近值
	ema20Last := ema20[last]
	ema50Last := ema50[last]
	ema200Last := ema200[last]
	adxLast := adx[last]
	bbWidthLast := (upper[last] - lower[last]) / middle[last]

	// 历史均值布林带宽（50根）
	var bbSum float64
	var count int
	for i := last - 50; i < last; i++ {
		if i >= 0 {
			bbSum += (upper[i] - lower[i]) / middle[i]
			count++
		}
	}
	bbWidthAvg := bbSum / float64(count)

	// === 打分逻辑 ===
	score := 0.0

	// 价格 vs 长均线
	if price > ema200Last {
		score += 1
	} else {
		score -= 1
	}

	// 均线多空排列 + 斜率
	slope20 := ema20Last - ema20[last-3] // 简单3根斜率
	slope50 := ema50Last - ema50[last-3]

	if ema20Last > ema50Last && slope20 > 0 && slope50 > 0 {
		score += 1
	} else if ema20Last < ema50Last && slope20 < 0 && slope50 < 0 {
		score -= 1
	}

	// ADX 趋势强度
	if adxLast > 25 {
		score += 1
	} else if adxLast < 20 {
		score -= 0.5
	}

	// 布林带宽收窄 → 横盘
	if bbWidthLast < bbWidthAvg*0.7 {
		score -= 0.5
	}

	// 限制范围
	if score > 3 {
		score = 3
	}
	if score < -3 {
		score = -3
	}

	return score
}

// 更新某币种趋势（内部 & 外部都可调用）
func (tm *Manager) save(state *TrendState) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.states[state.Symbol] = state
}

// 获取某币种趋势
func (tm *Manager) Get(symbol string) (*TrendState, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	st, ok := tm.states[symbol]
	return st, ok
}

//func (tm *Manager) IsTrendOk(symbol, side string) bool {
//	state, ok := tm.Get(symbol)
//	if ok && state.Direction.MatchesSide(model2.OrderSide(side)) {
//		return true
//	}
//	return false
//}
