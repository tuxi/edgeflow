package signal

import (
	"edgeflow/internal/config"
	"log"
	"sync"
	"time"
)

// 决策时需要的上下文
type DecisionContext struct {
	HasL2Position bool
	L2Entry       float64
	UnrealizedR   float64
	TrendOK       bool // 由策略提供的趋势/回撤过滤结果
}

// SignalManager 接口
// 信号裁判 判断哪些信号有用，什么时候出手
type Manager interface {
	Save(sig Signal)
	GetLastSignal(symbol string, level int) (Signal, bool)
	ShouldExecute(sig Signal) (execute bool, closeFirst bool)
	Decide(sig Signal, ctx DecisionContext) Decision
}

// DefaultSignalManager 默认实现
type defaultSignalManager struct {
	mu           sync.RWMutex
	state        map[string]*State
	level3Buffer []Signal
	cfg          config.StrategyConfig
}

func NewDefaultSignalManager(cfg config.StrategyConfig) Manager {
	return &defaultSignalManager{
		state:        make(map[string]*State),
		level3Buffer: []Signal{},
		cfg:          cfg,
	}
}

// 保存最新信号
func (m *defaultSignalManager) Save(sig Signal) {

	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.ensureStateLocked(sig.Symbol)
	s.LastByLevel[sig.Level] = sig
	if sig.Level == 2 {
		// 记录 L2 方向变化时间
		if s.L2Side != sig.Side {
			s.L2Side = sig.Side
			s.L2LastFlipAt = sig.Timestamp
		}
	}
}

// 获取最近某等级信号
func (m *defaultSignalManager) GetLastSignal(symbol string, level int) (Signal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sig := m.state[symbol].LastByLevel
	if sig != nil {
		s, ok := sig[level]
		return s, ok
	}

	return Signal{}, false
}

