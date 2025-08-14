package model

import "time"

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
type Signal struct {
	Strategy string  `json:"strategy"` // 指标名称标识符
	Symbol   string  `json:"symbol"`   // BTC/USDT
	Price    float64 `json:"price"`    // 当前价格
	Side     string  `json:"side"`     // buy / sell
	//Quantity  float64        `json:"quantity"`   // 数量
	//TpPercent float64 `json:"tp_pct"`     // 止盈比例
	//SlPercent float64 `json:"sl_pct"`     // 止损比例
	OrderType string         `json:"order_type"` // 订单类型：market、limit
	TradeType string         `json:"trade_type"` // 交易类型
	Comment   string         `json:"comment"`
	Leverage  int            `json:"leverage"`  // 下单的杠杆倍数
	Level     int            `json:"level"`     // 信号等级：1、2、3
	Score     int            `json:"score"`     // 信号强度，L3时有用
	Meta      map[string]any `json:"meta"`      // 附加字段：价格、图表ID等
	Timestamp time.Time      `json:"timestamp"` // 触发时间
}

// 当前信号是否过期
func (sig Signal) IsExpired() bool {
	isExpired := time.Since(sig.Timestamp) > signalExpiry[sig.Level]
	return isExpired
}

// 信号有效期
var signalExpiry = map[int]time.Duration{
	1: 6 * time.Hour,    // 1级信号有效期6小时
	2: 2 * time.Hour,    // 2级信号有效期2小时
	3: 45 * time.Minute, // 3级信号有效期30分钟
}
