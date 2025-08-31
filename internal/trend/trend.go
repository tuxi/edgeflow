package trend

import (
	"edgeflow/internal/exchange"
	model2 "edgeflow/internal/model"
	"fmt"
	"log"
	"math"
	"strings"
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
	s15, _ := tm.ScoreForPeriod(m15Klines)
	s1h, _ := tm.ScoreForPeriod(h1Klines)
	s4h, _ := tm.ScoreForPeriod(h4Klines)
	//fmt.Printf("%v 4小时：%v\n", symbol, s4hReasons)
	//fmt.Printf("%v 1小时：%v\n", symbol, s1hReasons)
	//fmt.Printf("%v 15分钟：%v\n", symbol, s15Reasons)

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
	strongM15 := math.Abs(s15) >= 1.5 && adx > 20

	// --- 趋势方向判定 ---
	dir := TrendNeutral // 横盘
	dirStr := "震荡"
	if total >= 1.0 {
		dir = TrendUp
		dirStr = "多头"
	} else if total <= -1.0 {
		dir = TrendDown
		dirStr = "空头"
	} else if strongM15 {
		// 如果短周期强势但总分未到大周期阈值，则按短周期方向开仓
		if s15 > 0 {
			dir = TrendUp
			dirStr = "多头"
		} else {
			dir = TrendDown
			dirStr = "空头"
		}
	} else {
		dir = TrendNeutral
	}

	//threshold := 1.0
	//dirStr := "震荡"

	//if total >= threshold {
	//	dir = TrendUp // 多头
	//	dirStr = "多头"
	//} else if total <= -threshold {
	//	dir = TrendDown // 空头
	//	dirStr = "空头"
	//}

	state := TrendState{
		Symbol:    symbol,
		Direction: dir,
		StrongM15: strongM15,

		LastPrice: last.Close,
		Timestamp: last.Timestamp,
	}

	des := fmt.Sprintf("[Trend $%v 4小时趋势:%v 15分钟强趋势:%v score:%f, 当前价格:%f 时间:%v]", state.Symbol, dirStr, state.StrongM15, total, last.Close, time.Now())
	state.Description = des

	return &state, nil
}

// 加权总分
func (tm *Manager) WeightedScore(s4h, s1h, s15m float64) float64 {
	//return 0.5*s4h + 0.3*s1h + 0.2*s15m
	return 0.4*s4h + 0.3*s1h + 0.3*s15m
}

