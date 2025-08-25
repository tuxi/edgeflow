package strategy

import (
	"context"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"errors"
	"fmt"
	"log"
)

// 15分钟短线反转单
type TVScalp15M struct {
	signalManager signal.Manager
	positionSvc   *service.PositionService
}

func NewTVScalp15M(sm signal.Manager, ps *service.PositionService) *TVScalp15M {
	return &TVScalp15M{signalManager: sm, positionSvc: ps}
}

func (t TVScalp15M) Name() string {
	return "tv-scalp-15m"
}

func (t TVScalp15M) Execute(ctx context.Context, req signal.Signal) error {

	// 保存信号
	t.signalManager.Save(req)

	// 判断是否执行以及是否需要先平仓
	execute, closeFirst := t.signalManager.ShouldExecute(req)
	if !execute {
		err := fmt.Sprintf("Level %d 信号不执行: %+v", req.Level, req)
		log.Println(err)
		return errors.New(err)
	}

	if closeFirst {
		// 平掉逆向仓位
		err := t.positionSvc.CloseAll(ctx, req)
		if err != nil {
			return err
		}
	}

	// 这是15分钟短线单，止盈止损低一些
	tpPercent := 0.78
	slPercent := 0.5
	if req.TpPct > 0 {
		tpPercent = req.TpPct
	}
	if req.SlPct > 0 {
		slPercent = req.SlPct
	}
	// 执行开仓/加仓
	return t.positionSvc.Open(ctx, req, tpPercent, slPercent)

}
