package strategy

import (
	"context"
	"edgeflow/internal/position"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"errors"
	"log"
	"time"
)

// 趋势/波段策略：基于 1H 周期，减少频繁进出，拉大止盈止损，追求稳定的中等收益率
type TVLevelStrategy struct {
	signalManager signal.Manager
	positionSvc   *position.PositionService
	trend         *trend.Manager
}

func NewTVLevelStrategy(sm signal.Manager,
	ps *position.PositionService, trend *trend.Manager) *TVLevelStrategy {
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

	state, _, err := t.positionSvc.State(sig)
	if err != nil {
		return err
	}
	metaL2 := t.positionSvc.GetPositionByLevel(sig.Symbol, 2)
	if metaL2 != nil {
		lastSig := t.signalManager.GetLastSignal(sig.Symbol, 2)
		if lastSig == nil {
			// 当服务端有l2仓位，本地没有信号缓存时，应该是服务重启了，数据丢失了，此时我们补全即可
			t.signalManager.Save(signal.Signal{
				Strategy:  sig.Strategy,
				Symbol:    sig.Symbol,
				Price:     sig.Price,
				Side:      metaL2.Side,
				OrderType: sig.OrderType,
				TradeType: sig.TradeType,
				Comment:   "后补信号",
				Leverage:  20,
				Level:     2,
				Timestamp: time.Now(),
			})
		}
	}
	upnl := 0.0
	if state != nil {
		upnl = state.UnrealizedPnl(sig.Price)
	}
	entryPrice := 0.0
	if metaL2 != nil {
		entryPrice = metaL2.EntryPrice
	}

	// 获取当前币的趋势
	st, ok := t.trend.Get(sig.Symbol)

	dCtx := signal.DecisionContext{
		HasL2Position: metaL2 != nil,
		L2Entry:       entryPrice,
		UnrealizedR:   upnl, // 从交易所仓位算
		TrendDir:      st.Direction,
		StrongM15:     ok && st.StrongM15 == true,
	}

	desc := t.signalManager.Decide(sig, dCtx)

	err = t.positionSvc.ApplyAction(ctx, desc.Action, sig, state)
	if err != nil {
		log.Printf("Execute error: %v", err)
	}
	if desc.Action == signal.ActIgnore {
		return errors.New(desc.Reason)
	}
	return err
}
