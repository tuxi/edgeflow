package talib_wrapper

import (
	"github.com/markcheno/go-talib"
)

// 为了保持代码简洁，我们只导出最新的指标计算结果
// 历史数据（如完整的MACD线）可以在内部使用，但函数只返回最新值

// -------------------------------------------------------------------
// 核心指标计算函数
// -------------------------------------------------------------------

// CalculateEMA 计算指数移动平均线 (Exponential Moving Average)
// 传入收盘价切片和周期，返回最新的 EMA 值
func CalculateEMA(closes []float64, period int) []float64 {
	if len(closes) < period {
		return []float64{0}
	}

	// go-talib 的 EMA 函数
	emas := talib.Ema(closes, period)

	// 返回完整的切片，供后续逻辑（如斜率/交叉）使用
	return emas
}

// CalculateMACD 计算移动平均收敛/发散指标
// 使用标准参数 (12, 26, 9)
// 返回完整的 MACD 线、信号线和 MACD 柱切片
func CalculateMACD(closes []float64, fastPeriod, slowPeriod, signalPeriod int) (macdLine, signalLine, hist []float64) {
	if len(closes) < slowPeriod+signalPeriod {
		// 返回长度为 1 的 0 值切片，防止在信号生成器中访问越界
		return []float64{0}, []float64{0}, []float64{0}
	}

	// go-talib 的 Macd 函数
	macdLine, signalLine, hist = talib.Macd(closes, fastPeriod, slowPeriod, signalPeriod)

	return macdLine, signalLine, hist
}

// CalculateRSI 计算相对强弱指数 (Relative Strength Index)
// 返回完整的 RSI 切片
func CalculateRSI(closes []float64, period int) []float64 {
	if len(closes) < period {
		return []float64{0}
	}

	// go-talib 的 Rsi 函数
	rsiVals := talib.Rsi(closes, period)
	return rsiVals
}

// CalculateADX 计算平均趋向指数 (Average Directional Index)
// 返回完整的 ADX 切片
func CalculateADX(highs, lows, closes []float64, period int) []float64 {
	if len(closes) < 2*period {
		return []float64{0}
	}

	// go-talib 的 Adx 函数
	adxVals := talib.Adx(highs, lows, closes, period)
	return adxVals
}

// CalculateATR 计算真实波动幅度均值 (Average True Range)
// 用于计算止损距离和波动性
func CalculateATR(highs, lows, closes []float64, period int) []float64 {
	if len(closes) < period {
		return []float64{0}
	}

	// go-talib 的 Atr 函数
	atrVals := talib.Atr(highs, lows, closes, period)
	return atrVals
}

// CalculateBBands 计算布林带 (Bollinger Bands)
// 返回上轨、中轨和下轨的完整切片
func CalculateBBands(closes []float64, period int, nbDevUp, nbDevDn float64, maType talib.MaType) (upper, middle, lower []float64) {
	if len(closes) < period {
		return []float64{0}, []float64{0}, []float64{0}
	}

	// go-talib 的 BBands 函数
	upper, middle, lower = talib.BBands(closes, period, nbDevUp, nbDevDn, maType)
	return upper, middle, lower
}

// -------------------------------------------------------------------
// 辅助函数（在 ScoreForPeriod 中可能使用）
// -------------------------------------------------------------------

// CalcKDJ 简化 KDJ 的计算
func CalcKDJ(highs, lows, closes []float64) (kVals, dVals, jVals []float64) {
	// KDJ 常用参数 (9, 3, 3)
	kVals, dVals = talib.Stoch(highs, lows, closes, 9, 3, talib.SMA, 3, talib.SMA)

	// J 值计算: J = 3 * K - 2 * D
	jVals = make([]float64, len(kVals))
	for i := 0; i < len(kVals); i++ {
		jVals[i] = 3*kVals[i] - 2*dVals[i]
	}
	return
}
