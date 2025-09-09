package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/trend"
	"math"
	"strconv"
	"time"
)

// 趋势减弱检测器
var detector = NewWeakTrendDetector(3, 0.5, 0.01, time.Minute*15) // 连续3次，FinalScore阈值0.5，Slope阈值0.01

// 决策
type DecisionEngine struct {
	Ctx Context
}

func NewDecisionEngine(ctx Context) *DecisionEngine {
	return &DecisionEngine{Ctx: ctx}
}

func (de *DecisionEngine) Run() Action {
	// 更新并获取当前的趋势方向
	currentTrendDirection := de.Ctx.Trend.Direction

	// 根据趋势方向来决定交易模式
	switch currentTrendDirection {
	case trend.TrendUp:
		return de.handleTrend(getBullishOperator())
	case trend.TrendDown:
		return de.handleTrend(getBearishOperator())
	case trend.TrendReversal:
		return de.handleReversal()
	case trend.TrendNeutral:
		return de.handleNeutral()
	}

	return ActIgnore
}

// 震荡模式
func (de *DecisionEngine) handleNeutral() Action {
	ctx := de.Ctx
	// 1. 如果没有持仓，保持不变，极度保守
	if ctx.Pos == nil {
		// 在震荡模式下，只考虑最强的逆势反转信号开仓
		if ctx.Sig.IsReversal && ctx.Sig.Strength >= 0.8 {
			// 这里可以加入价格触及布林带或重要支撑/阻力位的条件
			return ActOpen
		}
		return ActIgnore
	}

	// 2. 如果有持仓：执行高抛低吸的逻辑

	posDir := ctx.Pos.Dir
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

	// --- 高抛（减仓）逻辑 ---
	// 如果价格达到相对高位，且与你持仓方向相反的信号强度足够，则高抛减仓
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy") {

		// 只有在有显著盈利时才高抛（例如，盈利超过 3%）
		// 并且信号强度要足够高，以确认是高位
		if uplRatio > 0.03 && ctx.Sig.Strength >= 0.7 {
			return ActReduce
		}
	}

	// --- 极度严格的止损逻辑 ---
	// 即使是小幅亏损，也应严格执行
	if uplRatio < -0.03 {
		return ActClose
	}

	// --- 低吸（加仓）逻辑 ---
	// 在价格回调到相对低位，且信号与持仓方向一致时低吸加仓
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "buy") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "sell") {

		// 只有在回调时才低吸，确保价格低于你的平均成本价
		price := ctx.Sig.Price
		avgPrice := ctx.Pos.AvgPrice

		if price < avgPrice && ctx.Sig.Strength >= 0.3 { // 信号强度要求可以放低
			return ActAddSmall // 小幅加仓
		}
	}

	// --- 低吸（加仓）逻辑 ---
	// 在价格回调到相对低位，且信号与持仓方向一致时低吸加仓
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "buy") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "sell") {

		price := ctx.Sig.Price
		avgPrice := ctx.Pos.AvgPrice

		// 只有在价格大幅低于平均成本时才低吸
		if price < avgPrice*0.99 && ctx.Sig.Strength >= 0.5 { // 例如，价格低于平均成本 1%
			return ActAddSmall
		}
	}

	// 否则，保持仓位不变
	return ActIgnore
}

// 趋势反转
func (de *DecisionEngine) handleReversal() Action {
	ctx := de.Ctx
	// 1. 如果没有持仓，立即返回 ActIgnore
	if ctx.Pos == nil {
		return ActIgnore
	}

	posDir := ctx.Pos.Dir
	uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

	// 2. 紧急情况：如果浮亏严重，立即止损，优先级最高
	if uplRatio < -0.05 {
		return ActClose
	}

	// 3. 根据趋势和信号的严重程度决定平仓或减仓

	// 情况 A：大趋势确认反转，果断平仓
	// 例如，你持有多单，但现在趋势分数转为负值，斜率也为负
	if (posDir == model.OrderPosSideLong && ctx.Trend.Scores.FinalScore < 0) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Scores.FinalScore > 0) {

		if uplRatio >= 0 { // 如果在盈利，立即平仓锁定利润
			return ActClose
		} else { // 如果在亏损，降低止损幅度，快速离场
			if uplRatio < -0.02 {
				return ActClose
			}
		}
	}

	// 情况 B：15分钟信号反转，平仓或减仓
	// 例如，你持有多单，但15分钟信号已经出现卖出信号
	if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell") ||
		(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy") {

		// 如果信号强度足够高，并且是盈利状态，则平仓
		if ctx.Sig.Strength >= 0.7 && uplRatio > 0.05 {
			return ActClose
		}

		// 如果信号强度足够高，并且是小幅亏损或盈利，则减仓
		if ctx.Sig.Strength >= 0.5 {
			return ActReduce
		}
	}

	// 4. 如果平仓条件不满足，但趋势动能开始衰竭，考虑减仓
	if (posDir == model.OrderPosSideLong && ctx.Trend.Slope < 0) ||
		(posDir == model.OrderPosSideShort && ctx.Trend.Slope > 0) {

		// 只有在盈利时才考虑减仓
		if uplRatio > 0 {
			return ActReduce
		}
	}

	return ActIgnore
}

