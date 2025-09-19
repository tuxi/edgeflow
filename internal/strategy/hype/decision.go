package hype

import (
	"edgeflow/internal/model"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"edgeflow/pkg/hype/types"
)

// 决策
type DecisionEngine struct {
	Ctx Context
}

func NewDecisionEngine(ctx Context) *DecisionEngine {
	return &DecisionEngine{Ctx: ctx}
}

func (de *DecisionEngine) Run() signal.Action {
	// 更新并获取当前的趋势方向
	sig := de.Ctx.Sig
	pos := de.Ctx.Pos
	track := de.Ctx.TrackPos

	// 有仓位
	if pos != nil && pos.Amount > 0 {
		if track == nil {
			// 被跟踪者没有仓位，立即平仓
			return signal.ActClose
		}
		if model.OrderPosSide(sig.Dir) == pos.Dir {
			if sig.Action == "buy" {
				return signal.ActAdd
			}
			if sig.Action == "sell" {
				return signal.ActReduce
			}
		}
		return signal.ActIgnore
	}

	// 没仓位，跟踪位有仓位，开仓
	if pos == nil && track != nil {
		return signal.ActOpen
	}

	return signal.ActIgnore
}

// 决策的输入
type Context struct {

	// 大趋势
	Trend trend.TrendState

	// 交易信号
	Sig HypeTradeSignal

	// 用户仓位信息
	Pos *model.PositionInfo

	// 跟单对象的仓位信息
	TrackPos *types.AssetPosition
}
