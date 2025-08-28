package strategy

import (
	"context"
	"edgeflow/internal/signal"
	"fmt"
	"log"
	"sync"
	"time"
)

// 策略调度：根据 signal.level（和其他条件）找到对应的策略
// http(webhook) ---> WebhookHandler ---> StrategyDispatcher ---> Strategy (实现类)
type StrategyDispatcher struct {
	// // 策略注册表， 支持多策略注册 // key: strategy name
	strategies map[string]StrategyExecutor
	mu         sync.RWMutex
}

func NewStrategyDispatcher() *StrategyDispatcher {
	return &StrategyDispatcher{strategies: make(map[string]StrategyExecutor)}
}

func (d *StrategyDispatcher) Register(name string, s StrategyExecutor) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.strategies[name] = s
}

func (d *StrategyDispatcher) Dispatch(sig signal.Signal, callback func(err error)) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	// 找到对应级别的策略
	s, ok := d.strategies[sig.Strategy]
	if !ok {
		if callback != nil {
			callback(fmt.Errorf("no strategy for level %d", sig.Level))
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	go func() {
		defer cancel()
		// 执行策略
		err := s.Execute(ctx, sig)
		if err != nil {
			log.Printf("StrategyDispatcher Execute error: %v", err.Error())
		}

		if callback != nil {
			callback(err)
		}
	}()
}
