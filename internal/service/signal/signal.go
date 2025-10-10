package signal

import (
	"context"
	model22 "edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/internal/service/signal/decision_tree"
	"edgeflow/internal/service/signal/generator"
	"edgeflow/internal/service/signal/kline"
	"edgeflow/internal/service/signal/model"
	"edgeflow/internal/service/signal/repository"
	"edgeflow/internal/service/signal/trend"
	"edgeflow/utils/uuid"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"sync"
	"time"
)

// SignalProcessorService 是集成了所有功能的单体服务
type SignalProcessorService struct {
	TrendRepo  *trend.Manager              // Redis 接口
	SignalRepo repository.SignalRepository // DB 接口

	signalGen *generator.Generator
	DecTree   *decision_tree.DecisionTree

	KlineMgr *kline.KlineManager

	SymbolMgr *model.SymbolManager
	iSrv      uuid.SnowNode
}

func NewService(
	trendRepo *trend.Manager,
	signalRepo repository.SignalRepository,
	symbolMgr *model.SymbolManager,
) *SignalProcessorService {
	return &SignalProcessorService{
		TrendRepo:  trendRepo,
		SignalRepo: signalRepo,
		signalGen:  generator.NewGenerator(model2.Kline_15min),
		DecTree:    decision_tree.NewDecisionTree(2.0, 1.0),
		SymbolMgr:  symbolMgr,
		iSrv:       *uuid.NewNode(3),
	}
}

// StartSignalProcessor 接收并监听k线更新通道，驱动信号生成
func (s *SignalProcessorService) RunScheduled(ctx context.Context, updateTrendCh <-chan struct{}) {

	// 启动信号处理核心循环
	go s.runSignalLoop(ctx, updateTrendCh)
}

// 现在负责接收事件，并并发处理所有 symbols
func (s *SignalProcessorService) runSignalLoop(ctx context.Context, updateKlineCh <-chan struct{}) {
	fmt.Println("启动信号处理器 (数组模式，监听 K 线更新事件)...")

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

					s.processSingleSymbol(sym) // 封装核心逻辑
				}(symbol)
			}
			wg.Wait() // 等待所有符号处理完成
			fmt.Println("本轮信号分析全部完成。")
		}
	}
}

// processSingleSymbol 封装了单个 Symbol 的信号处理逻辑
func (s *SignalProcessorService) processSingleSymbol(symbol string) {
	// 核心逻辑：与之前的 runSymbolSignalLoop (Step 1, 2, 3, 4) 相同

	// Step 1: 获取 K线
	klines15m, ok := s.KlineMgr.Get(symbol, s.signalGen.TimeFrame)
	if !ok || len(klines15m) < 200 {
		fmt.Println("警告：无法获取足够的 15m K线数据，跳过本次信号生成。")
		return
	}
	rawSignal, err := s.signalGen.GenerateRawSignal(symbol, klines15m)
	if err != nil {
		return
	}

	// Step 2: 获取趋势状态
	latestTrendState := s.TrendRepo.GetLatestTrendState(symbol)
	if latestTrendState == nil {
		fmt.Printf("致命错误: 无法获取最新 TrendState。跳过本次信号过滤。\n")
		return // 致命错误日志
	}

	iSnapshot := model.TransformSnapshotsToJSONMap(latestTrendState.IndicatorSnapshot)

	// Step 3 & 4: 过滤并持久化
	passed, reason := s.DecTree.ApplyFilter(rawSignal, latestTrendState)
	if passed {

		sg := entity.Signal{
			Symbol:             rawSignal.Symbol,
			Command:            string(rawSignal.Command),
			Timestamp:          rawSignal.Timestamp,
			ExpiryTimestamp:    rawSignal.ExpiryTimestamp,
			Status:             rawSignal.Status,
			FinalScore:         rawSignal.Details.FinalScoreUsed,
			Explanation:        rawSignal.Details.BasisExplanation,
			Period:             rawSignal.TimeFrame,
			RecommendedSL:      rawSignal.Details.RecommendedTP,
			RecommendedTP:      rawSignal.Details.RecommendedSL,
			ChartSnapshotURL:   rawSignal.Details.ChartSnapshotURL,
			HighFreqIndicators: rawSignal.Details.HighFreqIndicators,
			EntryPrice:         rawSignal.EntryPrice,

			CreatedAt: time.Now(),
			TrendSnapshot: &entity.TrendSnapshot{
				Timestamp:  latestTrendState.Timestamp,
				Direction:  string(latestTrendState.Direction),
				LastPrice:  latestTrendState.LastPrice,
				Score4h:    latestTrendState.Scores.Score4h,
				Score1h:    latestTrendState.Scores.Score1h,
				Score30m:   latestTrendState.Scores.Score30m,
				FinalScore: latestTrendState.Scores.FinalScore,
				TrendScore: latestTrendState.Scores.TrendScore,
				ATR:        latestTrendState.ATR,
				ADX:        latestTrendState.ADX,
				RSI:        latestTrendState.RSI,
				Indicators: iSnapshot,
			},
		}
		err = s.SignalRepo.SaveSignalWithSnapshot(&sg)
		if err != nil {
			fmt.Printf("【致命错误】信号保存到数据库失败: %v\n", err)
		} else {
			fmt.Printf("【✅ 信号通过】%s 最终指令：%s。原因：%s\n", symbol, rawSignal.Command, reason)
			// 推送到 MQ
		}
	}
}

func (s *SignalProcessorService) SignalGetList(ctx context.Context) ([]model22.Signal, error) {
	return s.SignalRepo.GetSignalList(ctx)
}
