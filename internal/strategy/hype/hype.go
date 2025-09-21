package hype

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/position"
	"edgeflow/internal/signal"
	"edgeflow/internal/trend"
	"edgeflow/pkg/hype/rest"
	"edgeflow/pkg/hype/stream"
	"edgeflow/pkg/hype/types"
	"fmt"

	"log"
	"strconv"
	"sync"
	"time"
)

const (
	// 使用 Hyperliquid 官方 WS
	wsURL = "wss://api.hyperliquid.xyz/ws"

	// 要跟踪的目标地址
	targetAddress = "0x53babe76166eae33c861aeddf9ce89af20311cd0" // 跟单目标地址 jeff lover
)

type HypeTrackStrategy struct {
	mids     map[string]float64
	mu       sync.Mutex
	coins    []string
	ps       *position.PositionService
	trendMgr *trend.Manager
	account  *types.MarginData
}

func NewHypeTrackStrategy(ps *position.PositionService, trendMgr *trend.Manager) *HypeTrackStrategy {
	return &HypeTrackStrategy{ps: ps, trendMgr: trendMgr}
}

func (h *HypeTrackStrategy) Run() {
	h.coins = []string{"BTC", "ETH", "SOL", "DOGE", "HYPE", "LTC", "BNB", "PUMP", "XRP", ""}
	log.Println("[HypeTrackStrategy run] 开启hype交易跟单")
	websocketClient, _ := stream.NewHyperliquidWebsocketClient(wsURL)
	webSocketErr := websocketClient.StreamOrderUpdates(targetAddress)
	if webSocketErr != nil {
		fmt.Println("Error streaming all mids:", webSocketErr)
		return
	}
	websocketClient.StreamAllMids()

	go func() {
		for orders := range websocketClient.OrderChan {
			h.ReceiveOrders(orders)
		}
	}()

	go func() {
		for mids := range websocketClient.AllMidsChan {
			h.mu.Lock()
			h.mids = mids
			h.mu.Unlock()
		}
	}()

	go func() {
		for e := range websocketClient.ErrorChan {
			fmt.Printf("HypeTrackStrategy stream err: %v\n", e)
		}
	}()
}

func (h *HypeTrackStrategy) ReceiveOrders(orders []types.Order) {
	// 获取交易者的仓位
	client, err := rest.NewHyperliquidRestClient("https://api.hyperliquid.xyz", "https://stats-data.hyperliquid.xyz/Mainnet/leaderboard")
	if err != nil {
		log.Printf("初始化HyperliquidRestClient失败：%v", err)
	}
	h.mu.Lock()
	accountSummary, err := client.PerpetualsAccountSummary(targetAddress)
	if err != nil {
		log.Printf("获取交易者仓位失败: %v", err)
		h.mu.Unlock()
		return
	}
	h.account = &accountSummary
	h.mu.Unlock()

	// 生成信号
	signals := h.genSignalForFilledOrder(orders)
	for _, sig := range signals {
		h.runForSignal(sig)
	}
}

// 处理订单更新消息，只提取 filled 转换成 TradeSignal
func (h *HypeTrackStrategy) genSignalForFilledOrder(orders []types.Order) []HypeTradeSignal {

	var signals []HypeTradeSignal
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, order := range orders {
		if order.Status == "filled" {

			// 过滤掉不允许跟单的币种
			allow := false
			for _, coin := range h.coins {
				if coin == order.Order.Coin {
					allow = true
					break
				}
			}
			if allow == false {
				break
			}
			// 转换价格和数量为 float
			// order.LimitPx 限价单价格不对，我们获取最新的价格
			price, ok := h.mids[order.Order.Coin]
			if !ok {
				px, _ := strconv.ParseFloat(order.Order.LimitPx, 64)
				price = px
			}
			//size, _ := strconv.ParseFloat(order.Order.OrigSz, 64) // 用 origSz 表示原始成交量

			// 当前仓位
			var trackPos *types.AssetPosition
			for _, pos := range h.account.AssetPositions {
				if order.Order.Coin == pos.Position.Coin {
					trackPos = &pos
					break
				}
			}
			dir := model.OrderPosSideLong
			if trackPos != nil {
				szi, _ := strconv.ParseFloat(trackPos.Position.Szi, 64)
				if szi <= 0 {
					dir = model.OrderPosSideShort
				}
			}

			sig := HypeTradeSignal{
				Symbol:    fmt.Sprintf("%v/USDT", order.Order.Coin),
				Action:    mapSide(order.Order.Side),
				Price:     price,
				Dir:       dir,
				Timestamp: time.UnixMilli(order.Order.Timestamp),
			}
			signals = append(signals, sig)
		}
	}

	return signals
}

