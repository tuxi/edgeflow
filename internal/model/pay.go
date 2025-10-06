package model

import "github.com/go-pay/gopay/apple"

type AppleInAppsPayReq struct {
	Receipt   string `json:"receipt" validate:"required"`
	ProductId string `json:"product_id" validate:"required"`
}

type AppleInAppsPayNotifiationV2Req struct {
	SignedPayload string `json:"signedPayload"`
}

type AppleInAppsPayV2Req struct {
	// 代表每次下单的与苹果唯一订单
	TransactionId string `json:"transaction_id" validate:"required"`
	// 首次订阅的交易id，服务器应保存该字段到数据库，如果是消耗品，则只有一次
	OriginalTransactionId string `json:"original_transaction_id" validate:"required"`
	ProductId             string `json:"product_id" validate:"required"`
	SubscriptionGroupId   string `json:"subscription_group_id"`
	PriceCents            int    `json:"price_cents"`
}

type UserBillExtras struct {
	// 账单中的交易
	Transaction *apple.TransactionInfo `json:"transaction"`
	// 续订
	Renewal *apple.RenewalInfo `json:"renewal"`
	// 通知类型
	NotificationType    string `json:"notificationType"`
	Subtype             string `json:"subtype"`
	NotificationUUID    string `json:"notificationUUID"`
	NotificationVersion string `json:"notificationVersion"`
}

type AppleInAppsPayV2Res struct {
	TransactionId   string `json:"transaction_id"`
	AppAccountToken string `json:"app_account_token"`
}
