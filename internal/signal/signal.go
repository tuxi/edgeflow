package signal

import (
	"edgeflow/internal/config"
	"log"
	"time"
)

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
	Strategy  string         `json:"strategy"`   // 指标名称标识符
	Symbol    string         `json:"symbol"`     // BTC/USDT
	Price     float64        `json:"price"`      // 当前价格
	Side      string         `json:"side"`       // buy / sell
	OrderType string         `json:"order_type"` // 订单类型：market、limit
	TradeType string         `json:"trade_type"` // 交易类型
	Comment   string         `json:"comment"`
	Leverage  int            `json:"leverage"`  // 下单的杠杆倍数
	Level     int            `json:"level"`     // 信号等级：1、2、3
	Meta      map[string]any `json:"meta"`      // 附加字段：价格、图表ID等
	Timestamp time.Time      `json:"timestamp"` // 触发时间
	TpPct     float64        `json:"tp"`        // 止盈比例，默认为0时使用系统的
	SlPct     float64        `json:"sl"`        // 止损比例，默认为0时使用系统的
}

// 当前信号是否过期
func (sig Signal) IsExpired() bool {
	isExpired := time.Since(sig.Timestamp) > signalExpiry[sig.Level]
	return isExpired
}

// 信号有效期
var signalExpiry = map[int]time.Duration{
	1: 8 * time.Hour,    // 1级信号有效期6小时
	2: 3 * time.Hour,    // 2级信号有效期2小时
	3: 50 * time.Minute, // 3级信号有效期50分钟
}

type Action int

const (
	ActIgnore    Action = iota
	ActOpen             // 仅L2可触发
	ActAdd              // 仅L3与L2同向：加仓
	ActReduce           // L3反向且满足获利阀值：减仓/锁盈
	ActTightenSL        // L3反向但达到减仓阀值：仅收紧止损
	ActClose            // 仅L2反向时允许：平掉所有主仓
)

type Decision struct {
	Action        Action
	Reason        string
	ReducePercent float64 // ActReduce 时使用
}

func (d Decision) Log(sig Signal, cfg *config.StrategyConfig) {
	if !cfg.EnableDebugLog {
		return
	}
	log.Printf("[Decision] %s L%d %s → %s (reason=%s, reduce=%.2f%%",
		sig.Symbol,
		sig.Level,
		sig.Side,
		d.Action,
		d.Reason,
		d.ReducePercent,
	)
}

// 信号状态
type State struct {
	LastByLevel map[int]Signal
	// L2 当前方向与最近一次 L2 信号时间（用于冷静期）
	L2Side       string
	L2LastFlipAt time.Time
	// L2 是否持仓由 PositionService 查询，这里只做信号侧状态
}
