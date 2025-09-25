package types

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
