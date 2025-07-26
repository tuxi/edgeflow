package exchange

import (
	"context"
	"edgeflow/internal/model"
)

type Exchange interface {
	// 执行订单
	PlaceOrder(ctx context.Context, order model.Order) (model.OrderResponse, error)
	// 获取最新价格
	GetLastPrice(symbol string) (float64, error)
	// 取消订单
	CancelOrder(orderID string) error
	// 获取订单状态
	GetOrderStatus(orderID string) (model.OrderStatus, error)
}

// Account 账号结构接口
type Account interface {
	GetAvailableBalance() (float64, error) // 返回可用 USDT 余额
}
