package trend

import "fmt"

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
			fmt.Printf(sm.Symbol, "状态转换: %s  -> 反转\n", sm.CurrentState.Desc())
			sm.CurrentState = TrendReversal
		}
		return
	}

	// --- 其次判断强劲的看涨或看跌趋势 ---
	// 强劲的看涨趋势：同时需要短期和长期分数都高于阈值，且斜率为正
	if finalScore >= 1.5 && trendScore >= 0.5 && *finalSlope > 0.1 {
		if sm.CurrentState != TrendUp {
			fmt.Println(sm.Symbol, "状态转换: %s -> 上升\n", sm.CurrentState)
			sm.CurrentState = TrendUp
		}
		return
	}

	// 强劲的看跌趋势：同时需要短期和长期分数都低于阈值，且斜率为负
	if finalScore <= -1.5 && trendScore <= -0.5 && *finalSlope < -0.1 {
		if sm.CurrentState != TrendDown {
			fmt.Println(sm.Symbol, "状态转换: %s -> 下降\n", sm.CurrentState)
			sm.CurrentState = TrendDown
		}
		return
	}

	// --- 最后判断中性/震荡行情 ---
	if sm.CurrentState != TrendNeutral {
		fmt.Println(sm.Symbol, "状态转换: %s -> 中性\n", sm.CurrentState)
		sm.CurrentState = TrendNeutral
	}
}

//func (sm *StateMachine) Update(finalScore, trendScore float64, finalSlope *float64) {
//	// 如果没有足够的历史数据来计算斜率，或者斜率为nil，则强制进入中性状态
//	if finalSlope == nil {
//		if sm.CurrentState != TrendNeutral {
//			sm.CurrentState = TrendNeutral
//			// 可以在这里记录日志，表明进入了数据不足的默认状态
//			fmt.Println(sm.Symbol, "数据不足，强制进入中性状态")
//		}
//		return
//	}
//
//	// 否则，执行正常的趋势判断逻辑
//	switch sm.CurrentState {
//	case TrendUp:
//		// 趋势减弱或反转的判断
//		if finalScore < 0.5 || *finalSlope < 0 {
//			sm.CurrentState = TrendNeutral
//			fmt.Println(sm.Symbol, "状态转换: UpTrend -> Neutral")
//		}
//
//	case TrendDown:
//		// 趋势减弱或反转的判断
//		if finalScore > -0.5 || *finalSlope > 0 {
//			sm.CurrentState = TrendNeutral
//			fmt.Println(sm.Symbol, "状态转换: DownTrend -> Neutral")
//		}
//
//	case TrendNeutral:
//		// 从中性状态进入新趋势的判断
//		if finalScore > 1.0 && trendScore > 0.5 && *finalSlope > 0 {
//			sm.CurrentState = TrendUp
//			fmt.Println(sm.Symbol, "状态转换: Neutral -> UpTrend")
//		} else if finalScore < -1.0 && trendScore < -0.5 && *finalSlope < 0 {
//			sm.CurrentState = TrendDown
//			fmt.Println(sm.Symbol, "状态转换: Neutral -> DownTrend")
//		}
//	}
//
//}
