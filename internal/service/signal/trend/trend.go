package trend

import (
	"context"
	model2 "edgeflow/internal/model"
	"edgeflow/internal/service/signal/kline"
	model3 "edgeflow/internal/service/signal/model"
	"edgeflow/pkg/exchange"
	"errors"
	"fmt"
	"github.com/markcheno/go-talib"
	"github.com/nntaoli-project/goex/v2/model"
	"log"
	"math"
	"strings"
	"sync"
)

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

// TrendManager 负责管理多个币种的趋势状态
type Manager struct {
	mu       sync.RWMutex
	machines map[string]*StateMachine

	ex           exchange.Exchange // OKX 客户端
	cfg          TrendCfg
	klineManager *kline.KlineManager

	SymbolMgr *model3.SymbolManager
}

func NewManager(ex exchange.Exchange, klineManager *kline.KlineManager, symbolMgr *model3.SymbolManager) *Manager {
	return &Manager{
		machines:     make(map[string]*StateMachine),
		ex:           ex,
		cfg:          DefaultTrendCfg(),
		klineManager: klineManager,
		SymbolMgr:    symbolMgr,
	}
}

// 接收并监听k线更新通道，驱动趋势生成
func (s *Manager) StartListening(ctx context.Context, updateTrendCh <-chan struct{}, signalInputCh chan<- struct{}) {
	// TrendMgr 启动逻辑保持不变 (在外部 main.go 中处理)

	// 启动趋势处理核心循环
	go s.runTrendLoop(ctx, updateTrendCh, signalInputCh)
}

// 现在负责接收事件，并并发处理所有 symbols
func (s *Manager) runTrendLoop(ctx context.Context, updateKlineCh <-chan struct{}, signalInputCh chan<- struct{}) {
	fmt.Println("[trend.Manager runTrendLoop]启动趋势处理器 (监听 K 线更新趋势)...")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("信号处理器退出。")
			return
		case <-updateKlineCh:
			// K 线对齐事件触发！

			symbols := s.SymbolMgr.GetSymbols() // 获取当前所有活跃符号
			if len(symbols) == 0 {
				continue
			}

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, 5) // 控制并发数，例如 5 个

			// 循环并并发处理所有交易对的信号生成和过滤
			for _, symbol := range symbols {
				wg.Add(1)
				go func(sym string) {
					defer wg.Done()
					semaphore <- struct{}{}        // 获取信号量
					defer func() { <-semaphore }() // 释放信号量

					s.processTrendSymbol(sym) // 封装核心逻辑
				}(symbol)
			}
			wg.Wait() // 等待所有符号处理完成

			select {
			case signalInputCh <- struct{}{}:
			default:
				fmt.Println("[Manager]通道堵塞放弃本次发送signalInputCh")
			}

			fmt.Println("本轮趋势分析全部完成。")
		}
	}
}

func (tm *Manager) processTrendSymbol(symbol string) error {
	// 1.生成趋势状态
	state, err := tm.generateTrend(symbol)
	if err != nil {
		return err
	}
	tm.mu.Lock()

	// 2.获取状态机
	machine := tm.getStateMachine(symbol)
	if machine == nil {
		// 初始化币种的状态机
		machine = NewStateMachine(symbol)
		tm.machines[symbol] = machine
	}

	// 3.使用分数更新状态机
	machine.Update(state.Scores.FinalScore, state.Scores.TrendScore)
	tm.mu.Unlock()
	// 4.将状态机设置的最终方向赋值给 TrendState (以便 TrendState 存储正确的 Direction)
	state.Direction = machine.CurrentState

	// 5.报错最新状态 // 不存储趋势，只有策略匹配后再存储
	tm.save(state)

	fmt.Println(state.Description())
	return nil
}

