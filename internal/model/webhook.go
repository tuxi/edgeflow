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
	Strategy  string  `json:"strategy"`   // 策略标识符
	Symbol    string  `json:"symbol"`     // BTC/USDT
	Price     float64 `json:"price"`      // 当前价格
	Side      string  `json:"side"`       // buy / sell
	Quantity  float64 `json:"quantity"`   // 数量
	TpPercent float64 `json:"tp_pct"`     // 止盈比例
	SlPercent float64 `json:"sl_pct"`     // 止损比例
	OrderType string  `json:"order_type"` // 订单类型：market、limit
	TradeType string  `json:"trade_type"` // 交易类型
	Comment   string  `json:"comment"`
}
