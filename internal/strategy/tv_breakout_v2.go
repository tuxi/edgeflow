package strategy

import (
	"context"
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	"edgeflow/internal/risk"
	"errors"
	"fmt"
	"log"
	"strings"
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
	var side model.OrderSide
	switch strings.ToLower(req.Side) {
	case "buy":
		side = model.Buy
	case "sell":
		side = model.Sell
	default:
		return fmt.Errorf("invalid side: %s", req.Side)
	}

	price := req.Price
	//if price == 0 {
	//	// 可考虑调用市场价格作为 fallback
	//	price, err := t.Exchange.GetLastPrice(req.Symbol)
	//}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 0.01 // 默认值
	}

	// 计算止盈止损
	tpPrice := 0.0
	slPrice := 0.0
	if req.TpPercent > 0 {
		tpPrice = computeTP(req.Side, price, req.TpPercent)
	}
	if req.SlPercent > 0 {
		slPrice = computeSL(req.Side, price, req.SlPercent)
	}

	order := model.Order{
		Symbol:    req.Symbol,
		Side:      side,
		Price:     price,
		Quantity:  quantity,
		OrderType: model.OrderType(req.OrderType), // "market" / "limit"
		Strategy:  req.Strategy,
		TPPrice:   tpPrice,
		SLPrice:   slPrice,
		TradeType: model.OrderTradeTypeType(req.TradeType),
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
	if err == nil {
		fmt.Println(resp.Message)
	}

	// 下单成功，保存订单
	if err == nil {
		err = t.Rc.OrderCreateNew(ctx, order, resp.OrderId)
		if err != nil {
			log.Fatalf("创建订单失败:%v", err)
		}
	}

	return err
}
