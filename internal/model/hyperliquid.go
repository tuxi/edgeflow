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
