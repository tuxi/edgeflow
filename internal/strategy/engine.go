package strategy

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/position"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"fmt"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"strconv"
	"sync"

	"log"
	"time"
)

type StrategyEngine struct {
	trendMgr     *trend.Manager
	signalGen    *trend.SignalGenerator
	ps           *position.PositionService
	Signals      map[string]*trend.Signal // 交易完成的信号
	mu           sync.Mutex
	isTesting    bool
	tradeLimiter *signal.TradeLimiter
	kLineManager *trend.KlineManager
}

// 你可以把这些参数抽成配置
const (
	checkInterval   = 15 * time.Minute // 定时检查间隔15分钟
	maxHoldDuration = 5 * time.Hour    // 最长持仓时间
	takeProfit      = 0.2              // 达到 +20% 强制止盈
	stopLoss        = -0.15            // 达到 -15% 强制止损
)

func NewStrategyEngine(trendMgr *trend.Manager, signalGen *trend.SignalGenerator, ps *position.PositionService, kLineManager *trend.KlineManager) *StrategyEngine {
	config := signal.TradeLimiterConfig{
		MaxConsecutiveOpens: 2,                // 最多连续开仓2次
		MaxConsecutiveAdds:  2,                // 最多连续加仓2次
		OpenCooldownPeriod:  30 * time.Minute, // 开仓冷却30分钟
		AddCooldownPeriod:   15 * time.Minute, // 加仓冷却15分钟
		MaxOpensPerDay:      8,                // 每日最多开仓6次
		MaxAddsPerDay:       16,               // 每日最多加仓10次
	}
	se := &StrategyEngine{
		trendMgr:     trendMgr,
		signalGen:    signalGen,
		ps:           ps,
		Signals:      make(map[string]*trend.Signal),
		kLineManager: kLineManager,
		tradeLimiter: signal.NewTradeLimiter(config),
	}
	if se.isTesting {
		return se
	}
	// 检查盈亏状态
	se.startPnLWatcher()
	return se
}

func (se *StrategyEngine) Run(symbols []string) {
	// 初始化信号存储
	se.mu.Lock()

	if se.Signals == nil {
		se.Signals = map[string]*trend.Signal{}
		for _, symbol := range symbols {
			se.Signals[symbol] = nil
		}
	}
	defer se.mu.Unlock()

	// 启动策略引擎
	go se.runScheduled(symbols)
}

// 启动策略引擎
func (se *StrategyEngine) runScheduled(symbols []string) {

	// ✅ 立即执行一次，确保不会错过刚结束的K线
	log.Printf("5分钟K线完成，开始分析信号... 时间: %s", time.Now().Format("15:04:05"))
	se.runAllSymbols(symbols)
}

// 为所有交易对运行策略
func (se *StrategyEngine) runAllSymbols(symbols []string) {
	var wg sync.WaitGroup

	// 并发处理多个交易对，但限制并发数
	semaphore := make(chan struct{}, 3) // 最多3个并发

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()

			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			se.runForSymbol(sym)
		}(symbol)
	}

	wg.Wait()
	log.Println("本轮信号分析完成")
}
func (se *StrategyEngine) _run() {
	for k, _ := range se.Signals {
		se.runForSymbol(k)
		time.Sleep(time.Second * 5)
	}
}

