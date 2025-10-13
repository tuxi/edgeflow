package indicator

import (
	"edgeflow/internal/model"
	"fmt"
	"github.com/markcheno/go-talib"
	"math"
)

type IndicatorRationale struct {
	Text   string `json:"text"`
	Signal string `json:"signal"`
}

// 指标结果
type IndicatorResult struct {
	Name      string
	Values    map[string]float64
	Signal    string             // "buy", "sell", "hold"
	Strength  float64            // 指标强度0～1
	Rationale IndicatorRationale // 指标给出的判断依据文本
}

type Indicator interface {
	Calculate([]model.Kline) IndicatorResult
	GetName() string
}

// ========== EMA 指标 ==========
// 趋势确认
type EMAIndicator struct {
	FastPeriod  int
	SlowPeriod  int
	TrendPeriod int // 新增：用于多头/空头排列的第三条均线周期
}

func (*EMAIndicator) GetName() string {
	return "EMA"
}

func (e *EMAIndicator) Calculate(klines []model.Kline) IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < e.SlowPeriod {
		return IndicatorResult{Name: e.GetName(), Signal: "hold"}
	}

	fastEMA := talib.Ema(closes, e.FastPeriod)
	slowEMA := talib.Ema(closes, e.SlowPeriod)
	trendEMA := talib.Ema(closes, e.TrendPeriod) // 使用第三条均线

	fast := fastEMA[len(fastEMA)-1]
	slow := slowEMA[len(slowEMA)-1]
	trend := trendEMA[len(trendEMA)-1]

	// 策略：使用三均线排列确认更强的趋势
	signal := "hold"
	var rationale string
	//var score float64

	if fast > slow && slow > trend {
		// 看涨趋势确认（多头排列）
		signal = "buy"
		//score = 2.0
		rationale = fmt.Sprintf("EMA 三均线多头排列 (Fast: %.2f > Slow: %.2f > Trend: %.2f)，趋势强劲看涨。", fast, slow, trend)
	} else if fast < slow && slow < trend {
		// 看跌趋势确认（空头排列）
		signal = "sell"
		//score = -2.0
		rationale = fmt.Sprintf("EMA 三均线空头排列 (Fast: %.2f < Slow: %.2f < Trend: %.2f)，趋势强劲看跌。", fast, slow, trend)
	} else if fast > slow {
		// 快速上穿慢速, (金叉，弱买)，但不是完美排列，给予弱买
		signal = "weak_buy"
		//score = 1.0
		rationale = fmt.Sprintf("EMA 金叉: 快速线 (%.2f) 上穿慢速线 (%.2f)，趋势可能转为多头。", fast, slow)
	} else if fast < slow {
		// 快速下穿慢速，（死叉，弱卖）给予弱卖
		signal = "weak_sell"
		//score = -1.0
		rationale = fmt.Sprintf("EMA 死叉: 快速线 (%.2f) 下穿慢速线 (%.2f)，趋势可能转为空头。", fast, slow)
	} else {
		//score = 0
		rationale = "EMA 均线纠缠，排列不明确，市场处于盘整或震荡阶段。"
	}

	last := klines[len(klines)-1]
	diff := fast - slow
	strength := diff / last.Close
	return IndicatorResult{
		Name:     e.GetName(),
		Values:   map[string]float64{"ema_fast": fast, "ema_slow": slow, "ema_trend": trend},
		Signal:   signal,
		Strength: strength,
		Rationale: IndicatorRationale{
			Text:   rationale,
			Signal: signal,
		},
	}
}

// ========== MACD 指标 ==========
// 动量确认
type MACDIndicator struct {
	FastPeriod   int
	SlowPeriod   int
	SignalPeriod int
}

func (*MACDIndicator) GetName() string {
	return "MACD"
}

