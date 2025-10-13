package signal

import (
	"context"
	"edgeflow/internal/exchange"
	model22 "edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/internal/service/signal/backtest"
	"edgeflow/internal/service/signal/decision_tree"
	"edgeflow/internal/service/signal/indicator"
	"edgeflow/internal/service/signal/kline"
	"edgeflow/internal/service/signal/model"
	"edgeflow/internal/service/signal/repository"
	"edgeflow/internal/service/signal/trend"
	"encoding/json"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"log"
	"math"
	"sync"
	"time"
)

// SignalProcessorService 是集成了所有功能的单体服务
type SignalProcessorService struct {
	TrendRepo  *trend.Manager              // Redis 接口
	SignalRepo repository.SignalRepository // DB 接口

	signalGen *indicator.SignalGenerator
	DecTree   *decision_tree.DecisionTree

	KlineMgr *kline.KlineManager

	SymbolMgr     *model.SymbolManager
	signalTracker *backtest.SignalTracker
}

func NewService(
	trendRepo *trend.Manager,
	signalRepo repository.SignalRepository,
	KlineMgr *kline.KlineManager,
	symbolMgr *model.SymbolManager,
	signalTracker *backtest.SignalTracker,

) *SignalProcessorService {
	return &SignalProcessorService{
		TrendRepo:     trendRepo,
		SignalRepo:    signalRepo,
		signalGen:     indicator.NewSignalGenerator(model2.Kline_15min),
		DecTree:       decision_tree.NewDecisionTree(2.0, 1.0),
		SymbolMgr:     symbolMgr,
		KlineMgr:      KlineMgr,
		signalTracker: signalTracker,
	}
}

// 接收并监听k线更新通道，驱动信号生成
func (s *SignalProcessorService) ListenForUpdates(ctx context.Context, updateTrendCh <-chan struct{}) {

	// 启动信号处理核心循环
	go s.runSignalLoop(ctx, updateTrendCh)
}

// 现在负责接收事件，并并发处理所有 symbols
func (s *SignalProcessorService) runSignalLoop(ctx context.Context, updateKlineCh <-chan struct{}) {
	fmt.Println("[SignalProcessorService runSignalLoop]启动信号处理器 (监听 K 线更新事件)...")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("信号处理器退出。")
			return
		case <-updateKlineCh:
			// K 线对齐事件触发！

			symbols := s.SymbolMgr.GetSymbols() // 获取当前所有活跃符号
			if len(symbols) == 0 {
				continue
			}

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, 5) // 控制并发数，例如 5 个

			// 循环并并发处理所有交易对的信号生成和过滤
			for _, symbol := range symbols {
				wg.Add(1)
				go func(sym string) {
					defer wg.Done()
					semaphore <- struct{}{}        // 获取信号量
					defer func() { <-semaphore }() // 释放信号量

					s.processSingleSymbol(ctx, sym) // 封装核心逻辑
				}(symbol)
			}
			wg.Wait() // 等待所有符号处理完成
			fmt.Println("本轮信号分析全部完成。")
		}
	}
}

