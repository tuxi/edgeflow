package exchange

import (
	"context"
	model2 "edgeflow/internal/model"
	"errors"
	"fmt"
	goexv2 "github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex/v2/options"
	"strings"
)

type OkxExchange struct {
	pubApi goexv2.IPubRest
	prvApi goexv2.IPrvRest
}

func NewOkxExchange(apiKey, apiSecret, passphrase string) (*OkxExchange, error) {
	/*
		| 类型      | 是否持有实币 | 是否有交割日 | 支持杠杆   | 适合人群     |
		| ------- | ------ | ------ | ------ | -------- |
		| Spot 现货    | ✅ 持有实币 | ❌ 无交割  | 🚫 无杠杆 | 投资者/初学者  |
		| Futures 交割合约 | ❌ 不持币  | ✅ 有交割  | ✅ 高杠杆  | 专业交易者    |
		| Swap  永续合约  | ❌ 不持币  | ❌ 无交割  | ✅ 高杠杆  | 高频/策略交易者 |

	*/
	pubApi := goexv2.OKx.Spot
	// okxv5 api 如果要使用模拟交易，需要切到到模拟交易下创建apikey
	prvApi := pubApi.NewPrvApi(
		options.WithApiKey(apiKey),
		options.WithApiSecretKey(apiSecret),
		options.WithPassphrase(passphrase),
	)

	// 测试连接，创建订单时需要调用GetExchangeInfo获取pair
	info, _, err := pubApi.GetExchangeInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info: %w", err)
	}

	fmt.Printf("info: [%v]", info)
	return &OkxExchange{
		pubApi: pubApi,
		prvApi: prvApi,
	}, nil
}

// symbol 格式转换: "BTC/USDT" -> goex 需要的 CurrencyPair
func (e *OkxExchange) toCurrencyPair(symbol string) (model.CurrencyPair, error) {
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return model.CurrencyPair{}, errors.New("invalid symbol format, expected like BTC/USDT")
	}
	return e.pubApi.NewCurrencyPair(string(parts[0]), string(parts[1]))
}

func (e *OkxExchange) GetLastPrice(symbol string) (float64, error) {
	pair, err := e.toCurrencyPair(symbol)
	if err != nil {
		return 0, err
	}
	ticker, _, _ := e.pubApi.GetTicker(pair)
	if ticker == nil {
		return 0, errors.New("failed to get ticker")
	}
	return ticker.Last, nil
}

// 下单购买
// 注意限价和市价的Quantity单位不相同，当限价时Quantity的单位为币本身，当市价时Quantity的单位为USDT
func (e *OkxExchange) PlaceOrder(ctx context.Context, order model2.Order) (*model2.OrderResponse, error) {
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

	// 创建订单
	createdOrder, _, err := e.prvApi.CreateOrder(pair, order.Quantity, order.Price, side, orderType)
	if err != nil {
		return nil, err
	}

	return &model2.OrderResponse{
		OrderId: createdOrder.Id,
		Status:  int(createdOrder.Status),
	}, nil
}

func (e *OkxExchange) CancelOrder(orderID, symbol string) error {
	pair, err := e.toCurrencyPair(symbol)
	if err != nil {
		return err
	}
	_, err = e.prvApi.CancelOrder(pair, orderID)
	return err
}

func (e *OkxExchange) GetOrderStatus(orderID string, symbol string) (*model2.OrderStatus, error) {
	pari, err := e.toCurrencyPair(symbol)
	if err != nil {
		return nil, err
	}

	info, body, err := e.prvApi.GetOrderInfo(pari, orderID)
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
