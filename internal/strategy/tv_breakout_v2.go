package strategy

import (
	"context"
	"edgeflow/internal/exchange"
	"edgeflow/internal/exchange/okx"
	"edgeflow/internal/model"
	"edgeflow/internal/risk"
	"errors"
	"fmt"
	"log"
	"strings"
)

type TVBreakoutV2 struct {
	Exchange exchange.Exchange
	//recorder *recorder.JSONFileRecorder
	Rc *risk.RiskControl
}

func NewTVBreakoutV2(ex exchange.Exchange, rc *risk.RiskControl) *TVBreakoutV2 {
	return &TVBreakoutV2{Exchange: ex, Rc: rc}
}

func (t TVBreakoutV2) Name() string {
	return "tv-breakout-v2"
}

func (t TVBreakoutV2) ClosePosition(ctx context.Context, req model.Signal) error {
	tradeType := model.OrderTradeTypeType(req.TradeType)
	if tradeType == "" {
		return errors.New("未知的交易类型，不支持")
	}

	// 清仓
	// 检查是否有仓位
	long, short, err := t.Exchange.GetPosition(req.Symbol, tradeType)
	if err != nil {
		return err
	}
	var positions []*model.PositionInfo
	if long != nil {
		positions = append(positions, long)
	}
	if short != nil {
		positions = append(positions, short)
	}

	for _, item := range positions {
		// 清仓
		err = t.Exchange.ClosePosition(item.Symbol, string(item.Side), item.Amount, item.MgnMode, tradeType)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t TVBreakoutV2) Execute(ctx context.Context, req model.Signal) error {
	tradeType := model.OrderTradeTypeType(req.TradeType)
	if tradeType == "" {
		return errors.New("未知的交易类型，不支持")
	}

	var side model.OrderSide
	switch strings.ToLower(req.Side) {
	case "buy":
		side = model.Buy
	case "sell":
		side = model.Sell
	default:
		return fmt.Errorf("invalid side: %s", req.Side)
	}

	if req.OrderType == "market" {
		// 可考虑调用市场价格作为 fallback
		price, err := t.Exchange.GetLastPrice(req.Symbol, tradeType)
		if err != nil {
			return err
		}
		req.Price = price
	}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 0.01 // 默认值
	}

	// 计算止盈止损
	tpPrice := 0.0
	slPrice := 0.0
	if req.TpPercent > 0 {
		tpPrice = computeTP(req.Side, req.Price, req.TpPercent)
	}
	if req.SlPercent > 0 {
		slPrice = computeSL(req.Side, req.Price, req.SlPercent)
	}

	// 根据信号级别和分数计算下单占仓位的比例
	quantityPct := okx.CalculatePositionSize(req.Level, req.Score)
	if quantityPct <= 0 {
		return errors.New("当前仓位占比不足以开仓")
	}

	order := model.Order{
		Symbol:      req.Symbol,
		Side:        side,
		Price:       req.Price,
		Quantity:    quantity,
		OrderType:   model.OrderType(req.OrderType), // "market" / "limit"
		Strategy:    req.Strategy,
		TPPrice:     tpPrice,
		SLPrice:     slPrice,
		TradeType:   tradeType,
		Comment:     req.Comment,
		Leverage:    req.Leverage,
		QuantityPct: quantityPct,
		Level:       req.Level,
		Score:       req.Score,
	}

	// 风控检查，是否允许下单
	isAllow := t.Rc.Allow(ctx, order)
	if isAllow == false {
		return errors.New("触发风控，无法下单，稍后再试")
	}

	// 检查是否有仓位
	long, short, err := t.Exchange.GetPosition(req.Symbol, order.TradeType)
	if err != nil {
		return err
	}
	/*
		收到 buy 信号时：
		如果已有多仓，可以选择加仓；
		如果有空仓，先平空再开多。
		收到 sell 信号时：
		同理处理
	*/
	var closePs *model.PositionInfo
	if side == model.Buy && short != nil {
		closePs = short // 记录需要平的空单
	} else if side == model.Sell && long != nil {
		closePs = long // 记录需要平的多单
	}

	if closePs != nil {
		// 先平掉逆向的仓位
		err = t.Exchange.ClosePosition(closePs.Symbol, string(closePs.Side), closePs.Amount, closePs.MgnMode, order.TradeType)
		if err != nil {
			return err
		}
	}

	// 开仓or加仓
	log.Printf("[TVBreakoutV2] placing order: %+v", order)
	// 调用交易所api下单
	resp, err := t.Exchange.PlaceOrder(ctx, &order)
	if err != nil {
		return err
	}

	// 下单成功，保存订单
	err = t.Rc.OrderCreateNew(ctx, order, resp.OrderId)
	if err != nil {
		log.Fatalf("创建订单失败:%v", err)
	}

	return err
}
