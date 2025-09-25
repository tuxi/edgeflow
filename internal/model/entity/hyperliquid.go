package entity

import "time"

type Whale struct {
	Id          uint   `gorm:"primaryKey;column:id" json:"id"`
	Address     string `gorm:"size:100;index;not null;column:address;comment:钱包地址" json:"address"`
	DisplayName string `gorm:"size:100;column:display_name;comment:昵称" json:"display_name"`

	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

// TableName 可以显式指定表名
func (Whale) TableName() string {
	return "whale"
}

// 鲸鱼排行数据
type HyperLiquidWhaleStat struct {
	Id uint `gorm:"primaryKey;column:id" json:"id"`

	Address string `gorm:"size:100;index;not null;column:address;comment:钱包地址" json:"address"`

	AccountValue float64 `gorm:"column:account_value;comment:账户资产" json:"account_value"`

	// PnL 数据 盈亏金额
	PnLDay   float64 `gorm:"column:pnl_day;comment:日PNL" json:"pnl_day"`
	PnLWeek  float64 `gorm:"column:pnl_week;comment:周PNL" json:"pnl_week"`
	PnLMonth float64 `gorm:"column:pnl_month;comment:月PNL" json:"pnl_month"`
	PnLAll   float64 `gorm:"column:pnl_all_time;comment:全部时间PNL" json:"pnl_all_time"`

	// ROI 数据 收益率
	ROIDay   float64 `gorm:"column:roi_day;comment:日ROI" json:"roi_day"`
	ROIWeek  float64 `gorm:"column:roi_week;comment:周ROI" json:"roi_week"`
	ROIMonth float64 `gorm:"column:roi_month;comment:月ROI" json:"roi_month"`
	ROIAll   float64 `gorm:"column:roi_all_time;comment:全部时间ROI" json:"roi_all_time"`

	// VLM 数据 交易量
	VlmDay   float64 `gorm:"column:vlm_day;comment:日成交量" json:"vlm_day"`
	VlmWeek  float64 `gorm:"column:vlm_week;comment:周成交量" json:"vlm_week"`
	VlmMonth float64 `gorm:"column:vlm_month;comment:月成交量" json:"vlm_month"`
	VlmAll   float64 `gorm:"column:vlm_all_time;comment:全部时间成交量" json:"vlm_all_time"`

	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

// TableName 可以显式指定表名
func (HyperLiquidWhaleStat) TableName() string {
	return "hyper_whale_leaderboard"
}
