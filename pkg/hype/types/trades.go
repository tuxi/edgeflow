package types

const (
	OpCloseShort              = "Close Short"
	OpCloseLong               = "Close Long"
	OpLongToShort             = "Long > Short"      // 包含平多操作
	OpShortToLong             = "Short > Long"      // 包含平空操作
	OpAutoDeleveraging        = "Auto-Deleveraging" // 在特大风险时，比如10.11插针，很多交易员仓位被系统自动去杠杆
	OpLiquidatedCrossLong     = "Liquidated Cross Long"
	OpLiquidatedIsolatedLong  = "Liquidated Isolated Long"
	OpLiquidatedCrossShort    = "Liquidated Cross Short"
	OpLiquidatedIsolatedShort = "Liquidated Isolated Short"
	// Open Short/Long 和 Spot Buy/Sell 不计入胜率统计，它们是建仓操作
)

// 用户已成交的订单
type UserFillOrder struct {
	ClosedPnl     string `json:"closedPnl"`
	Coin          string `json:"coin"`
	Crossed       bool   `json:"crossed"`
	Dir           string `json:"dir"`
	Hash          string `json:"hash"`
	Oid           int    `json:"oid"`
	Px            string `json:"px"`
	Side          string `json:"side"`
	StartPosition string `json:"startPosition"`
	Sz            string `json:"sz"`
	Time          int64  `json:"time"`
	Fee           string `json:"fee"`
	FeeToken      string `json:"feeToken"`
	BuilderFee    string `json:"builderFee"`
	Tid           int64  `json:"tid"`
}

// 查询一次订单数据，包含开始查询和结束时间，方便下次追溯上次的查询时间
type UserFillOrderData struct {
	Data            []*UserFillOrder `json:"data"`              // 本次查询实际拉取到的订单记录。
	Start           int64            `json:"start"`             // 本次查询的起始时间点（最接近历史尽头）。
	End             int64            `json:"end"`               // 本次查询的结束时间点（最接近 now）。
	HasMore         bool             `json:"has_more"`          // 是否可以继续追溯历史
	NextWindowHours int              `json:"next_window_hours"` // 下一次查询订单使用的窗口大小，以小时为单位
}

// isClosed: 判断该订单填充记录是否代表一次平仓操作。
func (o *UserFillOrder) IsClosed() bool {
	switch o.Dir {
	case OpCloseShort, OpCloseLong, OpLongToShort, OpShortToLong, OpAutoDeleveraging:
		return true
	case OpLiquidatedCrossLong, OpLiquidatedIsolatedLong, OpLiquidatedCrossShort, OpLiquidatedIsolatedShort:
		return true
	default:
		// Open Short/Long, Spot Buy/Sell 等属于开仓操作，不计入胜率统计
		return false
	}
}

// 用户的委托订单，未成交的订单
type UserOpenOrder struct {
	Coin             string `json:"coin"`
	IsPositionTpsl   bool   `json:"isPositionTpsl"`
	IsTrigger        bool   `json:"isTrigger"`
	LimitPx          string `json:"limitPx"`
	Oid              int    `json:"oid"`
	OrderType        string `json:"orderType"`
	OrigSz           string `json:"origSz"`
	ReduceOnly       bool   `json:"reduceOnly"`
	Side             string `json:"side"`
	Sz               string `json:"sz"`
	Timestamp        int64  `json:"timestamp"`
	TriggerCondition string `json:"triggerCondition"`
	TriggerPx        string `json:"triggerPx"`
}

// 资金信息
type FundingDelta struct {
	Coin        string `json:"coin"`
	FundingRate string `json:"fundingRate"`
	Szi         string `json:"szi"`
	Type        string `json:"type"` // 资金类型
	USDC        string `json:"usdc"`
	Nonce       int64  `json:"nonce"`
	Fee         string `json:"fee"`
}

// 用户资金
type UserNonFunding struct {
	Delta FundingDelta `json:"delta"` // 资金信息
	Hash  string       `json:"hash"`
	Time  int64        `json:"time"`
}
