package strategy

import (
	"context"
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	"edgeflow/internal/risk"
	"errors"
	"fmt"
	"log"
)

type TVBreakoutV2 struct {
	Exchange exchange.Exchange
	//recorder *recorder.JSONFileRecorder
	Rc *risk.RiskControl
}

func NewTVBreakoutV2(ex exchange.Exchange, rc *risk.RiskControl) *TVBreakoutV2 {
	return &TVBreakoutV2{Exchange: ex, Rc: rc}
}

func (t TVBreakoutV2) Name() string {
	return "tv-breakout-v2"
}

func (t TVBreakoutV2) Execute(ctx context.Context, req model.WebhookRequest) error {
	params, err := ConvertToExecutionParams(req)
	if err != nil {
		return err
	}

	price := params.Price
	//if price == 0 {
	//	// 可考虑调用市场价格作为 fallback
	//	price = s.Exchange.GetLastPrice(req.Symbol)
	//}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 0.01 // 默认值
	}

	tpPrice := 0.0
	slPrice := 0.0
	if params.Tp > 0 {
		tpPrice = computeTP(req.Side, price, req.TpPercent)
	}
	if params.Sl > 0 {
		slPrice = computeSL(req.Side, price, req.SlPercent)
	}

	order := model.Order{
		Symbol:    req.Symbol,
		Side:      model.OrderSide(req.Side),
		Price:     price,
		Quantity:  quantity,
		OrderType: model.OrderType(req.OrderType), // "market" / "limit"
		Strategy:  req.Strategy,
		TPPrice:   tpPrice,
		SLPrice:   slPrice,
		Comment:   req.Comment,
	}

	// 风控检查，是否允许下单
	isAllow := t.Rc.Allow(ctx, order)
	if isAllow == false {
		return errors.New("触发风控，无法下单，稍后再试")
	}

	log.Printf("[TVBreakoutV2] placing order: %+v", order)
	// 调用交易所api下单
	resp, err := t.Exchange.PlaceOrder(ctx, order)
	fmt.Println(resp.Message)

	// 下单成功，保存订单
	if err != nil {
		_ = t.Rc.OrderCreateNew(ctx, order, resp.OrderId)
	}

	return err
}