func (tm *Manager) generateTrend(symbol string) (state *model3.TrendState, err error) {
	h4Klines, ok4 := tm.klineManager.Get(symbol, model.Kline_4h)
	if ok4 == false {
		errStr := fmt.Sprintf("[TrendManager] fetch 4hour kline error for %s", symbol)
		log.Println(errStr)
		return nil, errors.New(errStr)
	}
	h1Klines, ok1 := tm.klineManager.Get(symbol, model.Kline_1h)
	if ok1 == false {
		errStr := fmt.Sprintf("[TrendManager] fetch 1hour kline error for %s", symbol)
		log.Println(errStr)
		return nil, errors.New(errStr)
	}
	m30Klines, ok30 := tm.klineManager.Get(symbol, model.Kline_30min)
	if ok30 == false {
		errStr := fmt.Sprintf("[TrendManager] fetch 30m kline error for %s", symbol)
		log.Println(errStr)
		return nil, errors.New(errStr)
	}

	//tm.genCSV(symbol, tm.interval, latestFirst)

	// ------------------ 2. 计算各周期指标分数 ------------------
	s30m, indicatorS30m := tm.ScoreForPeriod(m30Klines, model.Kline_30min)
	s1h, indicatorS1h := tm.ScoreForPeriod(h1Klines, model.Kline_1h)
	s4h, indicatorS4h := tm.ScoreForPeriod(h4Klines, model.Kline_4h)

	var indicator = make(map[model.KlinePeriod]model3.IndicatorSnapshot)
	indicator[model.Kline_30min] = indicatorS30m
	indicator[model.Kline_1h] = indicatorS1h
	indicator[model.Kline_4h] = indicatorS4h

	// 加权平均，权重可调
	scores := tm.calcTrendScores(s4h, s1h, s30m)

	// ------------------ 4. 多周期趋势方向判定 ------------------
	// 趋势方向
	dir := model3.TrendNeutral

	if scores.FinalScore >= 1.0 {
		dir = model3.TrendUp
	} else if scores.FinalScore <= -1.0 {
		dir = model3.TrendDown
	}

	closes30m := make([]float64, len(m30Klines))
	highs30m := make([]float64, len(m30Klines))
	low30m := make([]float64, len(m30Klines))

	for i, line := range m30Klines {
		closes30m[i] = line.Close
		highs30m[i] = line.High
		low30m[i] = line.Low
	}

	// ------------------ 5. 构建TrendState ------------------
	atrVals := talib.Atr(highs30m, low30m, closes30m, 14) // 参数顺序: High, Low, Close, Period
	adxVals := talib.Adx(highs30m, low30m, closes30m, 14)
	rsiVals := talib.Rsi(closes30m, 14)

	last := m30Klines[len(m30Klines)-1]

	state = &model3.TrendState{
		Symbol:            symbol,
		Direction:         dir,
		ATR:               atrVals[len(atrVals)-1],
		ADX:               adxVals[len(adxVals)-1],
		RSI:               rsiVals[len(rsiVals)-1],
		LastPrice:         last.Close,
		Timestamp:         last.Timestamp,
		Scores:            scores,
		IndicatorSnapshot: indicator,
	}

	return state, nil
}