func (de *DecisionEngine) handleTrend(op TrendOperator) Action {
	ctx := de.Ctx
	// 1. 风险控制（先止损，再部分止盈，再大止盈 ）
	if ctx.Pos != nil {
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

		// IMPORTANT: uplRatio 是小数（例如 0.1 = 10%），阈值应结合杠杆和 ATR 设置
		// 止损：亏损超过阈值（这里给出更宽的默认值，避免被短期回撤洗掉）
		if uplRatio < -0.10 { // 把 -0.05 放宽为 -0.10（30x杠杆可再调）
			return ActClose
		}
		// 大止盈：如果盈利已经很高，优先了结
		if uplRatio > 0.30 {
			return ActClose
		}
		// 部分止盈：中等盈利时减仓锁利
		if uplRatio > 0.18 {
			return ActReduce
		}

		// --- 新增：严格处理短期反转信号（IsReversal） ---
		// 不再对任何 IsReversal 立即减仓，必须满足：持仓有盈利 + reversal 强度 >= thr + 高级确认（higher timeframe）
		if ctx.Sig.IsReversal {
			// 只在满足这些条件时才减仓：1) 有一定盈利；2) 信号强度高；3) 高级别趋势也显示弱化或 detector 触发
			reversalStrength := ctx.Sig.ReversalStrength
			uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
			// 趋势是否转弱
			isWeek :=
				detector.CheckWeak(ctx.Sig.Symbol, ctx.Pos.Dir, ctx.Trend.Scores.FinalScore, ctx.Trend.Slope)

			// 当盈利、信号高强度、趋势转弱时减仓
			if uplRatio > 0.02 && reversalStrength >= 0.65 && isWeek {
				return ActReduce
			}
			// 否则把短期反转视作回调信号，先忽略，避免追高被洗
		}
	}

	// 获取均线价格
	ema30, ema30OK := ctx.Sig.Values["EMA30"]
	rsiValue, rsiOK := ctx.Sig.Values["RSI"]
	if !ema30OK || !rsiOK {
		return ActIgnore
	}
	// 2. 根据信号进行开仓或加仓
	// 使用“弱信号开仓，强信号作为减仓/止盈参考”，此前强信号开仓一直开在最高点
	if op.isSignalWithTrend(model.OrderSide(ctx.Sig.Side), ctx.Trend.Direction) {
		// 无仓位时开仓
		// 无仓位时开仓 — 只允许在弱/中等强度下开仓（避免追高）
		if ctx.Pos == nil {
			// 允许开仓的 strength 区间： [0.25, 0.55]（可以回测调参）
			if ctx.Sig.Strength >= 0.25 && ctx.Sig.Strength <= 0.55 && op.isMomentumPositive(ctx.Trend.Slope) {
				// 严格要求价格接近均线或 RSI 支持（避免在独立大阳/大阴上开仓）
				if ctx.Sig.Side == "buy" && (ctx.Sig.Price <= ema30*1.01 || rsiValue < 40) {
					// 另外限制同一 symbol 一定时间内只能开一次
					detector.UpdateState(ctx.Sig.Symbol)
					return ActOpen
				}
				if ctx.Sig.Side == "sell" && (ctx.Sig.Price >= ema30*0.99 || rsiValue > 60) {
					detector.UpdateState(ctx.Sig.Symbol)
					return ActOpen
				}
			}
		} else {

			//
			// --- 趋势不同 有仓位时 ---
			uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)
			// 1) 趋势减弱
			if detector.CheckWeak(ctx.Sig.Symbol, ctx.Pos.Dir, ctx.Trend.Scores.FinalScore, ctx.Trend.Slope) {
				// 只有在盈利或信号强度确认时才做实减仓

				if uplRatio > 0.01 || ctx.Sig.Strength >= 0.6 {
					return ActReduce
				}
			}

			// 2) 强信号出现在超买/超卖区域 -> 减仓（保护利润）
			if (ctx.Sig.Side == "buy" && ctx.Pos.Dir == "long" && ctx.Sig.Strength >= 0.7 && rsiValue > 70) || // 超卖
				(ctx.Sig.Side == "sell" && ctx.Pos.Dir == "short" && ctx.Sig.Strength >= 0.7 && rsiValue < 30) { // 超买
				return ActReduce
			}

			// 3) 加仓逻辑：严格回调补仓和趋势延续加仓分开
			// 回调补仓：只有当价格较上次信号有小幅回撤（非追高），并且总加仓次数未超过上限
			if ctx.Sig.Strength > 0.35 && op.isMomentumPositive(ctx.Trend.Slope) {
				if !op.isPriceHigher(ctx.Sig.Price, ctx.LastSig.Price) &&
					math.Abs(ctx.LastSig.Price-ctx.Sig.Price)/ctx.LastSig.Price < 0.015 && // 放宽到1.5%回调容差
					detector.IsAllowAdd(ctx.Sig.Symbol) { // 限制加仓次数
					return ActAddSmall
				}
				// 趋势延续加仓：只有当 price 新高 且 slope 显著
				if op.isPriceHigher(ctx.Sig.Price, ctx.LastSig.Price) &&
					math.Abs(ctx.Trend.Slope) > 0.12 &&
					detector.IsAllowAdd(ctx.Sig.Symbol) {
					// 但避免在非常强的单根 K 线开加仓（防止追当根阳）
					if !IsSingleCandleSpike(ctx.Line, 2.0) {
						return ActAdd
					}
				}
			}

			// --- 趋势减弱减仓 ---
			if detector.CheckWeak(ctx.Sig.Symbol, ctx.Pos.Dir, ctx.Trend.Scores.FinalScore, ctx.Trend.Slope) {
				return ActReduce
			}
		}
	}

	// 3. 兜底：当亏损变大且出现明确反向信号时，进行保护性减仓或平仓
	if ctx.Pos != nil {
		posDir := ctx.Pos.Dir
		uplRatio, _ := strconv.ParseFloat(ctx.Pos.UplRatio, 64)

		// 如果亏损超过阈值并出现强反向信号 -> 先减仓再考虑平仓
		if uplRatio < -0.03 {
			if (posDir == model.OrderPosSideLong && ctx.Sig.Side == "sell" && ctx.Sig.Strength >= 0.6) ||
				(posDir == model.OrderPosSideShort && ctx.Sig.Side == "buy" && ctx.Sig.Strength >= 0.6) {
				return ActReduce
			}
		}
	}

	return ActIgnore
}

