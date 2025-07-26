package model

/*
来源于外部数据

	{
	  "symbol": "BTCUSDT",
	  "side": "buy",
	  "price": 29500,
	  "tp": 1.5,
	  "sl": 0.5,
	  "order_type": "market",
	  "quantity": 0.01,
	  "strategy": "tv-breakout-v2",
	  "comment": "突破+回踩买入"
	}
*/
type WebhookRequest struct {
	Strategy  string  `json:"strategy"` // 策略标识符
	Symbol    string  `json:"symbol"`   // BTCUSDT
	Price     float64 `json:"price"`    // 当前价格
	Side      string  `json:"side"`     // buy / sell
	Quantity  float64 `json:"quantity"` // 数量
	Tp        float64 `json:"tp"`       // 止盈比例
	Sl        float64 `json:"sl"`       // 止损比例
	OrderType string  `json:"order_type"`
	Comment   string  `json:"comment"`
}
