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
	Limit OrderType = "limit"
)

type OrderResponse struct {
	OrderId string
	Status  int
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
	TradeType OrderTradeTypeType
	TradeMode OrderMgnMode
}

// 交易类型
type OrderTradeTypeType string

// 保证金模式（cross / isolated）
type OrderMgnMode string

// posSide 持仓方向 做多long或者做空short
type OrderPosSide string

const (
	// 使用现货 API
	OrderTradeSpot OrderTradeTypeType = "spot"
	// 使用合约 API
	OrderTradeSwap OrderTradeTypeType = "swap"
	// 使用交割合约 API
	OrderTradeFutures OrderTradeTypeType = "futures"
	// 全仓模式
	OrderMgnModeCross = "cross"
	// 逐仓模式
	OrderMgnModeIsolated = "isolated"
	// 做多
	OrderPosSideLong = "long"
	// 做空
	OrderPosSideShort = "short"
)

// 用于记录订单的接口
type OrderRecorder interface {
	RrcordOrder(result *Order) error
}
