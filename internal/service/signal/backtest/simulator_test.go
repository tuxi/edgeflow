package backtest

import (
	model2 "edgeflow/internal/model"
	"fmt"
	"testing"
)

func TestSimulator(t *testing.T) {
	exampleSimulation()
}

// 示例用法:
func exampleSimulation() {
	// 这是一个使用 SimulateOutcome 的示例
	entry := 100.0
	// 假设信号计算出 SL=98.0 (2% loss), TP=104.0 (4% gain)
	sl := 98.0
	tp := 104.0
	isLong := true

	klines := []model2.Kline{
		{Open: 100.0, High: 100.5, Low: 99.0, Close: 100.2},
		{Open: 100.2, High: 101.5, Low: 99.8, Close: 101.0},
		{Open: 101.0, High: 102.5, Low: 100.5, Close: 102.0},
		{Open: 102.0, High: 104.5, Low: 101.8, Close: 104.0},
	}

	// 注意：现在需要传入 TP 和 SL 价格
	result := SimulateOutcome(entry, tp, sl, isLong, klines)

	fmt.Printf("Simulation Result: %s, PnL: %.2f%%\n", result.Outcome, result.FinalPnlPct*100)
}
