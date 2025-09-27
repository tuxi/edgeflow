package model

import (
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/hype/types"
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
	Address string `form:"address" json:"address"`
}

type HyperWhaleOpenOrdersReq struct {
	Address string `form:"address" json:"address"`
}

type WhaleUserNonFundingLedgerReq struct {
	Address string `form:"address" json:"address"`
}

// 鲸鱼多空数量
type WhaleLongShortResult struct {
	Direction string
	Total     float64
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

	PositionSkew  float64 `json:"positionSkew"`  // 仓位倾斜指数 [-1,1]
	HighRiskValue float64 `json:"highRiskValue"` // 潜在爆仓仓位价值

	// 这个信号分析模块是基于持仓数据的，不能替代专业的 K 线技术分析。 这是一个情绪指标，而不是一个完整的交易策略。
	SignalScore      float64 `json:"signalScore"`      // 合约开单信号评分: [-100, 100]
	SignalSuggestion string  `json:"signalSuggestion"` // 建议文本
}
