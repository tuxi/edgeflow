package indicator

import (
	"edgeflow/internal/model"
	"github.com/markcheno/go-talib"
	"math"
)

type Indicator interface {
	Calculate([]model.Kline) model.IndicatorResult
}

// ========== EMA 指标 ==========
// 趋势确认
type EMAIndicator struct {
	FastPeriod  int
	SlowPeriod  int
	TrendPeriod int // 新增：用于多头/空头排列的第三条均线周期
}

func (e *EMAIndicator) Calculate(klines []model.Kline) model.IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < e.SlowPeriod {
		return model.IndicatorResult{Name: "EMA", Signal: "hold"}
	}

	fastEMA := talib.Ema(closes, e.FastPeriod)
	slowEMA := talib.Ema(closes, e.SlowPeriod)
	trendEMA := talib.Ema(closes, e.TrendPeriod) // 使用第三条均线

	fast := fastEMA[len(fastEMA)-1]
	slow := slowEMA[len(slowEMA)-1]
	trend := trendEMA[len(trendEMA)-1]

	// 策略：使用三均线排列确认更强的趋势
	signal := "hold"

	// 看涨趋势确认（多头排列）
	if fast > slow && slow > trend {
		signal = "buy"
	} else if fast < slow && slow < trend {
		// 看跌趋势确认（空头排列）
		signal = "sell"
	} else if fast > slow {
		// 快速上穿慢速，但不是完美排列，给予弱买
		signal = "weak_buy"
	} else if fast < slow {
		// 快速下穿慢速，给予弱卖
		signal = "weak_sell"
	}

	last := klines[len(klines)-1]
	diff := fast - slow
	strength := diff / last.Close
	return model.IndicatorResult{
		Name:     "EMA",
		Values:   map[string]float64{"fast": fast, "slow": slow, "trend": trend},
		Signal:   signal,
		Strength: strength,
	}
}

// ========== MACD 指标 ==========
// 动量确认
type MACDIndicator struct {
	FastPeriod   int
	SlowPeriod   int
	SignalPeriod int
}

func (m *MACDIndicator) Calculate(klines []model.Kline) model.IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < m.SlowPeriod {
		return model.IndicatorResult{Name: "MACD", Signal: "hold"}
	}

	macd, signalLine, hist := talib.Macd(closes, m.FastPeriod, m.SlowPeriod, m.SignalPeriod)

	lastMacd := macd[len(macd)-1]
	lastSignal := signalLine[len(signalLine)-1]
	lastHist := hist[len(hist)-1]

	signal := "hold"
	// MACD 交叉判断
	if lastMacd > lastSignal {
		// 检查MACD是否在零轴上方，作为动量确认
		if lastMacd > 0 {
			signal = "buy"
		} else {
			signal = "weak_buy" // 零轴下方交叉，动量较弱
		}
	} else if lastMacd < lastSignal {
		if lastMacd < 0 {
			signal = "sell"
		} else {
			signal = "weak_sell" // 零轴上方交叉，动量较弱
		}
	}

	return model.IndicatorResult{
		Name:   "MACD",
		Values: map[string]float64{"macd": lastMacd, "signal": lastSignal, "hist": lastHist},
		Signal: signal,
	}
}

// ========== RSI 指标 ==========
// 超买超卖确认
type RSIIndicator struct {
	Period int
	Buy    float64 // 超卖 适合开多或者减仓空头
	Sell   float64 // 超买 适合开空或者减仓空头
}

func (r *RSIIndicator) Calculate(klines []model.Kline) model.IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < r.Period {
		return model.IndicatorResult{Name: "RSI", Signal: "hold"}
	}

	rsiArr := talib.Rsi(closes, r.Period)
	lastRsi := rsiArr[len(rsiArr)-1]

	signal := "hold"
	if lastRsi < r.Buy {
		signal = "buy"
	} else if lastRsi > r.Sell {
		signal = "sell"
	}
	rsiStrength := math.Abs(lastRsi-50) / 50
	return model.IndicatorResult{
		Name:     "RSI",
		Values:   map[string]float64{"RSI": lastRsi},
		Signal:   signal,
		Strength: rsiStrength,
	}
}

// 用来计算反转的指标
type ReversalDetector struct {
	Name string
}

func NewReversalDetector() *ReversalDetector {
	return &ReversalDetector{Name: "ReversalDetector"}
}

