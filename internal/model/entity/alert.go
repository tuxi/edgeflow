package entity

import (
	"database/sql"
	"time"
)

// AlertSubscription 提醒订阅表结构
type AlertSubscription struct {
	ID                 string          `gorm:"primaryKey;type:varchar(36)"`                   // 主键,唯一标识用户的订阅规则。
	UserID             string          `gorm:"index:idx_user_inst;type:varchar(36);not null"` // 索引。用户ID,快速查询某用户的所有订阅。
	InstID             string          `gorm:"index:idx_user_inst;type:varchar(30);not null"` // 索引。交易对ID（BTC-USDT）,快速查询某币种的所有订阅。
	AlertType          int             `gorm:"type:int"`                                      // 提醒类型（PRICE=1, SIGNAL=2）,对应 Protobuf AlertType
	Direction          string          `gorm:"type:varchar(10)"`                              // 方向（UP/DOWN/RATE）
	TargetPrice        sql.NullFloat64 `gorm:"type:decimal(20,8)"`                            // 目标价格（如果为价格突破）,可空
	ChangePercent      sql.NullFloat64 `gorm:"type:decimal(6,2)"`                             // 变化百分比（如果为极速提醒），可空
	WindowMinutes      sql.NullInt64   `gorm:"type:int"`                                      // 极速提醒的时间窗口（分钟），可空
	IsActive           bool            `gorm:"not null"`                                      // 当前状态：是否活跃/待触发
	LastTriggeredPrice sql.NullFloat64 `gorm:"type:decimal(20,8)"`                            // 上次触发时的价格，可空

	// 用于通用关口的粒度 (例如 1.0, 0.01)
	BoundaryStep sql.NullFloat64 `gorm:"column:boundary_step;type:decimal(10, 8)"`

	// 关口量级（只有当 BoundaryStep > 0 时使用）。
	// 例如：BTC 设为 10000 (万位关口) 或 1000 (千位关口)。
	BoundaryMagnitude sql.NullFloat64 `gorm:"column:boundary_magnitude;type:decimal(18, 8)"`

	CreatedAt time.Time // 创建时间
	UpdatedAt time.Time // 更新时间
}

func (AlertSubscription) TableName() string {
	return "alert_subscription"
}

// AlertHistory 提醒历史表结构 (对应已发送的 AlertMessage)
type AlertHistory struct {
	ID             string `gorm:"primaryKey;type:varchar(36)" json:"id"`                      // 主键，提醒消息的唯一ID
	UserID         string `gorm:"index:idx_user_ts;type:varchar(36);not null" json:"user_id"` // 索引。接收者用户ID，客户端查询历史记录的依据
	SubscriptionID string `gorm:"type:varchar(36)" json:"subscription_id"`                    // 关联的订阅ID（可选），用于追踪是哪个规则触发的
	Title          string `gorm:"type:varchar(100)" json:"title"`                             // 标题
	Content        string `gorm:"type:text" json:"content"`                                   // 详细内容
	Level          int    `gorm:"type:int" json:"level"`                                      // 消息级别，对应 Protobuf AlertLevel
	AlertType      int    `gorm:"type:int" json:"alert_type"`                                 // 消息类型，对应 Protobuf AlertType
	Timestamp      int64  `gorm:"index:idx_user_ts;type:bigint;not null" json:"timestamp"`    // 消息时间戳（毫秒），用于排序和分页查询
	ExtraJSON      string `gorm:"column:extra_json;type:json" json:"extra_json"`              // Protobuf 中的 extra Map，存储触发价格、当前价格等详细信息

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"` // 创建时间
}

func (AlertHistory) TableName() string {
	return "alert_history"
}
