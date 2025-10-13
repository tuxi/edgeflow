package indicator

import (
	model2 "edgeflow/internal/model"
	"edgeflow/internal/service/signal/model"
	"encoding/json"
	"errors"
	"github.com/markcheno/go-talib"
	model3 "github.com/nntaoli-project/goex/v2/model"
	"math"
	"time"
)

// ==================== Signal Generator (加权评分系统) ====================
// 信号生成系统
type SignalGenerator struct {
	Indicators []Indicator
	Weights    map[string]float64 // 核心：指标权重表
	TimeFrame  model3.KlinePeriod
}

func NewSignalGenerator(timeFrame model3.KlinePeriod) *SignalGenerator {
	// 定义权重：EMA (趋势) > MACD (动量) > RSI (过滤)
	defaultWeights := map[string]float64{
		"EMA":  3.0, // 趋势确认最重要
		"MACD": 2.0, // 动量确认次之
		"RSI":  1.0, // 超买超卖/过滤
		"ADX":  0.0, // ADX只用于强度计算，不参与方向投票（方向由DI线判断）
	}

	return &SignalGenerator{
		Indicators: []Indicator{
			&EMAIndicator{FastPeriod: 5, SlowPeriod: 10, TrendPeriod: 30}, // 5/10/30 三均线
			&MACDIndicator{FastPeriod: 12, SlowPeriod: 26, SignalPeriod: 9},
			&RSIIndicator{Period: 14, Buy: 30, Sell: 70},
			NewADXIndicator()},
		Weights:   defaultWeights,
		TimeFrame: timeFrame,
	}
}

