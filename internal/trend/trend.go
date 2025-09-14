package trend

import (
	"edgeflow/internal/exchange"
	model2 "edgeflow/internal/model"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	talib "github.com/markcheno/go-talib"
	"github.com/nntaoli-project/goex/v2/model"
)

// TrendManager 负责管理多个币种的趋势状态
type Manager struct {
	mu       sync.RWMutex
	machines map[string]*StateMachine

	ex           exchange.Exchange // OKX 客户端
	symbols      []string
	cfg          TrendCfg
	klineManager *KlineManager
}

func NewManager(ex exchange.Exchange, symbols []string, klineManager *KlineManager) *Manager {
	return &Manager{
		machines:     make(map[string]*StateMachine),
		ex:           ex,
		symbols:      symbols,
		cfg:          DefaultTrendCfg(),
		klineManager: klineManager,
	}
}

// 启动调度：独立于 15min 信号
func (tm *Manager) RunScheduled() {

	// 更新趋势
	tm.computeTrends(tm.symbols)
}

// 等待第一个对齐点（比如整30m）
func (tm *Manager) waitForNextAlignment() {
	now := time.Now()
	next := now.Truncate(15 * time.Minute).Add(15 * time.Minute)
	// 再加30秒确保交易所数据更新完成
	waitUntil := next.Add(30 * time.Second)
	sleep := time.Until(waitUntil)
	log.Printf("[Trend] 等待到 %s 开始调度", next.Format("15:04:05"))
	time.Sleep(sleep)
}

// 为所有交易对计算趋势
func (tm *Manager) computeTrends(symbols []string) {
	var wg sync.WaitGroup

	// 并发处理多个交易对，但限制并发数
	semaphore := make(chan struct{}, 3) // 最多3个并发

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()

			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			err := tm.computeTrend(sym)
			if err != nil {
				log.Printf("[Trend] %s 计算趋势失败: %+v", sym, err)
			}
		}(symbol)
	}

	wg.Wait()
	log.Println("本轮信号分析完成")
}

func (tm *Manager) computeTrend(symbol string) error {
	// 在自己的 Goroutine 中处理
	state, slope, err := tm.GenerateTrend(symbol)
	if err != nil {
		return err
	}
	// 获取状态机
	machine := tm.GetStateMachine(symbol)
	if machine == nil {
		// 初始化币种的状态机
		tm.machines[symbol] = NewStateMachine(symbol)
	}
	machine.Update(state.Scores.FinalScore, state.Scores.TrendScore, slope)
	fmt.Println(state.Description())
	return nil
}

