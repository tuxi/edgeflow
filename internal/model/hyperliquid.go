package model

import "edgeflow/internal/model/entity"

type WhaleEntryListRes struct {
	Total int64                    `json:"total"`
	List  []*HyperWhaleLeaderBoard `json:"list"`
}

// 鲸鱼排行数据
type HyperWhaleLeaderBoard struct {
	DisplayName string `gorm:"size:100;column:display_name;comment:昵称" json:"display_name"`

	entity.HyperLiquidWhaleStat
}
