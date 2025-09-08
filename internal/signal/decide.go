package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"math"
	"strconv"
)

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
	// 1. 如果有持仓，优先考虑平仓或减仓
	if ctx.Pos != nil {
		// 如果信号与持仓方向相反，并且强度足够，则平仓
		if (ctx.Pos.Dir == model.OrderPosSideLong && ctx.Sig.Side == "sell") ||
			(ctx.Pos.Dir == model.OrderPosSideShort && ctx.Sig.Side == "buy") {
			// 在震荡模式下，我们对信号强度的要求更高
			if ctx.Sig.Strength >= 0.7 {
				return ActClose
			}
		}

		// 如果没有相反信号，但盈利达到目标，也应平仓
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
		if uplRatio > 0.10 { // 比如盈利超过10%，在震荡中是很好的平仓点
			return ActClose
		}

		// 如果没有明确信号，但趋势分数已经变为中性，也应减仓
		// 这通常发生在趋势结束，进入震荡时
		if (ctx.Pos.Dir == model.OrderPosSideLong && ctx.Trend.Scores.FinalScore < 0.5) ||
			(ctx.Pos.Dir == model.OrderPosSideShort && ctx.Trend.Scores.FinalScore > -0.5) {
			return ActReduce
		}

		// 止损：即使是小幅亏损，也应严格执行
		if uplRatio < -0.03 { // 比如亏损3%就止损
			return ActClose
		}

		return ActIgnore // 否则，在没有明确信号时保持观望
	}

	// 2. 如果没有仓位，在震荡模式下极少开仓
	// 只有在非常明确的反转信号出现时，才考虑逆势开仓
	if ctx.Sig.IsReversal && ctx.Sig.Strength >= 0.8 {
		// 在这里可以加入额外的确认条件，比如价格触及布林带上下轨
		if (ctx.Sig.Side == "buy" && ctx.Trend.Scores.FinalScore < -0.5) ||
			(ctx.Sig.Side == "sell" && ctx.Trend.Scores.FinalScore > 0.5) {
			return ActOpen
		}
	}

	return ActIgnore
}

