package trend

import (
	"edgeflow/internal/exchange"
	model2 "edgeflow/internal/model"
	"log"
	"math"
	"sync"
	"time"

	"github.com/nntaoli-project/goex/v2/model"
)

// TrendManager 负责管理多个币种的趋势状态
type TrendManager struct {
	mu     sync.RWMutex
	states map[string]*TrendState

	ex       exchange.Exchange // OKX 客户端
	interval model.KlinePeriod
	symbols  []string
	stopCh   chan struct{}
	cfg      TrendCfg
}

func NewTrendManager(ex exchange.Exchange, symbols []string, interval model.KlinePeriod) *TrendManager {
	return &TrendManager{
		states:   make(map[string]*TrendState),
		ex:       ex,
		interval: interval,
		symbols:  symbols,
		stopCh:   make(chan struct{}),
		cfg:      DefaultTrendCfg(),
	}
}

// 启动自动更新（定时拉取 K 线）
func (tm *TrendManager) StartUpdater() {

	// 启动时立即更新一次
	for _, sym := range tm.symbols {
		tm.updateSymbol(sym)
	}

	// 使用定时器，每隔一分钟执行一次
	ticker := time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-ticker.C:
				for _, sym := range tm.symbols {
					tm.updateSymbol(sym)
				}
			case <-tm.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (tm *TrendManager) StopUpdater() {
	close(tm.stopCh)
}

func (tm *TrendManager) updateSymbol(symbol string) {
	// 拉取 300 根 K 线
	klines, err := tm.ex.GetKlineRecords(symbol, tm.interval, 300, 0, model2.OrderTradeSpot)
	// 数据小于200条 不足计算
	if err != nil || len(klines) < 200 || len(klines) == 0 {
		log.Printf("[TrendManager] fetch kline error for %s: %v", symbol, err)
		return
	}

	// 计算 MA200
	//ma200 := calcMA(klines, 200)
	// 计算 EMA20和EMA50
	ema20 := CalcEMA(klines, 20)
	ema50 := calcEMA(klines, 50)
	// 计算ADX14
	adx14 := CalcADXWilder(klines, 14)

	lastLine := klines[0]
	lastPrice := lastLine.Close

	// 计算 MA200
	hasMA200 := len(klines) >= 200
	ma200 := math.NaN()
	if hasMA200 {
		ma200 = CalcSMA(klines, 200)
	}

	// 趋势判定
	// 线性回归斜率 + R²（对最近窗口）
	win := tm.cfg.SlopeWindow
	if len(klines) < win {
		win = len(klines)
	}
	slope, r2 := LinRegSlopeR2Log(klines[len(klines)-win:])

	upMA := false
	downMA := false
	if hasMA200 {
		upMA = lastPrice > ma200 && ema50 > ma200 && confirmAbove(klines, ma200, tm.cfg.ConfirmBars)
		downMA = lastPrice < ma200 && ema50 < ma200 && confirmBelow(klines, ma200, tm.cfg.ConfirmBars)
	} else {
		// 新币/数据少：退化用 EMA20/EMA50
		upMA = lastPrice > ema50 && !math.IsNaN(ema20) && ema20 > ema50
		downMA = lastPrice < ema50 && !math.IsNaN(ema20) && ema20 < ema50
	}

	var dir TrendDirection
	// 两条路径：均线确认 + ADX  或  斜率+R² + 均线方向
	if (upMA && adx14 >= tm.cfg.ADXThreshold) || (slope > 0 && r2 >= tm.cfg.MinR2 && lastPrice > ema50) {
		dir = TrendUp
	} else if (downMA && adx14 >= tm.cfg.ADXThreshold) || (slope < 0 && r2 >= tm.cfg.MinR2 && lastPrice < ema50) {
		dir = TrendDown
	} else {
		dir = TrendUnknown
	}

	tm.Update(symbol, dir, ma200, ema50, adx14, lastPrice)

}

// 更新某币种趋势（内部 & 外部都可调用）
func (tm *TrendManager) Update(symbol string, dir TrendDirection, ma200 float64, ema50, adx14, lastPrice float64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	state := &TrendState{
		Symbol:     symbol,
		Direction:  dir,
		MA200:      ma200,
		EMA50:      ema50,
		ADX:        adx14,
		LastPrice:  lastPrice,
		LastUpdate: time.Now(),
	}
	tm.states[symbol] = state
}

// 获取某币种趋势
func (tm *TrendManager) Get(symbol string) (*TrendState, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	st, ok := tm.states[symbol]
	return st, ok
}

func (tm *TrendManager) IsTrendOk(symbol, side string) bool {
	state, ok := tm.Get(symbol)
	if ok && state.Direction.MatchesSide(model2.OrderSide(side)) {
		return true
	}
	return false
}

// CalcMA 计算简单移动平均线
func calcMA(lines []model2.Kline, period int) float64 {
	if len(lines) < period {
		return math.NaN()
	}
	sum := 0.0
	for i := len(lines) - period; i < len(lines); i++ {
		sum += lines[i].Close
	}
	return sum / float64(period)
}

// CalcEMA 计算指数移动平均线
func calcEMA(lines []model2.Kline, period int) float64 {
	if len(lines) < period {
		return math.NaN()
	}
	k := 2.0 / (float64(period) + 1)
	ema := lines[0].Close
	for i := 1; i < len(lines); i++ {
		ema = lines[i].Close*k + ema*(1-k)
	}
	return ema
}

//// CalcADX 计算 ADX（简化版）
//func calcADX(candles []model2.Kline, period int) float64 {
//	if len(candles) < period+1 {
//		return math.NaN()
//	}
//
//	trList := []float64{}
//	plusDM := []float64{}
//	minusDM := []float64{}
//
//	for i := 1; i < len(candles); i++ {
//		highDiff := candles[i].High - candles[i-1].High
//		lowDiff := candles[i-1].Low - candles[i].Low
//
//		// True Range
//		tr := math.Max(candles[i].High-candles[i].Low,
//			math.Max(math.Abs(candles[i].High-candles[i-1].Close),
//				math.Abs(candles[i].Low-candles[i-1].Close)))
//		trList = append(trList, tr)
//
//		// +DM 和 -DM
//		if highDiff > lowDiff && highDiff > 0 {
//			plusDM = append(plusDM, highDiff)
//		} else {
//			plusDM = append(plusDM, 0)
//		}
//
//		if lowDiff > highDiff && lowDiff > 0 {
//			minusDM = append(minusDM, lowDiff)
//		} else {
//			minusDM = append(minusDM, 0)
//		}
//	}
//
//	// 平滑平均
//	sumTR, sumPlusDM, sumMinusDM := 0.0, 0.0, 0.0
//	for i := len(trList) - period; i < len(trList); i++ {
//		sumTR += trList[i]
//		sumPlusDM += plusDM[i]
//		sumMinusDM += minusDM[i]
//	}
//
//	avgTR := sumTR / float64(period)
//	pDI := 100 * (sumPlusDM / avgTR)
//	mDI := 100 * (sumMinusDM / avgTR)
//	dx := 100 * math.Abs(pDI-mDI) / (pDI + mDI)
//
//	return dx
//}

// ADX (Wilder) 返回最后一个 ADX 值
func CalcADXWilder(c []model2.Kline, period int) float64 {
	if len(c) < period+1 {
		return math.NaN()
	}
	tr := make([]float64, len(c)-1)
	pdm := make([]float64, len(c)-1)
	mdm := make([]float64, len(c)-1)

	for i := 1; i < len(c); i++ {
		upMove := c[i].High - c[i-1].High
		downMove := c[i-1].Low - c[i].Low

		highLow := c[i].High - c[i].Low
		highClose := math.Abs(c[i].High - c[i-1].Close)
		lowClose := math.Abs(c[i].Low - c[i-1].Close)
		tr[i-1] = math.Max(highLow, math.Max(highClose, lowClose))

		if upMove > 0 && upMove > downMove {
			pdm[i-1] = upMove
		} else {
			pdm[i-1] = 0
		}
		if downMove > 0 && downMove > upMove {
			mdm[i-1] = downMove
		} else {
			mdm[i-1] = 0
		}
	}

	// 初始平滑
	sumTR, sumPDM, sumMDM := 0.0, 0.0, 0.0
	for i := 0; i < period; i++ {
		sumTR += tr[i]
		sumPDM += pdm[i]
		sumMDM += mdm[i]
	}
	atr := sumTR
	pDM14 := sumPDM
	mDM14 := sumMDM

	pDI := 100.0 * (pDM14 / atr)
	mDI := 100.0 * (mDM14 / atr)
	if pDI+mDI == 0 {
		return 0
	}
	dx := 100.0 * math.Abs(pDI-mDI) / (pDI + mDI)

	// 继续滚动 + 计算后续 DX，并对 ADX 进行 Wilder 平滑
	adx := dx
	count := 1.0

	for i := period; i < len(tr); i++ {
		atr = atr - (atr / float64(period)) + tr[i]
		pDM14 = pDM14 - (pDM14 / float64(period)) + pdm[i]
		mDM14 = mDM14 - (mDM14 / float64(period)) + mdm[i]

		if atr == 0 {
			continue
		}
		pDI = 100.0 * (pDM14 / atr)
		mDI = 100.0 * (mDM14 / atr)
		if pDI+mDI == 0 {
			continue
		}
		dxi := 100.0 * math.Abs(pDI-mDI) / (pDI + mDI)

		// Wilder smoothing for ADX
		adx = (adx*(float64(period)-1) + dxi) / float64(period)
		count++
	}
	if math.IsNaN(adx) || math.IsInf(adx, 0) {
		return 0
	}
	return adx
}

// 线性回归斜率与R²（对 ln(price) 计算，更稳定）
func LinRegSlopeR2Log(lines []model2.Kline) (slope, r2 float64) {
	n := len(lines)
	if n < 2 {
		return 0, 0
	}
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		if lines[i].Close <= 0 {
			return 0, 0
		}
		y[i] = math.Log(lines[i].Close)
	}
	var sumX, sumY, sumXX, sumXY float64
	for i := 0; i < n; i++ {
		x := float64(i + 1)
		sumX += x
		sumY += y[i]
		sumXX += x * x
		sumXY += x * y[i]
	}
	denom := float64(n)*sumXX - sumX*sumX
	if denom == 0 {
		return 0, 0
	}
	slope = (float64(n)*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / float64(n)

	// R²
	meanY := sumY / float64(n)
	var ssTot, ssRes float64
	for i := 0; i < n; i++ {
		x := float64(i + 1)
		fit := slope*x + intercept
		ssTot += (y[i] - meanY) * (y[i] - meanY)
		ssRes += (y[i] - fit) * (y[i] - fit)
	}
	if ssTot == 0 {
		return slope, 0
	}
	r2 = 1.0 - (ssRes / ssTot)
	return
}

func confirmAbove(lines []model2.Kline, base float64, bars int) bool {
	if math.IsNaN(base) || len(lines) < bars {
		return false
	}
	for i := len(lines) - bars; i < len(lines); i++ {
		if lines[i].Close <= base {
			return false
		}
	}
	return true
}

func confirmBelow(lines []model2.Kline, base float64, bars int) bool {
	if math.IsNaN(base) || len(lines) < bars {
		return false
	}
	for i := len(lines) - bars; i < len(lines); i++ {
		if lines[i].Close >= base {
			return false
		}
	}
	return true
}

func CalcSMA(x []model2.Kline, p int) float64 {
	if len(x) < p {
		return math.NaN()
	}
	s := 0.0
	for i := len(x) - p; i < len(x); i++ {
		s += x[i].Close
	}
	return s / float64(p)
}

// EMA：用首个窗口的SMA做种子，更稳
func CalcEMA(x []model2.Kline, p int) float64 {
	if len(x) < p {
		return math.NaN()
	}
	sum := 0.0
	for i := 0; i < p; i++ {
		sum += x[i].Close
	}
	ema := sum / float64(p)
	k := 2.0 / (float64(p) + 1)
	for i := p; i < len(x); i++ {
		ema = (x[i].Close-ema)*k + ema
	}
	return ema
}
