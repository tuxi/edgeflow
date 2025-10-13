package entity

import (
	"edgeflow/utils"
	"gorm.io/datatypes"
)

// 一个账单（Bill）可能关联到一个订阅（SubscribeProduct）或者金币的购买
type Bill struct {
	Id                    int64          `gorm:"column:id;primary_key;" json:"id"`
	UserId                int64          `gorm:"column:user_id" json:"user_id"`
	CostChange            float64        `gorm:"column:cost_change" json:"cost_change"` // 本次消耗的金额
	Balance               float64        `gorm:"column:balance" json:"balance"`         // 余额
	CostComment           string         `gorm:"column:cost_comment" json:"cost_comment"`
	CreatedAt             utils.JsonTime `gorm:"column:created_at" json:"created_at"`
	UpdatedAt             utils.JsonTime `gorm:"column:updated_at" json:"updated_at"`
	CdKeyId               *int64         `gorm:"column:cdkey_id" json:"cdkey_id"`
	Extras                datatypes.JSON `gorm:"column:extras;type:json" json:"extras"`                         // 苹果内购信息，比如苹果内购的订单及校验信息
	TransactionId         string         `gorm:"column:transaction_id" json:"transaction_id"`                   // 苹果内购交易id
	OriginalTransactionId string         `gorm:"column:original_transaction_id" json:"original_transaction_id"` // 苹果内购交易原始id

	SubscribeProductId *int64           `gorm:"column:subscribeproduct_id" json:"subscribeproduct_id"`
	SubscribeProduct   SubscribeProduct `gorm:"foreignKey:subscribeproduct_id"`

	BillType       int    `gorm:"column:bill_type" json:"bill_type"`               // BillType字段可以用来表示账单的类型，例如'consumable'表示购买金币，'subscription'表示购买订阅
	OriginalBillId *int64 `gorm:"column:original_bill_id" json:"original_bill_id"` // 原始账单id，当产生退款时，OriginalBillId指向原始账单
}

func (Bill) TableName() string {
	return "bill"
}
