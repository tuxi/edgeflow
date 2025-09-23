package model

type CurrencyOne struct {
	CcyId      int64  `gorm:"column:id" json:"ccy_id"`
	Ccy        string `gorm:"column:ccy" json:"ccy"`
	Name       string `gorm:"column:name" json:"name"`
	NameEN     string `gorm:"column:name_en" json:"name_en"`
	LogoUrl    string `gorm:"column:logo_url" json:"logo_url"`
	PriceScale int    `gorm:"column:price_scale" json:"price_scale"`
}

type CurrencyCreateNewRes struct {
	CcyId      string `json:"ccy_id"`
	Ccy        string `json:"ccy"`
	Name       string `json:"name"`
	NameEN     string `json:"name_en"`
	LogoUrl    string `json:"logo_url"`
	PriceScale int    `json:"price_scale"`
}

type CurrencyOneRes struct {
	CcyId      string `json:"ccy_id"`
	Ccy        string `json:"ccy"`
	Name       string `json:"name"`
	NameEN     string `json:"name_en"`
	LogoUrl    string `json:"logo_url"`
	PriceScale int    `json:"price_scale"`
}

type CurrencyListRes struct {
	Total int64            `json:"total"`
	List  []CurrencyOneRes `json:"list"`
}

// 查找币种请求体
type CurrencyListReq struct {
	ExId  string `json:"ex_id" form:"ex_id" validate:"required"`
	Limit int    `json:"limit" form:"limit" uri:"limit"`
	Page  int    `json:"page" form:"page" uri:"page"`
	Sort  string `json:"sort" form:"sort" uri:"sort"`
}

// 交易所
type Exchange struct {
	ExId   int64  `gorm:"column:id;primary_key;" json:"ex_id"`
	Name   string `gorm:"column:name" json:"name"`
	NameEn string `gorm:"column:name_en" json:"name_en"`
}