// 加权总分
func (tm *Manager) weightedScore(s4h, s1h, s30 float64) float64 {
	// 基础权重
	w4h, w1h, w30 := 0.4, 0.3, 0.3

	// 优先级最高的逻辑: 30m 与 4h 背离，可能是反转/回调信号
	// 优化后的背离情景：怀疑长期趋势，但不过分相信短线
	if (s30 > 0 && s4h < 0) || (s30 < 0 && s4h > 0) {
		// 趋势可能正在修正或处于震荡，系统应保持中立或观望。
		// 目标是让 FinalScore 接近 0，以过滤信号。
		w30 = 0.3
		w1h = 0.4 // 提高 1h 权重，让它来仲裁
		w4h = 0.3
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

func (tm *Manager) calcTrendScores(s4h, s1h, s30 float64) model3.TrendScores {
	// --- 大趋势权重 ---
	w4h, w1h := 0.7, 0.3
	longTermTrendScore := w4h*s4h + w1h*s1h

	// --- 综合分 ---
	final := tm.weightedScore(s4h, s1h, s30)

	return model3.TrendScores{
		TrendScore: longTermTrendScore,
		FinalScore: final,
		Score30m:   s30,
		Score1h:    s1h,
		Score4h:    s4h,
	}
}

// 计算周期趋势分数 -3 ~ +3（方向化 + 抖动抑制）
func (tm *Manager) ScoreForPeriod(klines []model2.Kline, period model.KlinePeriod) (float64, model3.IndicatorSnapshot) {
	// 初始化指标快照
	snapshot := model3.IndicatorSnapshot{}
	if len(klines) < 200 {
		return 0, snapshot
	}

	n := len(klines)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Vol
	}

	last := len(closes) - 1
	price := closes[last]

	ema20 := talib.Ema(closes, 20)
	ema50 := talib.Ema(closes, 50)
	ema200 := talib.Ema(closes, 200)
	adx := talib.Adx(highs, lows, closes, 14)
	upper, middle, lower := talib.BBands(closes, 20, 2, 2, 0)
	kVals, dVals := talib.Stoch(highs, lows, closes, 9, 3, talib.SMA, 3, talib.SMA)

	// 计算成交量的20周期EMA
	volumeEMA20 := talib.Ema(volumes, 20) // 计算成交量的 20 周期 EMA

	ema20Last := ema20[last]
	ema50Last := ema50[last]
	ema200Last := ema200[last]
	adxLast := adx[last]
	volumeEMA20Last := volumeEMA20[last] // 获取最新成交量的EMA
	bbWidthLast := (upper[last] - lower[last]) / middle[last]

	var bbSum float64
	var count int
	for i := last - 50; i < last; i++ {
		if i >= 0 && middle[i] > 0 {
			bbSum += (upper[i] - lower[i]) / middle[i]
			count++
		}
	}

	bbWidthAvg := 0.0
	if count > 0 {
		bbWidthAvg = bbSum / float64(count)
	}

	// === 打分 ===
	score := 0.0
	reasons := []string{}

	// 价格 vs EMA200
	if price > ema200Last {
		weight := 1.0
		// 4 小时图上的 EMA200 应该是最强的趋势支撑/阻力线，应该拥有最高的权重。
		if period == model.Kline_4h {
			weight = 1.5 // 提高 4 小时 EMA200 的权重
		}
		score += weight
		reasons = append(reasons, fmt.Sprintf("+%.1f(价格>EMA200)", weight))
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

	// ADX 趋势强度判断
	adxThreshold := 25.0
	// 可以在这里调整 ADX 阈值，例如：
	if period == model.Kline_4h {
		adxThreshold = 22.0
	} // 4h 趋势更难形成，可以降低阈值
	if period == model.Kline_30min {
		adxThreshold = 30.0
	} // 30m 趋势波动大，可以提高阈值

	// ADX 趋势强度优化
	if adxLast > adxThreshold {
		if score > 0 {
			score += 1.0 // 强化既有多头趋势
			reasons = append(reasons, "+1.0(ADX强化多头)")
		} else if score < 0 {
			score -= 1.0 // 强化既有空头趋势
			reasons = append(reasons, "-1.0(ADX强化空头)")
		} else {
			// score == 0 且 ADX > 25，强烈震荡/盘整，应避免开仓，分数不应变动或略微减分
			score -= 0.5 // 趋势强劲但方向不明，视为震荡风险
			reasons = append(reasons, "-0.5(ADX强劲方向不明)")
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
		reasons = append(reasons, "-0.5(KDJ死叉)")
	}

	//  J 值极端情况 ===
	jNow := jVals[last]
	if jNow > 100 {
		penalty := 0.5 // 极端超买，防止追高
		if period == model.Kline_30min {
			// 30 分钟图上的 J 值超买/超卖，应该成为强力的开仓抑制信号
			penalty = 1.0 // 30 分钟周期，惩罚加倍，强烈阻止追高
		}
		score -= penalty
		reasons = append(reasons, fmt.Sprintf("-%.1f(J>100超买)", penalty))
	}
	if jNow < 0 {
		score += 0.5 // 极端超卖，防止错过反弹
		reasons = append(reasons, "+0.5(J<0超卖)")
	}

	// MACD 参数 (12, 26, 9) 常见用法
	macdVals, signalVals, histVals := talib.Macd(closes, 12, 26, 9)
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

	// 新增：成交量确认打分
	if volumes[last] > volumeEMA20Last*1.2 { // 当前量 > 平均量 20%
		const VOLUME_SCORE_WEIGHT = 0.5
		if score > 1.0 { // 仅在有明确多头倾向时加分
			score += VOLUME_SCORE_WEIGHT
			reasons = append(reasons, fmt.Sprintf("+%.1f(高量确认多头)", VOLUME_SCORE_WEIGHT))
		} else if score < -1.0 { // 仅在有明确空头倾向时减分
			score -= VOLUME_SCORE_WEIGHT
			reasons = append(reasons, fmt.Sprintf("-%.1f(高量确认空头)", VOLUME_SCORE_WEIGHT))
		} else {
			// 如果趋势不明确，高量可能意味着顶部或底部争夺，不打分
			reasons = append(reasons, fmt.Sprintf("0(高量但趋势中性)", VOLUME_SCORE_WEIGHT))
		}
	} else if volumes[last] < volumeEMA20Last*0.7 { // 当前量 < 平均量 30%
		// 低成交量表示当前趋势动能减弱，无论方向如何，都应给予惩罚
		score -= 0.5
		reasons = append(reasons, "-0.5(低量警告)")
	}

	// --- 填充 IndicatorSnapshot ---
	snapshot = model3.IndicatorSnapshot{
		LastPrice:  price,
		EMA20:      ema20Last,
		EMA50:      ema50Last,
		EMA200:     ema200Last,
		ADX:        adxLast,
		BBWidth:    bbWidthLast,
		BBWidthAvg: bbWidthAvg,
		KVal:       kVals[last],
		DVal:       dVals[last],
		JVal:       jVals[last],
		RSI:        rsiVals[last],
		MACD:       macdVals[last],
		MACDSignal: signalVals[last],
		MACDHist:   histVals[last],
		Reasons:    strings.Join(reasons, ", "),
	}

	// 限制分数范围
	score = math.Min(score, 3)
	score = math.Max(score, -3)

	return score, snapshot
}

// SaveTrendState 存储某一币种的最新趋势状态
func (tm *Manager) SaveTrendState(trendState *model3.TrendState) {
	tm.save(trendState)
}

// GetLatestTrendState 获取某一币种的最新趋势状态，供决策树使用
func (tm *Manager) GetLatestTrendState(symbol string) *model3.TrendState {
	return tm.GetState(symbol)
}

// 更新某币种趋势（内部 & 外部都可调用）
func (tm *Manager) save(state *model3.TrendState) {
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
func (tm *Manager) GetState(symbol string) *model3.TrendState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	machine := tm.machines[symbol]
	if machine == nil {
		return nil
	}
	states := machine.StatesCaches
	if len(states) > 0 {
		// 返回最新的
		return states[len(states)-1]
	}
	return nil
}

func (tm *Manager) getStateMachine(symbol string) *StateMachine {
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
