package trend

//
//import (
//	"edgeflow/internal/exchange"
//	model2 "edgeflow/internal/model"
//	"encoding/csv"
//	"fmt"
//	"log"
//	"math"
//	"os"
//	"strconv"
//	"strings"
//	"sync"
//	"time"
//
//	talib "github.com/markcheno/go-talib"
//	"github.com/nntaoli-project/goex/v2/model"
//)
//
//// TrendManager 负责管理多个币种的趋势状态
//type TrendManager struct {
//	mu     sync.RWMutex
//	states map[string]*TrendState
//
//	ex       exchange.Exchange // OKX 客户端
//	interval model.KlinePeriod
//	symbols  []string
//	stopCh   chan struct{}
//	cfg      TrendCfg
//}
//
//func NewTrendManager(ex exchange.Exchange, symbols []string, interval model.KlinePeriod) *TrendManager {
//	return &TrendManager{
//		states:   make(map[string]*TrendState),
//		ex:       ex,
//		interval: interval,
//		symbols:  symbols,
//		stopCh:   make(chan struct{}),
//		cfg:      DefaultTrendCfg(),
//	}
//}
//
//// 启动自动更新（定时拉取 K 线）
//func (tm *TrendManager) StartUpdater() {
//
//	// 启动时立即更新一次
//	tm._update()
//
//	// 使用定时器，每隔一分钟执行一次
//	ticker := time.NewTicker(5 * time.Minute)
//
//	go func() {
//		for {
//			select {
//			case <-ticker.C:
//				tm._update()
//			case <-tm.stopCh:
//				ticker.Stop()
//				return
//			}
//		}
//	}()
//}
//
//func (tm *TrendManager) _update() {
//	for _, sym := range tm.symbols {
//		_, state, err := tm.updateSymbol(sym)
//		if err == nil {
//			tm.update(state)
//			fmt.Println(state.Description)
//		}
//		time.Sleep(time.Second * 3)
//	}
//}
//
//func (tm *TrendManager) StopUpdater() {
//	close(tm.stopCh)
//}
//
//func (tm *TrendManager) updateSymbol(symbol string) ([]TrendState, *TrendState, error) {
//	// 拉取 300 根 K 线
//	latestFirst, err := tm.ex.GetKlineRecords(symbol, tm.interval, 200, 0, model2.OrderTradeSpot)
//	n := len(latestFirst)
//	// 数据小于200条 不足计算
//	if err != nil || n < 200 || n == 0 {
//		log.Printf("[TrendManager] fetch kline error for %s: %v", symbol, err)
//		return nil, nil, err
//	}
//
//	latestFirst = normalizeCandles(latestFirst, tm.interval, false)
//
//	tm.genCSV(symbol, tm.interval, latestFirst)
//
//	// ---- 数据准备 (倒序 -> 正序) ----
//	closes := make([]float64, n)
//	opens := make([]float64, n)
//	highs := make([]float64, n)
//	lows := make([]float64, n)
//
//	for i, item := range latestFirst {
//		closes[i] = item.Close
//		opens[i] = item.Open
//		highs[i] = item.High
//		lows[i] = item.Low
//	}
//
//	// ---- 计算指标 ----
//	ema50 := talib.Ema(closes, 50)
//	ema200 := talib.Ema(closes, 200)
//	rsi := talib.Rsi(closes, 14)
//	macd, signal, _ := talib.Macd(closes, 12, 26, 9)
//	adx := talib.Adx(highs, lows, closes, 14)
//
//	basis := talib.Sma(closes, 20)
//	dev := talib.StdDev(closes, 20, 1.0)
//	upper := make([]float64, len(basis))
//	lower := make([]float64, len(basis))
//	for i := range basis {
//		upper[i] = basis[i] + 2*dev[i]
//		lower[i] = basis[i] - 2*dev[i]
//	}
//
//	// 最新一根数据
//	last := n - 1
//	lastClose := closes[last]
//	lastOpen := opens[last]
//	lastHigh := highs[last]
//	lastLow := lows[last]
//
//	// ---- K线形态 ----
//	body := math.Abs(lastClose - lastOpen)
//	upperWick := lastHigh - math.Max(lastOpen, lastClose)
//	lowerWick := math.Min(lastOpen, lastClose) - lastLow
//
//	bullWick := lowerWick > body*2
//	bearWick := upperWick > body*2
//
//	bullShape := bullWick || lastClose > upper[last]
//	bearShape := bearWick || lastClose < lower[last]
//
//	ema50Now := ema50[len(ema50)-1]
//	ema200Now := ema200[len(ema200)-1]
//	rsiNow := rsi[len(rsi)-1]
//	macdNow := macd[len(macd)-1]
//	signalNow := signal[len(signal)-1]
//	adxNow := adx[len(adx)-1]
//
//	// ---- 趋势方向 ----
//	bullTrend := ema50Now > ema200Now
//	bearTrend := ema50Now < ema200Now
//	// 趋势强度：只在最强信号使用，这里暂时不用
//	//strongTrend := adxNow > 30
//
//	// ---- 动能信号 ----
//	bullMomentum := (macdNow > signalNow) || (rsiNow > 55)
//	bearMomentum := (macdNow < signalNow) || (rsiNow < 45)
//
//	// ---- 信号判断 ----
//	var signals []TrendState
//	//now := time.UnixMilli(latestFirst[0].Timestamp) // 最新K线时间
//	now := latestFirst[0].Timestamp
//
//	// 买入
//	if bullTrend {
//		if bullMomentum && bullShape {
//			state := NewTrendState(symbol, TrendUp, 3, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//			signals = append(signals, state)
//		} else if bullMomentum {
//			state := NewTrendState(symbol, TrendUp, 2, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//			signals = append(signals, state)
//		} else if bullShape {
//			state := NewTrendState(symbol, TrendUp, 1, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//			signals = append(signals, state)
//		}
//	}
//	// 卖出
//	if bearTrend {
//		if bearMomentum && bearShape {
//			state := NewTrendState(symbol, TrendDown, 3, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//			signals = append(signals, state)
//		} else if bearMomentum {
//			state := NewTrendState(symbol, TrendDown, 2, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//			signals = append(signals, state)
//		} else if bearShape {
//			state := NewTrendState(symbol, TrendDown, 1, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//			signals = append(signals, state)
//		}
//	}
//
//	// === 横盘 ===
//	if len(signals) == 0 {
//		state := NewTrendState(symbol, TrendNeutral, 0, ema200Now, ema50Now, adxNow, rsiNow, lastClose, now)
//		signals = append(signals, state)
//	}
//
//	// ---- 最终信号：取最强优先级 ----
//	final := strongestSignal(signals)
//	return signals, final, nil
//}
//
//// 更新某币种趋势（内部 & 外部都可调用）
//func (tm *TrendManager) update(state *TrendState) {
//	tm.mu.Lock()
//	defer tm.mu.Unlock()
//
//	tm.states[state.Symbol] = state
//}
//
//// 获取某币种趋势
//func (tm *TrendManager) Get(symbol string) (*TrendState, bool) {
//	tm.mu.RLock()
//	defer tm.mu.RUnlock()
//	st, ok := tm.states[symbol]
//	return st, ok
//}
//
//func (tm *TrendManager) IsTrendOk(symbol, side string) bool {
//	state, ok := tm.Get(symbol)
//	if ok && state.Direction.MatchesSide(model2.OrderSide(side)) {
//		return true
//	}
//	return false
//}
//