// 反转
func (de *DecisionEngine) handleReversal() Action {
	ctx := de.Ctx
	// 1. 如果没有持仓，立即返回 ActIgnore
	// 在反转模式下，我们绝不开新仓，所以没有持仓就什么都不做
	if ctx.Pos == nil {
		return ActIgnore
	}

	// 2. 如果有持仓，立即考虑平仓或减仓
	// 根据仓位方向，判断是否与当前信号或趋势方向相反
	posDir := ctx.Pos.Dir

	// 如果持仓方向与大趋势的最新变化方向相反，果断平仓
	// 例如，你持有多单，但现在趋势分数转为负值，斜率也为负
	if (posDir == model.OrderPosSideLong && ctx.Trend.Scores.FinalScore < 0) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Scores.FinalScore > 0) {
		return ActClose
	}

	// 如果趋势尚未完全反转，但信号已经与持仓方向相反
	// 例如，你持有多单，但15分钟信号已经出现卖出信号
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy") {
		// 如果信号强度足够高，表明反转信号强烈，则平仓
		if ctx.Sig.Strength >= 0.5 {
			return ActClose
		}
	}

	// 3. 如果平仓条件不满足，但趋势动能开始衰减，考虑减仓
	// 这个逻辑可以作为一种软平仓，降低风险
	if (posDir == model.OrderPosSideLong && ctx.Trend.Slope < 0) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Slope > 0) {
		return ActReduce
	}

	// 否则，保持观望
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
		// 且反转信号与持仓方向相反
		if ctx.Pos.Dir == model.OrderPosSideLong && ctx.Sig.Side == "sell" ||
			ctx.Pos.Dir == model.OrderPosSideShort && ctx.Sig.Side == "long" {
			// 在大趋势强劲时，短期反转信号是平仓/减仓的好机会
			// 可以选择激进平仓，也可以选择保守减仓
			return ActReduce // 在这里减仓，锁定部分利润，降低风险
		}
	}

	// 2. 根据信号进行开仓或加仓
	// 注意这里我们不再直接使用 "buy" 或 "sell" 的字符串，而是使用 isDirectional
	if op.isSignalWithTrend(model.OrderSide(ctx.Sig.Side), ctx.Trend.Direction) {
		// 无仓位时开仓
		if ctx.Pos == nil {
			// 获取均线价格
			ema30, ema30OK := ctx.Sig.Values["EMA30"]
			rsiValue, rsiOK := ctx.Sig.Values["RSI"]
			if !ema30OK || rsiOK {
				return ActIgnore
			}
			// RIS低于30被视为超卖，可开多仓或者减仓空头，RSI高于30时被视为超买，可开空仓或者减仓多头

			// 开多时价格不能严重偏离均线 price <= ma30 * 1.01
			// 信号0.5强度中等，且必须有强劲的动量支持
			if op.isMomentumPositive(ctx.Trend.Slope) && ctx.Sig.Strength >= 0.5 {
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
			// 当 FinalScore 开始下降，或 FinalSlope 变为负值时
			if (ctx.Trend.Scores.FinalScore < 1.0 && ctx.Trend.Slope < 0) || // 由多转空时，趋势减弱
				(ctx.Trend.Scores.FinalScore > -1.0 && ctx.Trend.Slope > 0) { // 由空转多时，趋势变强
				return ActReduce
			}

			// --- 高强度信号减仓 ---
			// 当出现高强度信号，但它可能预示着一个顶部或底部时
			if (ctx.Sig.Side == "buy" && ctx.Pos.Dir == "long" && ctx.Sig.Strength >= 0.7) ||
				(ctx.Sig.Side == "sell" && ctx.Pos.Dir == "short" && ctx.Sig.Strength >= 0.7) {

				// 获取 RSI 值
				rsiValue, ok := ctx.Sig.Values["RSI"]
				if ok && (rsiValue > 70 || rsiValue < 30) {
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
		isWeakening := op.isSignalWithTrend(model.OrderSide(ctx.Sig.Side), ctx.Trend.Direction) &&
			!op.isMomentumPositive(ctx.Trend.Slope)
		if isWeakening {
			return ActReduce
		}
	}

	return ActIgnore
}

func RunDecide(ctx *Context) Action {
	// 屏蔽重复信号
	if ctx.Sig.Equal(ctx.LastSig) {
		return ActIgnore
	}
	// ---- 无仓位 → 顺势或逆势开仓 ----
	if ctx.Pos == nil {
		return openPosition(ctx)
	}

	// ---- 有仓位 → 管理仓位 ----
	action := managePosition(ctx)

	// ---- 横盘低吸高抛 ----
	if action == ActIgnore && (ctx.Trend.Direction == trend.TrendNeutral || ctx.Sig.Side == "hold") {
		return handleSideways(ctx)
	}
	return action
}

// 获取趋势方向
func trendDirection(ctx *Context) trend.TrendDirection {
	score := ctx.Trend.Scores.FinalScore
	if score >= 1.0 {
		return trend.TrendUp
	} else if score <= -1.0 {
		return trend.TrendDown
	}
	return trend.TrendNeutral
}

// 横盘低吸高抛逻辑
func handleSideways(ctx *Context) Action {
	price := ctx.Sig.Price
	upper := ctx.Sig.Values["Upper"]
	lower := ctx.Sig.Values["Lower"]
	rsi := ctx.Sig.Values["RSI"]
	strength := ctx.Sig.Strength
	buffer := 0.01 // 1%缓冲

	if ctx.Pos == nil {
		// 无仓位建仓
		if price <= lower*(1+buffer) && rsi < 40 && strength > 0.25 {
			ctx.Sig.Side = "buy"
			return ActOpen
		}
		if price >= upper*(1-buffer) && rsi > 60 && strength > 0.25 {
			ctx.Sig.Side = "sell"
			return ActOpen
		}
	} else {
		// 已有仓位管理
		switch ctx.Pos.Dir {
		case model.OrderPosSideLong:
			if price >= upper*(1-buffer) && rsi > 60 {
				return ActReduce
			}
			if price <= lower*(1+buffer) && rsi < 40 {
				return ActAdd
			}
		case model.OrderPosSideShort:
			if price <= lower*(1+buffer) && rsi < 40 {
				return ActReduce
			}
			if price >= upper*(1-buffer) && rsi > 60 {
				return ActAdd
			}
		}
	}
	return ActIgnore
}

// 无仓位顺势/逆势开仓
//func openPosition(ctx *Context) Action {
//	isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
//
//	// 顺势开仓
//	if isTrending && ctx.Sig.Strength > 0.5 {
//		return ActOpen
//	}
//
//	// 逆势短线反转开仓
//	if !isTrending && ctx.Sig.IsReversal && ctx.Sig.Strength >= 0.7 {
//		return ActOpen
//	}
//
//	scores := ctx.Trend.Scores
//	if ctx.Trend.Direction == trend.TrendUp {
//		if ctx.Trend.Scores.TrendScore > 1.0 && ctx.Sig.Side == "buy" && ctx.Sig.Strength > 0.35 {
//			return ActOpen // 顺势多
//		}
//	}
//	if ctx.Trend.Direction == trend.TrendDown {
//		if scores.TrendScore < -1.0 && ctx.Sig.Side == "sell" && ctx.Sig.Strength > 0.35 {
//			return ActOpen // 顺势空
//		}
//	}
//
//	// 空头排列 顺势做空 由多转空
//	if scores.Score4h >= scores.Score1h && scores.Score1h > scores.Score30m &&
//		ctx.Sig.Side == "sell" && ctx.Sig.Strength > 0.35 {
//		return ActOpen
//	}
//
//	// 多头排列 顺势做多  斜率转正了如果不能突破，也许会在山顶 由空转多
//	if scores.Score4h <= scores.Score1h && scores.Score1h < scores.Score30m &&
//		ctx.Sig.Side == "buy" && ctx.Sig.Strength > 0.35 {
//		return ActOpen
//	}
//
//	return ActIgnore // 没有明确信号
//}

// 开仓
func openPosition(ctx *Context) Action {
	// 确保有明确的信号方向
	if ctx.Sig.Side == "hold" {
		return ActIgnore
	}

	// --- 优先处理最强的信号：高强度顺势开仓 ---
	// 信号方向和趋势方向一致，且信号强度高
	isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
	if isTrending && ctx.Sig.Strength >= 0.7 {
		// 增加斜率确认，确保动量支持开仓
		if (ctx.Trend.Slope > 0 && ctx.Sig.Side == "buy") || (ctx.Trend.Slope < 0 && ctx.Sig.Side == "sell") {
			return ActOpen // 顺势开仓，高强度+动量确认
		}
	}

	// --- 其次处理中等强度的顺势开仓 ---
	if isTrending && ctx.Sig.Strength > 0.35 {
		// 趋势分数的确认，避免在趋势末端开仓
		if (ctx.Trend.Scores.FinalScore > 1.0 && ctx.Sig.Side == "buy") || (ctx.Trend.Scores.FinalScore < -1.0 && ctx.Sig.Side == "sell") {
			// 这里可以加入更严格的条件，例如趋势分数的斜率也为正
			return ActOpen
		}
	}

	// --- 再次处理逆势反转开仓（风险较高，需要更强的信号） ---
	// 信号方向和趋势方向相反，但信号是反转信号
	if !isTrending && ctx.Sig.IsReversal && ctx.Sig.Strength >= 0.7 {
		// 在这里可以加入额外的确认，例如斜率已经开始反转
		if (ctx.Trend.Slope > 0 && ctx.Sig.Side == "buy") || (ctx.Trend.Slope < 0 && ctx.Sig.Side == "sell") {
			return ActOpen
		}
	}

	return ActIgnore
}

func managePosition(ctx *Context) Action {
	// === 第一层：风控管理 ===
	// 止损：亏损达到某个阈值，立即平仓
	// 1. 已经有仓位，先考虑止盈止损
	// 高盈亏保护
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
	if uplRatio > 0.25 {
		return ActClose
	}
	if uplRatio > 0.15 {
		return ActClose
	}

	// === 第二层：趋势与信号管理 ===
	// 1. 趋势反转平仓
	// 假设你有持仓，但给出了相反方向的高强度信号
	isTrendReversal := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
	isSignalReversal := ctx.Sig.Side == "buy" && ctx.Pos.Dir == model.OrderPosSideShort ||
		ctx.Sig.Side == "sell" && ctx.Pos.Dir == model.OrderPosSideLong
	if isTrendReversal || isSignalReversal {
		// 在这里加入更严格的条件，例如信号强度足够高
		if ctx.Sig.Strength > 0.5 {
			return ActClose
		}
	}

	// 2. 趋势延续加仓
	// 只有在趋势和信号都一致，且趋势动量强劲时才考虑加仓
	isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
	isStrongMomentum := (ctx.Trend.Scores.FinalScore > 1.0 && ctx.Trend.Slope > 0) ||
		(ctx.Trend.Scores.FinalScore < -1.0 && ctx.Trend.Slope < 0)

	if isTrending && isStrongMomentum && ctx.Sig.Strength > 0.35 {
		// 在这里加入对仓位数量的限制，防止过度加仓
		return ActAdd
	}

	// 3. 趋势减弱减仓
	// 如果趋势开始减弱，但还没有完全反转，考虑减仓
	isWeakening := (ctx.Trend.Scores.FinalScore > 0 && ctx.Trend.Slope < 0) ||
		(ctx.Trend.Scores.FinalScore < 0 && ctx.Trend.Slope > 0)

	if isWeakening {
		return ActReduce
	}

	posDir := ctx.Pos.Dir

	// 趋势向上
	if ctx.Trend.Direction == trend.TrendUp {
		// 趋势向上，但是给出的信号确是卖， 信号与仓位反向相反，减仓
		if ctx.Sig.Side == "sell" &&
			posDir == model.OrderPosSideLong &&
			ctx.Sig.Strength >= 0.35 { // 多趋势减弱了 减仓 ctx.Trend.Scores.TrendScore < 1
			return ActReduce
		}

		if ctx.Sig.Side == "buy" &&
			posDir == model.OrderPosSideLong &&
			ctx.Sig.Strength >= 0.35 { // 多趋势变强了 加仓 && ctx.Trend.Scores.TrendScore > 1
			return ActAdd
		}
	}

	// 趋势向下
	if ctx.Trend.Direction == trend.TrendDown {
		// 信号与仓位反向相反，减仓
		if ctx.Sig.Side == "buy" &&
			posDir == model.OrderPosSideShort &&
			ctx.Sig.Strength >= 0.35 { // 空趋势变弱了 减空单 && ctx.Trend.Scores.TrendScore > -1
			return ActReduce
		}
		if ctx.Sig.Side == "sell" &&
			posDir == model.OrderPosSideShort &&
			ctx.Sig.Strength >= 0.35 { // 空的趋势变强了 加仓 && ctx.Trend.Scores.TrendScore < -1
			return ActAdd
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