func (tm *Manager) GenerateTrend(symbol string) (state *TrendState, finalSlope *float64, err error) {
	h4Klines, ok4 := tm.klineManager.Get(symbol, model.Kline_4h)
	if ok4 == false {
		log.Printf("[TrendManager] fetch 4hour kline error for %s", symbol)
		return nil, nil, err
	}
	h1Klines, ok1 := tm.klineManager.Get(symbol, model.Kline_1h)
	if ok1 == false {
		log.Printf("[TrendManager] fetch 1hour kline error for %s", symbol)
		return nil, nil, err
	}
	m30Klines, ok30 := tm.klineManager.Get(symbol, model.Kline_30min)
	if ok30 == false {
		log.Printf("[TrendManager] fetch 30m kline error for %s", symbol)
		return nil, nil, err
	}

	//tm.genCSV(symbol, tm.interval, latestFirst)

	// ------------------ 2. 计算各周期指标分数 ------------------
	s30m, _ := tm.ScoreForPeriod(m30Klines)
	s1h, _ := tm.ScoreForPeriod(h1Klines)
	s4h, _ := tm.ScoreForPeriod(h4Klines)

	// 加权平均，权重可调
	scores := tm.calcTrendScores(s4h, s1h, s30m)

	// ------------------ 4. 多周期趋势方向判定 ------------------
	// 趋势方向
	dir := TrendNeutral

	if scores.FinalScore >= 1.0 {
		dir = TrendUp
	} else if scores.FinalScore <= -1.0 {
		dir = TrendDown
	}

	closes1H := make([]float64, len(h1Klines))
	highs1H := make([]float64, len(h1Klines))
	lows1H := make([]float64, len(h1Klines))

	for i, line := range m30Klines {
		closes1H[i] = line.Close
		highs1H[i] = line.High
		lows1H[i] = line.Low
	}

	// ------------------ 5. 构建TrendState ------------------
	atr1H := talib.Atr(highs1H, lows1H, closes1H, 14)
	adx1H := talib.Adx(highs1H, lows1H, closes1H, 14)
	rsi1H := talib.Rsi(closes1H, 14)

	last := h1Klines[len(m30Klines)-1]

	state = &TrendState{
		Symbol:    symbol,
		Direction: dir,
		ATR:       atr1H[len(atr1H)-1],
		ADX:       atr1H[len(adx1H)-1],
		RSI:       rsi1H[len(atr1H)-1],
		LastPrice: last.Close,
		Timestamp: last.Timestamp,
		Scores:    scores,
	}

	// 计算加权平均分数，可以给越新的趋势权重越高
	tm.save(state)

	machine := tm.machines[symbol]
	slope := NewTrendSlope(machine.StatesCaches)

	if slope != nil {
		// 计算最终斜率方向
		finalSlope := tm.weightedScore(slope.Slope4h, slope.Slope1h, slope.Slope30m)
		state.Slope = finalSlope
		return state, &finalSlope, nil
	}

	return state, nil, nil
}

// 计算数组均值
func mean(arr []float64) float64 {
	if len(arr) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range arr {
		sum += v
	}
	return sum / float64(len(arr))
}

// 使用加权最小二乘法计算分数数组的斜率
// arr: 趋势分数数组，顺序为从旧到新
// 返回值: 拟合直线的斜率
func calcSlope(arr []float64) float64 {
	n := float64(len(arr))
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumXX, sumW float64
	for i, y := range arr {
		x := float64(i)
		// 动态计算权重。i越大（越新），权重越高。
		// 这里的权重因子可以根据你的需要调整，例如使用指数衰减
		w := math.Exp(x / n) // 简单示例：指数加权，让最近的数据影响更大

		sumX += w * x
		sumY += w * y
		sumXY += w * x * y
		sumXX += w * x * x
		sumW += w
	}
	// 最小二乘法公式的加权版本
	denom := sumW*sumXX - sumX*sumX
	if math.Abs(denom) < 1e-9 {
		return 0
	}

	return (sumW*sumXY - sumX*sumY) / denom
}

// 加权总分
func (tm *Manager) weightedScore(s4h, s1h, s30 float64) float64 {
	// 基础权重
	w4h, w1h, w30 := 0.4, 0.3, 0.3

	// 优先级最高的逻辑: 30m 与 4h 背离，可能是反转/回调信号
	if (s30 > 0 && s4h < 0) || (s30 < 0 && s4h > 0) {
		w30 = 0.6 // 大幅增加短线权重
		w1h = 0.2
		w4h = 0.2
	} else if (s30 > 0 && s1h > 0 && s4h > 0) || (s30 < 0 && s1h < 0 && s4h < 0) {
		// 所有时间框架方向一致，这是最强的趋势信号
		w4h = 0.5 // 增加长线权重，趋势最可靠
		w1h = 0.3
		w30 = 0.2
	} else if (s30 > 0 && s1h > 0) || (s30 < 0 && s1h < 0) {
		// 短线和中线一致，但与长线不一致，可能趋势正在形成
		w30 = 0.4
		w1h = 0.4
		w4h = 0.2
	}

	// 归一化
	sum := w4h + w1h + w30
	w4h /= sum
	w1h /= sum
	w30 /= sum

	return w4h*s4h + w1h*s1h + w30*s30
}

