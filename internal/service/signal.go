package service

import (
	"context"
	"edgeflow/internal/dao"
	model22 "edgeflow/internal/model"
	"edgeflow/pkg/exchange"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"time"
)

// SignalProcessorService 是集成了所有功能的单体服务
type SignalProcessorService struct {
	signalRepo dao.SignalDao // DB 接口

	ex exchange.Exchange
}

func NewSignalProcessorService(
	signalRepo dao.SignalDao,
	ex exchange.Exchange,
) *SignalProcessorService {
	return &SignalProcessorService{
		signalRepo: signalRepo,
		ex:         ex,
	}
}

func (s *SignalProcessorService) SignalGetList(ctx context.Context) ([]model22.Signal, error) {
	signals, err := s.signalRepo.GetAllActiveSignalList(ctx)
	if err != nil {
		return nil, err
	}

	for i, item := range signals {
		summary, err := s.signalRepo.GetSymbolPerformanceSummary(ctx, item.Symbol)
		if err != nil {
			continue
		}
		item.Summary = summary
		signals[i] = item
	}

	return signals, nil
}

func (s *SignalProcessorService) SignalGetDetail(ctx context.Context, signalID int64) (*model22.SignalDetail, error) {
	detail, err := s.signalRepo.GetSignalDetailByID(ctx, uint(signalID))
	if err != nil {
		return nil, err
	}

	// 192 是48个小时的15分钟k线数量
	start, end := calcKlineTimeRange(detail.Timestamp, 15, 192, time.Now())
	klines, err := s.ex.GetKlineRecords(detail.Symbol, model2.Kline_15min, 192, start, end, model22.OrderTradeSwap, true)
	if err != nil {
		return nil, err
	}
	if err == nil {
		detail.Klines = klines
		if len(klines) >= 2 {
			startTime := klines[0].Timestamp
			endTime := klines[len(klines)-1].Timestamp
			siganls, err := s.signalRepo.GetSignalsByTimeRange(ctx, detail.Symbol, startTime, endTime)
			if err == nil {
				detail.SignalHistories = siganls
			}
		}
	}
	return detail, nil
}

func calcKlineTimeRange(signalTime time.Time, periodMinutes int, count int, latestTime time.Time) (startTimeMs, endTimeMs int64) {
	periodSec := int64(periodMinutes * 60)
	signalUnix := signalTime.Unix()
	latestUnix := latestTime.Unix()

	// 默认：信号居中
	halfCount := count / 2
	startTime := signalUnix - int64(halfCount)*periodSec
	endTime := signalUnix + int64(halfCount)*periodSec

	// ✅ 如果信号靠近最新（比如距离最新时间少于 halfCount 根K线）
	if endTime > latestUnix {
		endTime = latestUnix
		startTime = endTime - int64(count)*periodSec
	}

	// ✅ 防止时间为负（极端情况）
	if startTime < 0 {
		startTime = 0
	}

	startTimeMs = startTime * 1000
	endTimeMs = endTime * 1000
	return
}

func (s *SignalProcessorService) ExecuteOrder(ctx context.Context, signalID int64, ex exchange.Exchange) error {
	// 查询信号
	signal, err := s.signalRepo.GetSignalByID(ctx, uint(signalID))
	if err != nil {
		return err
	}

	var side model22.OrderSide
	switch signal.Command {
	case "BUY", "REVERSAL_BUY":
		side = model22.Buy
	case "SELL", "REVERSAL_SELL":
		side = model22.Sell
	}

	//if req.OrderType == "market" {
	//	// 可考虑调用市场价格作为 fallback
	//	price, err := t.Exchange.GetLastPrice(req.Symbol, tradeType)
	//	if err != nil {
	//		return err
	//	}
	//	req.Price = price
	//}

	order := model22.Order{
		Symbol:      signal.Symbol,
		Side:        side,
		Price:       signal.EntryPrice,
		Quantity:    0,
		OrderType:   model22.Limit,
		TPPrice:     signal.RecommendedTP,
		SLPrice:     signal.RecommendedSL,
		Strategy:    fmt.Sprintf("%v - %v", signal.Symbol, signal.Period),
		Comment:     "",
		TradeType:   model22.OrderTradeSwap,
		MgnMode:     model22.OrderMgnModeIsolated,
		Leverage:    5,
		QuantityPct: 0.2,
		Level:       3,
		Timestamp:   time.Now(),
	}
	_, err = ex.PlaceOrder(ctx, &order)

	if err != nil {
		return err
	}
	return nil
}