func (m *MACDIndicator) Calculate(klines []model.Kline) IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < m.SlowPeriod {
		return IndicatorResult{Name: "MACD", Signal: "hold"}
	}

	macd, signalLine, hist := talib.Macd(closes, m.FastPeriod, m.SlowPeriod, m.SignalPeriod)

	lastMacd := macd[len(macd)-1]
	lastSignal := signalLine[len(signalLine)-1]
	lastHist := hist[len(hist)-1]

	signal := "hold"
	var rationale string
	//var score float64
	// MACD 交叉判断
	// 金叉 (MACD > Signal)
	if lastMacd > lastSignal {
		// 检查MACD是否在零轴上方，作为动量确认
		if lastMacd > 0 {
			// 强买：零轴上方金叉，动能强劲
			signal = "buy"
			//score = 2.0
			rationale = fmt.Sprintf("MACD 金叉确认 (Line: %.4f > Signal: %.4f)，且在零轴上方，多头动能强劲。", lastMacd, lastSignal)
		} else {
			// 弱买：零轴下方金叉，可能只是短暂反弹
			signal = "weak_buy" // 零轴下方交叉，动量较弱
			//score = 1.0
			rationale = fmt.Sprintf("MACD 形成金叉 (Line: %.4f > Signal: %.4f)，但仍处于零轴下方，动能较弱，需警惕。", lastMacd, lastSignal)
		}
	} else if lastMacd < lastSignal {
		// 死叉 (MACD < Signal)
		if lastMacd < 0 {
			// 强卖：零轴下方死叉，空头趋势确认
			signal = "sell"
			//score = -2.0
			rationale = fmt.Sprintf("MACD 死叉确认 (Line: %.4f < Signal: %.4f)，且在零轴下方，空头动能强劲。", lastMacd, lastSignal)
		} else {
			signal = "weak_sell" // 零轴上方交叉，动量较弱
			// 弱卖：零轴上方死叉，可能只是回调
			//score = -1.0
			rationale = fmt.Sprintf("MACD 形成死叉 (Line: %.4f < Signal: %.4f)，但仍处于零轴上方，需确认趋势反转。", lastMacd, lastSignal)
		}
	} else {
		// 3. 粘合 (MACD == Signal)
		//score = 0.0
		rationale = "MACD 线与信号线粘合，市场处于震荡或方向选择阶段。"
	}

	return IndicatorResult{
		Name:   m.GetName(),
		Values: map[string]float64{"macd": lastMacd, "macd_signal": lastSignal, "macd_hist": lastHist},
		Signal: signal,
		Rationale: IndicatorRationale{
			Text:   rationale,
			Signal: signal,
		},
	}
}

// ========== RSI 指标 ==========
// 超买超卖确认
type RSIIndicator struct {
	Period int
	Buy    float64 // 超卖 适合开多或者减仓空头
	Sell   float64 // 超买 适合开空或者减仓空头
}

func (*RSIIndicator) GetName() string {
	return "RSI"
}

func (r *RSIIndicator) Calculate(klines []model.Kline) IndicatorResult {
	closes := extractCloses(klines)
	if len(closes) < r.Period {
		return IndicatorResult{Name: r.GetName(), Signal: "hold"}
	}

	rsiArr := talib.Rsi(closes, r.Period)
	lastRsi := rsiArr[len(rsiArr)-1]

	signal := "hold"
	//var score float64
	var rationale string

	// rsi 超买 超卖
	if lastRsi < r.Buy {
		// 极度超卖，看多得分高
		//score = 2.0
		signal = "buy"
		rationale = fmt.Sprintf("RSI (%.2f) 极度超卖，强烈看多信号。", lastRsi)
	} else if lastRsi > r.Sell {
		// 极度超买，看空得分高
		signal = "sell"
		//score = -2.0
		rationale = fmt.Sprintf("RSI (%.2f) 极度超买，强烈看空信号。", lastRsi)
	} else if lastRsi >= 40 && lastRsi <= 60 {
		// 中性偏多，提供微弱支持
		signal = "hold"
		//score = 0.5
		rationale = fmt.Sprintf("RSI (%.2f) 处于中性区域，无明显方向。", lastRsi)
	} else {
		//score = 0.0
		rationale = fmt.Sprintf("RSI (%.2f) 处于中等区域。", lastRsi)
	}

	rsiStrength := math.Abs(lastRsi-50) / 50
	return IndicatorResult{
		Name:     r.GetName(),
		Values:   map[string]float64{"rsi": lastRsi},
		Signal:   signal,
		Strength: rsiStrength,
		Rationale: IndicatorRationale{
			Text:   rationale,
			Signal: signal,
		},
	}
}

// 用来计算反转的指标
type ReversalDetector struct {
}

func NewReversalDetector() *ReversalDetector {
	return &ReversalDetector{}
}

func (*ReversalDetector) GetName() string {
	return "ReversalDetector"
}