func (tm *Manager) calcTrendScores(s4h, s1h, s30 float64) TrendScores {
	// --- 大趋势权重 ---
	w4h, w1h := 0.6, 0.4
	trendScore := w4h*s4h + w1h*s1h

	// --- 短线信号 ---
	signalScore := s30

	// --- 综合分 ---
	final := tm.weightedScore(s4h, s1h, s30)

	return TrendScores{
		TrendScore:  trendScore,
		SignalScore: signalScore,
		FinalScore:  final,
		Score30m:    s30,
		Score1h:     s1h,
		Score4h:     s4h,
	}
}

// 计算周期趋势分数 -3 ~ +3（方向化 + 抖动抑制）
func (tm *Manager) ScoreForPeriod(klines []model2.Kline) (float64, string) {
	if len(klines) < 200 {
		return 0, ""
	}

	n := len(klines)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	last := len(closes) - 1
	price := closes[last]

	ema20 := talib.Ema(closes, 20)
	ema50 := talib.Ema(closes, 50)
	ema200 := talib.Ema(closes, 200)
	adx := talib.Adx(highs, lows, closes, 14)
	upper, middle, lower := talib.BBands(closes, 20, 2, 2, 0)
	kVals, dVals := talib.Stoch(highs, lows, closes, 9, 3, talib.SMA, 3, talib.SMA)

	ema20Last := ema20[last]
	ema50Last := ema50[last]
	ema200Last := ema200[last]
	adxLast := adx[last]
	bbWidthLast := (upper[last] - lower[last]) / middle[last]

	var bbSum float64
	var count int
	for i := last - 50; i < last; i++ {
		if i >= 0 {
			bbSum += (upper[i] - lower[i]) / middle[i]
			count++
		}
	}
	bbWidthAvg := bbSum / float64(count)

	// === 打分 ===
	score := 0.0
	reasons := []string{}

	// 价格 vs EMA200
	if price > ema200Last {
		score += 1
		reasons = append(reasons, "+1(价格>EMA200)")
	} else {
		score -= 1
		reasons = append(reasons, "-1(价格<EMA200)")
	}

	// 均线排列 + 斜率
	slope20 := ema20Last - ema20[last-3]
	slope50 := ema50Last - ema50[last-3]
	if ema20Last > ema50Last && slope20 > 0 && slope50 > 0 {
		score += 1
		reasons = append(reasons, "+1(EMA20>EMA50且向上)")
	} else if ema20Last < ema50Last && slope20 < 0 && slope50 < 0 {
		score -= 1
		reasons = append(reasons, "-1(EMA20<EMA50且向下)")
	}

	// ADX趋势强度
	//if adxLast > 25 {
	//	score += 1
	//	reasons = append(reasons, "+1(ADX强趋势)")
	//} else if adxLast < 20 {
	//	score -= 0.5
	//	reasons = append(reasons, "-0.5(ADX弱趋势)")
	//}
	// ADX 趋势强度
	if adxLast > 25 {
		if score > 0 {
			score += 1 // 多头趋势加强
			reasons = append(reasons, "+1(ADX强趋势)")
		} else if score < 0 {
			score -= 1 // 空头趋势加强
			reasons = append(reasons, "+1(ADX弱趋势)")
		} else {
			// 横盘也可以略微增加权重
			score += 0.5
			reasons = append(reasons, "+0.5(ADX强在横盘时略微增加权重)")
		}
	} else if adxLast < 20 {
		score -= 0.5
		reasons = append(reasons, "-0.5(ADX弱减弱趋势)")
	}
	// 布林带收窄 → 横盘
	if bbWidthLast < bbWidthAvg*0.7 {
		score -= 0.5
		reasons = append(reasons, "-0.5(布林收窄横盘)")
	}
	// kdj 金叉/死叉
	kVals, dVals, jVals := CalcKDJ(highs, lows, closes)
	if IsGoldenCross(kVals, dVals) {
		score += 0.5 // 金叉看多
		reasons = append(reasons, "+0.5(KDJ金叉)")
	}
	if IsDeadCross(kVals, dVals) {
		score -= 0.5 // 死叉看空
		reasons = append(reasons, "+0.5(KDJ金叉)")
	}

	//  J 值极端情况 ===
	jNow := jVals[last]
	if jNow > 100 {
		score -= 0.5 // 极端超买，防止追高
		reasons = append(reasons, "-0.5(J>100超买)")
	}
	if jNow < 0 {
		score += 0.5 // 极端超卖，防止错过反弹
		reasons = append(reasons, "+0.5(J<0超卖)")
	}

	// MACD 参数 (12, 26, 9) 常见用法
	macdVals, _, _ := talib.Macd(closes, 12, 26, 9)
	// MACD 背离
	divScore, divReason := CheckMacdDivergence(closes, macdVals, 30)
	score += divScore
	if divReason != "" {
		reasons = append(reasons, divReason)
	}

	// RSI 背离
	rsiVals := talib.Rsi(closes, 14)
	rsiDivScore, rsiDivReason := CheckRsiDivergence(closes, rsiVals, 30)
	score += rsiDivScore
	if rsiDivReason != "" {
		reasons = append(reasons, rsiDivReason)
	}

	// 水下金叉
	//cross := IsWaterMACDGoldenCross(closes)

	if score > 3 {
		score = 3
	}
	if score < -3 {
		score = -3
	}
	// === 调试日志 ===
	logs := fmt.Sprintf("Score=%.2f 详情: %v", score, strings.Join(reasons, ", "))
	return score, logs
}

