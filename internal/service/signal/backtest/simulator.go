package backtest

import (
	"context"
	"edgeflow/internal/dao"
	model2 "edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"fmt"
	"log"
	"sync"
	"time"
)

// Outcome 定义了模拟交易的最终结果
type Outcome string

const (
	HitTP   Outcome = "HIT_TP"  // 达到止盈 (胜利)
	HitSL   Outcome = "HIT_SL"  // 达到止损 (失败)
	Open    Outcome = "OPEN"    // 仍在运行中
	Expired Outcome = "EXPIRED" // 超过最大持有周期而平仓
)

// SimulationResult 存储模拟的结果和详细信息
type SimulationResult struct {
	Outcome     Outcome
	EntryPrice  float64
	TPPrice     float64
	SLPrice     float64
	FinalPrice  float64 // 最终平仓/收盘价
	FinalPnlPct float64 // 最终盈亏百分比 (百分比形式，如 0.05 代表 5%)
	CandlesUsed uint    // 使用的 K 线数量
}

// --- 活跃信号追踪结构 ---

// ActiveSignal 存储一个正在被追踪的信号实例
type ActiveSignal struct {
	ID         uint64    // 信号 ID，用于关联数据库 signals 表
	Symbol     string    // 交易对名称
	EntryPrice float64   // 进场价格
	EntryTime  time.Time // 进场时间
	IsLong     bool      // 方向: true为多头，false为空头

	// 信号自身推荐的止盈止损价格 (不再由 Similator 硬编码计算)
	TPPrice float64
	SLPrice float64

	Klines []model2.Kline // 自进场以来的所有后续K线数据

	InitKlinesCount int // 初始化时的k线数量，这些k线由于是在买入前生成的不计入盈亏计算
}

// SignalTracker 负责管理所有活跃的信号，并在数据更新时处理它们的生命周期
type SignalTracker struct {
	mu            sync.RWMutex
	ActiveSignals map[uint64]*ActiveSignal // 活跃信号列表: ID -> ActiveSignal

	// 策略参数
	MaxCandleHoldPeriod uint          // 最大持有K线数量 (例如 50根15M K线 = 12.5小时)
	MaxOverlapCandles   uint          // 信号抑制周期 (例如 4 根 K 线 = 1 小时)
	SignalDao           dao.SignalDao // 用于持久化信号结果到数据库
}

// NewSignalTracker 创建一个新的 SignalTracker 实例
func NewSignalTracker(maxHoldPeriod uint, signalDao dao.SignalDao) *SignalTracker {
	return &SignalTracker{
		ActiveSignals:       make(map[uint64]*ActiveSignal),
		MaxCandleHoldPeriod: maxHoldPeriod,
		MaxOverlapCandles:   4,
		SignalDao:           signalDao,
	}
}

// AddSignal 将一个新的信号加入到追踪列表
// 信号创建者必须提供其计算出的 TP/SL 价格。
func (st *SignalTracker) AddSignal(sig *ActiveSignal) {
	// 增加对 TP/SL 价格的验证，确保信号是完整的
	if sig.EntryPrice == 0 || sig.TPPrice == 0 || sig.SLPrice == 0 {
		fmt.Printf("[Tracker] ERROR: Signal %d rejected due to missing Entry, TP, or SL price.\n", sig.ID)
		return
	}

	// 抑制信号，防止震荡行情一个币发出很多多余未模拟盈亏完成的信号
	if st.hasRecentActiveSignal(sig.Symbol, sig.IsLong) {
		fmt.Println("Signal rejected: Active signal already exists in the suppression window.")
		return // 拒绝该信号，不添加到追踪器
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	st.ActiveSignals[sig.ID] = sig
	fmt.Printf("[Tracker] Added new active signal: %d @ %.2f (TP: %.2f, SL: %.2f)\n", sig.ID, sig.EntryPrice, sig.TPPrice, sig.SLPrice)
}

// HasRecentActiveSignal 检查是否存在与当前信号同方向、且未过抑制期的活跃信号。
// 外部调用者应该在生成新信号时，使用此函数进行过滤。
func (t *SignalTracker) hasRecentActiveSignal(symbol string, isLong bool) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, sig := range t.ActiveSignals {
		// 1. 检查交易对和方向是否一致
		if sig.Symbol == symbol && sig.IsLong == isLong {
			// 2. 检查信号是否仍在抑制期内
			// 如果 Klines 数量小于 MaxOverlapCandles (例如 4)，则认为信号仍处于抑制期。
			if uint(len(sig.Klines)) < t.MaxOverlapCandles {
				return true
			}
		}
	}
	return false
}