// processSingleSymbol 封装了单个 Symbol 的信号处理逻辑
func (s *SignalProcessorService) processSingleSymbol(ctx context.Context, symbol string) {
	// 核心逻辑：与之前的 runSymbolSignalLoop (Step 1, 2, 3, 4) 相同

	// Step 1: 获取 K线
	klines15m, ok := s.KlineMgr.Get(symbol, s.signalGen.TimeFrame)

	// 处理k线数据，分析信号盈亏
	s.signalTracker.ProcessKlines(symbol, klines15m)

	if !ok || len(klines15m) < 200 {
		fmt.Println("警告：无法获取足够的 15m K线数据，跳过本次信号生成。")
		return
	}
	rawSignal, err := s.signalGen.Generate(symbol, klines15m)
	if err != nil {
		return
	}

	// Step 2: 获取趋势状态
	latestTrendState := s.TrendRepo.GetLatestTrendState(symbol)
	if latestTrendState == nil {
		fmt.Printf("致命错误: 无法获取最新 TrendState。跳过本次信号过滤。\n")
		return // 致命错误日志
	}
	snapshotJson, err := json.Marshal(latestTrendState.IndicatorSnapshot)
	if err != nil {
		log.Printf("failed to marshal indicators: %v", err)
		return
	}

	// Step 3 & 4: 过滤并持久化
	passed, reason := s.DecTree.ApplyFilter(rawSignal, latestTrendState)
	if passed {

		isPremium := false
		if math.Abs(rawSignal.Score) > 1 && math.Abs(latestTrendState.Scores.Score1h) > 2 && math.Abs(latestTrendState.Scores.Score4h) > 2 {
			isPremium = true
		}

		indicatorsJSON, err := json.Marshal(rawSignal.Details.HighFreqIndicators)
		if err != nil {
			log.Printf("failed to marshal indicators: %v", err)
			return
		}

		sg := entity.Signal{
			Symbol:             rawSignal.Symbol,
			Command:            string(rawSignal.Command),
			Timestamp:          rawSignal.Timestamp,
			ExpiryTimestamp:    rawSignal.ExpiryTimestamp,
			Status:             rawSignal.Status,
			FinalScore:         rawSignal.Score,
			Explanation:        rawSignal.Details.BasisExplanation,
			Period:             rawSignal.TimeFrame,
			RecommendedSL:      rawSignal.Details.RecommendedSL,
			RecommendedTP:      rawSignal.Details.RecommendedTP,
			ChartSnapshotURL:   rawSignal.Details.ChartSnapshotURL,
			HighFreqIndicators: string(indicatorsJSON),
			EntryPrice:         rawSignal.EntryPrice,
			MarkPrice:          rawSignal.MarkPrice,
			IsPremium:          isPremium,

			CreatedAt: time.Now(),
			TrendSnapshot: &entity.TrendSnapshot{
				Timestamp:  latestTrendState.Timestamp,
				Direction:  string(latestTrendState.Direction),
				Symbol:     rawSignal.Symbol,
				LastPrice:  latestTrendState.LastPrice,
				Score4h:    latestTrendState.Scores.Score4h,
				Score1h:    latestTrendState.Scores.Score1h,
				Score30m:   latestTrendState.Scores.Score30m,
				FinalScore: latestTrendState.Scores.FinalScore,
				TrendScore: latestTrendState.Scores.TrendScore,
				ATR:        latestTrendState.ATR,
				ADX:        latestTrendState.ADX,
				RSI:        latestTrendState.RSI,
				Indicators: string(snapshotJson),
			},
		}
		err = s.SignalRepo.SaveSignalWithSnapshot(ctx, &sg)
		if err != nil {
			fmt.Printf("【致命错误】信号保存到数据库失败: %v\n", err)
		} else {
			s.signalTracker.AddSignal(&backtest.ActiveSignal{
				ID:              sg.ID,
				Symbol:          sg.Symbol,
				EntryPrice:      sg.EntryPrice,
				EntryTime:       sg.Timestamp,
				IsLong:          sg.Command == "BUY",
				TPPrice:         sg.RecommendedTP,
				SLPrice:         sg.RecommendedSL,
				Klines:          klines15m,
				InitKlinesCount: len(klines15m),
			})

			fmt.Printf("【✅ 信号通过】%s 最终指令：%s。原因：%s\n", symbol, rawSignal.Command, reason)
			// 推送到 MQ
		}
	} else {
		fmt.Printf("【❌ 信号未通过】%s 最终指令：%s。原因：%s\n", symbol, rawSignal.Command, reason)
	}
}

func (s *SignalProcessorService) SignalGetList(ctx context.Context) ([]model22.Signal, error) {
	signals, err := s.SignalRepo.GetAllActiveSignalList(ctx)
	if err != nil {
		return nil, err
	}

	for i, item := range signals {
		summary, err := s.SignalRepo.GetSymbolPerformanceSummary(ctx, item.Symbol)
		if err != nil {
			continue
		}
		item.Summary = summary
		signals[i] = item
	}

	return signals, nil
}

func (s *SignalProcessorService) SignalGetDetail(ctx context.Context, signalID int64) (*model22.SignalDetail, error) {
	detail, err := s.SignalRepo.GetSignalDetailByID(ctx, uint(signalID))
	if err != nil {
		return nil, err
	}

	// 192 是48个小时的15分钟k线数量
	start, end := calcKlineTimeRange(detail.Timestamp, 15, 192, time.Now())
	klines, err := s.KlineMgr.FetchKlines(detail.Symbol, start, end, 192, model2.Kline_15min, model22.OrderTradeSwap)
	if err == nil {
		detail.Klines = klines
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
	signal, err := s.SignalRepo.GetSignalByID(ctx, uint(signalID))
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
