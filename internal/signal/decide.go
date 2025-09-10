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
			return ActAdd // 小幅加仓
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
			return ActAdd
		}
	}

	// 否则，保持仓位不变
	return ActIgnore
}

// 趋势反转
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
	// 1. 风险控制（先止损，再部分止盈，再大止盈 ）
	if ctx.Pos != nil {
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

		// 更严格的止损：合约交易3-5%止损更合理
		if uplRatio < -0.05 { // 从-0.10改为-0.05
			return ActClose
		}

		// 更早的止盈：避免盈利回吐
		if uplRatio > 0.15 { // 从0.30改为0.15
			return ActClose
		}

		// 更早的部分止盈
		if uplRatio > 0.08 { // 从0.18改为0.08
			return ActReduce
		}

		// 更严格的反转信号处理
		if ctx.Sig.IsReversal && ctx.Sig.ReversalStrength >= 0.5 {
			// 简化条件：有盈利就减仓，亏损就止损
			if uplRatio > 0.01 {
				return ActReduce
			} else if uplRatio < -0.02 {
				return ActClose
			}
		}
	}

	// 获取技术指标
	ema30, ema30OK := ctx.Sig.Values["EMA30"]
	rsiValue, rsiOK := ctx.Sig.Values["RSI"]
	if !ema30OK || !rsiOK {
		return ActIgnore
	}

	// 2. 更严格的开仓条件
	if op.isSignalWithTrend(model.OrderSide(ctx.Sig.Side), ctx.Trend.Direction) {
		if ctx.Pos == nil {
			// 提高开仓信号强度要求，缩小区间
			if ctx.Sig.Strength >= 0.4 && ctx.Sig.Strength <= 0.7 && op.isMomentumPositive(ctx.Trend.Slope) {
				// 更严格的价格位置要求
				if ctx.Sig.Side == "buy" {
					// 多头：价格要明显低于均线或RSI超卖
					if ctx.Sig.Price <= ema30*0.995 || rsiValue < 35 {
						return ActOpen
					}
				}
				if ctx.Sig.Side == "sell" {
					// 空头：价格要明显高于均线或RSI超买
					if ctx.Sig.Price >= ema30*1.005 || rsiValue > 65 {
						return ActOpen
					}
				}
			}
		} else {
			// 3. 更保守的加仓逻辑
			uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

			// 只在盈利状态下考虑加仓
			if uplRatio > 0.01 {
				// 提高加仓信号强度要求
				if ctx.Sig.Strength > 0.5 && op.isMomentumPositive(ctx.Trend.Slope) {
					// 回调加仓：要求更明显的回调
					if ctx.LastSig != nil && ctx.LastSig.Price > 0 {
						priceChangeRatio := math.Abs(ctx.LastSig.Price-ctx.Sig.Price) / ctx.LastSig.Price

						// 回调幅度要在0.5%-2%之间（避免噪音和大跌）
						if priceChangeRatio >= 0.005 && priceChangeRatio <= 0.02 {
							if !op.isPriceHigher(ctx.Sig.Price, ctx.LastSig.Price) {
								return ActAdd
							}
						}
					}
				}
			}
		}
	}

	// 4. 更激进的反向信号处理
	if ctx.Pos != nil {
		posDir := ctx.Pos.Dir
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

		// 降低反向信号的处理阈值
		if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell" && ctx.Sig.Strength >= 0.5) ||
			(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy" && ctx.Sig.Strength >= 0.5) {

			// 有盈利就减仓，亏损就考虑止损
			if uplRatio > 0 {
				return ActReduce
			} else if uplRatio < -0.02 { // 小幅亏损就考虑止损
				return ActClose
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
