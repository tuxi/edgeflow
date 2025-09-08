package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"math"
	"strconv"
)

// 趋势减弱检测器
var detector = NewWeakTrendDetector(3, 0.5, 0.01) // 连续3次，FinalScore阈值0.5，Slope阈值0.01

// 决策
type DecisionEngine struct {
	Ctx Context
}

func NewDecisionEngine(ctx Context) *DecisionEngine {
	return &DecisionEngine{Ctx: ctx}
}

func (de *DecisionEngine) Run() Action {
	// 更新并获取当前的趋势方向
	currentTrendDirection := de.Ctx.Trend.Direction

	// 根据趋势方向来决定交易模式
	switch currentTrendDirection {
	case trend.TrendUp:
		return de.handleTrend(getBullishOperator())
	case trend.TrendDown:
		return de.handleTrend(getBearishOperator())
	case trend.TrendReversal:
		return de.handleReversal()
	case trend.TrendNeutral:
		return de.handleNeutral()
	}

	return ActIgnore
}

// 震荡模式
func (de *DecisionEngine) handleNeutral() Action {
	ctx := de.Ctx
	// 1. 如果没有持仓，保持不变，极度保守
	if ctx.Pos == nil {
		// 在震荡模式下，只考虑最强的逆势反转信号开仓
		if ctx.Sig.IsReversal && ctx.Sig.Strength >= 0.8 {
			// 这里可以加入价格触及布林带或重要支撑/阻力位的条件
			return ActOpen
		}
		return ActIgnore
	}

	// 2. 如果有持仓：执行高抛低吸的逻辑

	posDir := ctx.Pos.Dir
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

	// --- 高抛（减仓）逻辑 ---
	// 如果价格达到相对高位，且与你持仓方向相反的信号强度足够，则高抛减仓
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy") {

		// 只有在有显著盈利时才高抛（例如，盈利超过 3%）
		// 并且信号强度要足够高，以确认是高位
		if uplRatio > 0.03 && ctx.Sig.Strength >= 0.7 {
			return ActReduce
		}
	}

	// --- 极度严格的止损逻辑 ---
	// 即使是小幅亏损，也应严格执行
	if uplRatio < -0.03 {
		return ActClose
	}

	// --- 低吸（加仓）逻辑 ---
	// 在价格回调到相对低位，且信号与持仓方向一致时低吸加仓
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "buy") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "sell") {

		// 只有在回调时才低吸，确保价格低于你的平均成本价
		price := ctx.Sig.Price
		avgPrice := ctx.Pos.AvgPrice

		if price < avgPrice && ctx.Sig.Strength >= 0.3 { // 信号强度要求可以放低
			return ActAddSmall // 小幅加仓
		}
	}

	// --- 低吸（加仓）逻辑 ---
	// 在价格回调到相对低位，且信号与持仓方向一致时低吸加仓
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "buy") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "sell") {

		price := ctx.Sig.Price
		avgPrice := ctx.Pos.AvgPrice

		// 只有在价格大幅低于平均成本时才低吸
		if price < avgPrice*0.99 && ctx.Sig.Strength >= 0.5 { // 例如，价格低于平均成本 1%
			return ActAddSmall
		}
	}

	// 否则，保持仓位不变
	return ActIgnore
}

// 反转
func (de *DecisionEngine) handleReversal() Action {
	ctx := de.Ctx
	// 1. 如果没有持仓，立即返回 ActIgnore
	if ctx.Pos == nil {
		return ActIgnore
	}

	posDir := ctx.Pos.Dir
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

	// 2. 紧急情况：如果浮亏严重，立即止损，优先级最高
	if uplRatio < -0.05 {
		return ActClose
	}

	// 3. 根据趋势和信号的严重程度决定平仓或减仓

	// 情况 A：大趋势确认反转，果断平仓
	// 例如，你持有多单，但现在趋势分数转为负值，斜率也为负
	if (posDir == model.OrderPosSideLong && ctx.Trend.Scores.FinalScore < 0) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Scores.FinalScore > 0) {

		if uplRatio >= 0 { // 如果在盈利，立即平仓锁定利润
			return ActClose
		} else { // 如果在亏损，降低止损幅度，快速离场
			if uplRatio < -0.02 {
				return ActClose
			}
		}
	}

	// 情况 B：15分钟信号反转，平仓或减仓
	// 例如，你持有多单，但15分钟信号已经出现卖出信号
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy") {

		// 如果信号强度足够高，并且是盈利状态，则平仓
		if ctx.Sig.Strength >= 0.7 && uplRatio > 0.05 {
			return ActClose
		}

		// 如果信号强度足够高，并且是小幅亏损或盈利，则减仓
		if ctx.Sig.Strength >= 0.5 {
			return ActReduce
		}
	}

	// 4. 如果平仓条件不满足，但趋势动能开始衰竭，考虑减仓
	if (posDir == model.OrderPosSideLong && ctx.Trend.Slope < 0) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Slope > 0) {

		// 只有在盈利时才考虑减仓
		if uplRatio > 0 {
			return ActReduce
		}
	}

	return ActIgnore
}

