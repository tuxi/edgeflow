package exchange

import (
	"context"
	"edgeflow/internal/model"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"sync"
	"time"
)

// 模拟下单
type SimulatedOrderExecutor struct {
	// 用一个 map 存储所有订单记录，根据订单id存储订单状态
	orders map[string]*model.OrderStatus
	mu     sync.Mutex
	prices map[string]float64
}

func NewSimulatedOrderExecutor() *SimulatedOrderExecutor {
	return &SimulatedOrderExecutor{
		orders: make(map[string]*model.OrderStatus),
		mu:     sync.Mutex{},
		prices: make(map[string]float64),
	}
}

// 设置初始价格
func (s *SimulatedOrderExecutor) SetInitialPrice(symbol string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[symbol] = price
}

func (s *SimulatedOrderExecutor) PlaceOrder(ctx context.Context, req *model.Order) (*model.OrderResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建订单id
	orderID := uuid.NewString()
	status := &model.OrderStatus{
		OrderID:   orderID,
		Status:    "pendding", // 模拟立即成交
		Filled:    req.Price,
		Remaining: 0,
	}

	s.orders[orderID] = status

	return &model.OrderResponse{
		OrderId: orderID,
		Status:  1,
		Message: "Simulated order filled",
	}, nil
}

func (s *SimulatedOrderExecutor) CancelOrder(orderID string, symbol string, tradingType model.OrderTradeTypeType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.orders[orderID]; !ok {
		return fmt.Errorf("Order not found")
	}

	delete(s.orders, orderID)
	return nil
}

func (s *SimulatedOrderExecutor) GetOrderStatus(orderID string, symbol string, tradingType model.OrderTradeTypeType) (*model.OrderStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.orders[orderID]
	if !ok {
		return status, fmt.Errorf("Order not found")
	}
	return status, nil
}

// 模拟版，返回本地价格并做小幅浮动， 适合本地联调
func (s *SimulatedOrderExecutor) GetLastPrice(symbol string, tradingType model.OrderTradeTypeType) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	price, ok := s.prices[symbol]
	if !ok {
		// 如果没有初始化，随机一个价格并记录
		rand.Seed(time.Now().UnixNano())
		price = 10000 + rand.Float64()*2000 // e.g., $10000 ~ $12000
		s.prices[symbol] = price
	}

	// 模拟价格波动 ±0.5%
	fluctuation := (rand.Float64()*0.01 - 0.005) * price
	price += fluctuation
	s.prices[symbol] = price

	return price, nil
}
