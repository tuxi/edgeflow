package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
)

// ==== 上下文 ====

// Context 是 StrategyEngine 决策的输入
type Context struct {

	// 大趋势
	Trend trend.TrendState

	// 15分钟信号
	Sig trend.Signal

	// 仓位信息
	Pos *model.PositionInfo

	// 风控
	DailyR float64
}

// ==== 决策器 ====

func Decide(ctx Context) Action {

	// 第一种：无仓位 → 入场
	if ctx.Pos == nil {
		isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
		// 顺势开多:当大趋势方向相同，并且短期趋势强 顺势开多
		if isTrending {
			switch {
			case ctx.Sig.Strength > 0.6:
				return ActOpen // 强信号 → 正常开仓
			case ctx.Sig.Strength > 0.3:
				return ActOpen //ActSmallOpen // 中等信号 → 轻仓试探
			}
		}
		// 短周期反转入场（逆势机会）:当大趋势方向不同，但是短线强度高存在短线交易机会，逆势开仓
		//if !isTrending && ctx.Sig.IsReversal {
		//	if ctx.Sig.Strength > 0.7 {
		//		return ActSmallOpen // 反转且强信号 → 逆势轻仓试探
		//	}
		//}

		if !isTrending && (ctx.Sig.IsReversal || ctx.Sig.Strength > 0.5) {
			return ActOpen
		}
		return ActIgnore
	}

	// 第二种： 有仓位 → 管理仓位
	posDir := ctx.Pos.Dir // model.Buy / model.Sell
	// 仓位方向是否与大趋势一致
	isTrending := (posDir == model.OrderPosSideLong && ctx.Trend.Direction == trend.TrendUp) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Direction == trend.TrendDown)

	// 当仓位方向是在大趋势中
	if isTrending {
		// 顺势加仓
		if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "buy") ||
			(posDir == model.OrderPosSideShort && ctx.Sig.Side == "sell") {
			if ctx.Sig.Strength > 0.7 {
				return ActAdd // 强信号加仓
			}
			if ctx.Sig.Strength > 0.3 {
				return ActAdd //ActSmallAdd // 中等信号小幅加仓
			}
		}
		// 短线出现反转迹象，加仓保护
		if ctx.Sig.IsReversal {
			return ActReduce
		}
		return ActIgnore
	}

	// 大趋势不一致，如果出现短线反转，逆市加仓
	if ctx.Sig.IsReversal || ctx.Sig.Strength > 0.6 {
		return ActAdd //ActSmallAdd
	}
	// 横盘 / 明显逆势 → 平仓离场
	if ctx.Trend.Direction == trend.TrendNeutral {
		return ActIgnore
	}
	return ActClose
}

// ===== 入场逻辑 =====
func decideEntry(ctx Context) Action {
	isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
	// 顺势开多:当大趋势方向相同，并且短期趋势强 顺势开多
	if isTrending && ctx.Sig.Strength > 0.5 {
		return ActOpen
	}

	// 短周期反转入场（逆势机会）:当大趋势方向不同，但是短线强度高存在短线交易机会，逆势开仓
	if !isTrending && ctx.Sig.IsReversal && ctx.Sig.Strength > 0.7 {
		return ActOpen
	}
	return ActIgnore
}