// 更新某币种趋势（内部 & 外部都可调用）
func (tm *Manager) save(state *TrendState) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	stateMachine := tm.machines[state.Symbol]
	if stateMachine == nil {
		// 初始化币种的状态机
		stateMachine = NewStateMachine(state.Symbol)
	}
	newStates := append(stateMachine.StatesCaches, state)
	if len(newStates) >= 14 {
		// 移除索引0的元素
		newStates = newStates[1:]
	}
	stateMachine.StatesCaches = newStates
	tm.machines[state.Symbol] = stateMachine
}

// 获取某币种趋势
func (tm *Manager) GetState(symbol string) *TrendState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	machine := tm.machines[symbol]
	states := machine.StatesCaches
	if len(states) > 0 {
		// 返回最新的
		return states[len(states)-1]
	}
	return nil
}

func (tm *Manager) GetStateMachine(symbol string) *StateMachine {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	machine := tm.machines[symbol]
	return machine
}

// 计算KDJ
func CalcKDJ(highs, lows, closes []float64) ([]float64, []float64, []float64) {
	// 使用talib自带的随机指标 (Stochastic Oscillator)
	k, d := talib.Stoch(
		highs,
		lows,
		closes,
		9, // n
		3, // k smoothing
		talib.SMA,
		3, // d smoothing
		talib.SMA,
	)

	j := make([]float64, len(k))
	for i := range k {
		j[i] = 3*k[i] - 2*d[i]
	}
	return k, d, j
}

// 判断是否金叉
func IsGoldenCross(k, d []float64) bool {
	n := len(k)
	if n < 2 {
		return false
	}
	return k[n-2] < d[n-2] && k[n-1] > d[n-1]
}

// 判断是否死叉
func IsDeadCross(k, d []float64) bool {
	n := len(k)
	if n < 2 {
		return false
	}
	return k[n-2] > d[n-2] && k[n-1] < d[n-1]
}

