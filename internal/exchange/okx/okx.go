package okx

import (
	"context"
	"edgeflow/internal/account"
	model2 "edgeflow/internal/model"
	"errors"
	"fmt"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/model"
	"strings"
)

type OkxService interface {
	PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error)
	GetOrderStatus(orderID string, symbol string) (*model2.OrderStatus, error)
	CancelOrder(orderID, symbol string) error
	GetLastPrice(symbol string) (float64, error)
	GetExchangeInfo() (map[string]model.CurrencyPair, []byte, error)
}

// OKX 三种交易的基础结构：swap、future、spot
type Okx struct {
	prv     goexv2.IPrvRest
	Account *account.Service
	exInfo  map[string]model.CurrencyPair
	pub     goexv2.IPubRest
}

func (e *Okx) getPub() goexv2.IPubRest {
	return e.pub
}

// symbol 格式转换: "BTC/USDT" -> goex 需要的 CurrencyPair
func (e *Okx) toCurrencyPair(symbol string) (model.CurrencyPair, error) {
	parts := strings.Split(symbol, "/")
	if len(parts) == 1 { // 防止BTC-USDT-SWAP
		parts = strings.Split(symbol, "-")
	}
	if len(parts) > 2 { // 取前两个，防止BTC-USDT-SWAP
		parts = parts[:2]
		//return model.CurrencyPair{}, errors.New("invalid symbol format, expected like BTC/USDT")
	}
	return e.getPub().NewCurrencyPair(string(parts[0]), string(parts[1]))
}

// 获取最新价格
func (e *Okx) GetLastPrice(symbol string) (float64, error) {
	pair, err := e.toCurrencyPair(symbol)
	if err != nil {
		return 0, err
	}
	ticker, _, _ := e.getPub().GetTicker(pair)
	if ticker == nil {
		return 0, errors.New("failed to get ticker")
	}
	return ticker.Last, nil
}

// 取消订单
func (e *Okx) CancelOrder(orderID, symbol string) error {
	pair, err := e.toCurrencyPair(symbol)
	if err != nil {
		return err
	}
	_, err = e.prv.CancelOrder(pair, orderID)
	return err
}

// 获取订单状态
func (e *Okx) GetOrderStatus(orderID string, symbol string) (*model2.OrderStatus, error) {
	pari, err := e.toCurrencyPair(symbol)
	if err != nil {
		return nil, err
	}

	info, body, err := e.prv.GetOrderInfo(pari, orderID)
	if err != nil {
		return nil, err
	}
	fmt.Printf("GetOrderStatus : %v", body)
	return &model2.OrderStatus{
		OrderID:   info.Id,
		Status:    info.Status.String(),
		Filled:    info.ExecutedQty,
		Remaining: info.Qty - info.Qty,
	}, nil
}

// 下单购买
// 注意限价和市价的Quantity单位不相同，当限价时Quantity的单位为币本身，当市价时Quantity的单位为USDT
func (e *Okx) PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error) {
	return nil, nil
}

// 初始化时加载所有可交易币对
// 测试连接，创建订单时需要调用GetExchangeInfo获取pair
func (e *Okx) GetExchangeInfo() (map[string]model.CurrencyPair, []byte, error) {
	info, data, err := e.getPub().GetExchangeInfo()
	if err != nil {
		e.exInfo = info
	}
	return info, data, err
}
