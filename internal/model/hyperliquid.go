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
	Limit  int    `json:"limit"`
	Period string `json:"period"`
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