// removeSignalFromTracker 从追踪器中移除信号
func (t *SignalTracker) removeSignalFromTracker(id uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.ActiveSignals, id)
}

// ProcessKlines 接收 K 线批次，并更新所有活跃信号的状态。
func (t *SignalTracker) ProcessKlines(symbol string, klines []model2.Kline) {
	if len(klines) == 0 {
		return
	}

	t.mu.RLock()
	signalsToProcess := make([]*ActiveSignal, 0, len(t.ActiveSignals))
	// 复制活跃信号列表以供处理，避免在遍历时修改 map
	for _, sig := range t.ActiveSignals {
		if sig.Symbol != symbol {
			continue
		}
		signalsToProcess = append(signalsToProcess, sig)
	}
	t.mu.RUnlock()

	if len(signalsToProcess) == 0 {
		return
	}

	// Step 1: 遍历活跃信号
	for _, sig := range signalsToProcess {
		var lastKlineTime time.Time

		// 查找最后一条 K 线的时间，必须从完整的 Klines 列表中获取，包括初始历史
		if len(sig.Klines) > 0 {
			lastKlineTime = sig.Klines[len(sig.Klines)-1].Timestamp
		} else {
			// 如果 Klines 初始为空, 使用 AddedAt 作为基准
			lastKlineTime = sig.EntryTime
		}

		// 找到新 K 线的起始索引
		newKlineStartIndex := -1
		// 倒序遍历传入的 klines，查找第一根 CloseTime 严格晚于 T_last 的 K 线
		for i := len(klines) - 1; i >= 0; i-- {
			closeTime := klines[i].Timestamp
			// 找到第一根 CloseTime 严格晚于 T_last 的 K 线
			if closeTime.After(lastKlineTime) {
				newKlineStartIndex = i
			} else {
				// 由于 K 线是升序排列的，一旦遇到早于或等于 T_last 的，就可以中断
				break
			}
		}

		// 如果没有新 K 线，跳过
		if newKlineStartIndex == -1 && len(sig.Klines) > 0 {
			continue
		}
		// 如果 Klines 初始为空，则从 0 开始处理
		if newKlineStartIndex == -1 && len(sig.Klines) == 0 {
			newKlineStartIndex = 0
		}

		// 提取新的 K 线切片
		newKlines := klines[newKlineStartIndex:]

		if len(newKlines) == 0 {
			continue
		}

		// Step 2: 逐根 K 线检查，并在命中时即时平仓 (核心优化)
		var simulationResult SimulationResult
		isClosed := false

		for _, kline := range newKlines {
			// 1. 追加 K 线历史
			sig.Klines = append(sig.Klines, kline)

			// 2. 模拟单根 K 线的平仓情况
			simulationResult = SimulateOutcome(sig.EntryPrice, sig.TPPrice, sig.SLPrice, sig.IsLong, []model2.Kline{kline})

			if simulationResult.Outcome != Open {
				isClosed = true

				// *** 修正 CandlesUsed 计算：总长度 - 初始历史 K 线数量 ***
				simulationResult.CandlesUsed = uint(len(sig.Klines) - sig.InitKlinesCount)

				log.Printf("[Tracker HIT] Signal %d closed by %s at Price %.4f after %d candles (Total Klines: %d).",
					sig.ID, simulationResult.Outcome, simulationResult.FinalPrice, simulationResult.CandlesUsed, len(sig.Klines))
				break // 信号已平仓，中断对本批次后续 K 线的检查
			}
		}

		// 获取当前追踪的 K 线数量 (排除初始历史 K 线)
		currentTrackedCandles := uint(len(sig.Klines) - sig.InitKlinesCount)

		// Step 3: 如果信号仍未平仓，检查是否过期
		if !isClosed {
			// 检查是否达到最大蜡烛数限制 (使用排除历史后的数量)
			if currentTrackedCandles >= t.MaxOverlapCandles {

				// 标记为过期
				isClosed = true

				// 计算过期时的浮动盈亏（使用批次中最后一条 K 线的收盘价）
				lastKline := newKlines[len(newKlines)-1]

				// 由于 SimulateOutcome 只检查命中，我们需要手动计算过期时的 FinalPnlPct
				var finalPnlPct float64
				if sig.IsLong {
					finalPnlPct = (lastKline.Close / sig.EntryPrice) - 1
				} else {
					finalPnlPct = 1 - (lastKline.Close / sig.EntryPrice)
				}

				simulationResult = SimulationResult{
					Outcome:     Expired,
					EntryPrice:  sig.EntryPrice,
					TPPrice:     sig.TPPrice,
					SLPrice:     sig.SLPrice,
					FinalPrice:  lastKline.Close,
					FinalPnlPct: finalPnlPct,
					CandlesUsed: t.MaxOverlapCandles, // 过期时，使用的 K 线数量即为 MaxCandles
				}

				log.Printf("[Tracker EXPIRE] Signal %d expired after %d candles. PnL: %.4f",
					sig.ID, simulationResult.CandlesUsed, simulationResult.FinalPnlPct)
			}
		}

		// Step 4: 处理平仓 (HIT_TP, HIT_SL, EXPIRED)
		if isClosed {
			// 异步写入数据库 (不阻塞主线程)
			go func(res SimulationResult, signal *ActiveSignal) {

				// 计算平仓 K 线的索引，以便获取精确的 ClosedAt 时间
				closedKlineIndex := signal.InitKlinesCount + int(res.CandlesUsed) - 1
				signalOutcome := entity.SignalOutcome{
					SignalID:    signal.ID,
					Symbol:      signal.Symbol,
					Outcome:     string(res.Outcome),
					FinalPnlPct: res.FinalPnlPct,
					CandlesUsed: res.CandlesUsed,
					ClosedAt:    sig.Klines[closedKlineIndex].Timestamp,
					CreatedAt:   time.Now(),
				}

				if err := t.SignalDao.SaveSignalOutcome(context.Background(), &signalOutcome); err != nil {
					log.Printf("[Tracker DB ERROR] Failed to save signal outcome for %d: %v", signal.ID, err)
				}
			}(simulationResult, sig)

			// 从追踪器中移除
			t.removeSignalFromTracker(sig.ID)
		}
	}
}

