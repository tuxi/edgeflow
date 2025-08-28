package position

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/exchange"
	"edgeflow/internal/exchange/okx"
	"edgeflow/internal/model"
	"edgeflow/internal/signal"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// 本地仓位元信息（逻辑层面的补充）
type LocalPositionMeta struct {
	Symbol     string
	Level      int    // 由哪个信号级别触发 (1/2/3)
	Side       string // buy/sell
	EntryPrice float64
	Size       float64
	OpenTime   time.Time
}

// 仓位管理，统一的下单服务
type PositionService struct {
	Exchange exchange.Exchange
	d        *dao.OrderDao
	mu       sync.Mutex
	//metas    map[string]*LocalPositionMeta // 本地仓位信息
	metas map[string]map[int]*LocalPositionMeta // 本地仓位信息
}

func NewPositionService(ex exchange.Exchange, d *dao.OrderDao) *PositionService {
	return &PositionService{
		Exchange: ex,
		d:        d,
		metas:    make(map[string]map[int]*LocalPositionMeta),
	}
}

// 平仓
func (ps *PositionService) CloseAll(ctx context.Context, req signal.Signal) error {
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

// 平掉某个仓位
func (ps *PositionService) Close(ctx context.Context, state *model.PositionInfo, tradeType string) error {
	if tradeType == "" {
		return errors.New("未知的交易类型，不支持")
	}

	// 平仓
	var positions []*model.PositionInfo
	if state != nil && state.Amount > 0 {
		positions = append(positions, state)
	}

	for _, item := range positions {
		// 平仓
		log.Printf("平仓: %s %s %f", item.Symbol, item.Side, item.Amount)
		err := ps.Exchange.ClosePosition(item.Symbol, string(item.Side), item.Amount, item.MgnMode, model.OrderTradeTypeType(tradeType))
		if err != nil {
			return err
		}
	}

	return nil
}

// 开仓或者加仓
func (t *PositionService) Open(ctx context.Context, req signal.Signal, tpPercent, slPercent float64) error {
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

	// 保存本地元数据
	t.SaveMeta(order.Symbol, order.Level, string(order.Side), order.Price, order.Quantity)

	// 下单成功，保存订单
	err = t.OrderCreateNew(ctx, order, resp.OrderId)
	return err
}

// 获取仓位状态（交易所真实仓位+本地元信息）
func (ps *PositionService) State(sig signal.Signal) (state *model.PositionInfo, meta *LocalPositionMeta, err error) {
	long, short, err := ps.Exchange.GetPosition(sig.Symbol, model.OrderTradeTypeType(sig.TradeType))
	if err != nil || (long == nil && short == nil) {
		return nil, nil, err
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()
	meta = ps.metas[sig.Symbol][sig.Level]
	// 如果没有l2，则把服务端的仓位保存为l2
	meta2 := ps.metas[sig.Symbol][2]
	if long != nil {
		state = long
		if meta2 == nil {
			ps.SaveMeta(sig.Symbol, 2, string(model.Buy), state.AvgPrice, state.Amount)
		}
	}

	if short != nil {
		state = short
		if state != nil {
			if meta2 == nil {
				ps.SaveMeta(sig.Symbol, 2, string(model.Buy), state.AvgPrice, state.Amount)
			}
		}

	}

	return
}

// 记录开仓的元信息（在下单成功后调用）
func (ps *PositionService) SaveMeta(symbol string, level int, side string, entry float64, size float64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	m := make(map[int]*LocalPositionMeta)
	m[level] = &LocalPositionMeta{
		Symbol:     symbol,
		Level:      level,
		Side:       side,
		EntryPrice: entry,
		Size:       size,
		OpenTime:   time.Time{},
	}
	ps.metas[symbol] = m
}

func (ps *PositionService) GetPositionByLevel(symbol string, level int) *LocalPositionMeta {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	m, ok := ps.metas[symbol]
	if !ok {
		return nil
	}
	meta, ok := m[level]
	if ok != false {
		return nil
	}
	return meta
}

// 删除本地仓位信息（平仓时调用）
func (ps *PositionService) ClearMeta(symbol string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.metas, symbol)
}

// 减仓
func (ps *PositionService) reducePosition(ctx context.Context, sig signal.Signal, state *model.PositionInfo) error {
	if state.Amount <= 0 {
		return nil
	}
	reduceQty := state.Amount / 2 // 减半仓位
	state.Amount = reduceQty
	err := ps.Close(ctx, state, sig.TradeType)
	return err
}

func (ps *PositionService) ApplyAction(
	ctx context.Context,
	symbol string,
	action signal.Action,
	sig signal.Signal,
	state *model.PositionInfo,
) error {
	switch action {
	case signal.ActIgnore:
		fmt.Printf("[PositionService.ApplyAction: 忽略信号]")
		return nil
	case signal.ActOpen:
		return ps.Open(ctx, sig, 1, 1)

	case signal.ActAdd:
		return ps.Open(ctx, sig, 1, 1)

	case signal.ActReduce:
		return ps.reducePosition(ctx, sig, state)

	case signal.ActTightenSL:
		return ps.tightenStopLoss(ctx, symbol, sig, state)

	case signal.ActClose:
		return ps.Close(ctx, state, sig.TradeType)

	default:
		return fmt.Errorf("unknown action: %v", action)
	}
}

// 收紧止损
func (ps *PositionService) tightenStopLoss(ctx context.Context, symbol string, sig signal.Signal, state *model.PositionInfo) error {
	// 更新止损价格，而不是直接下单

	//newSL := calcTighterSL(sig.Side, sig.Price, state.AvgPrice,0.8)
	//return ps.Exchange.AmendAlgoOrder(state.Symbol, sig.TradeType, state)
	return nil
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

// calcTighterSL 根据最新价格动态计算收紧止损
// 参数：
//
//	side: "long" or "short"
//	entry: 开仓均价
//	lastPrice: 最新市价
//	lockProfitRatio: 锁定的最低盈利比例（例如 0.3 = 锁住至少30%的浮盈）
func calcTighterSL(side string, entry, lastPrice float64, lockProfitRatio float64) float64 {
	if entry <= 0 || lastPrice <= 0 {
		return 0
	}

	switch side {
	case "long":
		// 浮盈
		profit := lastPrice - entry
		if profit <= 0 {
			// 没有盈利，不移动止损
			return 0
		}
		// 动态止损价 = 开仓价 + 浮盈 * 锁定比例
		return entry + profit*lockProfitRatio

	case "short":
		profit := entry - lastPrice
		if profit <= 0 {
			return 0
		}
		return entry - profit*lockProfitRatio
	}

	return 0
}