func (r *ReversalDetector) Calculate(klines []model.Kline) model.IndicatorResult {
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))

	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	// --- RSI ---
	rsi := talib.Rsi(closes, 14)
	lastRSI := rsi[len(rsi)-1]

	// --- MACD ---
	macd, signal, _ := talib.Macd(closes, 12, 26, 9)
	lastMACD := macd[len(macd)-1]
	lastSignal := signal[len(signal)-1]

	// --- Bollinger Bands ---
	upper, middle, lower := talib.BBands(closes, 20, 2.0, 2.0, 0)
	lastUpper, lastMiddle, lastLower := upper[len(upper)-1], middle[len(middle)-1], lower[len(lower)-1]
	lastPrice := closes[len(closes)-1]

	// --- KDJ ---
	k, d := talib.Stoch(highs, lows, closes, 9, 3, 0, 3, 0)
	lastK := k[len(k)-1]
	lastD := d[len(d)-1]

	// --- 反转判定 + 加权 ---
	action := "hold"
	strength := 0.0

	// RSI 超买/超卖作为反转基准
	if lastRSI < 30 {
		action, strength = "buy", strength+0.3
	} else if lastRSI > 70 {
		action, strength = "sell", strength+0.3
	}

	// MACD 金叉/死叉结合RSI
	if lastMACD > lastSignal && lastRSI < 50 { // MACD金叉发生在RSI中轴下方
		action, strength = "buy", strength+0.3
	}
	if lastMACD < lastSignal && lastRSI > 50 { // MACD死叉发生在RSI中轴上方
		action, strength = "sell", strength+0.3
	}

	// 布林带极端突破
	if lastPrice < lastLower {
		action, strength = "buy", strength+0.2
	} else if lastPrice > lastUpper {
		action, strength = "sell", strength+0.2
	}

	// KDJ 超买超卖
	if lastK < 20 && lastD < 20 {
		action, strength = "buy", strength+0.2
	} else if lastK > 80 && lastD > 80 {
		action, strength = "sell", strength+0.2
	}

	// 限制在 0~1 之间
	if strength > 1.0 {
		strength = 1.0
	}

	return model.IndicatorResult{
		Name:   r.Name,
		Signal: action,
		Values: map[string]float64{
			"RSI":     lastRSI,
			"MACD":    lastMACD,
			"MACDsig": lastSignal,
			"Upper":   lastUpper,
			"Middle":  lastMiddle,
			"Lower":   lastLower,
			"Price":   lastPrice,
			"K":       lastK,
			"D":       lastD,
		},
		Strength: strength,
	}
}

// 趋势强度确认
type ADXIndicator struct {
	Name   string
	Period int
}

func NewADXIndicator() *ADXIndicator {
	return &ADXIndicator{
		Name:   "ADX",
		Period: 14,
	}
}

// ADX 指标应该只返回 强度，而 不返回 Buy/Sell 信号。如果非要返回信号，它应该返回“Trend_Strong”或“Trend_Weak”，并把 DI+ 和 DI− 的值添加到 Values 中，让聚合器根据 DI+ 和 DI− 的交叉来判断方向。
func (a *ADXIndicator) Calculate(klines []model.Kline) model.IndicatorResult {
	highs, lows := extractHighsLows(klines)
	closes := extractCloses(klines)

	if len(klines) < a.Period {
		return model.IndicatorResult{Name: a.Name, Signal: "weak_trend"}
	}

	adxValues := talib.Adx(highs, lows, closes, a.Period)
	diPlusValues := talib.PlusDI(highs, lows, closes, a.Period)
	diMinusValues := talib.MinusDI(highs, lows, closes, a.Period)

	lastADX := adxValues[len(adxValues)-1]
	lastDIPlus := diPlusValues[len(diPlusValues)-1]
	lastDIMinus := diMinusValues[len(diMinusValues)-1]

	var signal string
	// ADX 阈值判断：25 - 强趋势，20 - 震荡/趋势弱
	if lastADX > 25 {
		signal = "strong_trend"
	} else if lastADX < 20 {
		signal = "weak_trend" // 趋势不明显，可能震荡
	} else {
		signal = "hold"
	}

	// ADX 本身就是强度指标，将其归一化 0~1
	strength := lastADX / 50.0
	if strength > 1.0 {
		strength = 1.0
	}

	return model.IndicatorResult{
		Name:     a.Name,
		Signal:   signal,
		Values:   map[string]float64{"ADX": lastADX, "DI+": lastDIPlus, "DI-": lastDIMinus}, // 导出 DI+ 和 DI-
		Strength: strength,
	}
}

func extractCloses(klines []model.Kline) []float64 {
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}
	return closes
}

func extractHighsLows(klines []model.Kline) (highs, lows []float64) {
	highs = make([]float64, len(klines))
	lows = make([]float64, len(klines))
	for i, k := range klines {
		highs[i] = k.High
		lows[i] = k.Low
	}
	return highs, lows
}

// CalcATR 保持不变，用于止损计算
//func CalcATR(klines []model.Kline, period int) float64 {
//	if len(klines) < period+1 {
//		return 0
//	}
//	trs := make([]float64, 0, len(klines)-1)
//	for i := 1; i < len(klines); i++ {
//		high := klines[i].High
//		low := klines[i].Low
//		prevClose := klines[i-1].Close
//		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
//		trs = append(trs, tr)
//	}
//	sum := 0.0
//	start := len(trs) - period
//	if start < 0 {
//		start = 0
//	}
//	for i := start; i < len(trs); i++ {
//		sum += trs[i]
//	}
//	return sum / float64(period)
//}
