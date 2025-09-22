package model

import "time"

type CoinOne struct {
	CoinId     int64     `gorm:"column:id" json:"coin_id"`
	Coin       string    `gorm:"column:coin" json:"coin"`
	CategoryId int64     `gorm:"column:category_id" json:"category_id"`
	Name       string    `gorm:"column:name" json:"name"`
	NameEN     string    `gorm:"column:name_en" json:"name_en"`
	LogoUrl    string    `gorm:"column:logo_url" json:"logo_url"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

type CoinCreateNewRes struct {
	CoinId     string    `json:"coin_id"`
	Coin       string    `json:"coin"`
	CategoryId string    `json:"category_id"`
	Name       string    `json:"name"`
	NameEN     string    `json:"name_en"`
	LogoUrl    string    `json:"logo_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type CoinOneRes struct {
	CoinId     string    `json:"coin_id"`
	Coin       string    `json:"coin"`
	CategoryId string    `json:"category_id"`
	Name       string    `json:"name"`
	NameEN     string    `json:"name_en"`
	LogoUrl    string    `json:"logo_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type CoinListRes struct {
	CoinList []CoinOneRes `json:"coin_list"`
}

// 查找币种请求体
type CoinListReq struct {
	CategoryId string `json:"category_id" form:"category_id" validate:"required"`
	Limit      int    `json:"limit" form:"limit" uri:"limit"`
	Page       int    `json:"page" form:"page" uri:"page"`
	Sort       string `json:"sort" form:"sort" uri:"sort"`
}
