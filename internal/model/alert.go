package model

// CreateUpdateSubscriptionRequest 定义了创建和修改订阅时的请求体结构
type CreateUpdateSubscriptionRequest struct {
	// 基础信息 (创建时不需要 ID, 更新时 ID 在 Path 中)
	UserID string `json:"user_id" binding:"required"`
	InstID string `json:"inst_id" binding:"required"`

	// 提醒类型和方向
	AlertType int    `json:"alert_type" binding:"required"` // 对应 Protobuf AlertType
	Direction string `json:"direction" binding:"required"`  // UP, DOWN, RATE

	// 价格突破参数
	TargetPrice float64 `json:"target_price,omitempty"` // 突破价格 (TargetPrice > 0 时需要)

	// 极速提醒参数
	ChangePercent float64 `json:"change_percent,omitempty"` // 变化百分比 (ChangePercent > 0 时需要)
	WindowMinutes int     `json:"window_minutes,omitempty"` // 时间窗口 (分钟)

	// 其他如社交媒体、链上等自定义参数，可以通过 extra 传递，这里简化不列出。
}

// SubscriptionResponse 用于 GET 列表时的返回结构
// 字段与 CreateUpdateSubscriptionRequest 相似，但包含 ID 和状态
type SubscriptionResponse struct {
	ID                 string  `json:"id"`
	UserID             string  `json:"user_id"`
	InstID             string  `json:"inst_id"`
	AlertType          int     `json:"alert_type"`
	Direction          string  `json:"direction"`
	TargetPrice        float64 `json:"target_price"`
	ChangePercent      float64 `json:"change_percent"`
	WindowMinutes      int     `json:"window_minutes"`
	IsActive           bool    `json:"is_active"`            // 当前是否处于活跃待触发状态
	LastTriggeredPrice float64 `json:"last_triggered_price"` // 上次触发价格
}

type DeleteSubscriptionRequest struct {
	ID     string `json:"id"`
	InstID string `json:"inst_id"`
}
