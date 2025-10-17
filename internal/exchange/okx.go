package exchange

import (
	"context"
	"edgeflow/internal/exchange/okx"
	model2 "edgeflow/internal/model"
	"errors"
	"fmt"
	"github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex/v2/options"
	"log"
	"time"
)

type OkxExchange struct {
	apiCache map[model2.OrderTradeType]okx.OkxService
	//spot    *okx.OkxSpot
	//swap    *okx.OkxSwap
	//futures *okx.OkxFutures
	apiConf []options.ApiOption
}

// 构造函数只存储配置，不初始化接口
func NewOkxExchange(apiKey, apiSecret, passphrase string) *OkxExchange {
	/*
		| 类型      | 是否持有实币 | 是否有交割日 | 支持杠杆   | 适合人群     |
		| ------- | ------ | ------ | ------ | -------- |
		| Spot 现货    | ✅ 持有实币 | ❌ 无交割  | 🚫 无杠杆 | 投资者/初学者  |
		| Futures 交割合约 | ❌ 不持币  | ✅ 有交割  | ✅ 高杠杆  | 专业交易者    |
		| Swap  永续合约  | ❌ 不持币  | ❌ 无交割  | ✅ 高杠杆  | 高频/策略交易者 |

	*/

	// okxv5 api 如果要使用模拟交易，需要切到到模拟交易下创建apikey
	opts := []options.ApiOption{
		options.WithApiKey(apiKey),
		options.WithApiSecretKey(apiSecret),
		options.WithPassphrase(passphrase),
	}

	return &OkxExchange{
		apiCache: make(map[model2.OrderTradeType]okx.OkxService),
		apiConf:  opts,
	}
}

func (e *OkxExchange) Account(tradeType model2.OrderTradeType) (acc Account, err error) {
	api, err := e.getApi(tradeType)
	if err != nil {
		return nil, err
	}
	okxApi, ok := api.(*okx.Okx)
	if ok {
		return okxApi.Account, nil
	}
	return nil, errors.New("当前交易类型不支持获取账户余额")
}

// 懒加载api服务
func (e *OkxExchange) getApi(marketType model2.OrderTradeType) (okx.OkxService, error) {

	if group, ok := e.apiCache[marketType]; ok {
		return group, nil
	}

	var spotApi *okx.OkxSpot
	var swapApi *okx.OkxSwap
	var fApi *okx.OkxFutures

	switch marketType {
	case "spot":
		spotApi = okx.NewOkxSpot(e.apiConf)

		// 初始化时加载所有可交易币对
		// 测试连接，创建订单时需要调用GetExchangeInfo获取pair
		_, _, err := spotApi.GetExchangeInfo()
		if err != nil {
			fmt.Printf("GetExchangeInfo err : %v", err)
			return nil, err
		} else {
			e.apiCache[marketType] = spotApi
			return spotApi, nil
		}
	case "swap":
		swapApi = okx.NewOkxSwap(e.apiConf)
		_, _, err := swapApi.GetExchangeInfo()
		if err != nil {
			fmt.Printf("GetExchangeInfo err : %v", err)
			return nil, err
		} else {
			e.apiCache[marketType] = swapApi
			return swapApi, nil
		}
	case "futures":
		fApi = okx.NewOkxFutures(e.apiConf)
		_, _, err := fApi.GetExchangeInfo()
		if err != nil {
			fmt.Printf("GetExchangeInfo err : %v", err)
			return nil, err
		} else {
			e.apiCache[marketType] = fApi
			return fApi, nil
		}
	default:
		return nil, fmt.Errorf("unsupported market type: %s", marketType)
	}
}

func (e *OkxExchange) GetLastPrice(symbol string, tradingType model2.OrderTradeType) (float64, error) {
	api, err := e.getApi(tradingType)
	if err != nil {
		return 0, err
	}
	return api.GetLastPrice(symbol)
}

// 下单购买
// 注意限价和市价的Quantity单位不相同，当限价时Quantity的单位为币本身，当市价时Quantity的单位为USDT
func (e *OkxExchange) PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error) {
	api, err := e.getApi(order.TradeType)
	if err != nil {
		return nil, err
	}
	return api.PlaceOrder(ctx, order)
}

func (e *OkxExchange) CancelOrder(orderID, symbol string, tradingType model2.OrderTradeType) error {
	api, err := e.getApi(tradingType)
	if err != nil {
		return err
	}
	return api.CancelOrder(orderID, symbol)
}

func (e *OkxExchange) GetOrderStatus(orderID string, symbol string, tradingType model2.OrderTradeType) (*model2.OrderStatus, error) {
	api, err := e.getApi(tradingType)
	if err != nil {
		return nil, err
	}
	return api.GetOrderStatus(orderID, symbol)
}

// SetLeverage 设置合约杠杆
// instId     例如 "BTC-USDT-SWAP"，如果传入的是BTC/USDT，会通过CurrencyPair去查找对应的的instId
// leverage   杠杆倍数，例如 20、50
// marginMode 保证金模式：isolated（逐仓）或 cross（全仓）
// posSide    持仓方向：long（做多）、short（做空）、""（全仓模式下可为空）
func (e *OkxExchange) SetLeverage(symbol string, leverage int, marginMode, posSide string, tradeType model2.OrderTradeType) error {

	// 请求设置杠杆
	api, err := e.getApi(tradeType)
	if err != nil {
		return err
	}
	switch v := api.(type) {
	case *okx.OkxFutures:
		return v.SetLeverage(symbol, leverage, marginMode, posSide)
	case *okx.OkxSwap:
		return v.SetLeverage(symbol, leverage, marginMode, posSide)
	default:
		return errors.New("当前交易类型不支持设置杠杆倍数SetLeverage")
	}
}

