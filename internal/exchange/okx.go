package exchange

//
//import (
//	"context"
//	"edgeflow/internal/account"
//	model2 "edgeflow/internal/model"
//	"encoding/json"
//	"errors"
//	"fmt"
//	goexv2 "github.com/nntaoli-project/goex/v2"
//	"github.com/nntaoli-project/goex/v2/model"
//	"github.com/nntaoli-project/goex/v2/okx/futures"
//	"github.com/nntaoli-project/goex/v2/options"
//	"strconv"
//	"strings"
//)
//
//type OkxApiGroup struct {
//	Pub    goexv2.IPubRest
//	Prv    goexv2.IPrvRest
//	ExInfo map[string]model.CurrencyPair
//}
//
//type OkxExchange struct {
//	apiCache map[model2.OrderTradeTypeType]OkxApiGroup
//	apiConf  []options.ApiOption
//	account  Account
//}
//
//// 构造函数只存储配置，不初始化接口
//func NewOkxExchange(apiKey, apiSecret, passphrase string) *OkxExchange {
//	/*
//		| 类型      | 是否持有实币 | 是否有交割日 | 支持杠杆   | 适合人群     |
//		| ------- | ------ | ------ | ------ | -------- |
//		| Spot 现货    | ✅ 持有实币 | ❌ 无交割  | 🚫 无杠杆 | 投资者/初学者  |
//		| Futures 交割合约 | ❌ 不持币  | ✅ 有交割  | ✅ 高杠杆  | 专业交易者    |
//		| Swap  永续合约  | ❌ 不持币  | ❌ 无交割  | ✅ 高杠杆  | 高频/策略交易者 |
//
//	*/
//
//	// okxv5 api 如果要使用模拟交易，需要切到到模拟交易下创建apikey
//	opts := []options.ApiOption{
//		options.WithApiKey(apiKey),
//		options.WithApiSecretKey(apiSecret),
//		options.WithPassphrase(passphrase),
//	}
//
//	return &OkxExchange{
//		apiCache: make(map[model2.OrderTradeTypeType]OkxApiGroup),
//		apiConf:  opts,
//		account:  account.NewAccountService(goexv2.OKx.Swap.NewPrvApi(opts...)),
//	}
//}
//
//func (e *OkxExchange) Account() (acc Account) {
//	return e.account
//}
//
//func (e *OkxExchange) getApiGroup(marketType model2.OrderTradeTypeType) (OkxApiGroup, error) {
//	if group, ok := e.apiCache[marketType]; ok {
//		return group, nil
//	}
//
//	var pub goexv2.IPubRest
//	var prv goexv2.IPrvRest
//
//	switch marketType {
//	case "spot":
//		pub = goexv2.OKx.Spot
//		prv = goexv2.OKx.Spot.NewPrvApi(e.apiConf...)
//	case "swap":
//		pub = goexv2.OKx.Swap
//		prv = goexv2.OKx.Swap.NewPrvApi(e.apiConf...)
//	case "futures":
//		pub = goexv2.OKx.Futures
//		prv = goexv2.OKx.Futures.NewPrvApi(e.apiConf...)
//	default:
//		return OkxApiGroup{}, fmt.Errorf("unsupported market type: %s", marketType)
//	}
//
//	// 初始化时加载所有可交易币对
//	// 测试连接，创建订单时需要调用GetExchangeInfo获取pair
//	info, _, err := pub.GetExchangeInfo()
//
//	if err != nil {
//		return OkxApiGroup{}, fmt.Errorf("failed to get exchange info: %w", err)
//	}
//
//	group := OkxApiGroup{Pub: pub, Prv: prv, ExInfo: info}
//	e.apiCache[marketType] = group
//	return group, nil
//}
//
//// symbol 格式转换: "BTC/USDT" -> goex 需要的 CurrencyPair
//func (e *OkxExchange) toCurrencyPair(symbol string, apiGroup OkxApiGroup) (model.CurrencyPair, error) {
//	parts := strings.Split(symbol, "/")
//	if len(parts) == 1 { // 防止BTC-USDT-SWAP
//		parts = strings.Split(symbol, "-")
//	}
//	if len(parts) > 2 { // 取前两个，防止BTC-USDT-SWAP
//		parts = parts[:2]
//		//return model.CurrencyPair{}, errors.New("invalid symbol format, expected like BTC/USDT")
//	}
//	return apiGroup.Pub.NewCurrencyPair(string(parts[0]), string(parts[1]))
//}
//
//func (e *OkxExchange) CurrencyPairByTradingType(symbol string, tradingType model2.OrderTradeTypeType) (model.CurrencyPair, error) {
//	group, err := e.getApiGroup(tradingType)
//	if err != nil {
//		return model.CurrencyPair{}, err
//	}
//	pair, err := e.toCurrencyPair(symbol, group)
//	return pair, err
//}
//
//func (e *OkxExchange) GetLastPrice(symbol string, tradingType model2.OrderTradeTypeType) (float64, error) {
//	group, err := e.getApiGroup(tradingType)
//	if err != nil {
//		return 0, err
//	}
//	pair, err := e.toCurrencyPair(symbol, group)
//	if err != nil {
//		return 0, err
//	}
//	ticker, _, _ := group.Pub.GetTicker(pair)
//	if ticker == nil {
//		return 0, errors.New("failed to get ticker")
//	}
//	return ticker.Last, nil
//}
//
//// 下单购买
//// 注意限价和市价的Quantity单位不相同，当限价时Quantity的单位为币本身，当市价时Quantity的单位为USDT
//func (e *OkxExchange) PlaceOrder(ctx context.Context, order *model2.Order) (*model2.OrderResponse, error) {
//
//	group, err := e.getApiGroup(order.TradeType)
//	if err != nil {
//		return nil, err
//	}
//	pair, err := e.toCurrencyPair(order.Symbol, group)
//	if err != nil {
//		return nil, err
//	}
//	var posSide model2.OrderPosSide
//	var side model.OrderSide
//	switch strings.ToLower(string(order.Side)) {
//	case "buy":
//		if order.TradeType == model2.OrderTradeSpot {
//			side = model.Spot_Buy
//		} else {
//			side = model.Futures_OpenBuy
//		}
//		posSide = model2.OrderPosSideLong
//	case "sell":
//		if order.TradeType == model2.OrderTradeSpot {
//			side = model.Spot_Sell
//		} else {
//			side = model.Futures_OpenSell
//		}
//		posSide = model2.OrderPosSideShort
//	default:
//		return nil, errors.New("invalid order side")
//	}
//
//	var orderType model.OrderType
//	switch order.OrderType {
//	case model2.Limit:
//		orderType = model.OrderType_Limit
//	case model2.Market:
//		orderType = model.OrderType_Market
//	}
//
//	// 如果有止盈和止损
//	var opts []model.OptionParameter
//
//	// 如果设置了止盈
//	if order.TPPrice > 0 {
//		opts = append(opts, model.OptionParameter{
//			Key:   "tpTriggerPx",
//			Value: strconv.FormatFloat(order.TPPrice, 'f', -1, 64), // 止盈触发价
//		})
//		opts = append(opts, model.OptionParameter{
//			Key:   "tpOrdPx",
//			Value: "-1", // -1 表示市价止盈
//		})
//	}
//
//	// 如果设置了止损
//	if order.SLPrice > 0 {
//		opts = append(opts, model.OptionParameter{
//			Key:   "slTriggerPx",
//			Value: strconv.FormatFloat(order.SLPrice, 'f', -1, 64), // 止损触发价
//		})
//		opts = append(opts, model.OptionParameter{
//			Key:   "slOrdPx",
//			Value: "-1", // -1 表示市价止损
//		})
//	}
//
//	/*
//		合约交易需要设置tdMode
//		| 值          | 含义   |
//		| ---------- | ---- |
//		| `cross`    | 全仓模式 |
//		| `isolated` | 逐仓模式 |
//	*/
//	mgnMode := order.MgnMode
//	leverage := order.Leverage
//	if order.TradeType == model2.OrderTradeSwap {
//		if mgnMode == "" {
//			mgnMode = model2.OrderMgnModeIsolated
//		}
//		//这里统一使用逐仓模式
//		opts = append(opts, model.OptionParameter{
//			Key:   "tdMode",
//			Value: string(mgnMode),
//		})
//
//		opts = append(opts, model.OptionParameter{
//			Key:   "posSide",
//			Value: string(posSide),
//		})
//
//		if leverage <= 0 {
//			leverage = 20
//		}
//		order.Leverage = leverage
//		// 设置杠杆倍数
//		err = e.SetLeverage(order.Symbol, leverage, string(mgnMode), string(posSide))
//		if err != nil {
//			return nil, err
//		}
//
//		// 根据比例计算下单金额
//		if order.QuantityPct > 0 {
//			// 获取可用余额，根据比例计算下单数量
//			acc, err := e.account.GetAccount(ctx, "USDT")
//			if err != nil {
//				return nil, err
//			}
//
//			// 根据QuantityPct计算下单张数
//			qty := CalculateOrderSz(acc.Available*order.QuantityPct, leverage, order.Price, pair.ContractVal)
//			order.Quantity = float64(qty)
//		}
//	}
//	order.MgnMode = mgnMode
//
//	// 创建订单
//	createdOrder, resp, err := group.Prv.CreateOrder(pair, order.Quantity, order.Price, side, orderType, opts...)
//	if err != nil {
//		fmt.Printf("CreateOrder error：%v", resp)
//		return nil, err
//	}
//
//	return &model2.OrderResponse{
//		OrderId: createdOrder.Id,
//		Status:  int(createdOrder.Status),
//	}, nil
//}
//
//func (e *OkxExchange) CancelOrder(orderID, symbol string, tradingType model2.OrderTradeTypeType) error {
//	group, err := e.getApiGroup(tradingType)
//	if err != nil {
//		return err
//	}
//	pair, err := e.toCurrencyPair(symbol, group)
//	if err != nil {
//		return err
//	}
//	_, err = group.Prv.CancelOrder(pair, orderID)
//	return err
//}
//
//func (e *OkxExchange) GetOrderStatus(orderID string, symbol string, tradingType model2.OrderTradeTypeType) (*model2.OrderStatus, error) {
//	group, err := e.getApiGroup(tradingType)
//	if err != nil {
//		return nil, err
//	}
//	pari, err := e.toCurrencyPair(symbol, group)
//	if err != nil {
//		return nil, err
//	}
//
//	info, body, err := group.Prv.GetOrderInfo(pari, orderID)
//	if err != nil {
//		return nil, err
//	}
//	fmt.Printf("GetOrderStatus : %v", body)
//	return &model2.OrderStatus{
//		OrderID:   info.Id,
//		Status:    info.Status.String(),
//		Filled:    info.ExecutedQty,
//		Remaining: info.Qty - info.Qty,
//	}, nil
//}
//
//// 获取持仓数据
//func (e *OkxExchange) getPosition(symbol string) ([]model2.PositionInfo, error) {
//
//	group, err := e.getApiGroup(model2.OrderTradeSwap)
//	if err != nil {
//		return nil, err
//	}
//	pari, err := e.toCurrencyPair(symbol, group)
//	if err != nil {
//		return nil, err
//	}
//
//	swap, ok := group.Prv.(*futures.PrvApi)
//	if !ok {
//		return nil, errors.New("Prv() 不是 okex5.RestClient，无法获取仓位")
//	}
//
//	res, data, err := swap.GetPositions(pari)
//	if err != nil {
//		return nil, err
//	}
//	type JSONData struct {
//		Code string `json:"code"`
//		Data []struct {
//			MgnMode string `json:"mgnMode"`
//			LiqPx   string `json:"liqPx"`
//		} `json:"data"`
//		Msg string `json:"msg"`
//	}
//	var jsonData JSONData
//	err = json.Unmarshal(data, &jsonData)
//	if err != nil {
//		return nil, err
//	}
//
//	var items []model2.PositionInfo
//	for i, re := range res {
//		var item model2.PositionInfo
//		if re.Qty == 0 {
//			// 没有张数的仓位忽略
//			continue
//		}
//		var ps model2.OrderSide
//		switch re.PosSide {
//		case model.Futures_OpenBuy, model.Spot_Buy:
//			// 开多仓位
//			ps = model2.OrderPosSideLong
//		case model.Futures_OpenSell, model.Spot_Sell:
//			// 开空仓位
//			ps = model2.OrderPosSideShort
//		}
//		item.Symbol = pari.Symbol
//		item.Side = ps
//		item.Amount = re.Qty
//		item.AvgPrice = re.AvgPx
//		item.MgnMode = jsonData.Data[i].MgnMode
//		item.LiqPx = jsonData.Data[i].LiqPx
//		items = append(items, item)
//	}
//
//	return items, err
//}
//
//// SetLeverage 设置合约杠杆
//// instId     例如 "BTC-USDT-SWAP"，如果传入的是BTC/USDT，会通过CurrencyPair去查找对应的的instId
//// leverage   杠杆倍数，例如 20、50
//// marginMode 保证金模式：isolated（逐仓）或 cross（全仓）
//// posSide    持仓方向：long（做多）、short（做空）、""（全仓模式下可为空）
//func (e *OkxExchange) SetLeverage(symbol string, leverage int, marginMode, posSide string) error {
//
//	// 请求设置杠杆
//	group, err := e.getApiGroup(model2.OrderTradeSwap)
//
//	if err != nil {
//		return err
//	}
//
//	// 当传入的是BTC/USDT时，通过CurrencyPair匹配正确的instId
//	pair, err := e.toCurrencyPair(symbol, group)
//	var instId = symbol
//	if err == nil {
//		instId = pair.Symbol
//	}
//	okxPrv, ok := group.Prv.(*futures.PrvApi)
//	if !ok {
//		return errors.New("Prv() 不是 okex5.RestClient，无法设置杠杆")
//	}
//
//	var opts []model.OptionParameter
//	// 安全性检查
//	if marginMode != model2.OrderMgnModeIsolated && marginMode != model2.OrderMgnModeCross {
//		return fmt.Errorf("不支持的保证金模式: %s", marginMode)
//	}
//
//	if marginMode == "isolated" && (posSide != "long" && posSide != "short") {
//		return fmt.Errorf("逐仓模式下必须指定 posSide（long 或 short）")
//	}
//
//	opts = append(opts, model.OptionParameter{
//		Key:   "mgnMode",
//		Value: marginMode,
//	}, model.OptionParameter{
//		Key:   "posSide",
//		Value: posSide,
//	})
//	// posSide
//	resp, err := okxPrv.SetLeverage(instId, strconv.Itoa(leverage), opts...)
//	if err != nil {
//		return fmt.Errorf("设置杠杆失败: %w", err)
//	}
//
//	fmt.Println("杠杆设置响应:", string(resp))
//	return nil
//}
//
//// 平仓函数
//func (e *OkxExchange) ClosePosition(symbol string, side string, quantity float64, tdMode string) error {
//	// 请求设置杠杆
//	group, err := e.getApiGroup(model2.OrderTradeSwap)
//
//	if err != nil {
//		return err
//	}
//
//	// 当传入的是BTC/USDT时，通过CurrencyPair匹配正确的instId
//	pair, err := e.toCurrencyPair(symbol, group)
//	if err != nil {
//		return err
//	}
//	var orderSide model.OrderSide
//
//	// 如果是多仓 -> 需要做空（卖）来平仓
//	// 如果是空仓 -> 需要做多（买）来平仓
//	switch side {
//	case "long":
//		// 持有多单，平掉多单
//		orderSide = model.Futures_CloseBuy
//	case "short":
//		// 持有空单，平掉空单
//		orderSide = model.Futures_OpenSell
//	default:
//		return fmt.Errorf("unknown side: %s", side)
//	}
//
//	opts := []model.OptionParameter{
//		model.OptionParameter{
//			Key:   "tdMode",
//			Value: tdMode,
//		},
//	}
//
//	// 提交市价平仓订单
//	order, resp, err := group.Prv.CreateOrder(pair, quantity, 0, orderSide, model.OrderType_Market, opts...)
//	if err != nil {
//		fmt.Printf("CreateOrder error：%v", resp)
//		return err
//	}
//
//	fmt.Printf("平仓成功，订单ID：%s\n", order.Id)
//	return nil
//}
//
//// 查询是否有持仓
//func (e *OkxExchange) GetPosition(symbol string) (long *model2.PositionInfo, short *model2.PositionInfo, err error) {
//	positions, err := e.getPosition(symbol)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	for _, pos := range positions {
//		// 一般方向字段为 "long" 或 "short"，也可能是 "net"（净持仓模式）
//		switch pos.Side {
//		case "long":
//			if pos.Amount > 0 {
//				long = &pos
//			}
//		case "short":
//			if pos.Amount > 0 {
//				short = &pos
//			}
//		}
//	}
//
//	return
//}
//
//// 计算下单张数（按成本算）
//func CalculateOrderSz(costUSDT float64, leverage int, marketPrice float64, ctVal float64) int {
//	notional := costUSDT * float64(leverage) // 名义价值（目标开仓总金额）
//	oneContractValue := marketPrice * ctVal  // 每张合约对应的价值
//	sz := int(notional / oneContractValue)   // 得到张数（取整）
//	if sz < 1 {
//		sz = 1
//	}
//	return sz
//}
