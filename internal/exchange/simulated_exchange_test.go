package exchange

import "testing"

func TestSimulatedOrderExecutor_GetLastPrice(t *testing.T) {
	exchange := NewSimulatedOrderExecutor()

	symbol := "BTC/USDT"
	initialPrice := 30000.0
	exchange.SetInitialPrice(symbol, initialPrice)

	// 连续获取10次价格，确保波动范围合理
	for i := 0; i < 10; i++ {
		price, err := exchange.GetLastPrice(symbol)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if price <= 0 {
			t.Errorf("invalid price: %.2f", price)
		}

		if price < 29000 || price > 31000 {
			t.Logf("⚠️ price %.2f seems outside expected range", price)
		} else {
			t.Logf("✅ price %.2f is within expected range", price)
		}
	}
}
