package okx

import (
	"context"
	"edgeflow/internal/account"
	model2 "edgeflow/internal/model"
	"errors"
	"fmt"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex/v2/okx/spot"
	"github.com/nntaoli-project/goex/v2/options"
	"strconv"
	"strings"
)

// 现货
type OkxSpot struct {
	Okx
	pub spot.Spot
}

func NewOkxSpot(conf []options.ApiOption) *OkxSpot {
	pub := goexv2.OKx.Spot
	return &OkxSpot{
		Okx: Okx{
			prv:     pub.NewPrvApi(conf...),
			Account: account.NewAccountService(pub.NewPrvApi(conf...)),
			pub:     pub,
		},
		pub: *pub,
	}
}

// 下单购买
// 注意限价和市价的Quantity单位不相同，当限价时Quantity的单位为币本身，当市价时Quantity的单位为USDT
func (e *OkxSpot) PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error) {
	pair, err := e.toCurrencyPair(order.Symbol)
	if err != nil {
		return nil, err
	}
	var side model.OrderSide
	switch strings.ToLower(string(order.Side)) {
	case "buy":
		side = model.Spot_Buy
	case "sell":
		side = model.Spot_Sell
	default:
		return nil, errors.New("invalid order side")
	}

	var orderType model.OrderType
	switch order.OrderType {
	case model2.Limit:
		orderType = model.OrderType_Limit
	case model2.Market:
		orderType = model.OrderType_Market
	}

	// 如果有止盈和止损
	var opts []model.OptionParameter

	// 如果设置了止盈
	if order.TPPrice > 0 {
		opts = append(opts, model.OptionParameter{
			Key:   "tpTriggerPx",
			Value: strconv.FormatFloat(order.TPPrice, 'f', -1, 64), // 止盈触发价
		})
		opts = append(opts, model.OptionParameter{
			Key:   "tpOrdPx",
			Value: "-1", // -1 表示市价止盈
		})
	}

	// 如果设置了止损
	if order.SLPrice > 0 {
		opts = append(opts, model.OptionParameter{
			Key:   "slTriggerPx",
			Value: strconv.FormatFloat(order.SLPrice, 'f', -1, 64), // 止损触发价
		})
		opts = append(opts, model.OptionParameter{
			Key:   "slOrdPx",
			Value: "-1", // -1 表示市价止损
		})
	}

	// 创建订单
	createdOrder, resp, err := e.prv.CreateOrder(pair, order.Quantity, order.Price, side, orderType, opts...)
	if err != nil {
		fmt.Printf("CreateOrder error：%v", resp)
		return nil, err
	}

	return &model2.OrderResponse{
		OrderId: createdOrder.Id,
		Status:  int(createdOrder.Status),
	}, nil
}

func (e *OkxSpot) getPub() goexv2.IPubRest {
	return &e.pub
}
