package strategy

//
//import (
//	"context"
//	"edgeflow/internal/exchange"
//	"edgeflow/internal/model"
//	"edgeflow/internal/risk"
//	"errors"
//	"fmt"
//	"log"
//	"strings"
//)
//
//type ExecutorService struct {
//	Exchange exchange.Exchange
//	//recorder *recorder.JSONFileRecorder
//	Rc *risk.RiskControl
//}
//
//func NewExecutorService(ex exchange.Exchange, rc *risk.RiskControl) *ExecutorService {
//	return &ExecutorService{Exchange: ex, Rc: rc}
//}
//
//func (t ExecutorService) Execute(ctx context.Context, req model.Signal) error {
//	var side model.OrderSide
//	switch strings.ToLower(req.Side) {
//	case "buy":
//		side = model.Buy
//	case "sell":
//		side = model.Sell
//	default:
//		return fmt.Errorf("invalid side: %s", req.Side)
//	}
//
//	price := req.Price
//	//if price == 0 {
//	//	// 可考虑调用市场价格作为 fallback
//	//	price, err := t.Exchange.GetLastPrice(req.Symbol)
//	//}
//
//	quantity := req.Quantity
//	if quantity <= 0 {
//		quantity = 0.01 // 默认值
//	}
//
//	// 计算止盈止损
//	tpPrice := 0.0
//	slPrice := 0.0
//	if req.TpPercent > 0 {
//		tpPrice = computeTP(req.Side, price, req.TpPercent)
//	}
//	if req.SlPercent > 0 {
//		slPrice = computeSL(req.Side, price, req.SlPercent)
//	}
//
//	order := model.Order{
//		Symbol:      req.Symbol,
//		Side:        side,
//		Price:       price,
//		Quantity:    quantity,
//		OrderType:   model.OrderType(req.OrderType), // "market" / "limit"
//		Strategy:    req.Strategy,
//		TPPrice:     tpPrice,
//		SLPrice:     slPrice,
//		TradeType:   model.OrderTradeTypeType(req.TradeType),
//		Comment:     req.Comment,
//		Leverage:    req.Leverage,
//		//QuantityPct: req.QuantityPct,
//	}
//
//	// 风控检查，是否允许下单
//	isAllow := t.Rc.Allow(ctx, order)
//	if isAllow == false {
//		return errors.New("触发风控，无法下单，稍后再试")
//	}
//
//	// 检查是否有仓位
//	long, short, err := t.Exchange.GetPosition(req.Symbol, order.TradeType)
//	if err != nil {
//		return err
//	}
//	/*
//		收到 buy 信号时：
//		如果已有多仓，可以选择加仓；
//		如果有空仓，先平空再开多。
//		收到 sell 信号时：
//		同理处理
//	*/
//	var closePs *model.PositionInfo
//	if side == model.Buy && short != nil {
//		closePs = short // 记录需要平的空单
//	} else if side == model.Sell && long != nil {
//		closePs = long // 记录需要平的多单
//	}
//
//	if closePs != nil {
//		// 先平掉逆向的仓位
//		err = t.Exchange.ClosePosition(closePs.Symbol, string(closePs.Side), closePs.Amount, closePs.MgnMode, order.TradeType)
//		if err != nil {
//			return err
//		}
//	}
//
//	// 开仓or加仓
//	log.Printf("[TVScalp15M] placing order: %+v", order)
//	// 调用交易所api下单
//	resp, err := t.Exchange.PlaceOrder(ctx, &order)
//	if err != nil {
//		return err
//	}
//
//	// 下单成功，保存订单
//	err = t.Rc.OrderCreateNew(ctx, order, resp.OrderId)
//	if err != nil {
//		log.Fatalf("创建订单失败:%v", err)
//	}
//
//	return err
//}