// TrendOperator 定义了与方向相关的操作
type TrendOperator struct {
	// 动量的判断函数
	isMomentumPositive func(slope float64) bool
	// 价格比较函数
	isPriceHigher func(p1, p2 float64) bool
	// 信号与趋势方向是否相同
	isSignalWithTrend func(signalSide model.OrderSide, trendDir trend.TrendDirection) bool
}

// 获取看涨 TrendOperator
func getBullishOperator() TrendOperator {
	return TrendOperator{
		isMomentumPositive: func(slope float64) bool { return slope > 0 },
		isPriceHigher:      func(p1, p2 float64) bool { return p1 > p2 },
		isSignalWithTrend: func(signalSide model.OrderSide, trendDir trend.TrendDirection) bool {
			return signalSide == model.Buy && trendDir == trend.TrendUp
		},
	}
}

// 获取看跌信号的操作
func getBearishOperator() TrendOperator {
	return TrendOperator{
		isMomentumPositive: func(slope float64) bool { return slope < 0 },
		isPriceHigher:      func(p1, p2 float64) bool { return p1 < p2 }, // 价格越低，越是“高于”看跌趋势
		isSignalWithTrend: func(signalSide model.OrderSide, trendDir trend.TrendDirection) bool {
			return signalSide == model.Sell && trendDir == trend.TrendDown
		},
	}
}

