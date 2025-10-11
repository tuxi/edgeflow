package entity

import (
	"time"
)

// Signal信号 对应 TrendSnapshot趋势快照是一对一的关系
type Signal struct {
	ID uint `gorm:"primaryKey"`

	Symbol          string    `gorm:"type:varchar(20);not null;index:idx_symbol_status"`
	Command         string    `gorm:"type:varchar(10);not null"`                                    // BUY/SELL
	Timestamp       time.Time `gorm:"column:timestamp;type:timestamp;not null;index:idx_timestamp"` // k线收盘时间
	ExpiryTimestamp time.Time `gorm:"column:expiry_timestamp;type:timestamp;not null"`              // 该信号理论上应该被撤销或失效的最晚时间。用于风控和状态过期检查
	Status          string    `gorm:"type:varchar(10);not null;index:idx_symbol_status"`            // ACTIVE/EXPIRED
	EntryPrice      float64   `gorm:"column:entry_price;type:decimal(15,8)"`
	MarkPrice       float64   `gorm:"column:mark_price;type:decimal(15,8)"` // k线关闭价格

	FinalScore  float64 `gorm:"column:final_score;type:decimal(5,2);not null"` // 信号分数
	Explanation string  `gorm:"type:text"`

	Period string `gorm:"column:signal_period;type:varchar(30)"`

	// 新增：扁平化的SL/TP
	RecommendedSL float64 `gorm:"column:recommended_sl;type:decimal(15,8)"`
	RecommendedTP float64 `gorm:"column:recommended_tp;type:decimal(15,8)"`

	// 新增：扁平化的URL
	ChartSnapshotURL string `gorm:"column:chart_snapshot_url;type:varchar(255)"`

	// 核心修正：JSON字段存储复杂结构
	HighFreqIndicators string `gorm:"column:details_json;type:json"` // map[string]float64

	IsPremium bool `gorm:"column:is_premium"` // 是否为精选信号，为付费用户提供

	CreatedAt time.Time `gorm:"column:created_at"`

	// GORM 关联：使用 SignalID 关联到 TrendSnapshot (关键点)
	TrendSnapshot *TrendSnapshot `gorm:"foreignKey:SignalID;references:ID"`
}

func (Signal) TableName() string {
	return "signals"
}

type TrendSnapshot struct {
	ID uint `gorm:"primaryKey"`

	// 关键外键：关联 Signal.ID，并设置 Unique 确保一对一
	SignalID uint `gorm:"column:signal_id;not null;unique"`

	Symbol string `gorm:"type:varchar(10);not null"`

	Timestamp time.Time `gorm:"column:timestamp;type:timestamp;not null"`

	Direction string  `gorm:"type:varchar(10);not null"` // UP/DOWN/NEUTRAL/REVERSAL
	LastPrice float64 `gorm:"column:last_price;type:decimal(15,8);not null"`

	Score4h  float64 `gorm:"column:score_4h;type:decimal(5,2);not null"`
	Score1h  float64 `gorm:"column:score_1h;type:decimal(5,2);not null"`
	Score30m float64 `gorm:"column:score_30m;type:decimal(5,2);not null"`

	ATR float64 `gorm:"type:decimal(15,8);not null"`
	ADX float64 `gorm:"type:decimal(15,8);not null"`
	RSI float64 `gorm:"type:decimal(15,8);not null"`

	TrendScore float64 `gorm:"column:trend_score;type:decimal(5,2);not null"`
	FinalScore float64 `gorm:"column:final_score;type:decimal(5,2);not null"`

	Indicators string `gorm:"column:indicators_json;type:json"` // {周期(30m) : { "rsi": 28,... }} //  map[string]map[string]float64

}

func (TrendSnapshot) TableName() string {
	return "trend_snapshots"
}
