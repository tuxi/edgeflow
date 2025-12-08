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

// 冷却时间设置为 5 分钟
const boundaryCooldown = 5 * time.Minute

// SetBoundaryState 写入状态并设置 TTL
func (r *AlertBoundaryRepository) SetBoundaryState(ctx context.Context, subID string, state model.BoundaryState, isWhipsaw bool) error {
	key := r.getKey(subID)

	pipe := r.rdb.Pipeline()

	// 1. 设置 Hash 字段
	pipe.HMSet(ctx, key, map[string]interface{}{
		"last_boundary": state.LastBoundary,
		"direction":     state.TriggerDirection,
	})

	// 2. 根据是否为反向穿越 (Whipsaw) 设置冷却时间
	if isWhipsaw {
		// 如果是反向穿越，锁定 5 分钟，防止价格快速来回
		pipe.Expire(ctx, key, boundaryCooldown)
	} else {
		// 如果是突破新关口或首次触发，状态应长期有效，不设 TTL
		pipe.Persist(ctx, key) // 确保移除旧的 TTL
	}

	_, err := pipe.Exec(ctx)
	return err
}

// IsKeyInCooldown 检查订阅状态 Key 是否处于冷却期。
// 它通过检查 Key 的剩余生存时间 (TTL) 来判断。
func (r *AlertBoundaryRepository) IsKeyInCooldown(ctx context.Context, subID string) bool {
	key := r.getKey(subID)

	// 使用 TTL 命令获取 Key 的剩余生存时间
	duration, err := r.rdb.TTL(ctx, key).Result()
	if err != nil {
		// 记录错误，但为了系统健壮性，假设此时没有冷却 (返回 false)
		// 实际生产环境中应记录日志
		// log.Printf("Redis TTL error for %s: %v", key, err)
		return false
	}

	// TTL 命令的返回值含义：
	// 1. duration > 0：Key 存在且有剩余 TTL，处于冷却期。
	// 2. duration == -1：Key 存在，但没有设置 TTL (永久存储)。
	// 3. duration == -2：Key 不存在。

	// 只有 duration > 0 时才认为 Key 正在冷却中
	return duration > 0
}
