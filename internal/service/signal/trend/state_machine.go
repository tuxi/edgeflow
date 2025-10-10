package trend

import (
	"edgeflow/internal/service/signal/model"
	"fmt"
)

// 趋势状态机
type StateMachine struct {
	CurrentState model.TrendDirection
	StatesCaches []*model.TrendState
	Symbol       string
}

// 创建并初始化一个状态机
func NewStateMachine(symbol string) *StateMachine {
	// 初始化状态机为中性 横盘 没有方向
	return &StateMachine{
		CurrentState: model.TrendNeutral,
		Symbol:       symbol,
	}
}

// 根据趋势分数更新状态机
func (sm *StateMachine) Update(finalScore, trendScore float64) {
	// 1. 统一的趋势强度阈值 (可根据测试调整)
	const STRONG_THRESHOLD = 1.8 // 强劲趋势所需的分数，比开仓阈值略高
	const WEAK_THRESHOLD = 0.5   // 弱势趋势所需的分数

	// --- P1: 强劲趋势 (Strong Trend) ---
	// 要求：FinalScore 必须高于 Strong Threshold，且 TrendScore (长期/大周期分数) 必须支持

	// 强劲看涨 (TrendUp)
	if finalScore >= STRONG_THRESHOLD && trendScore >= WEAK_THRESHOLD {
		if sm.CurrentState != model.TrendUp {
			sm.transition(model.TrendUp, finalScore)
		}
		return
	}

	// 强劲看跌 (TrendDown)
	if finalScore <= -STRONG_THRESHOLD && trendScore <= -WEAK_THRESHOLD {
		if sm.CurrentState != model.TrendDown {
			sm.transition(model.TrendDown, finalScore)
		}
		return
	}

	// --- P2: 趋势反转/弱势信号 (Reversal/Weak) ---
	// 定义趋势反转为：当前状态是 Strong，但 FinalScore 已经越过 0，且低于 Weak Threshold

	// 从 TrendUp 转向：FinalScore 已显著下跌
	if sm.CurrentState == model.TrendUp && finalScore < WEAK_THRESHOLD {
		// 如果分数跌至负值，表明趋势可能已反转
		if finalScore <= -WEAK_THRESHOLD {
			sm.transition(model.TrendReversal, finalScore)
			return
		}
	}

	// 从 TrendDown 转向：FinalScore 已显著上涨
	if sm.CurrentState == model.TrendDown && finalScore > -WEAK_THRESHOLD {
		// 如果分数升至正值，表明趋势可能已反转
		if finalScore >= WEAK_THRESHOLD {
			sm.transition(model.TrendReversal, finalScore)
			return
		}
	}

	// --- P3: 中性/震荡行情 (Neutral) ---
	// 如果没有达到 Strong 趋势，且不处于反转状态
	if sm.CurrentState != model.TrendNeutral &&
		finalScore > -STRONG_THRESHOLD && finalScore < STRONG_THRESHOLD {
		// 只有当分数在 Strong 阈值内，且不在 Reversal 状态时，才归为 Neutral
		sm.transition(model.TrendNeutral, finalScore)
		return
	}

	// 否则保持当前状态（例如保持 Reversal 状态直到分数再次稳定）
}

// transition 封装状态转换的日志和操作
func (sm *StateMachine) transition(newState model.TrendDirection, score float64) {
	oldState := sm.CurrentState
	sm.CurrentState = newState
	fmt.Printf("%s 状态转换: %v(%.2f) -> %s\n",
		sm.Symbol, oldState, score, sm.CurrentState)
}
