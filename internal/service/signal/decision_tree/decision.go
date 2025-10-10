package decision_tree

import (
	"edgeflow/internal/service/signal/model"
	"fmt"
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

// ApplyFilter 接收原始信号和趋势状态，判断信号是否通过趋势过滤，并进行数据丰富化
func (dt *DecisionTree) ApplyFilter(
	rawSignal *model.Signal,
	trendState *model.TrendState,
) (passed bool, reason string) {

	finalScore := trendState.Scores.FinalScore
	rawCommand := rawSignal.Command // BUY 或 SELL

	// 定义更精细的阈值（假设dt.OpenThreshold为2.0，dt.CloseThreshold为1.0）
	strongThreshold := dt.OpenThreshold // 例如 2.0
	neutralThreshold := 1.0             // 低于此分数，视为震荡或弱趋势

	// ===============================================
	// P1: 基础数据检查与抑制
	// ===============================================

	// 1. 检查趋势方向是否明确 (必须是 UP 或 DOWN)
	if trendState.Direction == model.TrendNeutral {
		return false, fmt.Sprintf("REJECTED: Trend State is NEUTRAL (FinalScore %.2f). Ignoring all signals.", finalScore)
	}

	// ===============================================
	// P2: 顺势开仓过滤 (严格要求 FinalScore 达到 StrongThreshold)
	// ===============================================

	// --- 顺势开多判断 ---
	if rawCommand == model.CommandBuy {
		// 1. 检查是否达到强趋势分数
		if finalScore < strongThreshold {
			return false, fmt.Sprintf("REJECTED: BUY signal is weak. FinalScore (%.2f) below Strong Threshold (%.2f).", finalScore, strongThreshold)
		}

		// 2. 检查趋势方向是否一致
		if trendState.Direction == model.TrendUp {
			// 通过：分数够高，且趋势向上
			return dt.finalizeSignal(rawSignal, trendState, "APPROVED: Strong BUY signal aligns with strong UP trend.")
		}

		// 3. 逆大势信号：分数虽然高，但方向相反 (例如 4h 强多，但 5m 逆势多头平仓)
		if trendState.Direction == model.TrendDown {
			return false, fmt.Sprintf("REJECTED: Strong BUY score (%.2f) but main Trend Direction is DOWN.", finalScore)
		}
	}

	// --- 顺势开空判断 ---
	if rawCommand == model.CommandSell {
		// 1. 检查是否达到强趋势分数（负值判断）
		if finalScore > -strongThreshold {
			return false, fmt.Sprintf("REJECTED: SELL signal is weak. FinalScore (%.2f) above Negative Strong Threshold (-%.2f).", finalScore, strongThreshold)
		}

		// 2. 检查趋势方向是否一致
		if trendState.Direction == model.TrendDown {
			// 通过：分数够低，且趋势向下
			return dt.finalizeSignal(rawSignal, trendState, "APPROVED: Strong SELL signal aligns with strong DOWN trend.")
		}

		// 3. 逆大势信号
		if trendState.Direction == model.TrendUp {
			return false, fmt.Sprintf("REJECTED: Strong SELL score (%.2f) but main Trend Direction is UP.", finalScore)
		}
	}

	// ===============================================
	// P3: 弱势/观望情景处理
	// ===============================================

	// 如果 FinalScore 处于中性区间 (-neutralThreshold 到 neutralThreshold)
	if finalScore > -neutralThreshold && finalScore < neutralThreshold {
		return false, fmt.Sprintf("REJECTED: Market is within Neutral Zone (-%.2f to %.2f). Suppressing signal.", neutralThreshold, neutralThreshold)
	}

	// 默认拒绝所有未通过严格检查的信号
	return false, "REJECTED: Unmatched command or internal logic failure."
}

// finalizeSignal 负责通过过滤后，丰富信号数据并返回通过状态
func (dt *DecisionTree) finalizeSignal(rawSignal *model.Signal, trendState *model.TrendState, approvalReason string) (bool, string) {

	// 1. 丰富数据
	rawSignal.Details.FinalScoreUsed = trendState.Scores.FinalScore
	rawSignal.Details.BasisExplanation = fmt.Sprintf(
		"%s | 趋势详情: 4H(%.2f), 1H(%.2f), 30M(%.2f)",
		approvalReason,
		trendState.Scores.Score4h,
		trendState.Scores.Score1h,
		trendState.Scores.Score30m,
	)
	// 2. 更新信号状态
	rawSignal.Status = "ACTIVE"

	return true, approvalReason
}
