package entity

import "time"

// Tag 定义标签结构
type CryptoTag struct {
	ID          uint   `gorm:"primaryKey;column:id" json:"id"`
	Name        string `gorm:"unique;column:name" json:"name"`
	Description string `gorm:"column:description" json:"description"`
}

// Instrument 表示一个交易对的元数据
type CryptoInstrument struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`
	// 联合唯一索引的两个字段
	// uniqueIndex: 定义一个名为 uk_exchange_instrument 的联合唯一索引
	InstrumentID string `gorm:"column:instrument_id;uniqueIndex:uk_exchange_instrument" json:"instrument_id"`
	ExchangeID   uint   `gorm:"column:exchange_id;uniqueIndex:uk_exchange_instrument" json:"exchange_id"`
	BaseCcy      string `gorm:"column:base_ccy" json:"base_ccy"`   // 主币代码: BTC
	QuoteCcy     string `gorm:"column:quote_ccy" json:"quote_ccy"` // 计价币代码: USDT
	NameCN       string `gorm:"column:name_cn" json:"name_cn"`     // 中文名称: 比特币
	NameEN       string `gorm:"column:name_en" json:"name_en"`     // 英文名称: Bitcoin

	// 交易相关元数据
	Status string `gorm:"index;column:status" json:"status"` // 交易状态: LIVE, DELIST
	// 注意: Gorm 使用 float64 来表示 DECIMAL 类型，但在使用时需注意精度问题，可能需要配合自定义类型或 big.Float
	PricePrecision string `gorm:"type:decimal(40,8);column:price_precision" json:"price_precision"` // 价格精度 (tickSz)
	QtyPrecision   string `gorm:"type:decimal(40,8);column:qty_precision" json:"qty_precision"`     // 数量精度 (szIncrement)

	// 业务相关元数据 (移除 Category)
	MarketCap  uint64 `gorm:"column:market_cap" json:"market_cap"`   // 市值 (USD)
	IsContract bool   `gorm:"column:is_contract" json:"is_contract"` // 是否为合约标的

	// 多对多关系映射：Tags 字段将通过 instrument_tag_relations 表关联
	Tags []CryptoTag `gorm:"many2many:instrument_tag_relations;" json:"tags"`

	// 新增关联字段
	Exchange CryptoExchange `gorm:"foreignKey:ExchangeID" json:"exchange,omitempty"` // Gorm 关联对象 (可选)

	// 时间戳
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 为 GORM 指定表名
func (CryptoInstrument) TableName() string {
	return "crypto_instruments"
}

// TableName 为 GORM 指定表名
func (CryptoTag) TableName() string {
	return "crypto_tags"
}

// Exchange 表示交易所的元数据
type CryptoExchange struct {
	ID          uint   `gorm:"primaryKey;column:id" json:"id"`
	Code        string `gorm:"uniqueIndex;column:code" json:"code"`     // 交易所代码: OKX
	Name        string `gorm:"column:name" json:"name"`                 // 交易所名称: 欧易 OKX
	Status      string `gorm:"index;column:status" json:"status"`       // 服务状态: ACTIVE
	APIEndpoint string `gorm:"column:api_endpoint" json:"api_endpoint"` // 基础 API 地址
	Country     string `gorm:"column:country" json:"country"`           // 运营国家/地区

	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 为 GORM 指定表名
func (CryptoExchange) TableName() string {
	return "crypto_exchanges"
}
