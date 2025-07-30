package strategy

import (
	"context"
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	"edgeflow/pkg/recorder"
	"fmt"
	"log"
	"time"
)

type TVBreakoutV2 struct {
	Exchange exchange.Exchange
	recorder *recorder.JSONFileRecorder
}

func NewTVBreakoutV2(ex exchange.Exchange, recorder *recorder.JSONFileRecorder) *TVBreakoutV2 {
	return &TVBreakoutV2{Exchange: ex, recorder: recorder}
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
		tpPrice = computeTP(req.Side, price, req.TpPct)
	}
	if params.Sl > 0 {
		slPrice = computeSL(req.Side, price, req.SlPct)
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

	log.Printf("[TVBreakoutV2] placing order: %+v", order)
	// 调用交易所api下单
	resp, err := t.Exchange.PlaceOrder(ctx, order)
	fmt.Println(resp.Message)

	// 记录执行日志
	if t.recorder != nil {
		t.recorder.Record(model.ExecutionLog{
			Timestamp: time.Now(),
			Strategy:  "TVBreakoutV1",
			Symbol:    params.Symbol,
			Side:      string(params.Side),
			Price:     params.Price,
			Note:      params.Comment,
		})
	}

	return err
}