// 决定信号的执行决策
func (m *defaultSignalManager) Decide(
	sig Signal,
	ctx DecisionContext,
) Decision {
	st := m.getState(sig.Symbol)

	// 读取所需快照
	m.mu.RLock()
	lastL1, hasL1 := st.LastByLevel[1]
	lastL2, hasL2 := st.LastByLevel[2]
	lastL3, hasL3 := st.LastByLevel[3]
	l2Side := st.L2Side
	l2FlipAgo := time.Since(st.L2LastFlipAt)
	m.mu.RUnlock()
	// -------- Level 2：唯一有权开/平主仓 --------
	if sig.Level == 2 {
		// 防抖：与上一个 L2 同向且过近 -> 忽略
		if hasL2 && lastL2.Side == sig.Side && sig.Timestamp.Sub(lastL2.Timestamp) < m.cfg.MinSpacingL2 {
			decision := Decision{Action: ActIgnore, Reason: "L2-debounce"}
			decision.Log(sig, &m.cfg)
			return decision
		}

		if !ctx.HasL2Position {
			// 可选：需要 L1 同向确认
			if m.cfg.RequireL1ConfirmForL2Open {
				if !hasL1 || lastL1.Side != sig.Side || sig.Timestamp.Sub(lastL1.Timestamp) > m.cfg.L1ConfirmMaxDelay {
					decision := Decision{Action: ActIgnore, Reason: "L2-open-wait-L1-confirm"}
					decision.Log(sig, &m.cfg)
					return decision
				}
			}
			// 可选：趋势过滤
			if m.cfg.RequireTrendFilter && !ctx.TrendOK {
				decision := Decision{Action: ActIgnore, Reason: "L2-open-trend-filter-block"}
				decision.Log(sig, &m.cfg)
				return decision
			}
			// 开仓
			decision := Decision{Action: ActOpen, Reason: "L2-open"}
			decision.Log(sig, &m.cfg)
			return decision
		}

		// 已有 L2 仓位：若方向反转 -> 平仓
		if hasL2 && lastL2.Side != sig.Side {
			decision := Decision{Action: ActClose, Reason: "L2-flip-close"}
			decision.Log(sig, &m.cfg)
			return decision
		}
		// 同向就维持（是否二次加仓交给 L3）
		decision := Decision{Action: ActIgnore, Reason: "L2-same-keep"}
		decision.Log(sig, &m.cfg)
		return decision
	}

	// -------- Level 3：只能在 L2 框架内加减/收紧 --------
	if sig.Level == 3 {
		// 必须有 L2 仓位且经过冷静期
		if !ctx.HasL2Position || !hasL2 {
			decision := Decision{Action: ActIgnore, Reason: "L3-no-L2"}
			decision.Log(sig, &m.cfg)
			return decision
		}
		if l2FlipAgo < m.cfg.CooldownAfterL2Flip {
			decision := Decision{Action: ActIgnore, Reason: "L3-cooldown-after-L2-flip"}
			decision.Log(sig, &m.cfg)
			return decision
		}
		// 防抖：同向 L3 过于密集
		if hasL3 && lastL3.Side == sig.Side && sig.Timestamp.Sub(lastL3.Timestamp) < m.cfg.MinSpacingL3 {
			decision := Decision{Action: ActIgnore, Reason: "L3-debounce"}
			decision.Log(sig, &m.cfg)
			return decision
		}

		// 与 L2 同向：考虑趋势过滤（如“回撤到带内+趋势门槛”）
		if sig.Side == l2Side {
			if m.cfg.RequireTrendFilter && !ctx.TrendOK {
				decision := Decision{Action: ActIgnore, Reason: "L3-add-trend-filter-block"}
				decision.Log(sig, &m.cfg)
				return decision
			}
			// 可选：若 L1 近期反向，可只收紧止损而不加仓
			if hasL1 && lastL1.Side != sig.Side && sig.Timestamp.Sub(lastL1.Timestamp) <= 2*m.cfg.MinSpacingL3 {
				decision := Decision{Action: ActTightenSL, Reason: "L3-add-blocked-by-recent-L1-opposite"}
				decision.Log(sig, &m.cfg)
				return decision
			}
			decision := Decision{Action: ActAdd, Reason: "L3-add-with-L2"}
			decision.Log(sig, &m.cfg)
			return decision
		}

		// 与 L2 反向：不反手；按浮盈阈值做减仓或只收紧止损
		if ctx.UnrealizedR >= m.cfg.L3ReduceAtRMultiple {
			decision := Decision{Action: ActReduce, Reason: "L3-counter-reduce", ReducePercent: m.cfg.L3ReducePercent}
			decision.Log(sig, &m.cfg)
			return decision
		}
		decision := Decision{Action: ActTightenSL, Reason: "L3-counter-tightenSL"}
		decision.Log(sig, &m.cfg)
		return decision
	}

	// Level 1：只存储做参考，不直接驱动交易
	if sig.Level == 1 {
		decision := Decision{Action: ActIgnore, Reason: "L1-reference-only"}
		decision.Log(sig, &m.cfg)
		return decision
	}
	decision := Decision{Action: ActIgnore, Reason: "unknown-level"}
	decision.Log(sig, &m.cfg)
	return decision
}

//func (sm *defaultSignalManager) Decide(sig Signal, ctx DecisionContext) Decision {
//	// 🚨 先做趋势过滤
//	if !ctx.TrendOK {
//		log.Printf("signal %v ignored due to trend filter", sig.Level)
//		return ActIgnore
//	}
//
//	lastL1, hasL1 := sm.state[sig.Symbol].LastByLevel[1]
//	lastL2, hasL2 := sm.state[sig.Symbol].LastByLevel[2]
//	lastL3, hasL3 := sm.state[sig.Symbol].LastByLevel[3]
//
//	switch sig.Level {
//	case 1:
//		// L1 只存参考，不直接驱动
//		sm.Save(sig)
//		return ActIgnore
//
//	case 2:
//		sm.Save(sig)
//		if !ctx.HasL2Position { // 没有l2仓位
//			if hasL1 && sig.Side == lastL1.Side { // 有l1信号，并且l1信号与l2方向一致，可以开仓
//				return ActOpen // 第一次开仓
//			}
//		}
//		if hasL2 && sig.Side != lastL2.Side {
//			return ActClose // 方向反了 → 平仓
//		}
//		return ActIgnore
//
//	case 3:
//		sm.Save(sig)
//		if ctx.HasL2Position {
//			if sig.Side == lastL2.Side {
//				return ActAdd // 与L2同向 → 加仓
//			}
//			if sig.Side != lastL2.Side {
//				if ctx.UnrealizedR > 0.02 { // 盈利超过2% → 减仓锁盈
//					return ActReduce
//				}
//				return ActTightenSL // 否则收紧止损
//			}
//		}
//		return ActIgnore
//	}
//
//	return ActIgnore
//}

