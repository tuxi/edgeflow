package strategy

import "context"

// 策略执行器接口定义

type ExecutionParams struct {
	Symbol  string
	Side    string
	Comment string
	Payload map[string]interface{}
}

type StrategyExecutor interface {
	Name() string
	Execute(ctx context.Context, params ExecutionParams) error
}
