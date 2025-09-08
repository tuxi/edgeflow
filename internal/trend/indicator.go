package trend

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
	FastPeriod int
	SlowPeriod int
}

func (e *EMAIndicator) Calculate(klines []model.Kline) model.IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < e.SlowPeriod {
		return model.IndicatorResult{Name: "EMA", Signal: "hold"}
	}

	fastEMA := talib.Ema(closes, e.FastPeriod)
	slowEMA := talib.Ema(closes, e.SlowPeriod)
	ema30EMA := talib.Ema(closes, 30)

	// 取最后一个数值
	fast := fastEMA[len(fastEMA)-1]
	slow := slowEMA[len(slowEMA)-1]
	ema30 := ema30EMA[len(ema30EMA)-1]

	// 策略一：大趋势过滤
	signal := "hold"
	if fast > slow {
		// 当快线从下方上穿慢线时,这通常被视为一个看涨的买入信号，表明短期动量正在增强，可能预示着上涨趋势的开始。
		signal = "buy"
	} else if fast < slow {
		// 当快线从上方下穿慢线（fast < slow）时，则被视为一个看跌的卖出信号。
		signal = "sell"
	}

	// 策略二：均线排列过滤
	// 看涨趋势确认
	//if fast > slow && slow > ema30 {
	//	signal = "buy" // 均线多头排列
	//}
	//
	//// 看跌趋势确认
	//if fast < slow && slow < ema30 {
	//	signal = "sell" // 均线空头排列
	//}
	last := klines[len(klines)-1]

	diff := fast - slow
	strength := diff / last.Close
	return model.IndicatorResult{
		Name:     "EMA",
		Values:   map[string]float64{"fast": fast, "slow": slow, "EMA30": ema30},
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

	// 取最后值
	lastMacd := macd[len(macd)-1]
	lastSignal := signalLine[len(signalLine)-1]
	lastHist := hist[len(hist)-1]

	signal := "hold"
	if lastHist > 0 && lastMacd > lastSignal {
		signal = "buy"
	} else if lastHist < 0 && lastMacd < lastSignal {
		signal = "sell"
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

	// RSI 超买/超卖
	if lastRSI < 25 {
		action, strength = "buy", strength+0.3
	} else if lastRSI > 75 {
		action, strength = "sell", strength+0.3
	}

	// MACD 金叉/死叉
	if lastMACD > lastSignal && lastRSI < 40 {
		action, strength = "buy", strength+0.3
	}
	if lastMACD < lastSignal && lastRSI > 60 {
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

func (a *ADXIndicator) Calculate(klines []model.Kline) model.IndicatorResult {
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))

	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	adxValues := talib.Adx(highs, lows, closes, a.Period)
	lastADX := adxValues[len(adxValues)-1]

	var signal string
	if lastADX > 25 {
		signal = "buy" // 趋势明显，可顺势开仓
	} else {
		signal = "hold" // 趋势不明显，暂时观望
	}

	// ADX 本身就是强度指标，可直接归一化 0~1
	strength := lastADX / 50.0
	if strength > 1.0 {
		strength = 1.0
	}

	return model.IndicatorResult{
		Name:     a.Name,
		Signal:   signal,
		Values:   map[string]float64{"ADX": lastADX},
		Strength: strength,
	}
}
