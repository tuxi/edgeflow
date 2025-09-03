package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"math"
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
func Decide(ctx *Context) Action {
	// 这里把同一个信号屏蔽掉
	if ctx.Sig.Equal(ctx.LastSig) {
		return ActIgnore
	}

	// 第一种：无仓位 → 入场
	if ctx.Pos == nil {
		isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
		// 顺势开多:当大趋势方向相同，并且短期趋势强 顺势开多
		if isTrending {
			// 强信号 → 正常开仓
			if ctx.Sig.Strength > 0.5 {
				return ActOpen
			}

		} else {
			if ctx.Sig.IsReversal {
				return ActOpen // 反转信号通常是超卖，开大一点仓位
			}
		}
		return ActIgnore
	}

	var lastSig = ctx.LastSig
	// 获取当前线上价格
	currentPrice, err := strconv.ParseFloat(ctx.Pos.MarkPx, 64)
	if err != nil {
		currentPrice = ctx.Sig.Price
	}
	// 第二种： 有仓位 → 管理仓位
	posDir := ctx.Pos.Dir // model.Buy / model.Sell
	if posDir == model.OrderPosSideLong && ctx.Trend.Direction == trend.TrendUp {
		// 多头方向，顺势做多
		if ctx.Sig.Side == "buy" {
			if ctx.Sig.Strength >= 0.7 {
				return ActAdd // 强信号激进加仓
			}
			// 本次信号的强度大于上一次的信号时候加仓，并且判断不是同一个信息
			if ctx.isValidLastSignal() {
				if ctx.Sig.Strength > lastSig.Strength { // 信号强度越来越高加仓
					if ctx.Sig.Strength > 0.6 {
						return ActReduce // 信号越强代表快要到达顶部，减仓
					}
					if ctx.Sig.Strength > 0.3 {
						return ActAdd
					}
				}

			}
			// 多单：回调后做多
			if currentPrice < ctx.Pos.AvgPrice*0.995 && ctx.Sig.Strength > 0.25 {
				return ActAddSmall
			}
			return ActIgnore
		}
	}

	if posDir == model.OrderPosSideShort && ctx.Trend.Direction == trend.TrendDown {
		// 空头方向，顺势做空
		if ctx.Sig.Side == "sell" {
			if ctx.Sig.Strength >= 0.7 {
				return ActAdd // 强信号激进加仓
			}
			// 本次信号的强度大于上一次的信号时候加仓，并且判断不是同一个信息
			if ctx.isValidLastSignal() {
				if ctx.Sig.Strength > lastSig.Strength { // 信号强度越来越高加仓
					if ctx.Sig.Strength > 0.6 {
						return ActReduce // 信号越强代表快要到达顶部，减仓
					}
					if ctx.Sig.Strength > 0.3 {
						return ActAdd
					}
				}

			}

			// 空单：反弹后继续做空
			if currentPrice > ctx.Pos.AvgPrice*1.005 && ctx.Sig.Strength > 0.25 {
				return ActAddSmall
			}

			return ActIgnore
		}
	}

	// 横盘 做高抛低吸
	if ctx.Trend.Direction == trend.TrendNeutral {
		price := ctx.Sig.Price
		upper := ctx.Sig.Values["Upper"]
		lower := ctx.Sig.Values["Lower"]
		rsi := ctx.Sig.Values["RSI"]

		// 设置缓冲比例，例如 1% 区间
		//buffer := 0.01
		buffer := 0.0
		// ---- 无仓位 → 建仓 ----
		if ctx.Pos == nil {
			if price <= lower*(1+buffer) && rsi < 40 {
				ctx.Sig.Side = "buy"
				return ActOpen // 低位做多
			}
			if price >= upper*(1-buffer) && rsi > 60 {
				ctx.Sig.Side = "sell"
				return ActOpen // 高位做空
			}
		} else {
			// ---- 已有仓位 → 管理仓位 ----
			switch ctx.Pos.Dir {
			case model.OrderPosSideLong:
				if price >= upper*(1-buffer) && rsi > 60 {
					return ActReduce // 高位卖出
				}
				if price <= lower*(1+buffer) && rsi < 40 {
					return ActAdd // 低位加仓
				}
			case model.OrderPosSideShort:
				if price <= lower*(1+buffer) && rsi < 40 {
					return ActReduce // 低位回补
				}
				if price >= upper*(1-buffer) && rsi > 60 {
					return ActAdd // 高位加仓
				}
			}
		}
		return ActIgnore
	}

	// 方向不确定时，赚18个点就止盈
	// 获取未实现盈亏，逆势中，如果盈利中，加仓
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
	if uplRatio > 0.18 {
		return ActReduce
	}

	// 大趋势不一致，如果出现短线反转，逆市加仓
	if ctx.Sig.IsReversal {
		return ActAddSmall
	}

	return ActClose
}

// ===== 入场逻辑 =====
func decideEntry(ctx Context) Action {
	isTrending := ctx.Trend.Direction.MatchesSide(model.OrderSide(ctx.Sig.Side))
	// 顺势开多:当大趋势方向相同，并且短期趋势强 顺势开多
	if isTrending {
		// 强信号 → 正常开仓
		if ctx.Sig.Strength > 0.6 {
			return ActOpen
		}
		// 中等信号 → 轻仓试探
		if ctx.Sig.Strength > 0.35 {
			return ActOpenSmall
		}

		// 如果大趋势非常强，并且当前强度也不小，开仓
		if ctx.Sig.Side == "buy" {
			if ctx.Trend.Score >= 2.3 && ctx.Sig.Strength > 0.25 {
				return ActOpenSmall
			}
		}
		if ctx.Sig.Side == "sell" {
			if ctx.Trend.Score <= -2.3 && ctx.Sig.Strength > 0.25 {
				return ActOpenSmall
			}
		}

	} else {
		if ctx.Sig.IsReversal {
			return ActOpen // 反转信号通常是超卖，开大一点仓位
		}
	}

	// 大趋势明显，小趋势横盘，但是强度到达0.15开仓
	if math.Abs(ctx.Trend.Score) >= 2.5 && ctx.Sig.Side == "hold" && ctx.Sig.Strength > 0.15 {
		return ActOpenSmall

	}

	return ActIgnore
}
