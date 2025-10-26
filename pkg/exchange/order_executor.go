package exchange

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/pkg/account"
	model2 "github.com/nntaoli-project/goex/v2/model"
)

type Exchange interface {
	// 下单
	PlaceOrder(ctx context.Context, order *model.Order) (*model.OrderResponse, error)
	// 获取最新价格
	GetLastPrice(symbol string, tradingType model.OrderTradeType) (float64, error)
	// 撤销订单
	CancelOrder(orderID string, symbol string, tradingType model.OrderTradeType) error
	// 获取订单状态
	GetOrderStatus(orderID string, symbol string, tradingType model.OrderTradeType) (*model.OrderStatus, error)
	// 获取仓位
	GetPosition(symbol string, tradeType model.OrderTradeType) (long *model.PositionInfo, short *model.PositionInfo, err error)
	// 平仓
	ClosePosition(symbol string, side string, quantity float64, tdMode string, tradeType model.OrderTradeType) error
	Account(tradeType model.OrderTradeType) (Account, error)
	AmendAlgoOrder(instId string, tradeType model.OrderTradeType, algoId string, newSlTriggerPx, newTpTriggerPx float64) ([]byte, error)
	/*
		获取k线数据
		比如取 BTC-USDT 永续合约 1小时K线，顺序是从新到旧
		klines, err := okx.GetKlineRecords(
		    model.BTC_USDT,   // 交易对
		    model.KLINE_PERIOD_1H, // 1小时
		    200,              // 返回200根
		    0,                // since=0 表示最新
		)
	*/
	GetKlineRecords(symbol string, period model2.KlinePeriod, size int, start, end int64, tradeType model.OrderTradeType, includeUnclosed bool) ([]model.Kline, error)
}

// Account 账号结构接口
type Account interface {
	// 返回可用 USDT 余额
	GetAccount(ctx context.Context, coin string) (account *account.Account, err error)
}
