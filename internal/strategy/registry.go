package strategy

import (
	"errors"
	"sync"
)

var (
	// 策略注册表， 支持多策略注册
	registry = make(map[string]StrategyExecutor)
	mu       sync.RWMutex
)

func Register(s StrategyExecutor) {
	mu.Lock()
	defer mu.Unlock()
	registry[s.Name()] = s
}

func Get(name string) (StrategyExecutor, error) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := registry[name]
	if !ok {
		return nil, errors.New("Strategy not found: " + name)
	}
	return s, nil
}

func Any() (StrategyExecutor, error) {
	mu.RLock()
	defer mu.RUnlock()
	for _, value := range registry {
		return value, nil
	}
	return nil, errors.New("你尚未注册任何策略")
}