// 核心逻辑：判断是否执行信号以及是否需要先平仓
func (m *defaultSignalManager) ShouldExecute(sig Signal) (bool, bool) {

	// STEP 2: 获取最新缓存
	lastSignals := m.state[sig.Symbol].LastByLevel

	// SETP 4: 缓存信号
	defer m.Save(sig)

	// 获取最新缓存
	lvl1, hasL1 := lastSignals[1]
	lvl2, hasL2 := lastSignals[2]
	lvl3, hasL3 := lastSignals[3]

	// STEP 3: 不同等级的处理逻辑
	switch sig.Level {
	case 1:
		// 1级指标权重最高，直接放行
		return true, false
	case 2:
		if hasL1 && lvl1.Side == sig.Side {
			// 存在1级指标，并且方向一致，直接执行
			return true, false
		} else if hasL1 && lvl1.Side != sig.Side {
			// 存在1级指标，但是1级指标与当前指标方向不一致时，执行平仓，但是不继续下单
			return true, true
		} else if hasL3 && lvl3.Side == sig.Side {
			// 存在3级指标，但是3级指标与当前指标方向一致，下单
			return true, false
		} else if hasL2 && lvl2.Side == sig.Side {
			// 存在2级指标，并且指标方向一致，下单
			return true, false
		}
		// 剩余的认为方向不明确
		log.Println("等待L1方向，L2信号延迟执行")
		return false, false
	case 3:
		// 触发升级的最小数量
		level3UpgradeThreshold := 2
		level3Buffer := m.level3Buffer
		if hasL2 && lvl2.Side == sig.Side && hasL1 && lvl1.Side == sig.Side {
			sig.Score = 4
			// 1级和2级一致直接下单
			return true, false
		} else {
			// 只缓存同方向的3级信号
			if len(level3Buffer) > 0 {
				last := level3Buffer[len(level3Buffer)-1]
				if sig.Side != last.Side {
					// 方向不一致，清除旧缓存 → 重新统计
					level3Buffer = []Signal{}
				}
			}

			level3Buffer = append(level3Buffer, sig)
			m.level3Buffer = level3Buffer

			// 检查是否满足升级条件
			if len(level3Buffer) >= level3UpgradeThreshold {
				upgraded := sig
				upgraded.Level = 2
				upgraded.Score = 3
				//upgraded.Strategy += "-PromotedFromL3"
				log.Println("⬆️ 3级信号升级为2级信号:", upgraded)

				// 清空缓存避免重复触发
				level3Buffer = []Signal{}
				m.level3Buffer = level3Buffer

				// 递交给上级逻辑处理
				return m.ShouldExecute(sig)
			} else {
				log.Println("L3 信号仅记录，不执行")
			}
		}
		return true, false
	default:
		return false, false
	}
}

// 获取某个币的状态
func (m *defaultSignalManager) ensureStateLocked(sym string) *State {
	st, ok := m.state[sym]
	if !ok {
		st = &State{LastByLevel: make(map[int]Signal)}
		m.state[sym] = st
	}
	return st
}

// 线程安全 获取state（不存在则创建）
func (m *defaultSignalManager) getState(sym string) *State {
	m.mu.RLock()
	st := m.state[sym]
	m.mu.RUnlock()
	if st != nil {
		return st
	}
	m.mu.Lock()
	st = m.ensureStateLocked(sym)
	m.mu.Unlock()
	return st
}
