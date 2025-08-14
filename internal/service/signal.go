package service

import (
	"edgeflow/internal/model"
	"log"
	"sync"
)

// SignalManager 接口
type SignalManager interface {
	SaveSignal(sig model.Signal)
	GetLastSignal(symbol string, level int) (model.Signal, bool)
	ShouldExecute(sig model.Signal) (execute bool, closeFirst bool)
}

// DefaultSignalManager 默认实现
type defaultSignalManager struct {
	mu           sync.RWMutex
	lastSignals  map[string]map[int]model.Signal // symbol → level → latest signal
	level3Buffer []model.Signal
}

func NewDefaultSignalManager() SignalManager {
	return &defaultSignalManager{
		lastSignals:  make(map[string]map[int]model.Signal),
		level3Buffer: []model.Signal{},
	}
}

// 保存最新信号
func (m *defaultSignalManager) SaveSignal(sig model.Signal) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.lastSignals[sig.Symbol]; !ok {
		m.lastSignals[sig.Symbol] = make(map[int]model.Signal)
	}
	m.lastSignals[sig.Symbol][sig.Level] = sig
}

// 获取最近某等级信号
func (m *defaultSignalManager) GetLastSignal(symbol string, level int) (model.Signal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sig, ok := m.lastSignals[symbol]
	if ok {
		s, ok := sig[level]
		return s, ok
	}

	return model.Signal{}, false
}

// 核心逻辑：判断是否执行信号以及是否需要先平仓
func (m *defaultSignalManager) ShouldExecute(sig model.Signal) (bool, bool) {

	// STEP 2: 获取最新缓存
	lastSignals := m.lastSignals[sig.Symbol]

	// SETP 4: 缓存信号
	defer m.SaveSignal(sig)

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
					level3Buffer = []model.Signal{}
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
				level3Buffer = []model.Signal{}
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
