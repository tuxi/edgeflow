package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/exchange"
	"edgeflow/internal/exchange/okx"
	"edgeflow/internal/model"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// 仓位管理，统一的下单服务
type PositionService struct {
	Exchange exchange.Exchange
	d        *dao.OrderDao
}

func NewPositionService(ex exchange.Exchange, d *dao.OrderDao) *PositionService {
	return &PositionService{ex, d}
}

// 平仓
func (ps *PositionService) Close(ctx context.Context, req model.Signal) error {
	tradeType := model.OrderTradeTypeType(req.TradeType)
	if tradeType == "" {
		return errors.New("未知的交易类型，不支持")
	}

	// 清仓
	// 检查是否有仓位
	long, short, err := ps.Exchange.GetPosition(req.Symbol, tradeType)
	if err != nil {
		return err
	}
	var positions []*model.PositionInfo
	if long != nil && long.Amount > 0 {
		positions = append(positions, long)
	}
	if short != nil && short.Amount > 0 {
		positions = append(positions, short)
	}

	for _, item := range positions {
		// 平仓
		log.Printf("平仓: %s %s %f", item.Symbol, item.Side, item.Amount)
		err = ps.Exchange.ClosePosition(item.Symbol, string(item.Side), item.Amount, item.MgnMode, tradeType)
		if err != nil {
			return err
		}
	}

	return nil
}

// 开仓或者加仓
func (t *PositionService) Open(ctx context.Context, req model.Signal, tpPercent, slPercent float64) error {
	tradeType := model.OrderTradeTypeType(req.TradeType)

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

	// 下单总数，由内部计算
	//quantity := 0.7
	// 计算止盈止损
	tpPrice := computeTP(req.Side, req.Price, tpPercent)
	slPrice := computeSL(req.Side, req.Price, slPercent)

	// 根据信号级别和分数计算下单占仓位的比例
	quantityPct := okx.CalculatePositionSize(req.Level, req.Score)
	if quantityPct <= 0 {
		return errors.New("当前仓位占比不足以开仓")
	}

	order := model.Order{
		Symbol:      req.Symbol,
		Side:        side,
		Price:       req.Price,
		Quantity:    0,                              // 开多少数量由后端计算
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
		Timestamp:   req.Timestamp,
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
	log.Printf("[%v] placing order: %+v", "PositionService", order)
	// 调用交易所api下单
	resp, err := t.Exchange.PlaceOrder(ctx, &order)
	if err != nil {
		return err
	}

	// 下单成功，保存订单
	err = t.OrderCreateNew(ctx, order, resp.OrderId)
	return err
}

func (r *PositionService) OrderCreateNew(ctx context.Context, order model.Order, orderId string) error {

	record := &model.OrderRecord{
		OrderId:   orderId,
		Symbol:    order.Symbol,
		CreatedAt: time.Time{},
		Side:      order.Side,
		Price:     order.Price,
		Quantity:  order.Quantity,
		OrderType: order.OrderType,
		TP:        order.TPPrice,
		SL:        order.SLPrice,
		Strategy:  order.Strategy,
		Comment:   order.Comment,
		TradeType: order.TradeType,
		MgnMode:   order.MgnMode,
		Timestamp: order.Timestamp,
		Level:     order.Level,
		Score:     order.Score,
	}
	return r.d.OrderCreateNew(ctx, record)
}

// 计算止盈价
func computeTP(side string, price float64, tpPercent float64) float64 {
	if side == "buy" {
		// TP = 113990 × (1 + 0.005) ≈ 114559.95
		return round(price * (1 + tpPercent/100))
	}
	// SL = 113990 × (1 - 0.003) ≈ 113648.03
	return round(price * (1 - tpPercent/100))
}

// 计算止损价
func computeSL(side string, price float64, slPercent float64) float64 {
	if side == "buy" {
		return round(price * (1 - slPercent/100))
	}
	return round(price * (1 + slPercent/100))
}

func round(val float64) float64 {
	return math.Round(val*100) / 100
}
