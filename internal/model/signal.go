package model

import "time"

type Signal struct {
	SignalID   int64   `gorm:"column:id" json:"signal_id"` // 唯一的信号 ID
	Symbol     string  `gorm:"column:symbol" json:"symbol"`
	Command    string  `gorm:"column:command" json:"command"`                            // 交易指令
	EntryPrice float64 `gorm:"column:entry_price;type:decimal(15,8)" json:"entry_price"` // 建议入场价格
	TimeFrame  string  `gorm:"column:time_frame" json:"time_frame"`                      // 5min 或 15min (信号生成周期)

	// 状态和时效性
	Status          string    `gorm:"column:status" json:"status"`                     // ACTIVE, EXPIRED, EXECUTED
	IsPremium       bool      `gorm:"column:is_premium" json:"is_premium"`             // 付费信号标识
	ExpiryTimestamp time.Time `gorm:"column:expiry_timestamp" json:"expiry_timestamp"` // 信号失效时间

	Timestamp time.Time `gorm:"column:timestamp" json:"timestamp"` // k线收盘时间
}
