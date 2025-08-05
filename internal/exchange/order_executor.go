package exchange

import (
	"context"
	"edgeflow/internal/account"
	"edgeflow/internal/model"
)

type Exchange interface {
	// 下单
	PlaceOrder(ctx context.Context, order *model.Order) (*model.OrderResponse, error)
	// 获取最新价格
	GetLastPrice(symbol string, tradingType model.OrderTradeTypeType) (float64, error)
	// 撤销订单
	CancelOrder(orderID string, symbol string, tradingType model.OrderTradeTypeType) error
	// 获取订单状态
	GetOrderStatus(orderID string, symbol string, tradingType model.OrderTradeTypeType) (*model.OrderStatus, error)
	// 获取仓位
	GetPosition(symbol string) (long *model.PositionInfo, short *model.PositionInfo, err error)
	// 平仓
	ClosePosition(symbol string, side string, quantity float64, tdMode string) error
	Account() (acc Account)
}

// Account 账号结构接口
type Account interface {
	// 返回可用 USDT 余额
	GetAccount(ctx context.Context, coin string) (account *account.Account, err error)
}
