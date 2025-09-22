package entity

import "time"

type Coin struct {
	Id         int64     `gorm:"column:id;primary_key;" json:"id"`
	Coin       string    `gorm:"column:coin" json:"coin"`
	Name       string    `gorm:"column:name" json:"name"`
	NameEn     string    `gorm:"column:name_en" json:"name_en"`
	LogoUrl    string    `gorm:"column:logo_url" json:"logo_url"`
	CategoryID int64     `gorm:"column:category_id" json:"category_id"`
	PriceScale int       `gorm:"column:price_scale" json:"price_scale"`
	Status     int       `gorm:"column:status" json:"status"`
	Source     string    `gorm:"column:source" json:"source"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Coin) TableName() string {
	return "coins"
}

type CoinCategory struct {
	Id        int64     `gorm:"column:id;primary_key;" json:"id"`
	Name      string    `gorm:"column:name" json:"name"`
	NameEn    string    `gorm:"column:name_en" json:"name_en"`
	LogoUrl   string    `gorm:"column:logo_url" json:"logo_url"`
	Status    int       `gorm:"column:status" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CoinCategory) TableName() string {
	return "coin_categories"
}