// SimulateOutcome 模拟单个信号在【预设止盈止损价格】下的结果。
// 返回 SimulationResult 结构体，提供清晰的模拟结果数据。
func SimulateOutcome(entryPrice, tpPrice, slPrice float64, isLong bool, subsequentKlines []model2.Kline) SimulationResult {
	// 1. 计算 TP/SL 的盈亏百分比 (提前计算，用于结果结构体)
	var tpPct, slPct float64

	if entryPrice != 0 {
		if isLong {
			// 多头 TP: (TP - Entry) / Entry (正值)
			tpPct = (tpPrice - entryPrice) / entryPrice
			// 多头 SL: (SL - Entry) / Entry (负值)
			slPct = (slPrice - entryPrice) / entryPrice
		} else {
			// 空头 TP: (Entry - TP) / Entry (正值)
			tpPct = (entryPrice - tpPrice) / entryPrice
			// 空头 SL: (Entry - SL) / Entry (负值)
			slPct = (entryPrice - slPrice) / entryPrice
		}
	}

	// 初始化结果结构体
	result := SimulationResult{
		// 初始状态为 Open
		Outcome: Open,
	}

	// 2. 遍历后续 K 线数据，检查是否触及 TP/SL
	for i, k := range subsequentKlines {
		candlesUsed := uint(i + 1)

		if isLong {
			// 做多检查: High 触及 TP 还是 Low 触及 SL?
			if k.High >= tpPrice {
				result.Outcome = HitTP
				result.FinalPnlPct = tpPct
				result.FinalPrice = tpPrice
				result.CandlesUsed = candlesUsed
				return result
			}
			if k.Low <= slPrice {
				result.Outcome = HitSL
				result.FinalPnlPct = slPct
				result.FinalPrice = slPrice
				result.CandlesUsed = candlesUsed
				return result
			}
		} else {
			// 做空检查: Low 触及 TP (价格下跌) 还是 High 触及 SL (价格上涨)?
			if k.Low <= tpPrice {
				result.Outcome = HitTP
				result.FinalPnlPct = tpPct
				result.FinalPrice = tpPrice
				result.CandlesUsed = candlesUsed
				return result
			}
			if k.High >= slPrice {
				result.Outcome = HitSL
				result.FinalPnlPct = slPct
				result.FinalPrice = slPrice
				result.CandlesUsed = candlesUsed
				return result
			}
		}
	}

	// 3. 如果 K 线全部遍历完，但未触及 TP/SL，则结果仍为 OPEN。
	result.CandlesUsed = uint(len(subsequentKlines))

	// 计算当前未平仓的浮动盈亏 (以最后一条 K 线的收盘价计算)
	if len(subsequentKlines) > 0 {
		lastClose := subsequentKlines[len(subsequentKlines)-1].Close
		if isLong {
			result.FinalPnlPct = (lastClose / entryPrice) - 1
		} else {
			result.FinalPnlPct = 1 - (lastClose / entryPrice)
		}
		result.FinalPrice = lastClose
	} else {
		result.FinalPnlPct = 0.0
	}

	return result
}
