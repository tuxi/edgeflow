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
			//rsi := res.Values["RSI"]
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

	// 当所有指标（EMA, MACD, RSI, ADX）处理完之后

	// 计算波动
	// 计算最近 14 根 15分钟K线的atr值
	atr := CalcATR(klines, 14)
	// 这里计算挂单价格，可以这样：
	entryPrice := last.Close
	price := last.Close
	if finalAction == "buy" {
		price = entryPrice * (1 - 0.2*atr/entryPrice) // 压低一点，留20%的ATR空间
	} else if finalAction == "sell" {
		// 开空时（抬高一点卖出）
		price = entryPrice * (1 + 0.2*atr/entryPrice)
	}

	values["close"] = last.Close

	// 步骤 1: 生成基于所有指标的“主要信号”
	sig := &Signal{
		Symbol:     symbol,
		Price:      price,
		Side:       finalAction, // finalAction 是 EMA/MACD/RSI/ADX 综合出的主要方向
		Strength:   strength,    // strength 是 EMA/MACD/RSI/ADX 综合出的主要强度
		IsReversal: false,       // 默认不是反转信号
		Timestamp:  last.Timestamp,
		Values:     values,
	}

	// 步骤 2: 独立运行 ReversalDetector，获取反转判断结果
	//reversalDetected, reversalSide, reversalStrength := sg.runReversalDetector(last, values) // 假设这个函数独立计算反转信号

	// 步骤 3: 处理 ReversalDetector 的结果
	if reversalStrength >= 0.7 { // 如果 ReversalDetector 明确发出强反转信号
		sig.IsReversal = true
		// 这里的 sig.Side 和 sig.Strength 是否被覆盖，取决于你的决策引擎如何使用 IsReversal
		// 如果决策引擎会优先处理 IsReversal = true 的信号，那么可以不覆盖
		// 如果需要明确信号方向就是反转方向，可以覆盖：
		// sig.Side = reversalSide
		// sig.Strength = reversalStrength

		// 缓存这根强反转 K 线作为潜在反转的基准
		sg.reversalSignal[symbol] = &Signal{
			Symbol:     symbol,
			Price:      last.Close,
			Side:       reversalSignal,
			Strength:   reversalStrength,
			IsReversal: true,
			Timestamp:  last.Timestamp,
			Values:     values, // 缓存完整信号以便后续判断
		}
		return sig, nil // 返回标记为反转的信号
	}

	// 步骤 4: 检查缓存中是否存在「潜在反转」并进行验证或作废
	if prev, ok := sg.reversalSignal[symbol]; ok {
		// 如果当前 K 线方向继续跟随缓存的反转方向
		if (prev.Side == "buy" && last.Close > prev.Price) ||
			(prev.Side == "sell" && last.Close < prev.Price) {
			// 延续反转方向，更新当前信号的 IsReversal 标志
			sig.IsReversal = true
			// sig.Side 和 sig.Strength 仍然来自主要信号，除非你希望反转信号强制覆盖
			// 如果需要反转信号强制覆盖，则：
			// sig.Side = prev.Side
			// sig.Strength = prev.Strength // 可以沿用之前的反转强度
			return sig, nil
		}

		// --- 作废逻辑 ---
		// 1. 时间作废：超过 3 根 K 线未确认
		if sig.Timestamp.Sub(prev.Timestamp) > 45*time.Minute {
			delete(sg.reversalSignal, symbol)
		} else if (prev.Side == "buy" && last.Close < prev.Price) || // 2. 方向作废：价格向反方向移动
			(prev.Side == "sell" && last.Close > prev.Price) {
			delete(sg.reversalSignal, symbol)
		}
	}

	// 步骤 5: 如果没有强反转，也没有延续的反转，就返回主要信号
	return sig, nil

	//sig := &Signal{
	//	Symbol:     symbol,
	//	Price:      last.Close,
	//	Side:       finalAction,
	//	Strength:   strength,
	//	IsReversal: reversal,
	//	Timestamp:  last.Timestamp,
	//	Values:     values,
	//}
	//
	//// --- 如果出现反转信号 ---
	//if reversalStrength >= 0.7 && reversalSignal != "hold" {
	//	sig.IsReversal = true
	//	sig.Side = reversalSignal
	//	sig.Strength = reversalStrength
	//	// 第一根 → 潜在反转，存到缓存
	//	sg.reversalSignal[symbol] = sig
	//	return sig, nil
	//}
	//
	//// 检查缓存是否有「潜在反转」
	//if prev, ok := sg.reversalSignal[symbol]; ok {
	//	// 如果当前 K 线方向继续跟随反转方向，即使强度不够，也确认反转
	//	if prev.Side == "buy" && last.Close > prev.Price {
	//		// 价格高于反转触发价 → 延续反转多头
	//		sig.Side = "buy"
	//		sig.IsReversal = true
	//		sg.reversalSignal[symbol] = sig
	//		return sig, nil
	//	}
	//	if prev.Side == "sell" && last.Close < prev.Price {
	//		// 价格低于反转触发价 → 延续空头反转
	//		sig.Side = "sell"
	//		sig.IsReversal = true
	//		sg.reversalSignal[symbol] = sig
	//		return sig, nil
	//	}
	//
	//	// 超过 3 根 K 线未确认 → 作废
	//	if sig.Timestamp.Sub(prev.Timestamp) > 45*time.Minute {
	//		delete(sg.reversalSignal, symbol)
	//	}
	//}

	//return sig, nil
}

// CalcATR 计算给定K线序列的ATR值
// 参数 klines: K线数据（按时间升序排列）
// 参数 period: ATR周期，比如14
func CalcATR(klines []model.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0 // 数据不够
	}

	trs := make([]float64, 0, len(klines)-1)

	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		// True Range = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trs = append(trs, tr)
	}

	// 取最近 period 根的均值 (简单平均，或可改成EMA)
	sum := 0.0
	for i := len(trs) - period; i < len(trs); i++ {
		sum += trs[i]
	}

	return sum / float64(period)
}