// 计算周期趋势分数 -3 ~ +3（方向化 + 抖动抑制）
func (tm *Manager) ScoreForPeriod(klines []model2.Kline) (float64, string) {
	if len(klines) < 210 {
		return 0, ""
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

	last := len(closes) - 1
	price := closes[last]

	ema20 := talib.Ema(closes, 20)
	ema50 := talib.Ema(closes, 50)
	ema200 := talib.Ema(closes, 200)
	adx := talib.Adx(highs, lows, closes, 14)
	upper, middle, lower := talib.BBands(closes, 20, 2, 2, 0)
	kVals, dVals := talib.Stoch(highs, lows, closes, 9, 3, talib.SMA, 3, talib.SMA)

	ema20Last := ema20[last]
	ema50Last := ema50[last]
	ema200Last := ema200[last]
	adxLast := adx[last]
	bbWidthLast := (upper[last] - lower[last]) / middle[last]

	var bbSum float64
	var count int
	for i := last - 50; i < last; i++ {
		if i >= 0 {
			bbSum += (upper[i] - lower[i]) / middle[i]
			count++
		}
	}
	bbWidthAvg := bbSum / float64(count)

	// === 打分 ===
	score := 0.0
	reasons := []string{}

	// 价格 vs EMA200
	if price > ema200Last {
		score += 1
		reasons = append(reasons, "+1(价格>EMA200)")
	} else {
		score -= 1
		reasons = append(reasons, "-1(价格<EMA200)")
	}

	// 均线排列 + 斜率
	slope20 := ema20Last - ema20[last-3]
	slope50 := ema50Last - ema50[last-3]
	if ema20Last > ema50Last && slope20 > 0 && slope50 > 0 {
		score += 1
		reasons = append(reasons, "+1(EMA20>EMA50且向上)")
	} else if ema20Last < ema50Last && slope20 < 0 && slope50 < 0 {
		score -= 1
		reasons = append(reasons, "-1(EMA20<EMA50且向下)")
	}

	// ADX趋势强度
	//if adxLast > 25 {
	//	score += 1
	//	reasons = append(reasons, "+1(ADX强趋势)")
	//} else if adxLast < 20 {
	//	score -= 0.5
	//	reasons = append(reasons, "-0.5(ADX弱趋势)")
	//}
	// ADX 趋势强度
	if adxLast > 25 {
		if score > 0 {
			score += 1 // 多头趋势加强
			reasons = append(reasons, "+1(ADX强趋势)")
		} else if score < 0 {
			score -= 1 // 空头趋势加强
			reasons = append(reasons, "+1(ADX弱趋势)")
		} else {
			// 横盘也可以略微增加权重
			score += 0.5
			reasons = append(reasons, "+0.5(ADX强在横盘时略微增加权重)")
		}
	} else if adxLast < 20 {
		score -= 0.5
		reasons = append(reasons, "-0.5(ADX弱减弱趋势)")
	}
	// 布林带收窄 → 横盘
	if bbWidthLast < bbWidthAvg*0.7 {
		score -= 0.5
		reasons = append(reasons, "-0.5(布林收窄横盘)")
	}
	// kdj 金叉/死叉
	kVals, dVals, jVals := CalcKDJ(highs, lows, closes)
	if IsGoldenCross(kVals, dVals) {
		score += 0.5 // 金叉看多
		reasons = append(reasons, "+0.5(KDJ金叉)")
	}
	if IsDeadCross(kVals, dVals) {
		score -= 0.5 // 死叉看空
		reasons = append(reasons, "+0.5(KDJ金叉)")
	}

	//  J 值极端情况 ===
	jNow := jVals[last]
	if jNow > 100 {
		score -= 0.5 // 极端超买，防止追高
		reasons = append(reasons, "-0.5(J>100超买)")
	}
	if jNow < 0 {
		score += 0.5 // 极端超卖，防止错过反弹
		reasons = append(reasons, "+0.5(J<0超卖)")
	}

	// MACD 参数 (12, 26, 9) 常见用法
	macdVals, _, _ := talib.Macd(closes, 12, 26, 9)
	// MACD 背离
	divScore, divReason := CheckMacdDivergence(closes, macdVals, 30)
	score += divScore
	if divReason != "" {
		reasons = append(reasons, divReason)
	}

	// RSI 背离
	rsiVals := talib.Rsi(closes, 14)
	rsiDivScore, rsiDivReason := CheckRsiDivergence(closes, rsiVals, 30)
	score += rsiDivScore
	if rsiDivReason != "" {
		reasons = append(reasons, rsiDivReason)
	}

	if score > 3 {
		score = 3
	}
	if score < -3 {
		score = -3
	}
	// === 调试日志 ===
	logs := fmt.Sprintf("Score=%.2f 详情: %v", score, strings.Join(reasons, ", "))
	return score, logs
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

// 计算KDJ
func CalcKDJ(highs, lows, closes []float64) ([]float64, []float64, []float64) {
	// 使用talib自带的随机指标 (Stochastic Oscillator)
	k, d := talib.Stoch(
		highs,
		lows,
		closes,
		9, // n
		3, // k smoothing
		talib.SMA,
		3, // d smoothing
		talib.SMA,
	)

	j := make([]float64, len(k))
	for i := range k {
		j[i] = 3*k[i] - 2*d[i]
	}
	return k, d, j
}

// 判断是否金叉
func IsGoldenCross(k, d []float64) bool {
	n := len(k)
	if n < 2 {
		return false
	}
	return k[n-2] < d[n-2] && k[n-1] > d[n-1]
}

// 判断是否死叉
func IsDeadCross(k, d []float64) bool {
	n := len(k)
	if n < 2 {
		return false
	}
	return k[n-2] > d[n-2] && k[n-1] < d[n-1]
}

// CheckMacdDivergence 检测 MACD 顶背离/底背离
// 返回 (score, reason)
// 底背离 → +1.0, 顶背离 → -1.0, 否则 0
func CheckMacdDivergence(closes, macdVals []float64, lookback int) (float64, string) {
	last := len(closes) - 1
	if len(closes) < lookback+5 || len(macdVals) < lookback+5 {
		return 0, ""
	}

	// 找最近 lookback 区间内价格高低点
	priceHigh, priceHighIdx := closes[last], last
	priceLow, priceLowIdx := closes[last], last
	for i := last - lookback; i < last; i++ {
		if closes[i] > priceHigh {
			priceHigh = closes[i]
			priceHighIdx = i
		}
		if closes[i] < priceLow {
			priceLow = closes[i]
			priceLowIdx = i
		}
	}

	// 找最近 lookback 区间内 MACD 高低点
	macdHigh, macdHighIdx := macdVals[last], last
	macdLow, macdLowIdx := macdVals[last], last
	for i := last - lookback; i < last; i++ {
		if macdVals[i] > macdHigh {
			macdHigh = macdVals[i]
			macdHighIdx = i
		}
		if macdVals[i] < macdLow {
			macdLow = macdVals[i]
			macdLowIdx = i
		}
	}

	// === 背离判断 ===
	// 底背离：价格创新低，但 MACD 没创新低
	if priceLowIdx > macdLowIdx &&
		closes[priceLowIdx] < closes[macdLowIdx] &&
		macdVals[priceLowIdx] > macdVals[macdLowIdx] {
		return 1.0, "+1.0(MACD底背离)"
	}

	// 顶背离：价格创新高，但 MACD 没创新高
	if priceHighIdx > macdHighIdx &&
		closes[priceHighIdx] > closes[macdHighIdx] &&
		macdVals[priceHighIdx] < macdVals[macdHighIdx] {
		return -1.0, "-1.0(MACD顶背离)"
	}

	return 0, ""
}

// CheckRsiDivergence 检测 RSI 顶背离/底背离
// 返回 (score, reason)
// 底背离 → +0.5, 顶背离 → -0.5, 否则 0
func CheckRsiDivergence(closes, rsiVals []float64, lookback int) (float64, string) {
	last := len(closes) - 1
	if len(closes) < lookback+5 || len(rsiVals) < lookback+5 {
		return 0, ""
	}

	// 找最近 lookback 区间内价格高低点
	priceHigh, priceHighIdx := closes[last], last
	priceLow, priceLowIdx := closes[last], last
	for i := last - lookback; i < last; i++ {
		if closes[i] > priceHigh {
			priceHigh = closes[i]
			priceHighIdx = i
		}
		if closes[i] < priceLow {
			priceLow = closes[i]
			priceLowIdx = i
		}
	}

	// 找最近 lookback 区间内 RSI 高低点
	rsiHigh, rsiHighIdx := rsiVals[last], last
	rsiLow, rsiLowIdx := rsiVals[last], last
	for i := last - lookback; i < last; i++ {
		if rsiVals[i] > rsiHigh {
			rsiHigh = rsiVals[i]
			rsiHighIdx = i
		}
		if rsiVals[i] < rsiLow {
			rsiLow = rsiVals[i]
			rsiLowIdx = i
		}
	}

	// === 背离判断 ===
	// 底背离：价格创新低，但 RSI 没创新低
	if priceLowIdx > rsiLowIdx &&
		closes[priceLowIdx] < closes[rsiLowIdx] &&
		rsiVals[priceLowIdx] > rsiVals[rsiLowIdx] {
		return 0.5, "+0.5(RSI底背离)"
	}

	// 顶背离：价格创新高，但 RSI 没创新高
	if priceHighIdx > rsiHighIdx &&
		closes[priceHighIdx] > closes[rsiHighIdx] &&
		rsiVals[priceHighIdx] < rsiVals[rsiHighIdx] {
		return -0.5, "-0.5(RSI顶背离)"
	}

	return 0, ""
}
