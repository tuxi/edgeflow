package okx

import (
	"context"
	"edgeflow/internal/account"
	model2 "edgeflow/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex/v2/okx/futures"
	"github.com/nntaoli-project/goex/v2/options"
	"strconv"
	"strings"
)

// 永续合约
type OkxSwap struct {
	FuturesCommon
	pub futures.Swap
}

func NewOkxSwap(conf []options.ApiOption) *OkxSwap {
	pub := goexv2.OKx.Swap
	return &OkxSwap{
		FuturesCommon: FuturesCommon{Okx{
			prv:     pub.NewPrvApi(conf...),
			Account: account.NewAccountService(pub.NewPrvApi(conf...)),
			pub:     pub,
		}},
		pub: *pub,
	}
}

func (e *OkxSwap) getPub() goexv2.IPubRest {
	return &e.pub
}

// 下单购买
// 注意限价和市价的Quantity单位不相同，当限价时Quantity的单位为币本身，当市价时Quantity的单位为USDT
func (e *OkxSwap) PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error) {

	pair, err := e.toCurrencyPair(order.Symbol)
	if err != nil {
		return nil, err
	}
	var posSide model2.OrderPosSide
	var side model.OrderSide
	switch strings.ToLower(string(order.Side)) {
	case "buy":
		side = model.Futures_OpenBuy
		posSide = model2.OrderPosSideLong
	case "sell":
		side = model.Futures_OpenSell
		posSide = model2.OrderPosSideShort
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

	// okx v5 api要求带有止盈止损的开单必须放在attachAlgoOrds数组map中
	var attachAlgoOrds = make(map[string]string)

	// 如果设置了止盈
	if order.TPPrice > 0 {
		attachAlgoOrds["tpTriggerPx"] = strconv.FormatFloat(order.TPPrice, 'f', -1, 64) // 止盈触发价
		attachAlgoOrds["tpOrdPx"] = "-1"                                                // -1 表示市价止盈
	}

	// 如果设置了止损
	if order.SLPrice > 0 {
		attachAlgoOrds["slTriggerPx"] = strconv.FormatFloat(order.SLPrice, 'f', -1, 64) // 止损触发价
		attachAlgoOrds["slOrdPx"] = "-1"                                                // 表示市价止损
	}

	tpSlJSON, err := json.Marshal([]map[string]string{attachAlgoOrds})
	if err == nil {
		opts = append(opts, model.OptionParameter{
			Key:   "attachAlgoOrds",
			Value: string(tpSlJSON),
		})
	}
	/*
		合约交易需要设置tdMode
		| 值          | 含义   |
		| ---------- | ---- |
		| `cross`    | 全仓模式 |
		| `isolated` | 逐仓模式 |
	*/
	mgnMode := order.MgnMode
	leverage := order.Leverage
	if mgnMode == "" {
		mgnMode = model2.OrderMgnModeIsolated
	}
	//这里统一使用逐仓模式
	opts = append(opts, model.OptionParameter{
		Key:   "tdMode",
		Value: string(mgnMode),
	})

	opts = append(opts, model.OptionParameter{
		Key:   "posSide",
		Value: string(posSide),
	})

	if leverage <= 0 {
		leverage = 20
	}
	order.Leverage = leverage
	// 设置杠杆倍数
	err = e.SetLeverage(order.Symbol, leverage, string(mgnMode), string(posSide))
	if err != nil {
		return nil, err
	}

	// 根据比例计算下单金额
	if order.QuantityPct > 0 {

		// 获取可用余额，根据比例计算下单数量
		acc, err := e.Account.GetAccount(ctx, "USDT")
		if err != nil {
			return nil, err
		}

		// 0.98 是最大仓位的容差，防止价差导致余额不足
		_, qty := CalculateContractOrder(acc.Available*order.QuantityPct*0.98, leverage, order.Price, pair.ContractVal)
		order.Quantity = qty
	}
	order.MgnMode = mgnMode

	// 创建订单
	fmt.Printf("CreateOrder start: quantity:%v price:%v side:%v", order.Quantity, order.Price, order.Side)
	createdOrder, _, err := e.prv.CreateOrder(pair, order.Quantity, order.Price, side, orderType, opts...)
	if err != nil {
		fmt.Printf("CreateOrder error：%v", err.Error())
		return nil, err
	}

	return &model2.OrderResponse{
		OrderId: createdOrder.Id,
		Status:  int(createdOrder.Status),
	}, nil
}
