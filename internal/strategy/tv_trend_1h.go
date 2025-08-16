package strategy

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"errors"
	"fmt"
	"log"
)

// 趋势/波段策略：基于 1H 周期，减少频繁进出，拉大止盈止损，追求稳定的中等收益率
type TVTrendH struct {
	signalManager service.SignalManager
	positionSvc   *service.PositionService
}

func NewTVTrendH(sm service.SignalManager, ps *service.PositionService) *TVTrendH {
	return &TVTrendH{
		signalManager: sm,
		positionSvc:   ps,
	}
}

func (t TVTrendH) Name() string {
	return "tv-trend-1h"
}

func (t TVTrendH) Execute(ctx context.Context, req model.Signal) error {

	// 判断是否执行以及是否需要先平仓
	execute, closeFirst := t.signalManager.ShouldExecute(req)
	if !execute {
		err := fmt.Sprintf("Level %d 信号不执行: %+v", req.Level, req)
		log.Println(err)
		return errors.New(err)
	}

	if closeFirst {
		// 平掉逆向仓位
		err := t.positionSvc.Close(ctx, req)
		if err != nil {
			return err
		}
	}

	// 这是一小时趋势单，止盈止损提高一些
	tpPercent := 2.0
	slPercent := 1.0
	if req.TpPct > 0 {
		tpPercent = req.TpPct
	}
	if req.SlPct > 0 {
		slPercent = req.SlPct
	}
	// 执行开仓/加仓
	return t.positionSvc.Open(ctx, req, tpPercent, slPercent)

}
