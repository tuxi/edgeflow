package indicator

import (
	"edgeflow/internal/model"
	"fmt"
	"github.com/markcheno/go-talib"
	"math"
	"strings"
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

	macdLine, macdSignal, macdHist := talib.Macd(closes, m.FastPeriod, m.SlowPeriod, m.SignalPeriod)

	lastMacd := macdLine[len(macdLine)-1]
	lastSignal := macdSignal[len(macdSignal)-1]
	lastHist := macdHist[len(macdHist)-1]

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

// ===================================
// 反转确认指标 (ReversalConfirmationIndicator)
// ===================================

type ReversalConfirmationIndicator struct{}

// Calculate 整合多种指标的极值信号，生成一个高分确认。
func (r *ReversalConfirmationIndicator) Calculate(rsi, macdLine, macdSignal float64, klines []model.Kline) IndicatorResult {
	// 假设 data 中包含了所有必要的指标最新值
	lastRSI := rsi
	lastMACD := macdLine
	lastSignal := macdSignal

	highs, lows := extractHighsLows(klines)
	closes := extractCloses(klines)

	// 布林带 (BBands)：上轨, 下轨
	// 默认参数 (20, 2, 2, SMA)
	upper, _, lower := talib.BBands(closes, 20, 2, 2, talib.SMA)
	lastUpper := upper[len(upper)-1]
	lastLower := lower[len(lower)-1]
	// 注意：BBANDS的MIDDLE线可以用于EMA等其他指标的基准

	// 随机指标 (KDJ / Stochastics) - K 值, D 值
	// KDJ 默认参数 (9, 3, 3)
	// 注意：Go-TA-Lib 中的 Stoch 对应 KDJ
	kVal, dVal := talib.Stoch(highs, lows, closes, 9, 3, talib.SMA, 3, 0)
	lastK := kVal[len(kVal)-1]
	lastD := dVal[len(dVal)-1]

	lastPrice := closes[len(closes)-1]

	buyCount := 0
	sellCount := 0
	reasons := []string{}
	const maxReversalScore = 2.5 // 最大原始分数（5个点 * 0.5）

	// --- 1. RSI 超买/超卖作为反转基准 (权重 2) ---
	if lastRSI < 30 {
		buyCount += 2
		reasons = append(reasons, "RSI超卖(<30)")
	} else if lastRSI > 70 {
		sellCount += 2
		reasons = append(reasons, "RSI超买(>70)")
	}

	// --- 2. MACD 金叉/死叉结合RSI中轴 (权重 1) ---
	if lastMACD > lastSignal && lastRSI < 50 {
		buyCount++
		reasons = append(reasons, "MACD金叉且RSI<50")
	}
	if lastMACD < lastSignal && lastRSI > 50 {
		sellCount++
		reasons = append(reasons, "MACD死叉且RSI>50")
	}

	// --- 3. 布林带极端突破 (权重 1) ---
	if lastPrice < lastLower {
		buyCount++
		reasons = append(reasons, "布林带下轨突破")
	} else if lastPrice > lastUpper {
		sellCount++
		reasons = append(reasons, "布林带上轨突破")
	}

	// --- 4. KDJ 超买超卖 (权重 1) ---
	if lastK < 20 && lastD < 20 {
		buyCount++
		reasons = append(reasons, "KDJ极度超卖(<20)")
	} else if lastK > 80 && lastD > 80 {
		sellCount++
		reasons = append(reasons, "KDJ极度超买(>80)")
	}

	// 5. 最终得分和依据文本生成
	rawScore := 0.0
	var rationaleText string
	var detailedSignal string = "hold"

	// 只有当至少 3 个点确认同一方向时，才输出分数
	if buyCount >= 3 && sellCount == 0 {
		rawScore = float64(buyCount) * 0.5
		detailedSignal = "strong_reversal_buy"
		rationaleText = fmt.Sprintf("【多重反转确认】买入（%d点）：%s。", buyCount, strings.Join(reasons, ", "))
	} else if sellCount >= 3 && buyCount == 0 {
		rawScore = float64(sellCount) * -0.5
		detailedSignal = "strong_reversal_sell"
		rationaleText = fmt.Sprintf("【多重反转确认】卖出（%d点）：%s。", sellCount, strings.Join(reasons, ", "))
	} else {
		detailedSignal = "hold"
		rationaleText = fmt.Sprintf("反转指标确认不足或方向冲突 (买入点:%d, 卖出点:%d)。", buyCount, sellCount)
	}

	// 确保分数不超过 maxReversalScore
	if math.Abs(rawScore) > maxReversalScore {
		if rawScore > 0 {
			rawScore = maxReversalScore
		} else {
			rawScore = -maxReversalScore
		}
	}

	return IndicatorResult{
		Name: "ReversalConfirm",
		Values: map[string]float64{
			"bb_upper":   lastUpper,
			"bb_lower":   lastLower,
			"k_val":      lastK,
			"d_val":      lastD,
			"last_price": lastPrice,
		},
		Signal:   detailedSignal,
		Strength: clampStrength(rawScore, maxReversalScore),
		Rationale: IndicatorRationale{
			Text:   rationaleText,
			Signal: detailedSignal,
		},
	}
}

// 将绝对分数映射到 0.0 到 1.0 的强度范围，并设置上限。
func clampStrength(rawScore, maxScore float64) float64 {
	strength := math.Abs(rawScore) / maxScore
	return math.Min(strength, 1.0)
}

// GetVolumeConfirmationScore 根据交易量、信号方向和K线方向计算分数
// kline: 当前 K 线数据
// volStrength: vol强度
// isBuySignal: 调度中心当前正在评估的是买入信号 (true) 还是卖出信号 (false)
// 返回值: 交易量对当前信号的贡献分数
func CalculateVolumeConfirmationScores(klines []model.Kline) (buyConfirmationScore, sellConfirmationScore float64, rationale string) {
	n := len(klines)
	period := 20
	if n < period {
		return
	}

	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Vol
	}

	last := klines[len(klines)-1]
	volumeEMA20 := talib.Ema(volumes, period)          // 计算成交量的 20 周期 EMA
	volumeEMA20Last := volumeEMA20[len(volumeEMA20)-1] // 获取最新成交量的EMA
	ratio := last.Vol / volumeEMA20Last
	volMagnitude := getVolMagnitude(ratio) // 获取 VOL 绝对强度 (非负数)

	// 中性情况 (0.9 <= ratio <= 1.0)
	if volMagnitude == 0.0 {
		return 0, 0, "量价中立：成交量处于平均水平，不提供额外方向确认。"
	}

	correctionFactor := model.GetKLinePowerCorrection(last.High, last.Low, last.Open, last.Close)
	// 修正 VOL 强度：将其作为确认/否决的权重
	correctedMagnitude := volMagnitude * correctionFactor
	// --- 阳线/阴线判断 ---
	isBullishKLine := last.Close > last.Open // 基础方向判断不变

	// --- 交易死寂惩罚 (Ratio < 0.9) ---
	if ratio < 0.9 {
		// 对 BUY 和 SELL 都是风险，都应该扣分
		buyConfirmationScore = -volMagnitude  // -1.5 (惩罚)
		sellConfirmationScore = -volMagnitude // -1.5 (惩罚)
		rationale = "流动性风险：成交量极度萎缩（地量），市场交易死寂。强力否决任何买入/卖出信号。"
		return
	}

	if isBullishKLine {
		// 阳线放量：对 BUY 是确认，对 SELL 是抵抗/抛压
		rationale = "价涨量增：阳线放量，趋势健康。"
		if correctionFactor < 1.0 {
			rationale += "（但K线影线较长，存在一定多空争夺。）"
		} else if correctionFactor > 1.0 {
			rationale += "（K线实体饱满，多头力量极强。）"
		}
		buyConfirmationScore = correctedMagnitude
		sellConfirmationScore = -correctedMagnitude

		return
	} else {
		// 阴线放量：对 SELL 是确认，对 BUY 是抛压/否决
		rationale = "价跌量增：阴线放量，存在抛售压力。"
		if correctionFactor < 1.0 {
			rationale += "（但K线影线较长，表明底部存在买盘抵抗。）" // 这里的长下影线正是您想捕获的信号！
		} else if correctionFactor > 1.0 {
			rationale += "（K线实体饱满，空头力量极强。）"
		}
		buyConfirmationScore = -correctedMagnitude
		sellConfirmationScore = correctedMagnitude

		return
	}
}

func getVolMagnitude(ratio float64) float64 {
	if ratio > 1.5 {
		return 2.5
	} else if ratio > 1.2 {
		return 2.0
	} else if ratio > 1.1 {
		return 1.5
	} else if ratio > 1.0 {
		return 1.0
	}
	// 对于 0.9 <= ratio <= 1.0，返回 0.0 (中性，不加分)
	// 对于 ratio < 0.9 (交易死寂)，返回一个惩罚强度，例如 1.5
	if ratio < 0.9 {
		return 1.5 // 惩罚的绝对力度
	}
	return 0.0 // 默认中性
}
