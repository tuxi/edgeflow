package okx

import (
	"context"
	"edgeflow/internal/account"
	model2 "edgeflow/internal/model"
	"errors"
	"fmt"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex/v2/okx/common"
	"github.com/nntaoli-project/goex/v2/okx/futures"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type OkxService interface {
	PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error)
	GetOrderStatus(orderID string, symbol string) (*model2.OrderStatus, error)
	CancelOrder(orderID, symbol string) error
	GetLastPrice(symbol string) (float64, error)
	GetExchangeInfo() (map[string]model.CurrencyPair, []byte, error)
	AmendAlgoOrder(instId string, algoId string, newSlTriggerPx, newSlOrdPx, newTpTriggerPx, newTpOrdPx float64) ([]byte, error)
	GetKlineRecords(symbol string, period model.KlinePeriod, size int, start, end int64) ([]model2.Kline, error)
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

// AmendAlgoOrder 修改算法单（止盈止损/计划委托）
// 支持更新止盈止损触发价、委托价、数量等
func (e *Okx) AmendAlgoOrder(instId, algoId string, newSlTriggerPx, newSlOrdPx, newTpTriggerPx, newTpOrdPx float64) ([]byte, error) {
	prv, ok := e.prv.(*futures.PrvApi)
	if !ok {
		return nil, errors.New("AmendAlgoOrder err")
	}
	reqUrl := fmt.Sprintf("%s%s", prv.UriOpts.Endpoint, "/api/v5/trade/amend-algo-order")

	params := url.Values{}
	params.Set("instId", instId) //pair.Symbol)
	params.Set("algoId", algoId)
	if newSlTriggerPx > 0 {
		px := strconv.FormatFloat(newSlTriggerPx, 'f', -1, 64)
		params.Set("newSlTriggerPx", px)
	}
	if newSlOrdPx > 0 {
		px := strconv.FormatFloat(newSlTriggerPx, 'f', -1, 64)
		params.Set("newSlOrdPx", px)
	}
	if newTpTriggerPx > 0 {
		px := strconv.FormatFloat(newTpTriggerPx, 'f', -1, 64)
		params.Set("newTpTriggerPx", px)
	}
	if newTpOrdPx > 0 {
		px := strconv.FormatFloat(newTpOrdPx, 'f', -1, 64)
		params.Set("newTpOrdPx", px)
	}

	//util.MergeOptionParams(&params, opts...)
	common.AdaptOrderClientIDOptionParameter(&params)

	_, resp, err := prv.DoAuthRequest(http.MethodPost, reqUrl, &params, nil)
	if err != nil {
		fmt.Printf("[AmendOrder] response body =%s", string(resp))
		return resp, err
	}

	return resp, err
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

func (e *Okx) GetKlineRecords(symbol string, period model.KlinePeriod, size int, start, end int64) ([]model2.Kline, error) {
	pair, err := e.toCurrencyPair(symbol)
	if err != nil {
		return nil, err
	}

	var opts []model.OptionParameter
	if size > 0 {
		opts = append(opts, model.OptionParameter{
			Key:   "limit",
			Value: strconv.Itoa(size),
		})
	}
	if start > 0 {
		opts = append(opts, model.OptionParameter{
			Key:   "before",
			Value: fmt.Sprintf("%v", start),
		})
	}
	if end > 0 {
		opts = append(opts, model.OptionParameter{
			Key:   "after",
			Value: fmt.Sprintf("%v", end),
		})
	}
	info, _, err := e.getPub().GetKline(pair, period, opts...)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("[GetKlineRecords] data = %v", string(data))

	var items []model2.Kline
	for _, item := range info {
		items = append(items, model2.Kline{
			Timestamp: time.UnixMilli(item.Timestamp),
			Open:      item.Open,
			Close:     item.Close,
			High:      item.High,
			Low:       item.Low,
			Vol:       item.Vol,
		})
	}

	return items, nil
}