func (sg *SignalGenerator) Generate(symbol string, klines []model2.Kline) (*model.Signal, error) {
	if len(klines) == 0 {
		return nil, errors.New("klines is not empty")
	}

	score := 0.0
	last := klines[len(klines)-1]
	allIndicatorValues := make(map[string]float64) // 用于 HighFreqIndicators

	// 用于强度计算的变量和 Basis 文本所需的指标
	var rsiStrength, adxStrength float64
	var diPlus, diMinus float64
	var rsiNow float64                                                           // 用于 Basis 文本
	var totalRationale = make(map[string]IndicatorRationale, len(sg.Indicators)) // 用于存储所有指标的依据文本

	// --- 1. 指标计算与加权评分 ---
	for _, ind := range sg.Indicators {
		res := ind.Calculate(klines)

		weight := sg.Weights[res.Name]
		// 收集每个指标的依据文本
		totalRationale[ind.GetName()] = res.Rationale

		switch res.Signal {
		case "buy", "strong_trend":
			score += weight
		case "sell", "weak_trend":
			score -= weight
		case "weak_buy":
			score += weight * 0.5
		case "weak_sell":
			score -= weight * 0.5
		}

		for k, v := range res.Values {
			allIndicatorValues[k] = v
		}

		// 提取关键值
		switch res.Name {
		case "RSI":
			rsiStrength = res.Strength
			rsiNow = res.Values["rsi"]
		//case "MACD":
		//macdNow = res.Values["macd"]
		case "ADX":
			adxStrength = res.Strength
			diPlus = res.Values["di+"]
			diMinus = res.Values["di-"]
		}

	}

	// 信号依据详情
	totalRationaleJson, err := json.Marshal(totalRationale)
	var finalRationale string
	if err == nil {
		finalRationale = string(totalRationaleJson)
	}

	// --- 2. 最终方向判断（Command） ---
	var isReversalSignal bool = false // 用于标记是否触发了反转信号

	// 假设 finalScore 和 ADX 相关的指标已计算并传入
	finalScore := score // 使用 core score 作为最终分数

	// --- 【反转信号判断】抄底/逃顶逻辑（优先级最高） ---
	const RSI_OVERSOLD = 30.0   // 极度超卖
	const RSI_OVERBOUGHT = 70.0 // 极度超买

	// 如果 isReversalSignal 为 true，那么 finalAction 已经是 REVERSAL_BUY/SELL
	// 否则 finalAction 是 BUY/SELL 或空（如果 ADX 也无法确认）
	var finalAction model.CommandType

	// 判断是否出现超卖/抄底买入机会
	if rsiNow <= RSI_OVERSOLD {
		finalAction = model.CommandReversalBuy
		isReversalSignal = true
	} else if rsiNow >= RSI_OVERBOUGHT {
		// 判断是否出现超买/逃顶卖出机会
		finalAction = model.CommandReversalSell
		isReversalSignal = true
	}

	// --- 2. 【趋势跟随判断】核心投票逻辑（仅在没有反转信号时执行） ---
	if !isReversalSignal {
		if finalScore > 1.0 {
			finalAction = model.CommandBuy
		} else if finalScore < -1.0 {
			finalAction = model.CommandSell
		} else {
			// 投票结果不明确时，使用 ADX 的 DI 线来确认趋势方向
			if diPlus > diMinus && adxStrength > 0.4 {
				finalAction = model.CommandBuy
			} else if diMinus > diPlus && adxStrength > 0.4 {
				finalAction = model.CommandSell
			}
		}
	}

	// --- 3. 反转指标（独立运行，只获取值）---
	rd := NewReversalDetector()
	rdRes := rd.Calculate(klines)
	for k, v := range rdRes.Values {
		allIndicatorValues["rev_"+k] = v // 将反转指标值带前缀存入
	}

	// --- 4. 综合强度计算 (并存储) ---
	rawStrength := math.Abs(score) / float64(len(sg.Indicators))
	strength := 0.6*rawStrength + 0.2*rsiStrength + 0.2*adxStrength
	allIndicatorValues["finalStrength"] = strength // 存储最终强度值

	// --- 5. 挂单价格计算 (使用ATR进行优化) ---
	highs, lows := extractHighsLows(klines)
	closes := extractCloses(klines)
	atrVal := talib.Atr(highs, lows, closes, 14)
	// 短期移动平均来平滑 ATR 的最终值
	// 计算最近 3 到 5 根 K 线 ATR 值的 EMA 或 SMA
	atrEma3 := talib.Ema(atrVal, 3)
	// 使用平滑后的 ATR 值作为最终的波动率指标
	smoothedAtr := atrEma3[len(atrEma3)-1]

	entryPrice := last.Close

	if finalAction == model.CommandBuy {
		// 开多时（压低一点买入）: 留 20% ATR 的空间
		entryPrice = last.Close - 0.2*smoothedAtr
	} else if finalAction == model.CommandSell {
		// 开空时（抬高一点卖出）: 留 20% ATR 的空间
		entryPrice = last.Close + 0.2*smoothedAtr
	}
	if finalAction == model.CommandReversalBuy {
		// 开多时（压低一点买入）: 留 10% ATR 的空间
		entryPrice = last.Close - 0.1*smoothedAtr
	} else if finalAction == model.CommandReversalSell {
		// 开空时（抬高一点卖出）: 留 10% ATR 的空间
		entryPrice = last.Close + 0.1*smoothedAtr
	}
	allIndicatorValues["atr"] = smoothedAtr
	allIndicatorValues["close"] = last.Close

	// --- 6. 构建 SignalDetails ---
	details := model.SignalDetails{
		HighFreqIndicators: allIndicatorValues,
		BasisExplanation:   finalRationale,
		RecommendedSL:      sg.calculateSL(finalAction, entryPrice, smoothedAtr),
		RecommendedTP:      sg.calculateTP(finalAction, entryPrice, smoothedAtr),
	}

	// --- 7. 构建原始信号对象 ---
	rawSignal := &model.Signal{
		Symbol:          symbol,
		Command:         finalAction,
		EntryPrice:      entryPrice,
		TimeFrame:       string(sg.TimeFrame),
		Status:          "RAW",                            // 原始信号，等待过滤
		ExpiryTimestamp: time.Now().Add(30 * time.Minute), // 初始设置 30 分钟有效
		Timestamp:       last.Timestamp,
		Details:         details,
		Score:           score,
		MarkPrice:       last.Close,
	}

	return rawSignal, nil
}

// --- 辅助函数：根据新的信号结构所需添加 ---

// calculateSL 计算推荐止损（占位符实现）
func (sg *SignalGenerator) calculateSL(command model.CommandType, entryPrice float64, atr float64) float64 {
	// 使用 2 倍 ATR 作为止损
	if command == model.CommandBuy {
		return entryPrice - 2.0*atr
	} else if command == model.CommandSell {
		return entryPrice + 2.0*atr
	}
	if command == model.CommandReversalBuy {
		return entryPrice - 3.0*atr
	} else if command == model.CommandReversalSell {
		return entryPrice + 3.0*atr
	}
	return 0.0
}

// calculateTP 计算推荐止盈（占位符实现）
func (sg *SignalGenerator) calculateTP(command model.CommandType, entryPrice float64, atr float64) float64 {
	// 使用 3 倍 ATR 作为止盈
	if command == model.CommandBuy {
		return entryPrice + 3.0*atr
	} else if command == model.CommandSell {
		return entryPrice - 3.0*atr
	}
	if command == model.CommandReversalBuy {
		return entryPrice + 4.0*atr
	} else if command == model.CommandReversalSell {
		return entryPrice - 4.0*atr
	}
	return 0.0
}
