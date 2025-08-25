package signal

import "github.com/nntaoli-project/goex/v2/model"

/*
它就是一个 趋势过滤器，在信号触发时再加一道条件：
只允许 顺大级别趋势 的单子。
过滤掉“逆趋势的小级别信号”，避免反复被洗。
这样：
L1 = 大趋势参考 （不直接驱动交易，但作为过滤条件）
L2 = 主交易驱动力量
L3 = 精细调仓 / 加仓 / 止盈辅助
*/
type TrendFilter interface {
	Update(candles []model.Kline) // 用数据更新趋势
	IsOK() bool                   // 是否允许开仓
	Side() string                 // 当前趋势方向 ("long" / "short" / "neutral")
	MarkPrice() float64           // 当前价格，辅助计算浮盈/浮亏
}

type SignalTrendFilter struct {
	LastL1 *Signal
	Price  float64
}

func (f *SignalTrendFilter) IsOK() bool {
	if f.LastL1 == nil {
		return true // 没有参考时，默认不过滤
	}
	return true // 也可以更严格，例如只有趋势一致才允许
}

func (f *SignalTrendFilter) Direction() string {
	if f.LastL1 == nil {
		return "neutral"
	}
	return f.LastL1.Side
}

func (f *SignalTrendFilter) MarkPrice() float64 {
	return f.Price
}
