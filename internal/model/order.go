package model

type OrderSide string

const (
	Buy  OrderSide = "buy"
	Sell OrderSide = "sell"
)

type OrderType string

const (
	// 市价购买
	Market OrderType = "market"
	// 限价购买
	Limit OrderSide = "limit"
)

type OrderResponse struct {
	OrderId string
	//Status  string
	Success bool
	Message string
}

type OrderStatus struct {
	OrderID   string
	Status    string
	Filled    float64
	Remaining float64
}

type Order struct {
	Symbol    string // BTC-USDT
	Side      OrderSide
	Price     float64
	Quantity  float64
	OrderType OrderType
	TPPrice   float64
	SLPrice   float64
	Strategy  string
	Comment   string
}

// 用于记录订单的接口
type OrderRecorder interface {
	RrcordOrder(result *Order) error
}
