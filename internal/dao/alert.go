package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/cache"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
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

	// 触发后的统一更新
	UpdateSubscriptionAfterTrigger(ctx context.Context, id string, isActive bool, lastPrice float64, lastTime time.Time) error

	// 历史记录 (供 AlertService 写入和 App 查询 API 调用)

	// SaveAlertHistory 保存提醒消息历史
	SaveAlertHistory(ctx context.Context, history *entity.AlertHistory) error
	// GetHistoryByUserID 查询用户提醒历史 (用于 App API)
	GetHistoryByUserID(ctx context.Context, userID string, alertType int, limit int, offset int) ([]entity.AlertHistory, error)

	// 查询用户所有订阅
	GetSubscriptionsByUserID(ctx context.Context, userID string) ([]entity.AlertSubscription, error)
	// 更新整个订阅（用于客户端修改价格/百分比）
	UpdateSubscription(ctx context.Context, sub *entity.AlertSubscription) error
	GetSubscriptionByID(ctx context.Context, id string) (entity.AlertSubscription, error)
}

type AlertBoundaryRepository struct {
	rdb *redis.Client
}

func NewAlertBoundaryRepository() *AlertBoundaryRepository {
	return &AlertBoundaryRepository{rdb: cache.GetRedisClient()}
}

// getKey 生成 Redis Key: alert:boundary:SUB_ID
func (r *AlertBoundaryRepository) getKey(subscriptionID string) string {
	return fmt.Sprintf("alert:boundary:%s", subscriptionID)
}

// GetBoundaryState 从 Redis 获取上次的关口状态
func (r *AlertBoundaryRepository) GetBoundaryState(ctx context.Context, subID string) model.BoundaryState {
	key := r.getKey(subID)

	// 使用 HGetAll 获取所有字段
	data, err := r.rdb.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		// 返回默认/零值状态
		return model.BoundaryState{}
	}

	state := model.BoundaryState{}

	// 映射字段 (需要手动解析 float64)
	if lbStr, ok := data["last_boundary"]; ok {
		state.LastBoundary, _ = strconv.ParseFloat(lbStr, 64)
	}
	if direction, ok := data["direction"]; ok {
		state.TriggerDirection = direction
	}

	return state
}

// SetBoundaryState 将最新的关口状态写入 Redis
func (r *AlertBoundaryRepository) SetBoundaryState(ctx context.Context, subID string, state model.BoundaryState) error {
	key := r.getKey(subID)

	// 使用 HMSet 写入 Hash
	return r.rdb.HMSet(ctx, key, map[string]interface{}{
		"last_boundary": state.LastBoundary,
		"direction":     state.TriggerDirection,
	}).Err()
	// ⚠️ 可以在这里设置 TTL (例如 7天)，自动清除极老的、不再活跃的订阅状态
}
