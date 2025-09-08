package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"time"
)

// ==== 上下文 ====

// Context 是 StrategyEngine 决策的输入
type Context struct {

	// 大趋势
	Trend trend.TrendState

	// 交易信号
	Sig trend.Signal

	// 仓位信息
	Pos *model.PositionInfo

	// 风控
	DailyR float64

	// 上一次交易完成的信号
	LastSig *trend.Signal
}

func (ctx Context) isValidLastSignal() bool {
	// 定义冷却窗口
	cooldown := 50 * time.Minute
	now := time.Now()
	if ctx.LastSig == nil {
		return false
	}
	// 只在冷却期外，并且新信号强度 > 上一次加仓强度
	if now.Sub(ctx.LastSig.Timestamp) >= cooldown {
		// 上一个信号与当前信号对比只能在60分钟内有效
		// 使上一次信号无效
		return false
	}
	return true
}
