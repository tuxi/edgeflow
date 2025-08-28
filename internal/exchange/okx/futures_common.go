package okx

import (
	model2 "edgeflow/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nntaoli-project/goex/v2/model"
	"github.com/nntaoli-project/goex/v2/okx/futures"
	"math"
	"strconv"
)

// 合约公共结构体，为实现公共的方法
type FuturesCommon struct {
	Okx
}

// 只有合约才可以获取持仓数据
func (e *FuturesCommon) getPosition(symbol string) ([]model2.PositionInfo, error) {

	pari, err := e.toCurrencyPair(symbol)
	if err != nil {
		return nil, err
	}

	swap, ok := e.prv.(*futures.PrvApi)
	if !ok {
		return nil, errors.New("Prv() 不是 okex5.RestClient，无法获取仓位")
	}

	res, data, err := swap.GetPositions(pari)
	if err != nil {
		return nil, err
	}
	type JSONData struct {
		Code string `json:"code"`
		Data []struct {
			MgnMode string `json:"mgnMode"`
			LiqPx   string `json:"liqPx"`
			AlgoId  string `json:"algoId"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	var jsonData JSONData
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		return nil, err
	}

	var items []model2.PositionInfo
	for i, re := range res {
		var item model2.PositionInfo
		if re.Qty == 0 {
			// 没有张数的仓位忽略
			continue
		}
		var ps model2.OrderSide
		switch re.PosSide {
		case model.Futures_OpenBuy, model.Spot_Buy:
			// 开多仓位
			ps = model2.OrderPosSideLong
		case model.Futures_OpenSell, model.Spot_Sell:
			// 开空仓位
			ps = model2.OrderPosSideShort
		}
		item.Symbol = pari.Symbol
		item.Side = ps
		item.Amount = re.Qty
		item.AvgPrice = re.AvgPx
		item.MgnMode = jsonData.Data[i].MgnMode
		item.LiqPx = jsonData.Data[i].LiqPx
		item.AlgoId = jsonData.Data[i].AlgoId
		items = append(items, item)
	}

	return items, err
}

// SetLeverage 设置合约杠杆
// instId     例如 "BTC-USDT-SWAP"，如果传入的是BTC/USDT，会通过CurrencyPair去查找对应的的instId
// leverage   杠杆倍数，例如 20、50
// marginMode 保证金模式：isolated（逐仓）或 cross（全仓）
// posSide    持仓方向：long（做多）、short（做空）、""（全仓模式下可为空）
func (e *FuturesCommon) SetLeverage(symbol string, leverage int, marginMode, posSide string) error {

	// 当传入的是BTC/USDT时，通过CurrencyPair匹配正确的instId
	pair, err := e.toCurrencyPair(symbol)
	var instId = symbol
	if err == nil {
		instId = pair.Symbol
	}
	okxPrv, ok := e.prv.(*futures.PrvApi)
	if !ok {
		return errors.New("无法设置杠杆，Prv() 必须是合约")
	}

	var opts []model.OptionParameter
	// 安全性检查
	if marginMode != model2.OrderMgnModeIsolated && marginMode != model2.OrderMgnModeCross {
		return fmt.Errorf("不支持的保证金模式: %s", marginMode)
	}

	if marginMode == "isolated" && (posSide != "long" && posSide != "short") {
		return fmt.Errorf("逐仓模式下必须指定 posSide（long 或 short）")
	}

	opts = append(opts, model.OptionParameter{
		Key:   "mgnMode",
		Value: marginMode,
	}, model.OptionParameter{
		Key:   "posSide",
		Value: posSide,
	})
	// posSide
	resp, err := okxPrv.SetLeverage(instId, strconv.Itoa(leverage), opts...)
	if err != nil {
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	fmt.Println("杠杆设置响应:", string(resp))
	return nil
}

// 平仓函数
func (e *FuturesCommon) ClosePosition(symbol string, side string, quantity float64, tdMode string) error {

	// 当传入的是BTC/USDT时，通过CurrencyPair匹配正确的instId
	pair, err := e.toCurrencyPair(symbol)
	if err != nil {
		return err
	}
	var orderSide model.OrderSide

	// 如果是多仓 -> 需要做空（卖）来平仓
	// 如果是空仓 -> 需要做多（买）来平仓
	switch side {
	case "long":
		// 持有多单，平掉多单
		orderSide = model.Futures_CloseBuy
	case "short":
		// 持有空单，平掉空单
		orderSide = model.Futures_CloseSell
	default:
		return fmt.Errorf("unknown side: %s", side)
	}

	opts := []model.OptionParameter{
		model.OptionParameter{
			Key:   "tdMode",
			Value: tdMode,
		},
	}

	// 提交市价平仓订单
	order, resp, err := e.prv.CreateOrder(pair, quantity, 0, orderSide, model.OrderType_Market, opts...)
	if err != nil {
		fmt.Printf("CreateOrder error：%v", resp)
		return err
	}

	fmt.Printf("平仓成功，订单ID：%s\n", order.Id)
	return nil
}

// 查询是否有持仓
func (e *FuturesCommon) GetPosition(symbol string) (long *model2.PositionInfo, short *model2.PositionInfo, err error) {
	positions, err := e.getPosition(symbol)
	if err != nil {
		return nil, nil, err
	}

	for _, pos := range positions {
		// 一般方向字段为 "long" 或 "short"，也可能是 "net"（净持仓模式）
		switch pos.Side {
		case "long":
			if pos.Amount > 0 {
				long = &pos
			}
		case "short":
			if pos.Amount > 0 {
				short = &pos
			}
		}
	}

	return
}

// costUSDT: 你愿意花多少USDT保证金
// leverage: 杠杆倍数
// marketPrice: 标的价格
// ctVal: 每张合约代表多少币，比如BTC=0.01
func CalculateOrderSz(costUSDT float64, leverage int, marketPrice float64, ctVal float64) int {
	// 名义价值 = 保证金 * 杠杆
	notional := costUSDT * float64(leverage)

	// 每张的价值 = 市价 * 合约面值
	oneContractValue := marketPrice * ctVal

	// 计算张数
	sz := int(notional / oneContractValue)
	if sz < 1 {
		sz = 0 // 连1张都开不了
	}
	return sz
}

// costUSDT: 你愿意花多少USDT买
// marketPrice: 市价
func CalculateSpotQty(costUSDT float64, marketPrice float64, precision int) float64 {
	qty := costUSDT / marketPrice
	// 保留交易所允许的小数位
	factor := math.Pow10(precision)
	qty = math.Floor(qty*factor) / factor
	return qty
}

// 合约下单计算：返回 sz(张数) 和 qty(币数量)
func CalculateContractOrder(costUSDT float64, leverage int, marketPrice float64, ctVal float64) (sz float64, qty float64) {
	// 名义价值 = 保证金 * 杠杆
	notional := costUSDT * float64(leverage)

	// 实际币数量
	qty = notional / marketPrice

	// 张数
	sz = qty / ctVal

	//if sz < 1 {
	//	return 0, 0 // 连一张都下不了
	//}

	// 精确的币数量 = 张数 * ctVal
	qty = sz * ctVal

	sz = FloorFloat(sz, 2)
	qty = FloorFloat(qty, 3)
	return
}

// 根据信号等级Level和信号分数Score计算本次下单占仓位的百分比
func CalculatePositionSize(level int) float64 {
	baseSize := 0.2 // 默认基础仓位（0.2 = 25%仓位）

	switch level {
	case 1:
		return 0.30 // 30%，趋势信号，基础仓位
	case 2:
		return baseSize // 20%，确认用
	case 3:
		return 0.15
	default:
		return 0.0
	}
}

// 向下取整保留 n 位小数
func FloorFloat(val float64, n int) float64 {
	factor := math.Pow10(n)
	return math.Floor(val*factor) / factor
}
