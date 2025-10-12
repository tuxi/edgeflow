package model

import (
	"fmt"
	"github.com/goccy/go-json"
	"github.com/nntaoli-project/goex/v2/model"
	"time"
)

// CommandType 定义交易指令类型
type CommandType string

const (
	// 趋势信号：严格要求 4H 周期不能强烈逆向，1H 周期至少要是中性偏向。这保证了趋势跟随信号的高胜率。
	CommandBuy  CommandType = "BUY"  // 市场看涨
	CommandSell CommandType = "SELL" // 市场看跌
	// 反转信号：忽略趋势方向，但设定一个 “压倒性趋势”的安全阀。(REVERSAL_BUY/REVERSAL_SELL) 优先于一切趋势跟随信号被触发。
	CommandReversalBuy  CommandType = "REVERSAL_BUY"  // 价格在下跌趋势中出现超卖或多头背离，暗示反弹/反转
	CommandReversalSell CommandType = "REVERSAL_SELL" // 价格在上涨趋势中出现超买或空头背离，暗示回调/反转。
	CommandTrendExit    CommandType = "TREND_EXIT"    // 主趋势信号（BUY 或 SELL）持续太久，且动能衰竭，提示平仓
)

// SignalDetails 存储支撑信号的所有指标快照和文字依据
// 对应前端 SignalDetailView 的 "信号触发依据"
// 将存储在 signal_details 数据库表中
type SignalDetails struct {
	// 高频指标快照：15m 的 MACD/RSI/均线等原始数值（你的开仓依据）
	HighFreqIndicators map[string]float64 `json:"high_freq_indicators"`

	// 文字依据：用于前端展示的信号触发解释
	BasisExplanation string `json:"basis_explanation"`

	// 趋势分数快照：信号通过过滤时，使用的 FinalScore，方便复盘
	FinalScoreUsed float64 `json:"final_score_used"`

	// 策略建议的 SL/TP 价格
	RecommendedSL float64 `json:"recommended_sl"`
	RecommendedTP float64 `json:"recommended_tp"`

	// K线图快照 URL
	ChartSnapshotURL string `json:"chart_snapshot_url"`
}

// Signal 是经过决策树过滤后，最终推送到前端的可执行交易指令
// 将存储在 signals 数据库表中
type Signal struct {
	SignalID   string      `json:"signal_id"` // 唯一的信号 ID
	Symbol     string      `json:"symbol"`
	Command    CommandType `json:"command"`     // 交易指令
	EntryPrice float64     `json:"entry_price"` // 建议入场价格
	MarkPrice  float64     `json:"mark_price"`  // k线关闭价格
	TimeFrame  string      `json:"time_frame"`  // 5min 或 15min (信号生成周期)

	// 状态和时效性
	Status          string    `json:"status"`           // ACTIVE, EXPIRED, EXECUTED
	ExpiryTimestamp time.Time `json:"expiry_timestamp"` // 信号失效时间

	Timestamp time.Time `json:"timestamp"` // k线收盘时间

	// 绩效和追踪
	WinRate float64 `json:"win_rate"` // 策略历史胜率

	Score float64 `json:"score"` // 信号分数

	// 详细指标和依据（内嵌，或通过 SignalID 关联查询）
	Details SignalDetails `json:"details"`
}

// ExecutionCommandType 定义最终订单管理系统 (OMS) 需要执行的操作
type ExecutionCommandType string

const (
	ExecNone       ExecutionCommandType = "NONE"        // 不执行任何操作
	ExecOpenLong   ExecutionCommandType = "OPEN_LONG"   // 开多仓
	ExecOpenShort  ExecutionCommandType = "OPEN_SHORT"  // 开空仓
	ExecCloseLong  ExecutionCommandType = "CLOSE_LONG"  // 平多仓
	ExecCloseShort ExecutionCommandType = "CLOSE_SHORT" // 平空仓
	// ExecReverseLong ExecutionCommandType = "REVERSE_LONG"  // 平空并反手开多 (如果交易所支持，或分两步执行)
)

// DecisionResult 是决策树的输出结构
type DecisionResult struct {
	ExecuteCommand ExecutionCommandType // 最终要执行的操作
	IsReversal     bool                 // 是否涉及反手操作
	Reason         string               // 决策依据
}

// IndicatorSnapshot 存储了 ScoreForPeriod 计算出的所有原始指标的最终值。
// 它可以直接用于填充 TrendSnapshot 或 IndicatorSnapshot 表中的字段。
type IndicatorSnapshot struct {
	LastPrice  float64 `json:"last_price"`
	EMA20      float64 `json:"ema20"`
	EMA50      float64 `json:"ema50"`
	EMA200     float64 `json:"ema200"`
	ADX        float64 `json:"adx"`
	BBWidth    float64 `json:"bb_width"`     // 当前布林带宽度
	BBWidthAvg float64 `json:"bb_width_avg"` // 历史平均布林带宽度
	KVal       float64 `json:"k_val"`        // KDJ K 值
	DVal       float64 `json:"d_val"`        // KDJ D 值
	JVal       float64 `json:"j_val"`        // KDJ J 值
	RSI        float64 `json:"rsi"`
	MACD       float64 `json:"macd"`        // MACD Line
	MACDSignal float64 `json:"macd_signal"` // MACD Signal Line
	MACDHist   float64 `json:"macd_hist"`   // MACD Histogram
	Reasons    string  `json:"reasons"`
}

// TransformSnapshotsToJSONMap 将 map[KlinePeriod]IndicatorSnapshot 转换为 GORM JSON 字段所需格式。
// 目标格式: map[周期字符串]map[指标名称]指标数值
func TransformSnapshotsToJSONMap(snapshots map[model.KlinePeriod]IndicatorSnapshot) map[string]map[string]float64 {

	// 初始化最终的结果 map
	// 格式: map[string]map[string]float64
	result := make(map[string]map[string]float64)

	// 遍历所有周期的 IndicatorSnapshot
	for period, snapshot := range snapshots {
		// 1. 将 KlinePeriod 类型作为外层 key (例如 "4h", "30m")
		periodStr := string(period)

		// 2. 使用 JSON 序列化/反序列化实现自动映射，以应对 IndicatorSnapshot 字段的增加。
		// 这样做比手动映射更健壮，前提是 IndicatorSnapshot 中的字段都被正确导出或使用了 json tag。

		// 2.1: 序列化 struct
		jsonBytes, err := json.Marshal(snapshot)
		if err != nil {
			fmt.Printf("Error marshaling snapshot for period %s: %v\n", periodStr, err)
			continue // 跳过失败的快照
		}

		// 2.2: 反序列化为 map[string]interface{}
		var indicatorMapI map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &indicatorMapI); err != nil {
			fmt.Printf("Error unmarshaling JSON for period %s: %v\n", periodStr, err)
			continue // 跳过失败的快照
		}

		// 2.3: 转换 map[string]interface{} 为 map[string]float64
		indicatorMap := make(map[string]float64)
		for key, val := range indicatorMapI {
			// 假设 IndicatorSnapshot 中的所有值都是 float64 类型
			if f, ok := val.(float64); ok {
				indicatorMap[key] = f
			} else {
				// 如果有非 float64 字段 (例如 string)，可以根据需求忽略或尝试其他类型转换。
				// 鉴于我们的目标是 map[string]float64，这里我们只保留 float64。
			}
		}

		// 3. 将结果存入最终 map
		result[periodStr] = indicatorMap
	}

	return result
}
