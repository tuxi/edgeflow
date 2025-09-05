package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"strconv"
)

func RunDecide(ctx *Context) Action {
	// 屏蔽重复信号
	if ctx.Sig.Equal(ctx.LastSig) {
		return ActIgnore
	}
	// ---- 无仓位 → 顺势或逆势开仓 ----
	if ctx.Pos == nil {
		return openPosition(ctx)
	}

	// 获取趋势方向
	trendDir := trendDirection(ctx)

	// ---- 横盘低吸高抛 ----
	if trendDir == trend.TrendNeutral && ctx.Sig.Side == "hold" {
		return handleSideways(ctx)
	}

	// ---- 有仓位 → 管理仓位 ----
	return managePosition(ctx)

}

// 获取趋势方向
func trendDirection(ctx *Context) trend.TrendDirection {
	score := ctx.Trend.Scores.FinalScore
	if score >= 1.0 {
		return trend.TrendUp
	} else if score <= -1.0 {
		return trend.TrendDown
	}
	return trend.TrendNeutral
}

// 横盘低吸高抛逻辑
func handleSideways(ctx *Context) Action {
	price := ctx.Sig.Price
	upper := ctx.Sig.Values["Upper"]
	lower := ctx.Sig.Values["Lower"]
	rsi := ctx.Sig.Values["RSI"]
	strength := ctx.Sig.Strength
	buffer := 0.01 // 1%缓冲

	if ctx.Pos == nil {
		// 无仓位建仓
		if price <= lower*(1+buffer) && rsi < 40 && strength > 0.2 {
			ctx.Sig.Side = "buy"
			return ActOpen
		}
		if price >= upper*(1-buffer) && rsi > 60 && strength > 0.2 {
			ctx.Sig.Side = "sell"
			return ActOpen
		}
	} else {
		// 已有仓位管理
		switch ctx.Pos.Dir {
		case model.OrderPosSideLong:
			if price >= upper*(1-buffer) && rsi > 60 {
				return ActReduce
			}
			if price <= lower*(1+buffer) && rsi < 40 {
				return ActAdd
			}
		case model.OrderPosSideShort:
			if price <= lower*(1+buffer) && rsi < 40 {
				return ActReduce
			}
			if price >= upper*(1-buffer) && rsi > 60 {
				return ActAdd
			}
		}
	}
	return ActIgnore
}

// 无仓位顺势/逆势开仓
func openPosition(ctx *Context) Action {
	isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))

	// 顺势开仓
	if isTrending && ctx.Sig.Strength > 0.5 {
		return ActOpen
	}

	// 逆势短线反转开仓
	if !isTrending && ctx.Sig.IsReversal && ctx.Sig.Strength >= 0.7 {
		return ActOpen
	}

	// 背景趋势判断
	slopeDir := ctx.Trend.Direction
	slope := ctx.Trend.HistorySlope
	scores := ctx.Trend.Scores
	if slope != nil {
		slopeDir = slope.Dir
		// 如果 30m 分数的斜率已经转正，哪怕分数没到阈值，也可以认为“底部反弹正在发生”。core30m < -1.5说明超卖现在买入赔率低
		if slope.Slope30m > 0 &&
			ctx.Trend.Scores.Score30m < -1.5 && // 超卖区
			ctx.Sig.Side == "buy" && ctx.Sig.Strength > 0.3 {
			return ActOpen
		}

		if slope.Slope30m < 0 &&
			ctx.Trend.Scores.Score30m >= 1.5 && // 超买区
			ctx.Sig.Side == "sell" && ctx.Sig.Strength > 0.3 {
			return ActOpen
		}
	}
	if slopeDir == trend.TrendUp {
		if scores.TrendScore > 1.0 && ctx.Sig.Side == "buy" && ctx.Sig.Strength > 0.3 {
			return ActOpen // 顺势多
		}
	}
	if slopeDir == trend.TrendDown {
		if scores.TrendScore < -1.0 && ctx.Sig.Side == "sell" && ctx.Sig.Strength > 0.3 {
			return ActOpen // 顺势空
		}
	}

	// 空头排列 顺势做空 由多转空
	if scores.Score4h > scores.Score1h && scores.Score1h >= scores.Score30m &&
		ctx.Sig.Side == "sell" && ctx.Sig.Strength > 0.35 {
		return ActOpen
	}

	// 多头排列 顺势做多  斜率转正了如果不能突破，也许会在山顶 由空转多
	if scores.Score4h < scores.Score1h && scores.Score1h <= scores.Score30m &&
		ctx.Sig.Side == "buy" && ctx.Sig.Strength > 0.35 {
		return ActOpen
	}

	return ActIgnore // 没有明确信号
}

func managePosition(ctx *Context) Action {
	// 1. 已经有仓位，先考虑止盈止损
	// 高盈亏保护
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
	if uplRatio > 0.25 {
		return ActClose
	}
	if uplRatio > 0.15 {
		return ActClose
	}

	//lastSig := ctx.LastSig
	posDir := ctx.Pos.Dir
	//currentPrice, err := strconv.ParseFloat(ctx.Pos.MarkPx, 64)
	//if err != nil {
	//	currentPrice = ctx.Sig.Price
	//}
	//ma30 := ctx.Sig.Values["slow"]

	if ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side)) {
		// 仓位与信号方向不同，减仓
		if ctx.Sig.Side == "buy" && posDir == model.OrderPosSideShort ||
			ctx.Sig.Side == "sell" && posDir == model.OrderPosSideLong {
			if ctx.Sig.Strength > 0.5 {
				return ActReduce
			}
		}

		// 仓位与信号方向相同
		if ctx.Sig.Strength > 0.7 {
			return ActReduce
		}
		if ctx.Sig.Strength > 0.35 {
			return ActAdd
		}
	}

	// 趋势向上时
	if ctx.Trend.Direction == trend.TrendUp {
		if ctx.Sig.Side != "sell" && ctx.Sig.Strength >= 0.15 {
			if posDir == model.OrderPosSideShort { // 仓位与大方向反向时 平仓
				return ActClose
			}
			return ActAdd
		}
	}

	// 趋势向下
	if ctx.Trend.Direction == trend.TrendDown {
		if ctx.Sig.Side != "buy" && ctx.Sig.Strength >= 0.15 {
			if posDir == model.OrderPosSideLong { /// 仓位与大方向反向时 平仓
				return ActClose
			}
			return ActAdd
		}
	}

	//// 2. 趋势方向一致，信号转强时，开基础仓位
	//if posDir == model.OrderPosSideLong && ctx.Trend.Direction == trend.TrendUp && ctx.Sig.Side == "buy" {
	//	if currentPrice >= ma30 && ctx.Sig.Strength > 0.35 {
	//		return ActAdd // 趋势还在
	//	} else {
	//		return ActReduce // 回调过深，减仓或止损
	//	}
	//}
	//
	//if posDir == model.OrderPosSideShort && ctx.Trend.Direction == trend.TrendDown && ctx.Sig.Side == "sell" {
	//	if currentPrice <= ma30 && ctx.Sig.Strength > 0.35 {
	//		return ActAdd
	//	} else {
	//		return ActReduce
	//	}
	//}

	return ActIgnore
}
