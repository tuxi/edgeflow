package model

import (
	"edgeflow/pkg/utils"
	"sync"
)

type SymbolManager struct {
	symbols []string
	mu      sync.RWMutex
}

func NewSymbolManager(initialSymbols []string) *SymbolManager {
	newSymbols := make([]string, len(initialSymbols))
	for i, symbol := range initialSymbols {
		newSymbols[i] = utils.FormatSymbol(symbol)
	}
	return &SymbolManager{
		symbols: newSymbols,
	}
}

// GetSymbols 返回当前所有符号的副本 (Read Only)
func (sm *SymbolManager) GetSymbols() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 返回副本，防止外部修改
	copySymbols := make([]string, len(sm.symbols))
	copy(copySymbols, sm.symbols)
	return copySymbols
}

// AddSymbol/RemoveSymbol 等方法留给未来 Admin API 调用
// func (sm *SymbolManager) AddSymbol(symbol string) { ... }
// func (sm *SymbolManager) RemoveSymbol(symbol string) { ... }
