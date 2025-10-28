package types

import (
	"time"
)

type CumFunding struct {
	AllTime     string `json:"allTime"`     // 总资金费（可能正或负）
	SinceChange string `json:"sinceChange"` // 自最近价格变动后的资金费
	SinceOpen   string `json:"sinceOpen"`   // 自开仓以来累计资金费
}

type Leverage struct {
	RawUsd string `json:"rawUsd"`
	Type   string `json:"type"`
	Value  int    `json:"value"`
}

type Position struct {
	Coin           string     `json:"coin"`
	CumFunding     CumFunding `json:"cumFunding"`
	EntryPx        string     `json:"entryPx"`
	Leverage       Leverage   `json:"leverage"`
	LiquidationPx  string     `json:"liquidationPx"`
	MarginUsed     string     `json:"marginUsed"`
	MaxLeverage    int        `json:"maxLeverage"`
	PositionValue  string     `json:"positionValue"`
	ReturnOnEquity string     `json:"returnOnEquity"`
	Szi            string     `json:"szi"`
	UnrealizedPnl  string     `json:"unrealizedPnl"`
}

type AssetPosition struct {
	Position Position `json:"position"`
	Type     string   `json:"type"`
}

type CrossMarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalMarginUsed string `json:"totalMarginUsed"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
}

type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalMarginUsed string `json:"totalMarginUsed"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
}

type MarginData struct {
	AssetPositions             []AssetPosition    `json:"assetPositions"` // 仓位
	CrossMaintenanceMarginUsed string             `json:"crossMaintenanceMarginUsed"`
	CrossMarginSummary         CrossMarginSummary `json:"crossMarginSummary"`
	MarginSummary              MarginSummary      `json:"marginSummary"`
	Time                       int64              `json:"time"`
	Withdrawable               string             `json:"withdrawable"`
}

type WhalePortfolio struct {
	Total struct {
		Day, Week, Month, AllTime PeriodData
	}
	Perp struct {
		Day, Week, Month, AllTime PeriodData
	}
}

type PeriodData struct {
	AccountValue []DataPoint // 净值曲线
	Pnl          []DataPoint // 盈亏曲线
	Vlm          float64     // 成交量
}

type DataPoint struct {
	Time  time.Time
	Value float64
}
