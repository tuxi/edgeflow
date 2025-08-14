package risk

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"errors"
	"time"
)

// 用于风控系统
type RiskControl struct {
	dao *dao.OrderDao
	// 允许下单的时间间隔
	interval time.Duration
}

func NewRiskControl(dao *dao.OrderDao) *RiskControl {
	return &RiskControl{
		dao:      dao,
		interval: time.Millisecond * 5,
	}
}

func (r *RiskControl) OrderCreateNew(ctx context.Context, order model.Order, orderId string) error {

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
	return r.dao.OrderCreateNew(ctx, record)
}

// 是否允许下单
func (r *RiskControl) Allow(ctx context.Context, strategy, symbol, side, tradeType string) error {

	if tradeType == "" ||
		(tradeType != string(model.OrderTradeSwap) &&
			tradeType != string(model.OrderTradeFutures) &&
			tradeType != string(model.OrderTradeSpot)) {
		return errors.New("未知的交易类型，不支持")
	}

	// 查找同策略的下单状况，如果在5分钟内已下单，则不允许下单
	record, err := r.dao.OrderGetLast(ctx, strategy, symbol, side, tradeType)

	if err != nil {
		return err
	}

	// 1. 从缓存中读取该 uniqueStr 上次记录时间
	if time.Since(record.CreatedAt) < r.interval {
		// 小于设定的时间间隔，不允许重复下单
		return errors.New("小于设定的时间间隔，不允许重复下单")
	}

	// 2. 可以允许下单（但此时还未记录）
	return nil
}
