package indicator

import (
	model2 "edgeflow/internal/model"
	"edgeflow/internal/service/signal/model"
	"edgeflow/internal/service/signal/talib_wrapper"
	"fmt"
	model3 "github.com/nntaoli-project/goex/v2/model"
	"time"
)

// Generator 负责根据 5/15m K线数据生成原始交易指令
type Generator struct {
	// 存储配置，如周期、MACD/RSI参数等
	TimeFrame model3.KlinePeriod
}

func NewGenerator(timeFrame model3.KlinePeriod) *Generator {
	return &Generator{TimeFrame: timeFrame}
}

// GenerateRawSignal 根据最新 K 线数据生成一个原始信号和详细指标快照
func (g *Generator) GenerateRawSignal(symbol string, klines []model2.Kline) (*model.Signal, error) {
	if len(klines) < 200 {
		return nil, fmt.Errorf("klines not enough for %s analysis", g.TimeFrame)
	}

	n := len(klines)
	// --- 1. 指标计算 ---
	// 假设 talib_wrapper 提供了简洁的计算函数
	closes, highs, lows := extractPrices(klines)

	// MACD (常用的 12, 26, 9)
	macdLine, signalLine, _ := talib_wrapper.CalculateMACD(closes, 12, 26, 9)

	// RSI (14 周期)
	rsiVals := talib_wrapper.CalculateRSI(closes, 14)

	// EMA 快速线 (例如 EMA10)
	emaFast := talib_wrapper.CalculateEMA(closes, 10)

	// 获取最新指标值
	macdNow, macdPrev := macdLine[n-1], macdLine[n-2]
	signalNow, signalPrev := signalLine[n-1], signalLine[n-2]
	rsiNow := rsiVals[n-1]
	emaFastNow, _ := emaFast[n-1], emaFast[n-2]

	// --- 2. 原始指令判断逻辑 ---
	var command model.CommandType

	// 判断 MACD 金叉/死叉 (核心入场信号)
	isGoldenCross := (macdPrev < signalPrev) && (macdNow >= signalNow) // MACD 金叉
	isDeadCross := (macdPrev > signalPrev) && (macdNow <= signalNow)   // MACD 死叉

	// 简化开仓/平仓逻辑（仅基于 MACD 交叉，后续可扩展 RSI/EMA 过滤）
	if isGoldenCross {
		command = model.CommandBuy // 原始开多信号
	} else if isDeadCross {
		command = model.CommandSell // 原始开空信号
	} else {
		return nil, fmt.Errorf("no raw signal generated") // 没有明确的开仓或平仓信号
	}

	// --- 3. 构造指标快照与信号对象 ---
	if command == "" {
		return nil, fmt.Errorf("no raw signal generated")
	}

	// --- 3. 构造指标快照 ---
	indicators := map[string]float64{
		"MACD":        macdNow,
		"Signal_Line": signalNow,
		"RSI":         rsiNow,
		"EMA_Fast":    emaFastNow,
	}

	// 模拟生成 SL/TP，这部分在策略引擎中应更复杂
	// 这里使用简单的 ATR 模拟 SL/TP 距离
	atrVal := talib_wrapper.CalculateATR(highs, lows, closes, 14)[n-1]
	entryPrice := klines[n-1].Close
	timestamp := klines[n-1].Timestamp

	details := model.SignalDetails{
		HighFreqIndicators: indicators,
		BasisExplanation:   g.createBasisText(command, macdNow, rsiNow),
		RecommendedSL:      g.calculateSL(command, entryPrice, atrVal),
		RecommendedTP:      g.calculateTP(command, entryPrice, atrVal),
	}

	// --- 4. 构建原始信号对象 ---
	// 此时 Status 标记为 RAW，等待决策树服务过滤
	rawSignal := &model.Signal{
		// 不需要生成信号id，保存数据库时由数据库生成
		//SignalID:        fmt.Sprintf("%s-%s-%d", symbol, g.TimeFrame, time.Now().Unix()),
		Symbol:          symbol,
		Command:         command,
		EntryPrice:      entryPrice,
		TimeFrame:       string(g.TimeFrame),
		Status:          "RAW",                            // 原始信号，等待过滤
		ExpiryTimestamp: time.Now().Add(15 * time.Minute), // 初始设置 15 分钟有效
		Timestamp:       timestamp,
		Details:         details,
	}

	return rawSignal, nil
}

// 模拟 SL 计算：基于 ATR 的简单 SL 距离
func (g *Generator) calculateSL(command model.CommandType, entryPrice, atr float64) float64 {
	slDistance := atr * 1.5 // 1.5倍 ATR 作为止损距离
	if command == model.CommandBuy {
		return entryPrice - slDistance
	}
	if command == model.CommandSell {
		return entryPrice + slDistance
	}
	return 0
}

// 模拟 TP 计算：基于 ATR 的简单 TP 距离 (1:1.5 盈亏比)
func (g *Generator) calculateTP(command model.CommandType, entryPrice, atr float64) float64 {
	tpDistance := atr * 2.25 // 2.25倍 ATR 作为止盈距离 (1:1.5 R:R)
	if command == model.CommandBuy {
		return entryPrice + tpDistance
	}
	if command == model.CommandSell {
		return entryPrice - tpDistance
	}
	return 0
}

// 根据指标生成前端展示的文字依据
func (g *Generator) createBasisText(command model.CommandType, macd, rsi float64) string {
	if command == model.CommandBuy {
		return fmt.Sprintf("%s MACD 发生金叉，RSI 位于 %.1f。初步形成多头动能。", g.TimeFrame, rsi)
	}
	if command == model.CommandSell {
		return fmt.Sprintf("%s MACD 发生死叉，RSI 位于 %.1f。初步形成空头动能。", g.TimeFrame, rsi)
	}
	return "无明确指标依据。"
}

// 提取价格切片，供 talib 使用
func extractPrices(klines []model2.Kline) (closes, highs, lows []float64) {
	n := len(klines)
	closes = make([]float64, n)
	highs = make([]float64, n)
	lows = make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}
	return
}
