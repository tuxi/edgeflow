package trend

import (
	"edgeflow/internal/model"
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
	Symbol        string
	Direction     TrendDirection
	LastDirection TrendDirection // 上一根k线方向
	Description   string         // 解释原因
	LastPrice     float64
	Score         float64 // -3 ~ +3，综合多周期得分
	// 技术指标
	ATR       float64
	ADX       float64
	RSI       float64
	Timestamp time.Time
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
	Price     float64   `json:"price"`     // 当前价格
	Side      string    `json:"side"`      // buy / sell
	Timestamp time.Time `json:"timestamp"` // 触发时间
	/*
		越大代表信号越强（比如用RSI偏离度、ADX等算）
		0.3 视为「弱信号」
		0.5 视为「中等信号」
		0.7 视为「强信号」
	*/
	Strength   float64
	IsReversal bool // 是否底部/顶部反转信号
}
