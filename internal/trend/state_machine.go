package trend

import (
	"fmt"
)

// 趋势状态机
type StateMachine struct {
	CurrentState TrendDirection
	StatesCaches []*TrendState
	Slope        *TrendSlope
	Symbol       string
}

// 创建并初始化一个状态机
func NewStateMachine(symbol string) *StateMachine {
	// 初始化状态机为中性 横盘 没有方向
	return &StateMachine{
		CurrentState: TrendNeutral,
		Symbol:       symbol,
		Slope:        nil,
	}
}

// 更新状态机
func (sm *StateMachine) Update(finalScore, trendScore float64, finalSlope *float64) {
	// 1. 如果没有足够的历史数据，则强制进入中性状态。
	if finalSlope == nil {
		if sm.CurrentState != TrendNeutral {
			sm.CurrentState = TrendNeutral
			fmt.Println(sm.Symbol, "数据不足，强制进入中性状态")
		}
		return
	}

	// 2. 否则，执行完整的趋势判断逻辑。

	// --- 最高优先级：趋势反转 ---
	// 趋势分数和斜率出现方向背离，这是最危险的信号
	if (finalScore > 0 && *finalSlope < -0.05) || (finalScore < 0 && *finalSlope > 0.05) {
		if sm.CurrentState != TrendReversal {
			oldState := sm.CurrentState
			sm.CurrentState = TrendReversal
			fmt.Printf("%s 状态转换: %v  -> %s\n", sm.Symbol, oldState.Desc(), sm.CurrentState.Desc())
		}
		return
	}

	// --- 其次判断强劲的看涨或看跌趋势 ---
	// 强劲的看涨趋势：同时需要短期和长期分数都高于阈值，且斜率为正
	if finalScore >= 1.5 && trendScore >= 0.5 && *finalSlope > 0.1 {
		if sm.CurrentState != TrendUp {
			oldState := sm.CurrentState
			sm.CurrentState = TrendUp
			fmt.Printf("%s 状态转换: %v  -> %s", sm.Symbol, oldState.Desc(), sm.CurrentState.Desc())
		}
		return
	}

	// 强劲的看跌趋势：同时需要短期和长期分数都低于阈值，且斜率为负
	if finalScore <= -1.5 && trendScore <= -0.5 && *finalSlope < -0.1 {
		if sm.CurrentState != TrendDown {
			oldState := sm.CurrentState
			sm.CurrentState = TrendDown
			fmt.Printf("%s 状态转换: %v  -> %s\n", sm.Symbol, oldState.Desc(), sm.CurrentState.Desc())
		}
		return
	}

	// --- 最后判断中性/震荡行情 ---
	if sm.CurrentState != TrendNeutral {
		oldState := sm.CurrentState
		sm.CurrentState = TrendNeutral
		fmt.Printf("%s 状态转换: %v  -> %s\n", sm.Symbol, oldState.Desc(), sm.CurrentState.Desc())
	}
}
