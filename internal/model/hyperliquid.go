package model

import (
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/hype/types"
	"time"
)

type WhaleEntryListRes struct {
	Total int64                    `json:"total"`
	List  []*HyperWhaleLeaderBoard `json:"list"`
}

// 鲸鱼排行数据
type HyperWhaleLeaderBoard struct {
	DisplayName string `gorm:"size:100;column:display_name;comment:昵称" json:"display_name"`

	entity.HyperLiquidWhaleStat
}

type HyperWhalePortfolioRes struct {
	HyperWhaleLeaderBoard

	PortfolioData struct {
		TotalPnl     []types.DataPoint `json:"totalPnl"`
		PerpPnl      []types.DataPoint `json:"perpPnl"`
		TotalBalance []types.DataPoint `json:"totalBalance"`
		PerpBalance  []types.DataPoint `json:"perpBalance"`
	} `gorm:"-" json:"portfolio_data"`
}

// 鲸鱼排行数据
type HyperWhale struct {
	Address string `gorm:"size:100;index;not null;column:address;comment:钱包地址" json:"address"`
}

type HyperWhaleLeaderBoardReq struct {
	Limit        int    `json:"limit"`
	FilterPeriod string `json:"filterPeriod"`
	DatePeriod   string `json:"datePeriod"`
}

type HyperWhaleAccountReq struct {
	Address string `form:"address" json:"address"`
}

type HyperWhaleAccountRes struct {
	Info types.MarginData `json:"info"`
}

type HyperWhaleFillOrdersReq struct {
	Address         string `form:"address" json:"address"`
	Start           int64  `form:"start" json:"start"`
	MaxLookbackDays int    `form:"max_lookback_days" json:"max_lookback_days"`
	PrevWindowHours int    `form:"prev_window_hours" json:"prev_window_hours"`
}

type HyperWhaleOpenOrdersReq struct {
	Address string `form:"address" json:"address"`
}

type WhaleUserNonFundingLedgerReq struct {
	Address string `form:"address" json:"address"`
}

// 鲸鱼多空数量
type WhaleLongShortResult struct {
	Direction string `json:"direction"`
	Total     int64  `json:"total"`
}

// 鲸鱼多空比
type WhaleLongShortRatio struct {
	LongValue  float64 `json:"long_value"`
	ShortValue float64 `json:"short_value"`
	Ratio      float64 `json:"ratio"` // LongValue / ShortValue
}

// 鲸鱼仓位分析数据
type WhalePositionAnalysis struct {
	// 基础汇总
	TotalValue      float64 `json:"totalValue"`
	LongValue       float64 `json:"longValue"`
	ShortValue      float64 `json:"shortValue"`
	TotalMargin     float64 `json:"totalMargin"`
	LongMargin      float64 `json:"longMargin"`
	ShortMargin     float64 `json:"shortMargin"`
	TotalPnl        float64 `json:"totalPnl"`
	LongPnl         float64 `json:"longPnl"`
	ShortPnl        float64 `json:"shortPnl"`
	TotalFundingFee float64 `json:"totalFundingFee"`
	LongFundingFee  float64 `json:"longFundingFee"`
	ShortFundingFee float64 `json:"shortFundingFee"`

	// 扩展分析
	LongCount        int     `json:"longCount"`
	ShortCount       int     `json:"shortCount"`
	LongAvgValue     float64 `json:"longAvgValue"`
	ShortAvgValue    float64 `json:"shortAvgValue"`
	LongAvgLeverage  float64 `json:"longAvgLeverage"`
	ShortAvgLeverage float64 `json:"shortAvgLeverage"`
	LongProfitRate   float64 `json:"longProfitRate"`
	ShortProfitRate  float64 `json:"shortProfitRate"`

	LongPercentage  float64 `json:"longPercentage"`  // 多单价值占总价值比例
	ShortPercentage float64 `json:"shortPercentage"` // 空单价值占总价值比例

	PositionSkew  float64 `json:"positionSkew"`  // 仓位倾斜指数 [-1,1]，大于0多头主导、<0空头主导、=0平衡
	HighRiskValue float64 `json:"highRiskValue"` // 潜在爆仓仓位价值

	// 这个信号分析模块是基于持仓数据的，不能替代专业的 K 线技术分析。 这是一个情绪指标，而不是一个完整的交易策略。
	SignalScore      float64 `json:"signalScore"`      // 合约开单信号评分: [-100, 100]
	SignalSuggestion string  `json:"signalSuggestion"` // 建议文本
}

