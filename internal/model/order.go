package model

import (
	"math"
	"time"
)

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
	Symbol      string // BTC-USDT
	Side        OrderSide
	Price       float64
	Quantity    float64
	OrderType   OrderType
	TPPrice     float64
	SLPrice     float64
	Strategy    string
	Comment     string
	TradeType   OrderTradeType // 交易模式
	MgnMode     OrderMgnMode   // 保证金模式（cross/isolate）
	Leverage    int            // 杠杆倍数
	QuantityPct float64        // 下单金额相对可用金额的百分比
	Level       int
	Score       int
	Timestamp   time.Time // 信号触发时间
}

// 交易类型
type OrderTradeType string

// 保证金模式（cross / isolated）
type OrderMgnMode string

// posSide 持仓方向 做多long或者做空short
type OrderPosSide string

const (
	// 使用现货 API
	OrderTradeSpot OrderTradeType = "spot"
	// 使用合约 API
	OrderTradeSwap OrderTradeType = "swap"
	// 使用交割合约 API
	OrderTradeFutures OrderTradeType = "futures"
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

type OrderRecord struct {
	ID        uint      `gorm:"column:id;primary_key;" json:"id"` // 主键id，自增长，不用设置
	OrderId   string    `gorm:"column:order_id;" json:"order_id"` // 订单id
	Symbol    string    `gorm:"column:symbol" json:"symbol"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"` // 订单创建时间2025-08-07T21:54:30+08:00

	Side      OrderSide      `gorm:"column:side" json:"side"`
	Price     float64        `gorm:"column:price" json:"price"`
	Quantity  float64        `gorm:"column:quantity" json:"quantity"`
	OrderType OrderType      `gorm:"column:order_type" json:"order_type"`
	TP        float64        `gorm:"column:tp" json:"tp"`
	SL        float64        `gorm:"column:sl" json:"sl"`
	Strategy  string         `gorm:"column:strategy" json:"strategy"`
	Comment   string         `gorm:"column:comment" json:"comment"`
	TradeType OrderTradeType `gorm:"column:trade_type" json:"trade_type"`
	MgnMode   OrderMgnMode   `gorm:"column:mgn_mode" json:"mgn_mode"`
	Level     int            `gorm:"column:level" json:"level"`
	Score     int            `gorm:"column:score" json:"score"`
	Timestamp time.Time      `gorm:"column:timestamp" json:"timestamp"` // 信号触发时间

}

func (OrderRecord) TableName() string {
	return "order_record"
}

type PositionInfo struct {
	Symbol        string       // 币
	Dir           OrderPosSide // 方向
	Amount        float64      // 持有张数
	AvgPrice      float64      // 开仓均价
	MgnMode       string       // 保证金模式
	LiqPx         string       // 强平价
	AlgoId        string
	PositionId    string // 仓位id
	UnrealizedPnl string // 未实现的盈亏
	UplRatio      string // 未实现的收益率
	MarkPx        string // 当前价格
	Margin        string
	Lever         string  // 杠杆倍数
	NotionalUsd   string  // 仓位名义价值
	Last          float64 // 最后成交价
	CTime         string  // 开仓时间戳
}

// UnrealizedPnl 计算未实现盈亏
//func (ps *PositionInfo) UnrealizedPnl(lastPrice float64) float64 {
//	pnl := 0.0
//	if ps.Side == OrderPosSideLong && ps.Amount > 0 {
//		pnl += lastPrice - ps.AvgPrice*ps.Amount
//	}
//	if ps.Side == OrderPosSideShort && ps.Amount > 0 {
//		pnl += (ps.AvgPrice - lastPrice) * ps.Amount
//	}
//	return pnl
//}

type Kline struct {
	Timestamp time.Time `json:"time"`
	Open      float64   `json:"open"`
	Close     float64   `json:"close"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Vol       float64   `json:"vol"`     // 成交量 以币为单位
	VolCcy    float64   `json:"vol_ccy"` // 成交额 以USDT为单位
}

// 指标结果
type IndicatorResult struct {
	Name     string
	Values   map[string]float64
	Signal   string  // "buy", "sell", "hold"
	Strength float64 // 指标强度0～1
}

// GetKLinePowerCorrection 根据 K 线形态计算 VOL 强度的修正因子
// 目标：长影线削弱实体，长实体增强 VOL 确认力度。
func GetKLinePowerCorrection(high, low, open, close float64) float64 {
	// 整个 K 线的总振幅
	klineRange := high - low

	// 如果价格没有波动，直接返回中性
	if klineRange == 0 {
		return 0.0
	}

	// 实体长度
	bodySize := math.Abs(close - open)

	// ----------------------------------------------------
	// 计算 K 线实体占总幅度的比例 (Body/Range Ratio)
	// 比例越大 (接近 1)，K 线越干净，趋势越强劲，VOL 确认度越高
	// 比例越小 (接近 0)，K 线影线越长，多空争夺越激烈，VOL 确认度越低
	// ----------------------------------------------------

	bodyRatio := bodySize / klineRange

	// 修正因子的定义：
	// - 实体比例 > 0.6 时，给予 VOL 适度增强 (例如 1.2 倍)
	// - 实体比例 < 0.3 时，对 VOL 确认力度进行削弱 (例如 0.5 倍)

	if bodyRatio >= 0.6 {
		return 1.2 // K 线趋势强劲，VOL 确认力度增强 20%
	} else if bodyRatio <= 0.3 {
		// 长影线 (例如锤子线/十字星)：VOL 确认力度削弱 50%
		return 0.5
	} else {
		return 1.0 // 中性 K 线，VOL 确认力度不变
	}
}
