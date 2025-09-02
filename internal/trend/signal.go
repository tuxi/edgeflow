package trend

import (
	"edgeflow/internal/model"
	"errors"
	"fmt"
	"math"
	"time"
)

// ========== 信号生成器 ==========
// 单周期信号工厂：15分钟
type SignalGenerator struct {
	Indicators []Indicator
}

func NewSignalGenerator() *SignalGenerator {
	return &SignalGenerator{
		Indicators: []Indicator{
			&EMAIndicator{FastPeriod: 5, SlowPeriod: 10},
			&MACDIndicator{FastPeriod: 12, SlowPeriod: 26, SignalPeriod: 9},
			&RSIIndicator{Period: 14, Buy: 30, Sell: 70},
			NewADXIndicator(),
			NewReversalDetector()},
	}
}

func (sg *SignalGenerator) Generate(klines []model.Kline, symbol string) (*Signal, error) {
	if len(klines) == 0 {
		return nil, errors.New("klines is not empty.")
	}

	score := 0.0
	reversal := false
	last := klines[len(klines)-1]

	var rsiStrength, adxStrength float64
	for _, ind := range sg.Indicators {
		res := ind.Calculate(klines)
		fmt.Printf("[%s] %s -> signal=%s values=%v\n", symbol, res.Name, res.Signal, res.Values)

		switch res.Signal {
		case "buy":
			score++
		case "sell":
			score--
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

	// 综合强度
	rawStrength := math.Abs(score) / float64(len(sg.Indicators))
	strength := 0.5*rawStrength + 0.3*rsiStrength + 0.2*adxStrength

	sig := &Signal{
		Symbol:     symbol,
		Price:      last.Close,
		Side:       finalAction,
		Strength:   strength,
		IsReversal: reversal,
		Timestamp:  time.Now(),
	}

	// --- 如果出现反转信号 ---
	if reversalStrength > 0.7 && reversalSignal != "hold" {
		sig.IsReversal = true
		sig.Side = reversalSignal
		sig.Strength = reversalStrength
	}

	return sig, nil
}