// 平仓函数
func (e *OkxExchange) ClosePosition(symbol string, side string, quantity float64, tdMode string, tradeType model2.OrderTradeType) error {
	api, err := e.getApi(tradeType)
	if err != nil {
		return err
	}

	switch v := api.(type) {
	case *okx.OkxFutures:
		return v.ClosePosition(symbol, side, quantity, tdMode)
	case *okx.OkxSwap:
		return v.ClosePosition(symbol, side, quantity, tdMode)
	default:
		return errors.New("当前交易类型不支持关闭仓位ClosePosition")
	}
}

// 查询是否有持仓
func (e *OkxExchange) GetPosition(symbol string, tradeType model2.OrderTradeType) (long *model2.PositionInfo, short *model2.PositionInfo, err error) {
	api, err := e.getApi(tradeType)
	if err != nil {
		return nil, nil, err
	}
	switch v := api.(type) {
	case *okx.OkxSwap:
		return v.GetPosition(symbol)
	case *okx.OkxFutures:
		return v.GetPosition(symbol)
	default:
		return nil, nil, errors.New("当前交易类型不支持获取仓位GetPosition")
	}

}

func (e *OkxExchange) AmendAlgoOrder(instId string, tradeType model2.OrderTradeType, algoId string, newSlTriggerPx, newTpTriggerPx float64) ([]byte, error) {
	api, err := e.getApi(tradeType)
	if err != nil {
		return nil, err
	}

	return api.AmendAlgoOrder(instId, algoId, newSlTriggerPx, -1, newTpTriggerPx, -1)
}

func (e *OkxExchange) GetKlineRecords(symbol string, period model.KlinePeriod, size int, start, end int64, tradeType model2.OrderTradeType, includeUnclosed bool) ([]model2.Kline, error) {
	klines, err := e.getKlineRecords(symbol, period, size, start, end, tradeType, includeUnclosed)
	if err != nil {
		return nil, err
	}

	reLines := make([]model2.Kline, len(klines))
	for i := 0; i < len(klines); i++ {
		reLines[i] = klines[len(klines)-1-i] // 最新 -> 最前
	}

	return reLines, nil
}

func (e *OkxExchange) getKlineRecords(symbol string, period model.KlinePeriod, size int, start, end int64, tradeType model2.OrderTradeType, includeUnclosed bool) ([]model2.Kline, error) {

	api, err := e.getApi(tradeType)
	if err != nil {
		return nil, err
	}
	var result []model2.Kline
	// 最多重试 3 次
	for i := 0; i < 3; i++ {
		result, err = api.GetKlineRecords(symbol, period, size, start, end, includeUnclosed)
		if err == nil {
			return result, err // 成功直接返回
		}
		if err != nil && err.Error() == "not found currency pair" {
			return nil, err
		}
		log.Printf("GetKlineRecords failed (try %d): %v", i+1, err)
		time.Sleep(time.Second * time.Duration(i+1)) // 指数退避: 1s, 2s, 3s
	}

	// 如果 3 次都失败，就返回最后的错误
	return nil, fmt.Errorf("GetKlines failed after 3 retries: %w", err)
}

// 将最新在前的切片，转换为 从旧到新（必要时丢弃未收盘）
// latestFirst 是从okx获取的原始k线数组，顺序是从新到旧
// dropUnclosed 是否丢掉未收盘的当前bar
//func ReverseKlines(latestFirst []model2.Kline, period model.KlinePeriod, dropUnclosed bool) []model2.Kline {
//	if len(latestFirst) == 0 {
//		return nil
//	}
//	tf := periodToDuration(period)
//	start := 0
//	if dropUnclosed && isUnclosedBar(latestFirst[0].Timestamp, tf, time.Now()) {
//		start = 1 // 丢掉正在形成的那根
//	}
//	if start >= len(latestFirst) {
//		return nil
//	}
//	// 反转为 从旧到新
//	n := len(latestFirst) - start
//	out := make([]model2.Kline, n)
//	for i := 0; i < n; i++ {
//		out[i] = latestFirst[len(latestFirst)-1-i] // 最旧 → 最前
//	}
//	return out
//}

func periodToDuration(p model.KlinePeriod) time.Duration {
	switch p {
	case model.Kline_1min:
		return time.Minute
	case model.Kline_5min:
		return 5 * time.Minute
	case model.Kline_15min:
		return 15 * time.Minute
	case model.Kline_30min:
		return 30 * time.Minute
	case model.Kline_60min, model.Kline_1h:
		return time.Hour
	case model.Kline_4h:
		return 4 * time.Hour
	case model.Kline_6h:
		return 6 * time.Hour
	case model.Kline_1day:
		return 24 * time.Hour
	case model.Kline_1week:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

// 判断最前面那根是否未收盘（以周期判断）
func isUnclosedBar(tsMillis time.Time, tf time.Duration, now time.Time) bool {
	barStart := tsMillis
	barEnd := barStart.Add(tf)
	return now.Before(barEnd) // 还没到收盘时间 → 未收盘
}
