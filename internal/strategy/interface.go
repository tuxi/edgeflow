package strategy

import (
	"context"
	"edgeflow/internal/model"
	"math"
)

// 策略执行器接口定义

// 系统“内部”处理用的通用格式
type ExecutionParams struct {
	Symbol    string
	Price     float64
	Side      model.OrderSide
	Comment   string
	Quantity  float64
	TpPercent float64 // 止盈
	SlPercent float64 // 止损
	Payload   any     // 保留原始数据（如WebHook JSON，方便策略回溯）
}

type StrategyExecutor interface {
	Name() string
	Execute(ctx context.Context, req model.Signal) error
	ClosePosition(ctx context.Context, req model.Signal) error
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
