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

// 代表一个鲸鱼的详情数据
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

	// 存储最新胜率滚动数据，并且是累积的最新总胜率
	// 胜率和总交易笔数: 根据交易订单计算而来的数据，并非hyper官方api的
	WinRateUpdatedAt *time.Time `gorm:"column:win_rate_updated_at;comment:胜率数据最后更新时间" json:"win_rate_updated_at"` // 最后一次更新胜率的时间 (用于判断数据新鲜度)

	/// 【新增】胜率累计的绝对起始时间 (用于前端展示数据的追溯期)
	WinRateCalculationStartTime *int64 `gorm:"column:win_rate_calc_start_time;comment:开始统计胜率的时间" json:"win_rate_calc_start_time"`

	// 关键增量基线：上次成功快照的结束时间戳 (用于下次增量拉取的 'since')
	LastSuccessfulTime *int64 `gorm:"column:last_successful_time;comment:最后成功保存胜率快照的时间" json:"last_successful_time"`

	// 关键累计状态：所有已处理快照的总交易笔数，针对的都是关闭订单
	TotalAccumulatedTrades *int64 `gorm:"column:total_accumulated_trades;comment:总平仓笔数" json:"total_accumulated_trades"`

	// 关键累计状态：所有已处理快照的总盈利笔数 (Σ WinningTrades)
	TotalAccumulatedProfits *int64 `gorm:"column:total_accumulated_profits;comment:总的盈利单数" json:"total_accumulated_profits"`

	// 关键累计状态：所有已处理快照的总 PnL (Σ TotalPnL)
	TotalAccumulatedPnL *float64 `gorm:"column:total_accumulated_pnl;comment:已实现的总盈利" json:"total_accumulated_pnl"`

	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at" json:"created_at"`
}

// TableName 可以显式指定表名
func (HyperLiquidWhaleStat) TableName() string {
	return "hyper_whale_leaderboard"
}

// 鲸鱼仓位快照
//
//	确保一个仓位唯一的关键要素：(address, coin, leverage_type, leverage_value)
type HyperWhalePosition struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Address string `gorm:"size:100;not null;column:address;comment:用户地址" json:"address"`
	Coin    string `gorm:"size:50;not null;column:coin;comment:币种" json:"coin"`
	Type    string `gorm:"size:20;not null;column:type;comment:仓位类型(oneWay单向/TwoWay双向)" json:"type"`

	EntryPx        string `gorm:"DECIMAL(40, 18);column:entry_px;comment:进场价格" json:"entry_px"`
	PositionValue  string `gorm:"type:DECIMAL(40, 18);column:position_value;comment:仓位价值" json:"position_value"`
	Szi            string `gorm:"type:DECIMAL(40, 18);column:szi;comment:仓位数量，当为负时是开空、为正开多" json:"szi"`
	LiquidationPx  string `gorm:"type:DECIMAL(40, 18);column:liquidation_px;comment:爆仓价" json:"liquidation_px"`
	UnrealizedPnl  string `gorm:"size:50;column:unrealized_pnl;comment:未实现盈亏" json:"unrealized_pnl"`
	ReturnOnEquity string `gorm:"size:50;column:return_on_equity;comment:股本回报率" json:"return_on_equity"`

	LeverageType  string `gorm:"size:20;not null;column:leverage_type;comment:杠杆类型" json:"leverage_type"`
	LeverageValue int    `gorm:"not null;column:leverage_value;comment:杠杆倍数" json:"leverage_value"`

	Side       string    `gorm:"size:20;not null;column:side;comment:仓位方向(long/short)" json:"side"`
	MarginUsed string    `gorm:"type:DECIMAL(40, 18);column:margin_used;comment:保证金" json:"margin_used"`
	FundingFee string    `gorm:"type:DECIMAL(40, 18);column:funding_fee;comment:资金费用" json:"funding_fee"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (HyperWhalePosition) TableName() string {
	return "hyper_whale_position"
}

// 存储每次成功计算后的时间块胜率的聚合结果
type WhaleWinRateSnapshot struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Address   string    `gorm:"index" json:"address"`    // 鲸鱼地址
	StartTime time.Time `gorm:"index" json:"start_time"` // 本次计算的开始时间 (用于聚合查询)
	EndTime   time.Time `gorm:"index" json:"end_time"`   // 本次计算的结束时间 (用于增量基线)
	// 该时间块的总平仓笔数
	TotalClosedTrades int64   `gorm:"column:total_closed_trades;comment:该时间块的总平仓笔数" json:"total_closed_trades"`
	WinningTrades     int64   `gorm:"column:winning_trades;comment:该时间块的盈利笔数" json:"winning_trades"`
	TotalPnL          float64 `gorm:"column:total_pnl;comment:该时间块的总PnL" json:"total_pnl"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (WhaleWinRateSnapshot) TableName() string {
	return "whale_winrate_snapshots"
}
