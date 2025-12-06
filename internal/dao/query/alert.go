package query

import (
	"context"
	"database/sql"
	"edgeflow/internal/dao"
	"edgeflow/internal/model/entity"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AlertDAOImpl Gorm 实现
type AlertDAOImpl struct {
	db *gorm.DB
}

// NewAlertDAO 创建 DAO 实例
func NewAlertDAO(db *gorm.DB) dao.AlertDAO {
	return &AlertDAOImpl{db: db}
}

// --- 订阅管理实现 ---

// GetAllActiveSubscriptions 获取所有活跃订阅（用于 AlertService 初始化内存）
func (d *AlertDAOImpl) GetAllActiveSubscriptions(ctx context.Context) ([]entity.AlertSubscription, error) {
	var subs []entity.AlertSubscription
	if err := d.db.WithContext(ctx).Where("is_active = ?", true).Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("failed to get active subscriptions: %w", err)
	}
	return subs, nil
}

// CreateSubscription 创建新的订阅
func (d *AlertDAOImpl) CreateSubscription(ctx context.Context, sub *entity.AlertSubscription) error {
	if sub.ID == "" {
		return errors.New("subscription ID is required")
	}
	return d.db.WithContext(ctx).Create(sub).Error
}

// DeleteSubscription 删除订阅
func (d *AlertDAOImpl) DeleteSubscription(ctx context.Context, id string) error {
	return d.db.WithContext(ctx).Delete(&entity.AlertSubscription{}, id).Error
}

// --- 状态更新实现 ---

// UpdateSubscriptionAfterTrigger 触发后的统一更新
func (d *AlertDAOImpl) UpdateSubscriptionAfterTrigger(ctx context.Context, id string, isActive bool, lastPrice float64, lastTime time.Time) error {
	// 结构体用于部分更新
	updates := map[string]interface{}{
		"is_active":  isActive,
		"updated_at": time.Now(),
	}
	// 仅在价格 > 0 时更新 LastTriggeredPrice 字段，否则 Gorm 会更新为 NULL
	if lastPrice > 0 {
		updates["last_triggered_price"] = lastPrice
	} else {
		// 如果重置，显式设置为 NULL (sql.NullFloat64 对应)
		updates["last_triggered_price"] = sql.NullFloat64{Valid: false}
	}
	// 🚀 智能处理时间：
	// 如果传入了有效时间（非零值），则更新时间（触发场景）。
	// 如果是重置场景，我们通常希望保留“上次触发时间”作为历史参考，
	// 所以这里我们可以选择：如果不传时间就不更新，或者总是更新。
	// 既然方法名叫 AfterTrigger，通常意味着“最近一次相关变动的时间”。
	// 针对您的需求，如果传入的时间是零值，我们可以不更新该字段。
	if !lastTime.IsZero() {
		updates["last_triggered_time"] = lastTime
	}
	return d.db.WithContext(ctx).Model(&entity.AlertSubscription{}).Where("id = ?", id).Updates(updates).Error
}

// --- 历史记录实现 ---

// SaveAlertHistory 保存提醒消息历史
func (d *AlertDAOImpl) SaveAlertHistory(ctx context.Context, history *entity.AlertHistory) error {
	// 确保 Timestamp 存在，便于查询
	if history.Timestamp == 0 {
		history.Timestamp = time.Now().UnixMilli()
	}
	return d.db.WithContext(ctx).Create(history).Error
}

// GetHistoryByUserID 查询用户提醒历史 (用于 App API)
func (d *AlertDAOImpl) GetHistoryByUserID(ctx context.Context, userID string, alertTYpe int, limit int, offset int) ([]entity.AlertHistory, error) {
	var history []entity.AlertHistory
	// 按时间倒序排序
	q := d.db.WithContext(ctx).Where("user_id = ?", userID)
	if alertTYpe > 0 {
		q = q.Where("alert_type = ?", alertTYpe)
	}
	if err := q.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get alert history for user %s: %w", userID, err)
	}
	return history, nil
}

// GetSubscriptionsByInstID 可以简单实现，这里省略，但它是 AlertService 内存初始化的辅助函数
func (d *AlertDAOImpl) GetSubscriptionsByInstID(ctx context.Context, instID string) ([]entity.AlertSubscription, error) {
	var subs []entity.AlertSubscription
	err := d.db.WithContext(ctx).Where("inst_id = ?", instID).Find(&subs).Error
	return subs, err
}

// GetSubscriptionsByUserID 查询用户所有订阅
func (d *AlertDAOImpl) GetSubscriptionsByUserID(ctx context.Context, userID string) ([]entity.AlertSubscription, error) {
	var subs []entity.AlertSubscription
	if err := d.db.WithContext(ctx).Where("user_id = ?", userID).Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("failed to get subscriptions for user %s: %w", userID, err)
	}
	return subs, nil
}

// UpdateSubscription 更新整个订阅
// ⚠️ 注意：GORM 的 Save() 方法会更新所有字段。如果 model.AlertSubscription 字段为零值，
// 且它不是 sql.Null* 类型，数据库中对应的字段会被更新为零值。
func (d *AlertDAOImpl) UpdateSubscription(ctx context.Context, sub *entity.AlertSubscription) error {
	sub.UpdatedAt = time.Now()

	// 使用 Save 方法，Gorm 会根据主键 (ID) 查找记录并更新所有字段
	// 确保 sub.ID 字段在传入时已设置
	result := d.db.WithContext(ctx).Save(sub)
	if result.Error != nil {
		return fmt.Errorf("failed to update subscription %s: %w", sub.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("subscription with ID %s not found for update", sub.ID)
	}
	return nil
}

// 根据订阅ID获取订阅信息
func (d *AlertDAOImpl) GetSubscriptionByID(cxt context.Context, id string) (entity.AlertSubscription, error) {
	var sub entity.AlertSubscription
	// 使用 First 确保只获取一条记录
	err := d.db.Where("id = ?", id).First(&sub).Error

	// Gorm 会在找不到记录时返回 gorm.ErrRecordNotFound
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sub, nil // 返回空结构体和 nil 错误，表示不存在
	}
	if err != nil {
		return sub, fmt.Errorf("failed to get subscription by ID %s: %w", id, err)
	}
	return sub, nil
}