// 保存每一个趋势减弱的状态
type DeclineState struct {
	AddCount       int       // 加仓次数
	LastActionTime time.Time // 上一次执行时间（开仓/加仓）
	WeakSigCount   int       //连续弱信号次数
}

// 趋势减弱检测器
type WeakTrendDetector struct {
	declineCounts map[string]*DeclineState // 每个 symbol 对应的衰弱计数
	threshold     int                      // 连续次数阈值
	scoreTh       float64                  // FinalScore 阈值
	slopeTh       float64                  // Slope 阈值
	interval      time.Duration
	maxAdds       int // 最多允许的加仓次数
}

// 初始化
func NewWeakTrendDetector(threshold int, scoreTh, slopeTh float64, interval time.Duration) *WeakTrendDetector {
	return &WeakTrendDetector{
		declineCounts: make(map[string]*DeclineState),
		threshold:     threshold,
		scoreTh:       scoreTh,
		slopeTh:       slopeTh,
		interval:      interval,
		maxAdds:       5,
	}
}

// RecentActionTooFrequent 检查是否距离上一次操作太近
func (w *WeakTrendDetector) RecentActionTooFrequent(symbol string) bool {
	state, ok := w.declineCounts[symbol]
	if !ok || state.LastActionTime.IsZero() {
		return false
	}
	return time.Since(state.LastActionTime) < w.interval
}

// UpdateState 更新 symbol 状态（执行开仓/加仓后调用）
func (w *WeakTrendDetector) UpdateState(symbol string) {
	state, ok := w.declineCounts[symbol]
	if !ok {
		state = &DeclineState{}
		w.declineCounts[symbol] = state
	}
	state.AddCount++
	state.LastActionTime = time.Now()
}

// ResetState 在行情明显变化或平仓后调用，重置状态
func (w *WeakTrendDetector) ResetState(symbol string) {
	delete(w.declineCounts, symbol)
}

// CanOpenNewPosition 判断能否添加仓位
func (w *WeakTrendDetector) IsAllowAdd(symbol string) bool {
	if w.RecentActionTooFrequent(symbol) {
		return false
	}
	state, ok := w.declineCounts[symbol]
	if !ok {
		return true // 没有记录，允许开仓
	}
	return state.AddCount >= w.threshold
}

// Check 判断某个 symbol 的趋势是否衰弱
// - posDir: 当前持仓方向 (long/short)
// - finalScore: 当前趋势分数
// - slope: 当前趋势斜率
// 返回 true 表示趋势连续衰弱，触发减仓/平仓信号
func (w *WeakTrendDetector) CheckWeak(symbol string, posDir model.OrderPosSide, finalScore, slope float64) bool {
	state := w.declineCounts[symbol]
	if state == nil {
		state = &DeclineState{}
	}

	if posDir == model.OrderPosSideLong {
		// 多单衰弱条件：趋势分数偏弱 且 动能转负
		if finalScore < w.scoreTh && slope < -w.slopeTh {
			state.WeakSigCount++
		} else {
			state.WeakSigCount = 0
		}
	} else if posDir == model.OrderPosSideShort {
		// 空单衰弱条件：趋势分数偏强 且 动能转正
		if finalScore > -w.scoreTh && slope > w.slopeTh {
			state.WeakSigCount++
		} else {
			state.WeakSigCount = 0
		}
	}

	// 更新缓存
	w.declineCounts[symbol] = state

	if state.WeakSigCount >= w.threshold {
		// 连续满足才触发一次，触发后清零
		state.WeakSigCount = 0
		w.declineCounts[symbol] = state
		return true
	}
	return false
}

// 是否是单根 K 线的极端插针（spike）
// 用于避免被异常波动误导开仓/止损
func IsSingleCandleSpike(kline model.Kline, factor float64) bool {
	// factor 建议 1.5 ~ 2.0，用来判断上下影线是否过长
	body := math.Abs(kline.Close - kline.Open)
	upperWick := kline.High - math.Max(kline.Close, kline.Open)
	lowerWick := math.Min(kline.Close, kline.Open) - kline.Low

	if upperWick > body*factor || lowerWick > body*factor {
		return true
	}
	return false
}