// 为单个交易对运行策略
func (h *HypeTrackStrategy) runForSignal(hypeSig HypeTradeSignal) {
	symbol := hypeSig.Symbol
	defer func() {
		if r := recover(); r != nil {
			log.Printf("交易对 %s 处理出错: %v", hypeSig.Symbol, r)
		}
	}()

	// 1.获取交易所仓位
	long, short, err := h.ps.Exchange.GetPosition(symbol, model.OrderTradeSwap)
	if err != nil {
		log.Printf("[HypeTrackStrategy.runForSymbol] 获取%s我的仓位失败: %v", symbol, err)
		return
	}
	pos := long
	if pos == nil {
		pos = short
	}

	// 2. 获取大趋势
	//state := h.trendMgr.GetState(symbol)
	//if state == nil {
	//	log.Printf("[SignalStrategy] 获取%v大趋势失败\n", symbol)
	//	return
	//}

	log.Printf("开始分析 %s", symbol)

	// 当前仓位
	var trackPos *types.AssetPosition
	for _, pos := range h.account.AssetPositions {
		s := fmt.Sprintf("%v/USDT", pos.Position.Coin)
		if symbol == s {
			trackPos = &pos
			break
		}
	}

	// 组装上下文
	ctx := Context{
		//Trend:    *state,
		Sig:      hypeSig,
		Pos:      pos,
		TrackPos: trackPos,
	}

	// 4. 决策
	action := NewDecisionEngine(ctx).Run()

	switch action {
	case signal.ActIgnore:
		fmt.Printf("[HypeTrackStrategy.run %v: 忽略方向: %v  趋势方向:%v]\n", hypeSig.Symbol, hypeSig.Action, ctx.Trend.Direction.Desc())
	case signal.ActAdd:
		fmt.Printf("[HypeTrackStrategy.run %v: 增加仓位: %v  趋势方向:%v]\n", hypeSig.Symbol, hypeSig.Action, ctx.Trend.Direction.Desc())
	case signal.ActOpen:
		fmt.Printf("[HypeTrackStrategy.run %v: 开仓: %v  趋势方向:%v]\n", hypeSig.Symbol, hypeSig.Action, ctx.Trend.Direction.Desc())
	case signal.ActReduce:
		fmt.Printf("[HypeTrackStrategy.run %v: 减仓: %v  趋势方向:%v]\n", hypeSig.Symbol, hypeSig.Action, ctx.Trend.Direction.Desc())
	case signal.ActClose:
		fmt.Printf("[HypeTrackStrategy.run %v: 平仓: %v  趋势方向:%v]\n", hypeSig.Symbol, hypeSig.Action, ctx.Trend.Direction.Desc())
	}

	var leverage = 20
	if trackPos != nil {
		leverage = trackPos.Position.Leverage.Value
	}
	if ctx.Pos != nil && ctx.Pos.Lever != "" {
		value, _ := strconv.ParseInt(ctx.Pos.Lever, 10, 64)
		leverage = int(value)
	}

	side := model.Sell
	if hypeSig.Dir == model.OrderPosSideLong {
		side = model.Buy
	}
	sig := signal.Signal{
		Strategy:  "hype-track-Engine",
		Symbol:    ctx.Sig.Symbol,
		Price:     ctx.Sig.Price,
		Side:      string(side),
		OrderType: string(model.Market),
		TradeType: "swap",
		Comment:   fmt.Sprintf("hype_track_trade_%v", action),
		Leverage:  leverage,
		Level:     2,
		Meta:      nil,
		Timestamp: time.Now(),
	}

	// 5.执行交易逻辑
	err = h.ps.ApplyAction(context.Background(), action, sig, pos)
	if err != nil {
		log.Printf("[HypeTrackStrategy run] 执行 %s 交易失败: %v", symbol, err)
	}
}

// 辅助函数：把 "B"/"S" 转成 "buy"/"sell"
func mapSide(side string) string {
	if side == "B" {
		return "buy"
	}
	if side == "S" {
		return "sell"
	}
	return "unknown"
}
