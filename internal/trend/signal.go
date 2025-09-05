package trend

import (
	"edgeflow/internal/model"
	"errors"
	"math"
	"time"
)

// ========== 信号生成器 ==========
// 单周期信号工厂：15分钟
type SignalGenerator struct {
	Indicators []Indicator
	/*
		保存反转信号的
		第一根强反弹记为「潜在反转」；
		如果下一根继续上涨，即使强度不够，也沿用反转方向 → 提前抓住行情；
		如果 2–3 根内又回到下跌 → 那么作废。
	*/
	reversalSignal map[string]*Signal
}

func NewSignalGenerator() *SignalGenerator {
	return &SignalGenerator{
		Indicators: []Indicator{
			&EMAIndicator{FastPeriod: 5, SlowPeriod: 10},
			&MACDIndicator{FastPeriod: 12, SlowPeriod: 26, SignalPeriod: 9},
			&RSIIndicator{Period: 14, Buy: 30, Sell: 70},
			NewADXIndicator()},
		reversalSignal: make(map[string]*Signal),
	}
}

func (sg *SignalGenerator) Generate(klines []model.Kline, symbol string) (*Signal, error) {
	if len(klines) == 0 {
		return nil, errors.New("klines is not empty.")
	}

	score := 0.0
	reversal := false
	last := klines[len(klines)-1]
	values := make(map[string]float64)
	var rsiStrength, adxStrength float64
	for _, ind := range sg.Indicators {
		res := ind.Calculate(klines)
		//fmt.Printf("[%s] %s -> signal=%s values=%v\n", symbol, res.Name, res.Signal, res.Values)

		switch res.Signal {
		case "buy":
			score++
		case "sell":
			score--
		}
		for k, v := range res.Values {
			values[k] = v
		}
		// 把指标值拿出来算强度/反转
		switch res.Name {
		case "RSI":
			rsi := res.Values["rsi"]
			if rsi < 25 || rsi > 75 {
				reversal = true
			}
			rsiStrength = res.Strength
		case "ADX":
			adxStrength = res.Strength
		}
	}

	// --- 普通投票结果 ---
	finalAction := "hold"
	if score > 0 {
		finalAction = "buy"
	} else if score < 0 {
		finalAction = "sell"
	}

	// 反转指标是综合指标，单独计算
	rd := NewReversalDetector()
	rdRes := rd.Calculate(klines)
	// 反转强度
	reversalStrength := rdRes.Strength
	// 反转信号的方向
	reversalSignal := rdRes.Signal
	for k, v := range rdRes.Values {
		values[k] = v
	}

	// 综合强度
	rawStrength := math.Abs(score) / float64(len(sg.Indicators))
	strength := 0.5*rawStrength + 0.3*rsiStrength + 0.2*adxStrength

	sig := &Signal{
		Symbol:     symbol,
		Price:      last.Close,
		Side:       finalAction,
		Strength:   strength,
		IsReversal: reversal,
		Timestamp:  last.Timestamp,
		Values:     values,
	}

	// --- 如果出现反转信号 ---
	if reversalStrength >= 0.7 && reversalSignal != "hold" {
		sig.IsReversal = true
		sig.Side = reversalSignal
		sig.Strength = reversalStrength
		// 第一根 → 潜在反转，存到缓存
		sg.reversalSignal[symbol] = sig
		return sig, nil
	}

	// 检查缓存是否有「潜在反转」
	if prev, ok := sg.reversalSignal[symbol]; ok {
		// 如果当前 K 线方向继续跟随反转方向，即使强度不够，也确认反转
		if prev.Side == "buy" && last.Close > prev.Price {
			// 价格高于反转触发价 → 延续反转多头
			sig.Side = "buy"
			sig.IsReversal = true
			sg.reversalSignal[symbol] = sig
			return sig, nil
		}
		if prev.Side == "sell" && last.Close < prev.Price {
			// 价格低于反转触发价 → 延续空头反转
			sig.Side = "sell"
			sig.IsReversal = true
			sg.reversalSignal[symbol] = sig
			return sig, nil
		}

		// 超过 3 根 K 线未确认 → 作废
		if sig.Timestamp.Sub(prev.Timestamp) > 45*time.Minute {
			delete(sg.reversalSignal, symbol)
		}
	}

	return sig, nil
}
