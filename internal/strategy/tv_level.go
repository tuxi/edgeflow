package strategy

import (
	"context"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"log"
)

// 趋势/波段策略：基于 1H 周期，减少频繁进出，拉大止盈止损，追求稳定的中等收益率
type TVLevelStrategy struct {
	signalManager signal.Manager
	positionSvc   *service.PositionService
	//trendFilter   signal.TrendFilter
	trend *trend.TrendManager
}

func NewTVLevelStrategy(sm signal.Manager,
	ps *service.PositionService, trend *trend.TrendManager) *TVLevelStrategy {
	return &TVLevelStrategy{
		signalManager: sm,
		positionSvc:   ps,
		trend:         trend,
	}
}

func (t *TVLevelStrategy) Name() string {
	return "tv-level"
}

func (t *TVLevelStrategy) Execute(ctx context.Context, sig signal.Signal) error {

	state, meta, err := t.positionSvc.State(sig)
	if err != nil {
		return err
	}
	trendOk := t.trend.IsTrendOk(sig.Symbol, sig.Side)

	dCtx := signal.DecisionContext{
		HasL2Position: meta != nil && meta.Level == 2,
		L2Entry:       meta.EntryPrice,
		UnrealizedR:   state.UnrealizedPnl(sig.Price), // 从交易所仓位算
		TrendOK:       trendOk,
	}
	desc := t.signalManager.Decide(sig, dCtx)

	err = t.positionSvc.ApplyAction(context.Background(), sig.Symbol, desc.Action, sig, state)
	if err != nil {
		log.Printf("Execute error: %v", err)
	}

	return err
}
