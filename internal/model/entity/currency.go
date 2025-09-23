package entity

import "time"

// 币
type Currency struct {
	Id         int64     `gorm:"column:id;primary_key;" json:"id"`
	Ccy        string    `gorm:"column:ccy" json:"ccy"`
	Name       string    `gorm:"column:name" json:"name"`
	NameEn     string    `gorm:"column:name_en" json:"name_en"`
	LogoUrl    string    `gorm:"column:logo_url" json:"logo_url"`
	PriceScale int       `gorm:"column:price_scale" json:"price_scale"`
	Status     int       `gorm:"column:status" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Currency) TableName() string {
	return "currencies"
}

// 交易所
type Exchange struct {
	Id     int64  `gorm:"column:id;primary_key;" json:"id"`
	Name   string `gorm:"column:name" json:"name"`
	NameEn string `gorm:"column:name_en" json:"name_en"`
}

func (Exchange) TableName() string {
	return "exchanges"
}

type CurrencyExchanges struct {
	Id         int64 `gorm:"column:id;primary_key;" json:"id"`
	CurrencyId int64 `gorm:"column:currency_id" json:"currency_id"`
	ExchangeId int64 `gorm:"column:exchange_id" json:"exchange_id"`
	IsActive   bool  `gorm:"column:is_active;default:true" json:"is_active"`

	// 关联的货币
	Currency Currency `gorm:"foreignKey:CurrencyId;references:Id"`
	// 关联的交易所
	Exchange Exchange `gorm:"foreignKey:ExchangeId;references:Id"`
}

func (CurrencyExchanges) TableName() string {
	return "exchange_currencies"
}