func (r *ReversalDetector) Calculate(klines []model.Kline) IndicatorResult {
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

	return IndicatorResult{
		Name:   r.GetName(),
		Signal: action,
		Values: map[string]float64{
			"rsi":        lastRSI,
			"macd":       lastMACD,
			"macd_sig":   lastSignal,
			"upper":      lastUpper,
			"middle":     lastMiddle,
			"lower":      lastLower,
			"last_price": lastPrice,
			"k_val":      lastK,
			"d_val":      lastD,
		},
		Strength: strength,
	}
}

// 趋势强度确认
type ADXIndicator struct {
	Name   string
	Period int
}

func (i *ADXIndicator) GetName() string {
	return i.Name
}

func NewADXIndicator() *ADXIndicator {
	return &ADXIndicator{
		Name:   "ADX",
		Period: 14,
	}
}

// ADX 指标应该只返回 强度，而 不返回 Buy/Sell 信号。如果非要返回信号，它应该返回“Trend_Strong”或“Trend_Weak”，并把 DI+ 和 DI− 的值添加到 Values 中，让聚合器根据 DI+ 和 DI− 的交叉来判断方向。
func (a *ADXIndicator) Calculate(klines []model.Kline) IndicatorResult {
	highs, lows := extractHighsLows(klines)
	closes := extractCloses(klines)

	if len(klines) < a.Period {
		return IndicatorResult{Name: a.Name, Signal: "weak_trend"}
	}

	adxValues := talib.Adx(highs, lows, closes, a.Period)
	diPlusValues := talib.PlusDI(highs, lows, closes, a.Period)
	diMinusValues := talib.MinusDI(highs, lows, closes, a.Period)

	lastADX := adxValues[len(adxValues)-1]
	lastDIPlus := diPlusValues[len(diPlusValues)-1]
	lastDIMinus := diMinusValues[len(diMinusValues)-1]

	var signal string
	//var score float64
	var rationale string

	// ADX 阈值设置
	const strongThreshold = 25.0 // 强趋势阈值
	const weakThreshold = 20.0   // 弱趋势/震荡阈值

	// 趋势方向判断 (DI+ vs DI-)
	isBullish := lastDIPlus > lastDIMinus

	// 2. 趋势强度和得分判断
	if lastADX > strongThreshold {
		signal = "strong_trend" // 不论空还是多，adx只作为动能，不判断方向
		// 强趋势
		if isBullish {
			//score = 2.0 // 强劲多头趋势
			rationale = fmt.Sprintf("ADX (%.2f) > %.0f 确认趋势强劲，且 DI+ (%.2f) 领先 DI- (%.2f)，多头趋势确认。",
				lastADX, strongThreshold, lastDIPlus, lastDIMinus)
		} else {
			//score = -2.0 // 强劲空头趋势
			rationale = fmt.Sprintf("ADX (%.2f) > %.0f 确认趋势强劲，且 DI- (%.2f) 领先 DI+ (%.2f)，空头趋势确认。",
				lastADX, strongThreshold, lastDIMinus, lastDIPlus)
		}
	} else if lastADX > weakThreshold {
		// 中等趋势，可能震荡
		signal = "weak_trend"
		if isBullish {
			//score = 1.0 // 中等多头趋势
			rationale = fmt.Sprintf("ADX (%.2f) 处于中等强度，但 DI+ 领先，多头动能占据优势。", lastADX)
		} else {
			//score = -1.0 // 中等空头趋势
			rationale = fmt.Sprintf("ADX (%.2f) 处于中等强度，但 DI- 领先，空头动能占据优势。", lastADX)
		}
	} else {
		// 弱趋势或盘整
		//score = 0.0
		signal = "hold"
		rationale = fmt.Sprintf("ADX (%.2f) < %.0f，趋势强度极低，市场可能处于盘整或震荡状态。", lastADX, weakThreshold)
	}

	// ADX 本身就是强度指标
	// 强度计算 (使用 ADX/50.0 归一化)
	strength := lastADX / 50.0
	strength = math.Min(strength, 1.0) // 确保强度不超过 1.0

	return IndicatorResult{
		Name:     a.Name,
		Signal:   signal,
		Values:   map[string]float64{"adx": lastADX, "di+": lastDIPlus, "di-": lastDIMinus}, // 导出 DI+ 和 DI-
		Strength: strength,
		Rationale: IndicatorRationale{
			Text:   rationale,
			Signal: signal,
		},
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
