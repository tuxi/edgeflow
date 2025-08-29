package trend

import (
	"edgeflow/internal/model"
)

// 趋势方向
type TrendDirection int

const (
	// 震荡趋势
	TrendNeutral TrendDirection = iota
	// 上升趋势
	TrendUp
	// 下降趋势
	TrendDown
)

func (d TrendDirection) MatchesSide(side model.OrderSide) bool {
	switch d {
	case TrendUp:
		return side == model.Buy
	case TrendDown:
		return side == model.Sell
	default:
		return false
	}
}

// 单币种趋势状态
type TrendState struct {
	Symbol      string
	Direction   TrendDirection
	Description string // 解释原因
	LastPrice   float64
	Score       float64 // -3 ~ +3，综合多周期得分
	StrongM15   bool    // 是否满足强M15推进条件
	Timestamp   int64
}

type TrendCfg struct {
	ADXThreshold float64 // 趋势强度门槛，山寨/新币可用 18~22
	MinR2        float64 // 线性回归的最小拟合度
	SlopeWindow  int     // 斜率窗口（bar数），4h*60≈10天
	ConfirmBars  int     // 突破确认所需的连续收盘数
}

func DefaultTrendCfg() TrendCfg {
	return TrendCfg{
		ADXThreshold: 20,
		MinR2:        0.25,
		SlopeWindow:  60,
		ConfirmBars:  2,
	}
}
