package strategy

import (
	"context"
	"edgeflow/internal/model"
	"fmt"
	"math"
	"strings"
)

// 策略执行器接口定义

// 系统“内部”处理用的通用格式
type ExecutionParams struct {
	Symbol   string
	Price    float64
	Side     model.OrderSide
	Comment  string
	Quantity float64
	Tp       float64 `json:"tp"` // 止盈
	Sl       float64 `json:"sl"` // 止损
	Payload  any     // 保留原始数据（如WebHook JSON，方便策略回溯）
}

type StrategyExecutor interface {
	Name() string
	Execute(ctx context.Context, req model.WebhookRequest) error
}

func ConvertToExecutionParams(req model.WebhookRequest) (ExecutionParams, error) {
	var side model.OrderSide
	switch strings.ToLower(req.Side) {
	case "buy":
		side = model.Buy
	case "sell":
		side = model.Sell
	default:
		return ExecutionParams{}, fmt.Errorf("invalid side: %s", req.Side)
	}

	return ExecutionParams{
		Symbol:   req.Symbol,
		Price:    req.Price,
		Side:     side,
		Quantity: req.Quantity,
		Tp:       req.Tp,
		Sl:       req.Sl,
		Payload:  req,
	}, nil
}

// 计算止盈价
func computeTP(side string, price float64, tpPercent float64) float64 {
	if side == "buy" {
		return round(price * (1 + tpPercent/100))
	}
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