func (de *DecisionEngine) handleTrend(op TrendOperator) Action {
	ctx := de.Ctx
	// 1. 风险控制
	if ctx.Pos != nil {
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
		if uplRatio < -0.05 {
			return ActClose // 止损
		}
		if uplRatio > 0.25 {
			return ActClose // 止盈
		}
		if uplRatio > 0.15 {
			return ActReduce // 部分止盈
		}

	}

	// --- 新增逻辑：处理大趋势与小信号分歧 ---
	// 如果持有仓位，且15分钟信号给出反转信号
	// 趋势反转时，锁定部分利润，降低风险
	if ctx.Pos != nil && ctx.Sig.IsReversal {
		// 在大趋势强劲时，短期反转信号是平仓/减仓的好机会
		// 可以选择激进平仓，也可以选择保守减仓
		return ActReduce // 在这里减仓，锁定部分利润，降低风险
	}
	// 获取均线价格
	ema30, ema30OK := ctx.Sig.Values["EMA30"]
	rsiValue, rsiOK := ctx.Sig.Values["RSI"]
	if !ema30OK || !rsiOK {
		return ActIgnore
	}
	// 2. 根据信号进行开仓或加仓
	// 注意这里我们不再直接使用 "buy" 或 "sell" 的字符串，而是使用 isDirectional
	if op.isSignalWithTrend(model.OrderSide(ctx.Sig.Side), ctx.Trend.Direction) {
		// 无仓位时开仓
		if ctx.Pos == nil {
			// RIS低于30被视为超卖，可开多仓或者减仓空头，RSI高于30时被视为超买，可开空仓或者减仓多头

			// 开多时价格不能严重偏离均线 price <= ma30 * 1.01
			// 信号0.5强度中等，且必须有强劲的动量支持
			if op.isMomentumPositive(ctx.Trend.Slope) && ctx.Sig.Strength >= 0.45 {
				// 避免在价格严重偏离均线（通常是趋势末期）时追高。
				// rsiValue < 30 超卖
				if ctx.Sig.Side == "buy" && (ctx.Sig.Price <= ema30*1.01 || rsiValue < 30) {
					return ActOpen
				}
				// rsiValue > 70 超买
				if ctx.Sig.Side == "sell" && (ctx.Sig.Price >= ema30*0.99 || rsiValue > 70) {
					return ActOpen
				}

				return ActIgnore
			}
		} else {

			// --- 趋势减弱减仓 ---
			if detector.Check(ctx.Sig.Symbol, ctx.Pos.Dir, ctx.Trend.Scores.FinalScore, ctx.Trend.Slope) {
				return ActReduce
			}

			// --- 高强度信号减仓 ---
			// 当出现高强度信号，但它可能预示着一个顶部或底部时
			if (ctx.Sig.Side == "buy" && ctx.Pos.Dir == "long" && ctx.Sig.Strength >= 0.7) ||
				(ctx.Sig.Side == "sell" && ctx.Pos.Dir == "short" && ctx.Sig.Strength >= 0.7) {

				// 获取 RSI 值
				if rsiValue > 70 || rsiValue < 30 {
					return ActReduce // 在超买或超卖区域减仓
				}
			}

			// 有仓位时加仓
			if ctx.Sig.Strength > 0.35 && op.isMomentumPositive(ctx.Trend.Slope) {
				// 回调补仓
				if !op.isPriceHigher(ctx.Sig.Price, ctx.LastSig.Price) &&
					math.Abs(ctx.LastSig.Price-ctx.Sig.Price)/ctx.LastSig.Price < 0.01 {
					return ActAddSmall
				}
				// 趋势延续加仓
				if op.isPriceHigher(ctx.Sig.Price, ctx.LastSig.Price) && math.Abs(ctx.Trend.Slope) > 0.1 {
					return ActAdd
				}
			}
		}
	}

	// 3. 趋势减弱时减仓
	if ctx.Pos != nil {
		posDir := ctx.Pos.Dir
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

		// --- 优先检查盈利状况，只有盈利时才考虑主动减仓 ---
		if uplRatio > 0.01 { // 只有盈利超过1%才考虑主动减仓
			// --- 条件一：趋势动能显著减弱，且15分钟信号给出反向信号 ---
			// 例如：当前持有长仓，但 FinalScore 已经明显下降，或者 FinalSlope 变为负值，
			// 同时 15分钟信号也出现了卖出信号。

			// 判断看涨趋势是否减弱
			isBullishWeakening := (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell" && (ctx.Trend.Scores.FinalScore < 0.5 || // FinalScore 低于某个阈值
				ctx.Trend.Slope < 0)) // FinalSlope 变为负值

			// 判断看跌趋势是否减弱
			isBearishWeakening := (posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy" && (ctx.Trend.Scores.FinalScore > -0.5 || // FinalScore 高于某个阈值
				ctx.Trend.Slope > 0)) // FinalSlope 变为正值

			if isBullishWeakening || isBearishWeakening {
				return ActReduce
			}

			// --- 条件二：价格超买/超卖，且信号强度较高时减仓 ---
			// 这通常发生在趋势末期，价格短暂冲高/跌低
			if (posDir == model.OrderPosSideLong && rsiValue > 70 && ctx.Sig.Strength >= 0.6) ||
				(posDir == model.OrderPosSideShort && rsiValue < 30 && ctx.Sig.Strength >= 0.6) {
				return ActReduce
			}
		} else if uplRatio < -0.01 { // 如果小幅亏损，且出现明确的反向信号，也考虑减仓
			// 这个条件是为了在亏损时也能及时止损，避免小亏变大亏
			if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell" && ctx.Sig.Strength >= 0.6) ||
				(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy" && ctx.Sig.Strength >= 0.6) {
				return ActReduce
			}
		}

	}

	return ActIgnore
}

