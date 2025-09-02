package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"strconv"
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

// ==== 决策器 ====
func Decide(ctx Context) Action {
	// 这里把同一个信号屏蔽掉
	if ctx.Sig.Equal(ctx.LastSig) {
		return ActIgnore
	}
	// 第一种：无仓位 → 入场
	if ctx.Pos == nil {
		isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
		// 顺势开多:当大趋势方向相同，并且短期趋势强 顺势开多
		if isTrending {
			switch {
			case ctx.Sig.Strength > 0.6:
				return ActOpen // 强信号 → 正常开仓
			case ctx.Sig.Strength > 0.35:
				return ActOpenSmall // 中等信号 → 轻仓试探
			}
		}

		if !isTrending && (ctx.Sig.IsReversal || ctx.Sig.Strength >= 0.6) {
			return ActOpen // 反转信号通常是超卖，开大一点仓位
		}
		return ActIgnore
	}

	// 第二种： 有仓位 → 管理仓位
	posDir := ctx.Pos.Dir // model.Buy / model.Sell
	// 仓位方向是否与大趋势一致
	isTrending := (posDir == model.OrderPosSideLong && ctx.Trend.Direction == trend.TrendUp) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Direction == trend.TrendDown)

	// 获取未实现盈亏，逆势中，如果盈利中，加仓
	upnl, _ := strconv.ParseFloat(ctx.Pos.UnrealizedPnl, 64)

	var lastSig = ctx.LastSig
	// 当仓位方向是在大趋势中
	if isTrending {
		// 顺势加仓
		isSide := (posDir == model.OrderPosSideLong && ctx.Sig.Side == "buy") ||
			(posDir == model.OrderPosSideShort && ctx.Sig.Side == "sell")
		if isSide {

			if ctx.Sig.Strength >= 0.7 {
				return ActAdd // 强信号激进加仓
			}
			// 本次信号的强度大于上一次的信号时候加仓，并且判断不是同一个信息
			if ctx.isValidLastSignal() {
				if ctx.Sig.Strength > lastSig.Strength && ctx.Sig.Strength > 0.3 { // 层数限制
					return ActAdd
				}

				// 多单：避免追高，要求价格 ≤ 上次加仓价 * (1 + 0.5%)
				if ctx.Pos.Dir == model.OrderPosSideLong && ctx.Sig.Price <= lastSig.Price*1.005 {
					return ActAddSmall
				}

				// 空单：避免杀低，要求价格 ≥ 上次加仓价 * (1 - 0.5%)
				if ctx.Pos.Dir == model.OrderPosSideShort && ctx.Sig.Price >= lastSig.Price*0.995 {
					return ActAddSmall
				}
			}

			if ctx.Sig.Strength > 0.35 && upnl >= 0 {
				return ActAddSmall
			}

			return ActIgnore
		}
		// 短线出现反转迹象，加仓保护
		if ctx.Sig.IsReversal {
			return ActReduce
		}
		return ActIgnore
	}

	// 大趋势不一致，如果出现短线反转，逆市加仓
	if ctx.Sig.IsReversal || ctx.Sig.Strength > 0.6 {

		// 逆势亏损时加仓（摊平成本 / 马丁格尔）
		if upnl <= 0 {
			return ActAddSmall
		}

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
