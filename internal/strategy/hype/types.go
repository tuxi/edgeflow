package hype

import "time"

type HypeTradeSignal struct {
	Symbol    string    `json:"symbol"`    // BTC/USDT
	Price     float64   `json:"price"`     // 进场价格价格
	Action    string    `json:"action"`    // buy / sell 这个只代表操作是买还是卖，不代表多空
	Dir       string    `json:"dir"`       // 方向 long/short
	Timestamp time.Time `json:"timestamp"` // 触发时间
}