// CheckMacdDivergence 检测 MACD 顶背离/底背离
// 返回 (score, reason)
// 底背离 → +1.0, 顶背离 → -1.0, 否则 0
func CheckMacdDivergence(closes, macdVals []float64, lookback int) (float64, string) {
	last := len(closes) - 1
	if len(closes) < lookback+5 || len(macdVals) < lookback+5 {
		return 0, ""
	}

	// 找最近 lookback 区间内价格高低点
	priceHigh, priceHighIdx := closes[last], last
	priceLow, priceLowIdx := closes[last], last
	for i := last - lookback; i < last; i++ {
		if closes[i] > priceHigh {
			priceHigh = closes[i]
			priceHighIdx = i
		}
		if closes[i] < priceLow {
			priceLow = closes[i]
			priceLowIdx = i
		}
	}

	// 找最近 lookback 区间内 MACD 高低点
	macdHigh, macdHighIdx := macdVals[last], last
	macdLow, macdLowIdx := macdVals[last], last
	for i := last - lookback; i < last; i++ {
		if macdVals[i] > macdHigh {
			macdHigh = macdVals[i]
			macdHighIdx = i
		}
		if macdVals[i] < macdLow {
			macdLow = macdVals[i]
			macdLowIdx = i
		}
	}

	// === 背离判断 ===
	// 底背离：价格创新低，但 MACD 没创新低
	if priceLowIdx > macdLowIdx &&
		closes[priceLowIdx] < closes[macdLowIdx] &&
		macdVals[priceLowIdx] > macdVals[macdLowIdx] {
		return 1.0, "+1.0(MACD底背离)"
	}

	// 顶背离：价格创新高，但 MACD 没创新高
	if priceHighIdx > macdHighIdx &&
		closes[priceHighIdx] > closes[macdHighIdx] &&
		macdVals[priceHighIdx] < macdVals[macdHighIdx] {
		return -1.0, "-1.0(MACD顶背离)"
	}

	return 0, ""
}

// CheckRsiDivergence 检测 RSI 顶背离/底背离
// 返回 (score, reason)
// 底背离 → +0.5, 顶背离 → -0.5, 否则 0
func CheckRsiDivergence(closes, rsiVals []float64, lookback int) (float64, string) {
	last := len(closes) - 1
	if len(closes) < lookback+5 || len(rsiVals) < lookback+5 {
		return 0, ""
	}

	// 找最近 lookback 区间内价格高低点
	priceHigh, priceHighIdx := closes[last], last
	priceLow, priceLowIdx := closes[last], last
	for i := last - lookback; i < last; i++ {
		if closes[i] > priceHigh {
			priceHigh = closes[i]
			priceHighIdx = i
		}
		if closes[i] < priceLow {
			priceLow = closes[i]
			priceLowIdx = i
		}
	}

	// 找最近 lookback 区间内 RSI 高低点
	rsiHigh, rsiHighIdx := rsiVals[last], last
	rsiLow, rsiLowIdx := rsiVals[last], last
	for i := last - lookback; i < last; i++ {
		if rsiVals[i] > rsiHigh {
			rsiHigh = rsiVals[i]
			rsiHighIdx = i
		}
		if rsiVals[i] < rsiLow {
			rsiLow = rsiVals[i]
			rsiLowIdx = i
		}
	}

	// === 背离判断 ===
	// 底背离：价格创新低，但 RSI 没创新低
	if priceLowIdx > rsiLowIdx &&
		closes[priceLowIdx] < closes[rsiLowIdx] &&
		rsiVals[priceLowIdx] > rsiVals[rsiLowIdx] {
		return 0.5, "+0.5(RSI底背离)"
	}

	// 顶背离：价格创新高，但 RSI 没创新高
	if priceHighIdx > rsiHighIdx &&
		closes[priceHighIdx] > closes[rsiHighIdx] &&
		rsiVals[priceHighIdx] < rsiVals[rsiHighIdx] {
		return -0.5, "-0.5(RSI顶背离)"
	}

	return 0, ""
}

// 水下金叉：MACD 线从下往上穿过信号线，但两者都在零轴下方
// 水下金叉通常意味着：
// 价格在空头区域经历一定下跌后的 短期反弹信号
// 可能是 低位买入机会，但趋势整体仍偏空
// 相比零轴以上的金叉，水下金叉 更弱，但风险/收益比相对高。
func IsWaterMACDGoldenCross(closes []float64) bool {
	if len(closes) < 26+9 { // EMA计算长度保证
		return false
	}

	macdVals, signalVals, _ := talib.Macd(closes, 12, 26, 9)
	last := len(macdVals) - 1

	// 水下金叉条件
	if macdVals[last-1] < signalVals[last-1] &&
		macdVals[last] > signalVals[last] &&
		macdVals[last] < 0 && signalVals[last] < 0 {
		return true
	}
	return false
}
