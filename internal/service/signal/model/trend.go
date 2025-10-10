package model

import (
	"edgeflow/internal/model"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"time"
)

// TrendDirection 定义趋势方向常量
type TrendDirection string

const (
	TrendUp      TrendDirection = "UP"      // 上涨趋势
	TrendDown    TrendDirection = "DOWN"    // 下跌趋势
	TrendNeutral TrendDirection = "NEUTRAL" // 震荡/中性
	// 趋势反转/动量背离 这是一种趋势背离（Divergence）的判断，代表着宏观大趋势的反转
	TrendReversal = "REVERSAL"
)

// TrendScores 存储各个周期的打分和最终的加权分数
type TrendScores struct {
	FinalScore float64 `json:"final_score"` // 最终加权分数 (根据动态权重计算得出，用于决策树)

	// 单周期分数 (用于调试和分析)
	Score30m float64 `json:"score_30m"`
	Score1h  float64 `json:"score_1h"`
	Score4h  float64 `json:"score_4h"`

	// 子趋势分数 (方便复盘你的 calcTrendScores 内部逻辑)
	TrendScore float64 `json:"trend_score"` // 4h/1h 长期趋势分数（固定权重）
}

// TrendState 存储某一币种在某一时刻的完整趋势状态
// 将存储在 Redis 或数据库中，供决策服务查询
type TrendState struct {
	Symbol    string         `json:"symbol"`
	Direction TrendDirection `json:"direction"` // 最终判断方向 (UP/DOWN/NEUTRAL)
	Timestamp time.Time      `json:"timestamp"` // K线收盘时间（状态生成时间）

	Scores TrendScores `json:"scores"` // 详细的打分情况

	// 关键风险指标（从 30m K线计算，用于下单浮窗计算风险）
	ATR float64 `json:"atr"` // 波动性
	ADX float64 `json:"adx"` // 趋势强度
	RSI float64 `json:"rsi"`

	LastPrice float64 `json:"last_price"` // 趋势计算时的价格快照

	// 原始指标快照 (JSON 格式，用于复盘 ScoreForPeriod 函数的输入)
	IndicatorSnapshot map[model2.KlinePeriod]IndicatorSnapshot `json:"indicator_snapshot"`

	description string
}

func (ts *TrendState) Description() string {
	ts.description = fmt.Sprintf(
		"[Trend %s %s] 趋势score:%.2f 综合score: %.2f, 4h:%.1f 1h:%.1f 30min:%.1f 30分钟收盘价格: %.2f 当前时间:%v",
		ts.Symbol, ts.Direction.Desc(), ts.Scores.TrendScore, ts.Scores.FinalScore, ts.Scores.Score4h, ts.Scores.Score1h, ts.Scores.Score30m, ts.LastPrice, time.Now().Format("2006-01-02 15:04:05"),
	)
	return ts.description
}

func (d TrendDirection) Desc() string {
	dirStr := map[TrendDirection]string{
		TrendUp:       "多头",
		TrendDown:     "空头",
		TrendNeutral:  "横盘",
		TrendReversal: "反转",
	}[d]
	return dirStr
}

func (d TrendDirection) MatchesSide(side model.OrderSide) bool {
	switch d {
	case TrendUp:
		return side == model.Buy
	case TrendDown:
		return side == model.Sell
	default:
		return false
	}
}
