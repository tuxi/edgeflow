package trend

import (
	"edgeflow/internal/exchange"
	model2 "edgeflow/internal/model"
	"fmt"
	"log"
	"math"
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
	m15Klines, err := tm.ex.GetKlineRecords(symbol, model.Kline_15min, 210, 0, model2.OrderTradeSpot)
	if err != nil {
		log.Printf("[TrendManager] fetch 15min kline error for %s: %v", symbol, err)
		return nil, err
	}
	// 反转为正序
	m15Klines = normalizeCandles(m15Klines, model.Kline_15min, false)

	h1Klines, err := tm.ex.GetKlineRecords(symbol, model.Kline_1h, 210, 0, model2.OrderTradeSpot)
	if err != nil {
		log.Printf("[TrendManager] fetch 1hour kline error for %s: %v", symbol, err)
		return nil, err
	}
	// 反转为正序
	h1Klines = normalizeCandles(h1Klines, model.Kline_1h, false)

	h4Klines, err := tm.ex.GetKlineRecords(symbol, model.Kline_4h, 210, 0, model2.OrderTradeSpot)
	if err != nil {
		log.Printf("[TrendManager] fetch 4hour kline error for %s: %v", symbol, err)
		return nil, err
	}
	// 反转为正序
	h4Klines = normalizeCandles(h4Klines, model.Kline_4h, false)

	//tm.genCSV(symbol, tm.interval, latestFirst)

	// ---- 分别计算各周期分数----
	s15 := tm.ScoreForPeriod(m15Klines)
	s1h := tm.ScoreForPeriod(h1Klines)
	s4h := tm.ScoreForPeriod(h4Klines)

	// 加权平均，权重：大周期更重要
	total := tm.WeightedScore(s4h, s1h, s15)

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
	adx := adxM15[len(adxM15)-1]
	strongM15 := (math.Abs(s15) >= 2.0 && adx > 25)

	threshold := 1.0
	dirStr := "震荡"
	dir := TrendNeutral // 横盘
	if total >= threshold {
		dir = TrendUp // 多头
		dirStr = "多头"
	} else if total <= -threshold {
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

	des := fmt.Sprintf("[Trend $%v 4小时趋势:%v 15分钟强趋势:%v 当前价格:%f 时间:%v]", state.Symbol, dirStr, state.StrongM15, last.Close, time.Now())
	state.Description = des

	return &state, nil
}

// 加权总分
func (tm *Manager) WeightedScore(s4h, s1h, s15m float64) float64 {
	return 0.5*s4h + 0.3*s1h + 0.2*s15m
}

// 计算周期趋势分数 -3 ~ +3（方向化 + 抖动抑制）
func (tm *Manager) ScoreForPeriod(klines []model2.Kline) float64 {
	if len(klines) < 210 {
		return 0
	}

	n := len(klines)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	last := n - 2 // 上一根收盘
	if last < 200 {
		last = n - 1
	}
	price := closes[last]

	ema20 := talib.Ema(closes, 20)
	ema50 := talib.Ema(closes, 50)
	ema200 := talib.Ema(closes, 200)
	adx := talib.Adx(highs, lows, closes, 14)
	plusDI := talib.PlusDI(highs, lows, closes, 14)
	minusDI := talib.MinusDI(highs, lows, closes, 14)
	upper, middle, lower := talib.BBands(closes, 20, 2, 2, 0)
	atr := talib.Atr(highs, lows, closes, 14)

	ema20Last, ema50Last, ema200Last := ema20[last], ema50[last], ema200[last]
	adxLast, pdiLast, mdiLast := adx[last], plusDI[last], minusDI[last]

	bbWidth := 0.0
	if middle[last] != 0 {
		bbWidth = (upper[last] - lower[last]) / middle[last]
	}
	start := int(math.Max(0, float64(last-50)))
	var sumW float64
	var cnt int
	for i := start; i < last; i++ {
		if middle[i] != 0 {
			sumW += (upper[i] - lower[i]) / middle[i]
			cnt++
		}
	}
	bbWidthAvg := 0.0
	if cnt > 0 {
		bbWidthAvg = sumW / float64(cnt)
	}

	score := 0.0

	// 均线趋势 + 斜率
	slope20 := ema20Last - ema20[max(0, last-5)]
	slope50 := ema50Last - ema50[max(0, last-5)]
	if ema20Last > ema50Last && slope20 > 0 && slope50 > 0 {
		score += 1.2
	} else if ema20Last < ema50Last && slope20 < 0 && slope50 < 0 {
		score -= 1.2
	}

	// DMI + ADX 方向化
	diDelta := pdiLast - mdiLast
	if adxLast >= 20 {
		if diDelta > 3 {
			score += 1.3
		} else if diDelta < -3 {
			score -= 1.3
		}
	} else if adxLast < 15 {
		score *= 0.7
	}

	// 价格 vs EMA200 （ATR死区）
	if atr[last] > 0 {
		distNorm := (price - ema200Last) / atr[last]
		if distNorm > 0.3 {
			score += 0.7
		} else if distNorm < -0.3 {
			score -= 0.7
		}
	}

	// 布林带扩张
	if bbWidthAvg > 0 {
		if bbWidth > bbWidthAvg*1.05 {
			if price > upper[last] {
				score += 0.8
			} else if price < lower[last] {
				score -= 0.8
			}
		} else if bbWidth < bbWidthAvg*0.80 {
			score *= 0.85
		}
	}

	// 一致性
	upCnt := 0
	win := 8
	for i := max(0, last-win+1); i <= last; i++ {
		if closes[i] > ema20[i] {
			upCnt++
		}
	}
	ratio := float64(upCnt) / float64(win)
	if ratio >= 0.75 {
		score += 0.6
	} else if ratio <= 0.25 {
		score -= 0.6
	}

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
