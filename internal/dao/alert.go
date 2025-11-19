package dao

import (
	"context"
	"edgeflow/internal/model/entity"
)

// AlertDAO 提醒数据访问对象接口
type AlertDAO interface {
	// 订阅管理 (供 AlertService 初始化和 API 增删改查)

	// GetAllActiveSubscriptions 获取所有活跃订阅（用于 AlertService 初始化内存）
	GetAllActiveSubscriptions(ctx context.Context) ([]entity.AlertSubscription, error)
	// GetSubscriptionsByInstID 查询某个币种的所有订阅
	GetSubscriptionsByInstID(ctx context.Context, instID string) ([]entity.AlertSubscription, error)
	// CreateSubscription 创建新的订阅
	CreateSubscription(ctx context.Context, sub *entity.AlertSubscription) error
	// DeleteSubscription 删除订阅
	DeleteSubscription(ctx context.Context, id string) error

	// 状态更新 (供 AlertService 在收到 MDS 通知时调用)

	// UpdateSubscriptionState 更新订阅状态 (IsActive, LastTriggeredPrice)
	UpdateSubscriptionState(ctx context.Context, id string, isActive bool, lastPrice float64) error

	// 历史记录 (供 AlertService 写入和 App 查询 API 调用)

	// SaveAlertHistory 保存提醒消息历史
	SaveAlertHistory(ctx context.Context, history *entity.AlertHistory) error
	// GetHistoryByUserID 查询用户提醒历史 (用于 App API)
	GetHistoryByUserID(ctx context.Context, userID string, limit int, offset int) ([]entity.AlertHistory, error)

	// 查询用户所有订阅
	GetSubscriptionsByUserID(ctx context.Context, userID string) ([]entity.AlertSubscription, error)
	// 更新整个订阅（用于客户端修改价格/百分比）
	UpdateSubscription(ctx context.Context, sub *entity.AlertSubscription) error
}
