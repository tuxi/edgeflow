package strategy

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/position"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"

	"log"
	"time"
)

type StrategyEngine struct {
	trendMgr  *trend.Manager
	signalGen *trend.SignalGenerator
	ps        *position.PositionService
	symbols   []string
}

func NewStrategyEngine(trendMgr *trend.Manager, signalGen *trend.SignalGenerator, ps *position.PositionService) *StrategyEngine {
	return &StrategyEngine{
		trendMgr:  trendMgr,
		signalGen: signalGen,
		ps:        ps,
	}
}

func (se *StrategyEngine) Run(interval time.Duration, symbols []string) {

	ticker := time.NewTicker(interval)
	quit := make(chan struct{})
	se.symbols = symbols

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
	for _, symbol := range se.symbols {
		se.runForSymbol(symbol)
		time.Sleep(time.Second * 2)
	}
}

func (se *StrategyEngine) runForSymbol(symbol string) {
	// 0.获取交易锁仓位
	long, short, _ := se.ps.Exchange.GetPosition(symbol, model.OrderTradeSwap)

	pos := long
	if short != nil {
		pos = short
	}

	// 1. 获取大趋势
	state, ok := se.trendMgr.Get(symbol)
	if ok == false {
		log.Println("[StrategyEngine] 获取大趋势失败")
		return
	}

	lines15m, err := se.ps.Exchange.GetKlineRecords(symbol, model2.Kline_15min, 210, 0, model.OrderTradeSpot, false)

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
		Trend: *state,
		Sig:   *sig15,
		Pos:   pos,
	}

	// 4. 决策
	action := signal.Decide(ctx)

	sig := signal.Signal{
		Strategy:  "auto-Strategy-Engine",
		Symbol:    sig15.Symbol,
		Price:     sig15.Price,
		Side:      sig15.Side,
		OrderType: "market",
		TradeType: "swap",
		Comment:   "",
		Leverage:  20,
		Level:     2,
		Meta:      nil,
		Timestamp: time.Now(),
	}

	// 5. 执行交易
	err = se.ps.ApplyAction(context.Background(), action, sig, pos)
	if err != nil {
		fmt.Printf("[StrategyEngine.run error: %v]", err.Error())
	}
}
