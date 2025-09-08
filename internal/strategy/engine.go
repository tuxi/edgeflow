package strategy

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/position"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"strconv"
	"sync"

	"log"
	"time"
)

type StrategyEngine struct {
	trendMgr  *trend.Manager
	signalGen *trend.SignalGenerator
	ps        *position.PositionService
	Signals   map[string]*trend.Signal // 交易完成的信号
	mu        sync.Mutex
	ticker    *time.Ticker
	isTesting bool
}

// 你可以把这些参数抽成配置
const (
	checkInterval   = 2 * time.Minute  // 定时检查间隔1分钟
	maxHoldDuration = 35 * time.Minute // 最长持仓时间
	takeProfit      = 0.1              // 达到 +10% 强制止盈
	stopLoss        = -0.09            // 达到 -5% 强制止损
)

func NewStrategyEngine(trendMgr *trend.Manager, signalGen *trend.SignalGenerator, ps *position.PositionService, isTesting bool) *StrategyEngine {
	se := &StrategyEngine{
		trendMgr:  trendMgr,
		signalGen: signalGen,
		ps:        ps,
		Signals:   make(map[string]*trend.Signal),
		isTesting: isTesting,
	}
	if se.isTesting {
		return se
	}
	// 检查盈亏状态
	se.startPnLWatcher()
	return se
}

func (se *StrategyEngine) Run(interval time.Duration, symbols []string) {

	ticker := time.NewTicker(interval)
	quit := make(chan struct{})
	for _, symbol := range symbols {
		se.Signals[symbol] = nil
	}

	go se._run()

	go func() {
		for {
			select {
			case <-ticker.C:
				se._run()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

}

func (se *StrategyEngine) _run() {
	for k, _ := range se.Signals {
		se.runForSymbol(k)
		time.Sleep(time.Second * 2)
	}
}

func (se *StrategyEngine) runForSymbol(symbol string) {
	// 0.获取交易锁仓位
	long, short, err := se.ps.Exchange.GetPosition(symbol, model.OrderTradeSwap)
	if err != nil {
		fmt.Printf("[StrategyEngine.run GetPosition error: %v]\n", err.Error())
		return
	}
	pos := long
	if pos == nil {
		pos = short
	}

	// 1. 获取大趋势
	state := se.trendMgr.GetState(symbol)
	if state == nil {
		log.Printf("[StrategyEngine] 获取%v大趋势失败\n", symbol)
		return
	}

	lines15m, err := se.ps.Exchange.GetKlineRecords(symbol, model2.Kline_15min, 210, 0, model.OrderTradeSwap, false)

	if err != nil {
		return
	}
	// 2. 获取15分钟周期信号
	sig15, err := se.signalGen.Generate(lines15m, symbol)
	if err != nil {
		return
	}

	// 3. 组装上下文
	ctx := signal.Context{
		Trend:   *state,
		Sig:     *sig15,
		Pos:     pos,
		LastSig: se.Signals[symbol],
	}

	// 4. 决策
	action := signal.NewDecisionEngine(ctx).Run()
	var leverage int64 = 30
	if action == signal.ActAddSmall || action == signal.ActOpenSmall {
		leverage = 20
	}
	if ctx.Pos != nil && ctx.Pos.Lever != "" {
		leverage, _ = strconv.ParseInt(ctx.Pos.Lever, 10, 64)
	} else {
		if symbol != "BTC/USDT" && symbol != "ETH/USDT" && symbol != "SOL/USDT" {
			leverage = 20
		}
	}
	sig := signal.Signal{
		Strategy:  "auto-Strategy-Engine",
		Symbol:    ctx.Sig.Symbol,
		Price:     ctx.Sig.Price,
		Side:      ctx.Sig.Side,
		OrderType: "market",
		TradeType: "swap",
		Comment:   "",
		Leverage:  int(leverage),
		Level:     2,
		Meta:      nil,
		Timestamp: time.Now(),
	}
	se.Signals[symbol] = sig15
	if se.isTesting {
		return
	}
	// 5. 执行交易
	err = se.ps.ApplyAction(context.Background(), action, sig, pos)
	if err != nil {
		fmt.Printf("[StrategyEngine.run error: %v]\n", err.Error())
	}

}

// 定时检查盈利情况，防止系统的止盈止损太高未被触发
func (tv *StrategyEngine) startPnLWatcher() {
	if tv.ticker != nil {
		return
	}
	ticker := time.NewTicker(checkInterval)

	go func() {
		for range ticker.C {
			tv.checkPnL()
		}
	}()
}

func (tv *StrategyEngine) checkPnL() {

	tv.mu.Lock()
	defer tv.mu.Unlock()
	for symbol, _ := range tv.Signals {
		long, short, err := tv.ps.Exchange.GetPosition(symbol, model.OrderTradeSwap)
		if err != nil {
			continue
		}

		var positions []*model.PositionInfo
		if long != nil {
			positions = append(positions, long)
		}
		if short != nil {
			positions = append(positions, short)
		}

		for _, pos := range positions {
			openTimeMs, _ := strconv.ParseInt(pos.CTime, 10, 64)
			openTime := time.UnixMilli(openTimeMs)
			holdDuration := time.Now().Sub(openTime)

			// 转 float
			uplRatio, _ := strconv.ParseFloat(pos.UplRatio, 64)

			// 持仓时间超过最大时间
			if holdDuration > maxHoldDuration {
				// 仓位开超过半小时，检查盈亏比
				if uplRatio >= takeProfit {
					log.Printf("[%s] 仓位超过35分钟, 盈利%.3f%% 强制止盈\n", pos.Symbol, uplRatio*100)
					go tv.ps.Close(context.Background(), pos, model.OrderTradeSwap) // 异步平仓，避免阻塞
				} else if uplRatio <= stopLoss {
					log.Printf("[%s] 仓位超过50分钟, 亏损%.5f%% 强制止损\n", pos.Symbol, uplRatio*100)
					go tv.ps.Close(context.Background(), pos, model.OrderTradeSwap) // 异步平仓，避免阻塞
				} else {
					log.Printf("[%s] 仓位超过50分钟, 但盈亏比 %.2f%% 未达条件, 暂不处理\n", pos.Symbol, uplRatio*100)
				}
			}
		}

		time.Sleep(time.Second * 5)

	}

}