// 为单个交易对运行策略
func (se *StrategyEngine) runForSymbol(symbol string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("交易对 %s 处理出错: %v", symbol, r)
		}
	}()

	// 0.获取交易所仓位
	long, short, err := se.ps.Exchange.GetPosition(symbol, model.OrderTradeSwap)
	if err != nil {
		log.Printf("[StrategyEngine.runForSymbol] 获取%s仓位失败: %v", symbol, err)
		return
	}
	pos := long
	if pos == nil {
		pos = short
	}

	// 1. 获取大趋势
	state := se.trendMgr.GetState(symbol)
	if state == nil {
		log.Printf("[StrategyEngine] 获取%v大趋势失败\n", symbol)
		return
	}

	log.Printf("开始分析 %s", symbol)

	// 2.获取完整的K线数据（不包含未完成的K线）
	timeframeData, ok := se.kLineManager.Get(symbol, model2.Kline_15min)
	if ok == false {
		log.Printf("[StrategyEngine] 获取%s K线数据失败", symbol)
		return
	}

	// 验证K线数据质量
	if len(timeframeData) < 200 {
		log.Printf("[StrategyEngine] %s K线数据不足，需要至少200根，当前:%d", symbol, len(timeframeData))
		return
	}

	// 3.分析信号
	sig15, err := se.signalGen.Generate(timeframeData, symbol)
	if err != nil {
		log.Printf("[StrategyEngine] %s 信号生成失败: %v", symbol, err)
		return
	}
	if sig15 == nil {
		log.Printf("[StrategyEngine] %s 未生成有效信号", symbol)
		return
	}

	lastLine := timeframeData[len(timeframeData)-1]
	// 组装上下文
	ctx := signal.Context{
		Trend:   *state,
		Sig:     *sig15,
		Pos:     pos,
		LastSig: se.Signals[symbol],
		Line:    lastLine,
	}

	// 4. 决策
	action := signal.NewDecisionEngine(ctx).Run()

	// 检查是否可以执行交易
	canExecute := false
	switch action {
	case signal.ActOpen:
		canExecute = se.tradeLimiter.CanOpen(symbol)
		if !canExecute {
			log.Printf("[StrategyEngine] %s 开仓被限制器阻止", symbol)
		}
	case signal.ActAdd:
		canExecute = se.tradeLimiter.CanAdd(symbol)
		if !canExecute {
			log.Printf("[StrategyEngine] %s 加仓被限制器阻止", symbol)
		}
	case signal.ActIgnore:
		fmt.Printf("[StrategyEngine.run %v: 忽略信号方向: %v  强度:%.2f 趋势方向:%v]\n", sig15.Symbol, sig15.Side, sig15.Strength, ctx.Trend.Direction.Desc())
	default:
		canExecute = true // 其他动作不受限制
	}

	if !canExecute {
		return // 被限制器阻止，不执行交易
	}

	var leverage int64 = 50
	if ctx.Pos != nil && ctx.Pos.Lever != "" {
		leverage, _ = strconv.ParseInt(ctx.Pos.Lever, 10, 64)
	} else {
		// 对于非主流币种使用较低杠杆
		if symbol != "BTC/USDT" && symbol != "ETH/USDT" && symbol != "SOL/USDT" {
			leverage = 20
		}
	}

	sig := signal.Signal{
		Strategy:  "auto-Strategy-Engine",
		Symbol:    ctx.Sig.Symbol,
		Price:     ctx.Sig.Price,
		Side:      ctx.Sig.Side,
		OrderType: string(model.Limit),
		TradeType: "swap",
		Comment:   fmt.Sprintf("15min_complete_kline_action_%s", action), // 标记使用完整K线
		Leverage:  int(leverage),
		Level:     2,
		Meta:      nil,
		Timestamp: time.Now(),
	}

	// 更新信号
	se.mu.Lock()
	se.Signals[symbol] = sig15
	se.mu.Unlock()

	if se.isTesting {
		return
	}
	// 5.执行交易逻辑
	err = se.ps.ApplyAction(context.Background(), action, sig, pos)
	if err != nil {
		log.Printf("[StrategyEngine] 执行 %s 交易失败: %v", symbol, err)
	} else {
		switch action {
		case signal.ActOpen:
			se.tradeLimiter.RecordTrade(symbol, action, sig.Price, sig.Side)
		case signal.ActAdd:
			se.tradeLimiter.RecordTrade(symbol, action, sig.Price, sig.Side)
		case signal.ActClose:
			se.tradeLimiter.RecordClose(symbol, sig.Price, sig.Side)
		}
	}
}

// 定时检查盈利情况，防止系统的止盈止损太高未被触发
func (tv *StrategyEngine) startPnLWatcher() {
	ticker := time.NewTicker(checkInterval)

	go func() {
		for range ticker.C {
			tv.checkPnL()
		}
	}()
}

func (tv *StrategyEngine) checkPnL() {

	tv.mu.Lock()
	defer tv.mu.Unlock()
	for symbol, _ := range tv.Signals {
		long, short, err := tv.ps.Exchange.GetPosition(symbol, model.OrderTradeSwap)
		if err != nil {
			continue
		}

		var positions []*model.PositionInfo
		if long != nil {
			positions = append(positions, long)
		}
		if short != nil {
			positions = append(positions, short)
		}

		for _, pos := range positions {
			openTimeMs, _ := strconv.ParseInt(pos.CTime, 10, 64)
			openTime := time.UnixMilli(openTimeMs)
			holdDuration := time.Now().Sub(openTime)

			// 转 float
			uplRatio, _ := strconv.ParseFloat(pos.UplRatio, 64)

			// 持仓时间超过最大时间
			if holdDuration > maxHoldDuration {
				// 仓位开超过半小时，检查盈亏比
				if uplRatio >= takeProfit {
					log.Printf("[%s] 仓位超过35分钟, 盈利%.3f%% 强制止盈\n", pos.Symbol, uplRatio*100)
					go tv.ps.Close(context.Background(), pos, model.OrderTradeSwap) // 异步平仓，避免阻塞
				} else if uplRatio <= stopLoss {
					log.Printf("[%s] 仓位超过50分钟, 亏损%.5f%% 强制止损\n", pos.Symbol, uplRatio*100)
					go tv.ps.Close(context.Background(), pos, model.OrderTradeSwap) // 异步平仓，避免阻塞
				} else {
					log.Printf("[%s] 仓位超过50分钟, 但盈亏比 %.2f%% 未达条件, 暂不处理\n", pos.Symbol, uplRatio*100)
				}
			}
		}

		time.Sleep(time.Second * 5)

	}

}

// 等K线完成再交易
func (t *StrategyEngine) waitForKlineCompletion() {
	now := time.Now()

	// 计算下一个15分钟K线完成时间
	next15min := now.Truncate(5 * time.Minute).Add(5 * time.Minute)

	// 等到K线完成后再检查信号
	time.Sleep(time.Until(next15min.Add(30 * time.Second))) // 多等30秒确保数据更新
}