// TrendOperator 定义了与方向相关的操作
type TrendOperator struct {
	// 动量的判断函数
	isMomentumPositive func(slope float64) bool
	// 价格比较函数
	isPriceHigher func(p1, p2 float64) bool
	// 信号与趋势方向是否相同
	isSignalWithTrend func(signalSide model.OrderSide, trendDir trend.TrendDirection) bool
}

// 获取看涨 TrendOperator
func getBullishOperator() TrendOperator {
	return TrendOperator{
		isMomentumPositive: func(slope float64) bool { return slope > 0 },
		isPriceHigher:      func(p1, p2 float64) bool { return p1 > p2 },
		isSignalWithTrend: func(signalSide model.OrderSide, trendDir trend.TrendDirection) bool {
			return signalSide == model.Buy && trendDir == trend.TrendUp
		},
	}
}

// 获取看跌信号的操作
func getBearishOperator() TrendOperator {
	return TrendOperator{
		isMomentumPositive: func(slope float64) bool { return slope < 0 },
		isPriceHigher:      func(p1, p2 float64) bool { return p1 < p2 }, // 价格越低，越是“高于”看跌趋势
		isSignalWithTrend: func(signalSide model.OrderSide, trendDir trend.TrendDirection) bool {
			return signalSide == model.Sell && trendDir == trend.TrendDown
		},
	}
}

// 趋势减弱检测器
type WeakTrendDetector struct {
	declineCounts map[string]int // 每个 symbol 对应的衰弱计数
	threshold     int            // 连续次数阈值
	scoreTh       float64        // FinalScore 阈值
	slopeTh       float64        // Slope 阈值
}

// 初始化
func NewWeakTrendDetector(threshold int, scoreTh, slopeTh float64) *WeakTrendDetector {
	return &WeakTrendDetector{
		declineCounts: make(map[string]int),
		threshold:     threshold,
		scoreTh:       scoreTh,
		slopeTh:       slopeTh,
	}
}

// 检查是否趋势减弱
func (w *WeakTrendDetector) Check(symbol string, posDir model.OrderPosSide, finalScore, slope float64) bool {
	count := w.declineCounts[symbol]
	trigger := false

	if posDir == model.OrderPosSideLong {
		if finalScore < w.scoreTh && slope < -w.slopeTh { // 由多转空时，趋势减弱
			count++
		} else {
			count = 0
		}
	} else if posDir == model.OrderPosSideShort {
		if finalScore > -w.scoreTh && slope > w.slopeTh { // 由空转多时，趋势变强
			count++
		} else {
			count = 0
		}
	}

	// 更新缓存
	w.declineCounts[symbol] = count

	// 判断是否触发
	if count >= w.threshold {
		trigger = true
		// 触发一次后清零，避免重复触发
		w.declineCounts[symbol] = 0
	}

	return trigger
}
