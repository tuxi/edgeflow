package trend

// DivergenceResult 表示一次背离检测的结果
type DivergenceResult struct {
	Score  float64
	Reason string
}

// DivergenceManager 封装 MACD + RSI 背离逻辑
type DivergenceManager struct{}

// CheckDivergences 检查 MACD & RSI 背离
func (dm *DivergenceManager) CheckDivergences(closes, macdVals, rsiVals []float64, lookback int) []DivergenceResult {
	results := []DivergenceResult{}

	// MACD 背离
	macdScore, macdReason := CheckMacdDivergence(closes, macdVals, lookback)
	if macdReason != "" {
		results = append(results, DivergenceResult{Score: macdScore, Reason: macdReason})
	}

	// RSI 背离
	rsiScore, rsiReason := CheckRsiDivergence(closes, rsiVals, lookback)
	if rsiReason != "" {
		results = append(results, DivergenceResult{Score: rsiScore, Reason: rsiReason})
	}

	return results
}

// === 背离检测函数 ===

// MACD 顶/底背离
func checkMacdDivergence(closes, macdVals []float64, lookback int) (float64, string) {
	n := len(closes)
	if n < lookback+2 {
		return 0, ""
	}

	pricePrev := closes[n-lookback]
	priceNow := closes[n-1]
	macdPrev := macdVals[n-lookback]
	macdNow := macdVals[n-1]

	// 底背离: 价格创新低, MACD 没创新低
	if priceNow < pricePrev && macdNow > macdPrev {
		return +1.0, "MACD 底背离"
	}

	// 顶背离: 价格创新高, MACD 没创新高
	if priceNow > pricePrev && macdNow < macdPrev {
		return -1.0, "MACD 顶背离"
	}

	return 0, ""
}

// RSI 顶/底背离
func checkRsiDivergence(closes, rsiVals []float64, lookback int) (float64, string) {
	n := len(closes)
	if n < lookback+2 {
		return 0, ""
	}

	pricePrev := closes[n-lookback]
	priceNow := closes[n-1]
	rsiPrev := rsiVals[n-lookback]
	rsiNow := rsiVals[n-1]

	// 底背离: 价格创新低, RSI 没创新低
	if priceNow < pricePrev && rsiNow > rsiPrev {
		return +0.5, "RSI 底背离"
	}

	// 顶背离: 价格创新高, RSI 没创新高
	if priceNow > pricePrev && rsiNow < rsiPrev {
		return -0.5, "RSI 顶背离"
	}

	return 0, ""
}
