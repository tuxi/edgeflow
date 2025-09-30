package model

import "edgeflow/internal/model/entity"

type CurrencyOne struct {
	InstrumentID string `gorm:"column:instrument_id;uniqueIndex:uk_exchange_instrument" json:"instrument_id"`
	ExchangeID   uint   `gorm:"column:exchange_id;uniqueIndex:uk_exchange_instrument" json:"exchange_id"`
	BaseCcy      string `gorm:"column:base_ccy" json:"base_ccy"`   // 主币代码: BTC
	QuoteCcy     string `gorm:"column:quote_ccy" json:"quote_ccy"` // 计价币代码: USDT
	NameCN       string `gorm:"column:name_cn" json:"name_cn"`     // 中文名称: 比特币
	NameEN       string `gorm:"column:name_en" json:"name_en"`     // 英文名称: Bitcoin

	// 交易相关元数据
	Status string `gorm:"index;column:status" json:"status"` // 交易状态: LIVE, DELIST
	// 注意: Gorm 使用 float64 来表示 DECIMAL 类型，但在使用时需注意精度问题，可能需要配合自定义类型或 big.Float
	PricePrecision string `gorm:"type:decimal(18,8);column:price_precision" json:"price_precision"` // 价格精度 (tickSz)
	QtyPrecision   string `gorm:"type:decimal(18,8);column:qty_precision" json:"qty_precision"`     // 数量精度 (szIncrement)

	// 业务相关元数据 (移除 Category)
	MarketCap  uint64 `gorm:"column:market_cap" json:"market_cap"`   // 市值 (USD)
	IsContract bool   `gorm:"column:is_contract" json:"is_contract"` // 是否为合约标的

	// 多对多关系映射：Tags 字段将通过 instrument_tag_relations 表关联
	Tags []entity.CryptoTag `gorm:"many2many:instrument_tag_relations;" json:"tags"`

	Exchange Exchange `gorm:"foreignKey:ExchangeID" json:"exchange,omitempty"`
}

type CurrencyCreateNewRes struct {
	ID             string `json:"id"`
	InstrumentID   string `json:"instrument_id"`
	ExchangeID     string `json:"exchange_id"`
	BaseCcy        string `json:"base_ccy"`
	QuoteCcy       string `json:"quote_ccy"`
	NameCN         string `json:"name_cn"`
	NameEN         string `json:"name_en"`
	PricePrecision string `json:"price_precision"`
	QtyPrecision   string `json:"qty_precision"`
}

type CurrencyOneRes struct {
	ID             string `json:"id"`
	InstrumentID   string `json:"instrument_id"`
	ExchangeID     string `json:"exchange_id"`
	BaseCcy        string `json:"base_ccy"`
	QuoteCcy       string `json:"quote_ccy"`
	NameCN         string `json:"name_cn"`
	NameEN         string `json:"name_en"`
	PricePrecision string `json:"price_precision"`
	QtyPrecision   string `json:"qty_precision"`
}

type CurrencyListRes struct {
	Total int64                     `json:"total"`
	List  []entity.CryptoInstrument `json:"list"`
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
	ExId int64  `gorm:"column:id;primary_key;" json:"ex_id"`
	Code string `gorm:"column:code" json:"code"`
	Name string `gorm:"column:name" json:"name"`
}

type InstrumentGetAllReq struct {
	ExId string `json:"ex_id" form:"ex_id" validate:"required"`
}
