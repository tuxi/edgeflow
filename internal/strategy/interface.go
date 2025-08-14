package strategy

import (
	"context"
	"edgeflow/internal/model"
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
}
