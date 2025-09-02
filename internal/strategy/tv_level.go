package strategy

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/position"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"
)

// 趋势/波段策略：基于 1H 周期，减少频繁进出，拉大止盈止损，追求稳定的中等收益率
type TVLevelStrategy struct {
	signalManager  signal.Manager
	positionSvc    *position.PositionService
	trend          *trend.Manager
	symbolsWatcher []string // 需要定时检查盈利状态的币
	mu             sync.Mutex
	ticker         *time.Ticker
}

// 你可以把这些参数抽成配置
const (
	checkInterval   = 2 * time.Minute  // 定时检查间隔1分钟
	maxHoldDuration = 35 * time.Minute // 最长持仓时间
	takeProfit      = 0.03             // 达到 +3% 强制止盈
	stopLoss        = -0.05            // 达到 -5% 强制止损
)

func NewTVLevelStrategy(sm signal.Manager,
	ps *position.PositionService, trend *trend.Manager) *TVLevelStrategy {
	tv := &TVLevelStrategy{
		signalManager:  sm,
		positionSvc:    ps,
		trend:          trend,
		symbolsWatcher: []string{"BTC/USDT"},
	}
	// 检查盈亏状态
	tv.startPnLWatcher()
	return tv
}

func (t *TVLevelStrategy) Name() string {
	return "tv-level"
}

func (t *TVLevelStrategy) Execute(ctx context.Context, sig signal.Signal) error {

	state, _, err := t.positionSvc.State(sig)
	if err != nil {
		return err
	}
	metaL2 := t.positionSvc.GetPositionByLevel(sig.Symbol, 2)
	if metaL2 != nil {
		lastSig := t.signalManager.GetLastSignal(sig.Symbol, 2)
		if lastSig == nil {
			// 当服务端有l2仓位，本地没有信号缓存时，应该是服务重启了，数据丢失了，此时我们补全即可
			t.signalManager.Save(signal.Signal{
				Strategy:  sig.Strategy,
				Symbol:    sig.Symbol,
				Price:     sig.Price,
				Side:      metaL2.Side,
				OrderType: sig.OrderType,
				TradeType: sig.TradeType,
				Comment:   "后补信号",
				Leverage:  20,
				Level:     2,
				Timestamp: time.Now(),
			})
		}
	}
	upnl := 0.0
	if state != nil {
		upnl, _ = strconv.ParseFloat(state.UnrealizedPnl, 64)
	}
	entryPrice := 0.0
	if metaL2 != nil {
		entryPrice = metaL2.EntryPrice
	}

	// 获取当前币的趋势
	st, _ := t.trend.Get(sig.Symbol)

	dCtx := signal.DecisionContext{
		HasL2Position: metaL2 != nil,
		L2Entry:       entryPrice,
		UnrealizedR:   upnl, // 从交易所仓位算
		TrendDir:      st.Direction,
		StrongM15:     false,
	}

	desc := t.signalManager.Decide(sig, dCtx)

	err = t.positionSvc.ApplyAction(ctx, desc.Action, sig, state)
	if err != nil {
		log.Printf("Execute error: %v", err)
	}
	if desc.Action == signal.ActIgnore {
		return errors.New(desc.Reason)
	}

	t.mu.Lock()
	var hasWatcher = false
	for _, symbol := range t.symbolsWatcher {
		if symbol == sig.Symbol {
			hasWatcher = true
			break
		}
	}

	if !hasWatcher {
		t.symbolsWatcher = append(t.symbolsWatcher, sig.Symbol)
	}

	t.mu.Unlock()

	return err
}

// 定时检查盈利情况，防止系统的止盈止损太高未被触发
func (tv *TVLevelStrategy) startPnLWatcher() {
	if tv.ticker != nil {
		return
	}
	ticker := time.NewTicker(checkInterval)

	go func() {
		for range ticker.C {
			tv.checkPnL()
		}
	}()
}

func (tv *TVLevelStrategy) checkPnL() {

	tv.mu.Lock()
	defer tv.mu.Unlock()
	for _, symbol := range tv.symbolsWatcher {
		long, short, err := tv.positionSvc.Exchange.GetPosition(symbol, model.OrderTradeSwap)
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
					go tv.positionSvc.Close(context.Background(), pos, model.OrderTradeSwap) // 异步平仓，避免阻塞
				}
				//else if uplRatio <= stopLoss {
				//	log.Printf("[%s] 仓位超过50分钟, 亏损%.5f%% 强制止损\n", pos.Symbol, uplRatio*100)
				//	go tv.positionSvc.Close(context.Background(), pos, model.OrderTradeSwap) // 异步平仓，避免阻塞
				//} else {
				//	log.Printf("[%s] 仓位超过50分钟, 但盈亏比 %.2f%% 未达条件, 暂不处理\n", pos.Symbol, uplRatio*100)
				//}
			}
		}

		time.Sleep(time.Second * 5)

	}

}
