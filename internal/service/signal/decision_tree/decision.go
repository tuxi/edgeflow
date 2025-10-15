package decision_tree

import (
	"edgeflow/internal/service/signal/model"
	"fmt"
	"math"
)

// DecisionTree 封装了决策所需的阈值和方法
type DecisionTree struct {
	OpenThreshold  float64 // 核心阈值：定义开仓所需的最低 FinalScore
	CloseThreshold float64 // 平仓所需的最低/最高 FinalScore
}

func NewDecisionTree(openThreshold, closeThreshold float64) *DecisionTree {
	return &DecisionTree{
		OpenThreshold:  openThreshold,
		CloseThreshold: closeThreshold,
	}
}

// ApplyFilter 是信号的最终决策层，它根据信号类型应用不同的业务过滤规则。
//
// 传入参数：
// rawSignal: 由 SignalGenerator 生成的原始信号数据
// trendState: 信号发生时的多周期趋势快照数据
//
// 返回值：
// passed: 信号是否通过过滤 (bool)
// reason: 未通过的原因或通过的确认信息 (string)
func (dt *DecisionTree) ApplyFilter(
	rawSignal *model.Signal,
	trendState *model.TrendState,
) (passed bool, reason string) {

	// 1. 检查信号的基本合法性，确保 Command 不是空值
	if rawSignal.Command == "" {
		return false, "Command is empty, signal rejected."
	}

	// 2. 根据 Command 类型执行不同的过滤策略
	switch rawSignal.Command {

	case model.CommandBuy, model.CommandSell:
		// --- 【趋势跟随信号】过滤：要求趋势必须与信号方向一致 ---

		// 动能抑制检查：如果 ADX 低于 20.0，判定为横盘/震荡，避免假信号、拒绝趋势信号
		const MAX_ADX_FOR_TREND_ENTRY = 20.0
		adx15m := rawSignal.Details.HighFreqIndicators["adx"]
		if adx15m < MAX_ADX_FOR_TREND_ENTRY {
			return false, fmt.Sprintf("Trend Follow REJECTED: ADX (%.2f) is too low. Market is consolidating.", adx15m)
		}

		// 我们不允许在强烈逆势中跟随趋势（例如，4H 趋势强烈看跌，但 15M 给了 BUY 信号）
		const MAX_COUNTER_TREND_SCORE = -2.0 // 4H 周期允许的最大逆势分数

		// 信号爆炸性分数门槛：超过此分数（例如 4.0，最大5），则允许覆盖 4H 周期惯性。
		const EXPLOSIVE_SCORE_OVERRIDE = 4.0
		// 检查 4H 周期是否过于强势地逆向
		isOverridden := false

		// 2.1. 检查 4H 周期是否过于强势地逆向
		if rawSignal.Command == model.CommandBuy {
			// 检查是否达到爆炸性分数：Score >= 4.0
			if rawSignal.Score >= EXPLOSIVE_SCORE_OVERRIDE {
				isOverridden = true
			} else if trendState.Scores.TrendScore < MAX_COUNTER_TREND_SCORE {
				// 只有在非爆炸性情况下，才执行正常的 4H 逆势检查
				return false, fmt.Sprintf("Trend Follow BUY rejected: trend score is too bearish (%.2f < %.2f).", trendState.Scores.Score4h, MAX_COUNTER_TREND_SCORE)
			}
		} else if rawSignal.Command == model.CommandSell {
			// 检查是否达到爆炸性分数：Score <= -4.0
			if rawSignal.Score <= -EXPLOSIVE_SCORE_OVERRIDE {
				isOverridden = true
			} else if trendState.Scores.TrendScore > math.Abs(MAX_COUNTER_TREND_SCORE) {
				return false, fmt.Sprintf("Trend Follow SELL rejected: trend score is too bullish (%.2f > %.2f).", trendState.Scores.Score4h, math.Abs(MAX_COUNTER_TREND_SCORE))
			}
		}

		if isOverridden {
			// 记录覆盖行为，但继续执行 1H 检查，确保风险可控。
			fmt.Printf("[INFO] Explosive Signal Override: %s (Score %.2f) bypassed trend check (Score %.2f).\n", rawSignal.Command, rawSignal.Score, trendState.Scores.Score4h)
		}

		// ********** 2.2. 额外的安全检查：确保 30M 周期至少不强烈逆向 **********
		// 确保 30M 周期至少不是强劲的逆向动能 (MIN_ALIGNED_SCORE_30M = -1.5)
		const MIN_ALIGNED_SCORE_30M = -1.5
		if rawSignal.Command == model.CommandBuy && trendState.Scores.Score30m < MIN_ALIGNED_SCORE_30M {
			return false, fmt.Sprintf("Trend Follow BUY rejected: 30M score is too low (%.2f). Requires at least %.2f.", trendState.Scores.Score30m, MIN_ALIGNED_SCORE_30M)
		}
		if rawSignal.Command == model.CommandSell && trendState.Scores.Score30m > math.Abs(MIN_ALIGNED_SCORE_30M) {
			return false, fmt.Sprintf("Trend Follow SELL rejected: 30M score is too high (%.2f). Requires at most %.2f.", trendState.Scores.Score30m, math.Abs(MIN_ALIGNED_SCORE_30M))
		}

		return dt.finalizeSignal(rawSignal, trendState, "Trend Follow signal approved, aligned with higher timeframes.")

	case model.CommandReversalBuy, model.CommandReversalSell:
		// --- 【反转信号】过滤：允许逆势，但要求不能过于激进 ---
		// (ADX 不应用于反转信号，因为反转发生在 ADX 处于低谷期)

		// 压倒性趋势分数：4H 周期达到最大 (-3.0 或 3.0)，此时拒绝任何逆势操作
		const OVERWHELMING_TREND_SCORE = 3.0

		// 1H 风险检查：如果 1H 周期仍处于强劲的同向趋势 (例如 <-2.0)，则认为反转风险过高。
		const MAX_REVERSAL_RISK_1H_SCORE = -2.0

		// 2.3. 反转买入 (抄底) 时的安全阀
		if rawSignal.Command == model.CommandReversalBuy {
			// 检查 4H 终极风险：如果 4H 趋势是压倒性的空头 (<-3.0)，拒绝。
			if trendState.Scores.Score4h < -OVERWHELMING_TREND_SCORE {
				return false, fmt.Sprintf("Reversal BUY rejected: 4H trend is overwhelmingly BEARISH (%.2f).", trendState.Scores.Score4h)
			}
			// 检查 1H 先行确认：如果 1H 趋势仍是强空头 (<-2.0)，拒绝。
			if trendState.Scores.Score1h < MAX_REVERSAL_RISK_1H_SCORE {
				return false, fmt.Sprintf("Reversal BUY rejected: 1H trend is too strong BEARISH (%.2f < %.2f). Reversal risk too high.", trendState.Scores.Score1h, MAX_REVERSAL_RISK_1H_SCORE)
			}
		}

		// 2.4. 反转卖出 (逃顶) 时的安全阀
		if rawSignal.Command == model.CommandReversalSell {
			// 检查 4H 终极风险：如果 4H 趋势是压倒性的多头 (>3.0)，拒绝。
			if trendState.Scores.Score4h > OVERWHELMING_TREND_SCORE {
				return false, fmt.Sprintf("Reversal SELL rejected: 4H trend is overwhelmingly BULLISH (%.2f).", trendState.Scores.Score4h)
			}
			// 检查 1H 先行确认：如果 1H 趋势仍是强多头 (>2.0)，拒绝。
			if trendState.Scores.Score1h > math.Abs(MAX_REVERSAL_RISK_1H_SCORE) {
				return false, fmt.Sprintf("Reversal SELL rejected: 1H trend is too strong BULLISH (%.2f > %.2f). Reversal risk too high.", trendState.Scores.Score1h, math.Abs(MAX_REVERSAL_RISK_1H_SCORE))
			}
		}

		// 反转信号只需要通过 RSI 极值确认，决策树仅作为高周期风险控制。
		return dt.finalizeSignal(rawSignal, trendState, "Reversal signal approved, passed high-risk trend check.")

	default:
		// 3. 任何未识别的命令（例如：ADX 无法确认方向）
		return false, fmt.Sprintf("Unknown or unconfirmed command: %s", rawSignal.Command)
	}
}

// finalizeSignal 负责通过过滤后，丰富信号数据并返回通过状态
func (dt *DecisionTree) finalizeSignal(rawSignal *model.Signal, trendState *model.TrendState, approvalReason string) (bool, string) {

	// 1. 丰富数据
	rawSignal.Details.FinalScoreUsed = trendState.Scores.FinalScore
	// 2. 更新信号状态
	rawSignal.Status = "ACTIVE"

	return true, approvalReason
}