// 鲸鱼仓位查询条件
type WhalePositionFilterReq struct {
	// 筛选条件
	Coin             string `form:"coin" json:"coin,omitempty"`                           // 币种名称 (如: "ETH")
	Side             string `form:"side" json:"side,omitempty"`                           // 方向 ("long" 或 "short")
	PnlStatus        string `form:"pnl_status" json:"pnl_status,omitempty"`               // 盈亏状态 ("profit", "loss", "neutral")
	FundingFeeStatus string `form:"fundingfee_status" json:"fundingfee_status,omitempty"` // 资金费状态 ("profit", "loss", "neutral")

	// 排序和分页 (保持不变)
	Limit  int `form:"limit" json:"limit"`   // 每页数量
	Offset int `form:"offset" json:"offset"` // 偏移量
}

type WhalePositionFilterRes struct {
	Total     int64                        `json:"total"`
	Positions []*entity.HyperWhalePosition `json:"positions"`
}

type WhalePositionFilterRedisRes struct {
	Total     int64                 `json:"total"`
	Positions []*HyperWhalePosition `json:"positions"`
}

// 鲸鱼仓位快照，数字类型都使用string，方便在redis中存取
//
//	确保一个仓位唯一的关键要素：(address, coin, leverage_type, leverage_value)
type HyperWhalePosition struct {
	Address string `json:"address"`
	Coin    string `json:"coin"`
	Type    string `json:"type"`

	EntryPx        string `json:"entry_px"`
	PositionValue  string `json:"position_value"`
	Szi            string `json:"szi"`
	LiquidationPx  string `json:"liquidation_px"`
	UnrealizedPnl  string `json:"unrealized_pnl"`
	ReturnOnEquity string `json:"return_on_equity"`

	LeverageType  string `json:"leverage_type"`
	LeverageValue string `json:"leverage_value"`

	Side       string    `json:"side"`
	MarginUsed string    `json:"margin_used"`
	FundingFee string    `json:"funding_fee"`
	UpdatedAt  time.Time `json:"updated_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// 用于存储累积状态
type WinRateStats struct {
	LastProcessedTime  int64 // 上次处理的时间戳 (毫秒级)
	AccumulatedProfits int64 // 累积盈利成交笔数
	AccumulatedTotals  int64 // 累积总平仓成交笔数
	FirstProcessedTime int64 // 首次处理时间
}

// 鲸鱼胜率返回结构体，包含所有必须返回的值
type WinRateResult struct {
	Address     string
	WinRate     float64
	TotalTrades int64
	MaxTime     int64
	IsSuccess   bool // 新增：标记本次计算是否成功
}

// 用于展示我们自己计算的鲸鱼排行榜中的单个条目
type CustomLeaderboardEntry struct {
	Rank    int     `json:"rank"`     // 排名 (1-based)
	Address string  `json:"address"`  // 鲸鱼地址
	WinRate float64 `json:"win_rate"` // 胜率 (Score)
}

// 定义一个专门用于排行榜结果的结构体，包含计算出的 WinRate
type WhaleRankResult struct {
	Address                 string  `gorm:"column:address" json:"address"`
	TotalAccumulatedTrades  int64   `gorm:"column:total_accumulated_trades" json:"total_accumulated_trades"`
	TotalAccumulatedProfits int64   `gorm:"column:total_accumulated_profits" json:"total_accumulated_profits"`
	WinRate                 float64 `gorm:"column:win_rate" json:"win_rate"` // 计算出的胜率
}

type CustomLeaderboardReq struct {
	Limit int64 `json:"limit" form:"address"`
}
type CustomLeaderboardEntryRes struct {
	Data                 []CustomLeaderboardEntry `json:"data"`
	LastUpdatedTimestamp int64                    `json:"last-updated-timestamp"`
}

type CustomLeaderboardEntryDBRes struct {
	Data                 []WhaleRankResult `json:"data"`
	LastUpdatedTimestamp int64             `json:"last-updated-timestamp"`
}
