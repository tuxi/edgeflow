package trend

import (
	"edgeflow/internal/model"
	"fmt"
	"time"
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
	// 趋势反转/动量背离 这是一种趋势背离（Divergence）的判断，代表着宏观大趋势的反转
	TrendReversal
)

func (d TrendDirection) Desc() string {
	dirStr := map[TrendDirection]string{
		TrendUp:       "多头",
		TrendDown:     "空头",
		TrendNeutral:  "横盘",
		TrendReversal: "反转",
	}[d]
	return dirStr
}

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
	description string // 解释原因
	LastPrice   float64
	// 技术指标
	ATR float64 // 波动性
	ADX float64 // 趋势强度
	RSI float64

	Timestamp time.Time

	Scores TrendScores

	// 历史斜率分数
	Slope float64

	// 原始指标快照 (JSON 格式，用于复盘 ScoreForPeriod 函数的输入)
	IndicatorSnapshot map[string]float64
}

func (ts *TrendState) Description() string {
	ts.description = fmt.Sprintf(
		"[Trend %s %s] 趋势score:%.2f 综合score: %.2f, 4h:%.1f 1h:%.1f 30min:%.1f slope:%.2f 30分钟收盘价格: %.2f 当前时间:%v",
		ts.Symbol, ts.Direction.Desc(), ts.Scores.TrendScore, ts.Scores.FinalScore, ts.Scores.Score4h, ts.Scores.Score1h, ts.Scores.Score30m, ts.Slope, ts.LastPrice, time.Now().Format("2006-01-02 15:04:05"),
	)
	return ts.description
}

type TrendScores struct {
	TrendScore  float64 // 4h/1h 长期趋势分数（固定权重）
	SignalScore float64 // 30m 短周期信号分
	FinalScore  float64 // 综合分 -3 ~ +3，综合多周期得分，包含 4h/1h/30m，动态加权。这个分数更全面，更灵活，可以捕捉到市场细微的变化和趋势的早期迹象。
	Score30m    float64
	Score1h     float64
	Score4h     float64
}

// 趋势斜率 结合历史最多14条数据计算的斜率
type TrendSlope struct {
	// 4h 斜率 → 背景过滤（只要不反着就行）
	Slope4h float64
	// 1h 斜率 → 主趋势
	Slope1h float64
	// 30m 斜率 → 入场信号 & 强弱确认
	Slope30m float64

	Avg4h  float64
	Avg1h  float64
	Avg30m float64

	// 斜率分数
	//FinalSlope float64

	//Dir         TrendDirection
	//Description string
}

// 从历史趋势中计算最新的综合指标
func NewTrendSlope(history []*TrendState) *TrendSlope {
	if history == nil || len(history) <= 1 {
		return nil
	}
	var arr4h, arr1h, arr30m []float64
	for _, h := range history {
		arr4h = append(arr4h, h.Scores.Score4h)
		arr1h = append(arr1h, h.Scores.Score1h)
		arr30m = append(arr30m, h.Scores.Score30m)
	}

	slope := &TrendSlope{
		Slope4h:  calcSlope(arr4h),
		Slope1h:  calcSlope(arr1h),
		Slope30m: calcSlope(arr30m),

		Avg4h:  mean(arr4h),
		Avg1h:  mean(arr1h),
		Avg30m: mean(arr30m),
	}

	//var dir TrendDirection
	//var explanation string
	//// 决策逻辑
	//if slope.Slope4h > 0 && slope.Slope1h > 0 {
	//	if slope.Slope30m > 0 {
	//		dir = TrendUp
	//		explanation = "大周期与中周期均向上，短周期继续放大 → 顺势开多"
	//	} else {
	//		dir = TrendNeutral
	//		explanation = "大周期与中周期向上，但短周期走弱 → 等待确认或止盈"
	//	}
	//} else if slope.Slope4h < 0 && slope.Slope1h < 0 {
	//	if slope.Slope30m < 0 {
	//		dir = TrendDown
	//		explanation = "大周期与中周期均向下，短周期继续走弱 → 顺势开空"
	//	} else {
	//		dir = TrendNeutral
	//		explanation = "大周期与中周期向下，但短周期反弹 → 等待确认或止盈"
	//	}
	//} else {
	//	dir = TrendNeutral
	//	explanation = "大周期与中周期不一致 → 观望为主"
	//}
	//
	//slope.Dir = dir
	//slope.Description = explanation
	return slope
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

type Signal struct {
	Symbol    string    `json:"symbol"`    // BTC/USDT
	Price     float64   `json:"price"`     // 进场价格价格
	Side      string    `json:"side"`      // buy / sell
	Timestamp time.Time `json:"timestamp"` // 触发时间
	/*
		越大代表信号越强（比如用RSI偏离度、ADX等算）
		0.3 视为「弱信号」
		0.5 视为「中等信号」
		0.7 视为「强信号」
	*/
	Strength         float64
	Values           map[string]float64
	IsReversal       bool    // 是否底部/顶部反转信号
	ReversalStrength float64 // 反转强度
}

func (sig *Signal) Equal(sig1 *Signal) bool {
	if sig1 == nil {
		return false
	}
	return sig.Symbol == sig1.Symbol && sig.Side == sig1.Side && sig.Timestamp.Equal(sig1.Timestamp) && sig.Strength == sig1.Strength
}
