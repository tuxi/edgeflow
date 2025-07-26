package strategy

import (
	"context"
	"edgeflow/internal/model"
	"fmt"
)

// 实现一个策略
type TVBreakoutV1 struct{}

func (s *TVBreakoutV1) Name() string {
	return "tv-breakout-v1"
}

func (s *TVBreakoutV1) Execute(ctx context.Context, req model.WebhookRequest) error {
	// 实际策略逻辑，这里先打印
	fmt.Printf("执行策略 %s：%s %s - %s\n", s.Name(), req.Side, req.Symbol, req.Price)
	return nil
}
