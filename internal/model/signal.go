package model

import "time"

type Signal struct {
	SignalID   int64   `gorm:"column:id" json:"signal_id"` // 唯一的信号 ID
	Symbol     string  `gorm:"column:symbol" json:"symbol"`
	Command    string  `gorm:"column:command" json:"command"`                            // 交易指令
	EntryPrice float64 `gorm:"column:entry_price;type:decimal(15,8)" json:"entry_price"` // 建议入场价格
	MarkPrice  float64 `gorm:"column:mark_price;type:decimal(15,8)" json:"mark_price"`   // 标记价格
	TimeFrame  string  `gorm:"column:signal_period" json:"time_frame"`                   // 5min 或 15min (信号生成周期)

	// 状态和时效性
	Status          string              `gorm:"column:status" json:"status"`                     // ACTIVE, EXPIRED, EXECUTED
	IsPremium       bool                `gorm:"column:is_premium" json:"is_premium"`             // 付费信号标识
	ExpiryTimestamp time.Time           `gorm:"column:expiry_timestamp" json:"expiry_timestamp"` // 信号失效时间
	Summary         *PerformanceSummary `gorm:"-" json:"summary"`                                // 策略胜率，用于展示信任

	Timestamp time.Time `gorm:"column:timestamp" json:"timestamp"` // k线收盘时间
}

func (Signal) TableName() string {
	return "signals"
}

type SignalDetailReq struct {
	SignalID string `json:"signal_id" form:"signal_id"`
}

type SignalDetail struct {
	ID int64 `gorm:"column:id" json:"signal_id"` // 唯一的信号 ID

	Symbol string `gorm:"column:symbol" json:"symbol"`

	Timestamp time.Time `gorm:"column:timestamp" json:"timestamp"` // k线收盘时间

	FinalScore  float64 `gorm:"column:final_score;type:decimal(5,2);not null" json:"final_score"` // 信号分数
	Explanation string  `gorm:"type:text" json:"explanation"`

	Period string `gorm:"column:signal_period;type:varchar(30)" json:"period"`

	// 新增：扁平化的SL/TP
	RecommendedSL float64 `gorm:"column:recommended_sl;type:decimal(15,8)" json:"recommended_sl"`
	RecommendedTP float64 `gorm:"column:recommended_tp;type:decimal(15,8)" json:"recommended_tp"`

	// 新增：扁平化的URL
	ChartSnapshotURL string `gorm:"column:chart_snapshot_url;type:varchar(255)" json:"chart_snapshot_url"`

	// 核心修正：JSON字段存储复杂结构
	HighFreqIndicators string `gorm:"column:details_json;type:json" json:"details_json"` // map[string]float64

	// GORM 关联：使用 SignalID 关联到 TrendSnapshot (关键点)
	TrendSnapshot *TrendSnapshot `gorm:"foreignKey:SignalID;references:ID" json:"trend_snapshot"`

	Klines []Kline `gorm:"-" json:"klines"`
}

func (SignalDetail) TableName() string {
	return "signals"
}

type TrendSnapshot struct {
	ID        int64     `gorm:"column:id" json:"trend_id"`
	SignalID  uint      `gorm:"column:signal_id;not null;unique" json:"signal_id"`
	Timestamp time.Time `gorm:"column:timestamp;type:timestamp;not null" json:"timestamp"`

	Direction string  `gorm:"type:varchar(10);not null" json:"direction"` // UP/DOWN/NEUTRAL/REVERSAL
	LastPrice float64 `gorm:"column:last_price;type:decimal(15,8);not null" json:"last_price"`

	Score4h  float64 `gorm:"column:score_4h;type:decimal(5,2);not null" json:"score_4h"`
	Score1h  float64 `gorm:"column:score_1h;type:decimal(5,2);not null" json:"score_1h"`
	Score30m float64 `gorm:"column:score_30m;type:decimal(5,2);not null" json:"score_30m"`

	ATR float64 `gorm:"type:decimal(15,8);not null" json:"atr"`
	ADX float64 `gorm:"type:decimal(15,8);not null" json:"adx"`
	RSI float64 `gorm:"type:decimal(15,8);not null" json:"rsi"`

	TrendScore float64 `gorm:"column:trend_score;type:decimal(5,2);not null" json:"trend_score"`
	FinalScore float64 `gorm:"column:final_score;type:decimal(5,2);not null" json:"final_score"`

	Indicators string `gorm:"column:indicators_json;type:json" json:"indicators"` // {周期(30m) : { "rsi": 28,... }} //  map[string]map[string]float64

}

func (TrendSnapshot) TableName() string {
	return "trend_snapshots"
}

// PerformanceSummary aggregates key performance indicators for a given symbol.
// PerformanceSummary 聚合了给定交易对的关键绩效指标。
type PerformanceSummary struct {
	WinRate            float64 `json:"win_rate"`             // 策略胜率 (0-100%)
	TotalPnL           float64 `json:"total_pnl"`            // 总收益率 (累加的 FinalPnlPct)
	TotalClosedSignals int64   `json:"total_closed_signals"` // 总交易笔数
}

type ActiveSignalsRes struct {
	Signals []Signal `json:"signals"`
}

// 信号下单的请求
type SignalExecutionReq struct {
	SignalID string `json:"signal_id" form:"signal_id"`
}
