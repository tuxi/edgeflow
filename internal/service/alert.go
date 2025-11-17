package service

import (
	"context"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"log"
	"sync"
	"time"
)

// AlertService 用于消费上游告警来源并提供订阅通道给 gateway。
type AlertService struct {
	producer kafka.ProducerService
	// 价格提醒订阅存储 (InstID -> []Subscription)
	// ⚠️ 注意：这是一个临界资源，必须在 mu 锁保护下访问
	priceAlerts map[string][]*PriceAlertSubscription
	mu          sync.RWMutex
}

type AlertPublisher interface {
	PublishToDevice(alert *pb.AlertMessage)
	GetSubscriptionsForInstID(instID string) []*PriceAlertSubscription
	// 标记为已触发，并记录触发价格
	MarkSubscriptionAsTriggered(instID string, subscriptionID string, triggeredPrice float64)
	// 标记为已重置，重新激活
	MarkSubscriptionAsReset(instID string, subscriptionID string)
}

// 提醒订阅结构体（MDS 内部存储）
type PriceAlertSubscription struct {
	UserID         string // 对应 Kafka Key 和客户端 ID
	SubscriptionID string // 用户的订阅唯一 ID
	InstID         string // 交易对，如 BTC-USDT
	IsActive       bool   // 是否已触发或活跃

	// 极速提醒字段
	ChangePercent float64 // 变化百分比 (例如 5.0 代表 5%)
	WindowMinutes int     // 时间窗口 (例如 5 代表 5分钟)

	// 现有价格突破字段
	TargetPrice float64 // 目标价格
	Direction   string  // "UP", "DOWN" (现在也用于极速提醒的上升/下降)

	// 上次触发时的价格（用于判断是否重置）
	LastTriggeredPrice float64
}

func NewAlertService(producer kafka.ProducerService) *AlertService {
	return &AlertService{
		producer:    producer,
		priceAlerts: make(map[string][]*PriceAlertSubscription),
	}
}

// 写入全量推送 Topic
func (s *AlertService) PublishBroadcast(msg *pb.AlertMessage) {
	protoMsg := kafka.Message{
		Key: "ALERT_BROADCAST", // 固定KEY
		Data: &pb.WebSocketMessage{
			Type:    "ALERT_SUBSCRIBE",
			Payload: &pb.WebSocketMessage_AlertMessage{AlertMessage: msg},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := s.producer.Produce(ctx, kafka.TopicAlertSystem, protoMsg); err != nil {
		log.Printf("ERROR: AlertService topic=%s 广播写入kafka数据失败: %v", kafka.TopicAlertSystem, err)
	}
}

// 写入定向推送 Topic
func (s *AlertService) PublishToDevice(msg *pb.AlertMessage) {
	// 1. 构造消息
	protoMsg := kafka.Message{
		// Kafka Key 必须是 deviceId
		Key: msg.UserId,
		Data: &pb.WebSocketMessage{
			Type:    "ALERT_DIRECT",
			Payload: &pb.WebSocketMessage_AlertMessage{AlertMessage: msg},
		},
	}

	// 2. 写入定向 Topic
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// 使用定向推送 Topic
	if err := s.producer.Produce(ctx, kafka.TopicAlertDirect, protoMsg); err != nil {
		// 定向推送写入失败，记录日志
		log.Printf("ERROR: AlertService 定向推送写入 Kafka失败 (Device: %s): %v", msg.UserId, err)
	}
}

// AlertService 暴露获取订阅的方法
// MDS 将通过这个方法获取订阅列表
func (s *AlertService) GetSubscriptionsForInstID(instID string) []*PriceAlertSubscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本或不可修改视图是最佳实践
	subs, ok := s.priceAlerts[instID]
	if !ok {
		return nil
	}
	// 返回副本，防止外部修改
	return append([]*PriceAlertSubscription{}, subs...)
}

// AlertService 管理订阅的方法 (供外部 API 调用)
func (s *AlertService) AddPriceAlert(sub PriceAlertSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.priceAlerts[sub.InstID]
	// 假设这里执行去重、更新等复杂逻辑
	list = append(list, &sub)
	s.priceAlerts[sub.InstID] = list
}

// MarkSubscriptionAsTriggered 标记订阅为已触发，并记录价格
func (s *AlertService) MarkSubscriptionAsTriggered(instID string, subscriptionID string, triggeredPrice float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, ok := s.priceAlerts[instID]
	if !ok {
		log.Printf("WARN: AlertService 尝试标记触发，但 InstID %s 不存在。", instID)
		return
	}

	for _, sub := range subs {
		if sub.SubscriptionID == subscriptionID && sub.IsActive {
			sub.IsActive = false
			sub.LastTriggeredPrice = triggeredPrice // 记录触发价格
			// 📢 实际应用中，这里必须触发数据库持久化更新
			log.Printf("INFO: 订阅 %s 已标记为已触发 (价格: %.2f)。", subscriptionID, triggeredPrice)
			return
		}
	}
}

// MarkSubscriptionAsReset 标记订阅为已重置，重新激活
func (s *AlertService) MarkSubscriptionAsReset(instID string, subscriptionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, ok := s.priceAlerts[instID]
	if !ok {
		log.Printf("WARN: AlertService 尝试标记重置，但 InstID %s 不存在。", instID)
		return
	}

	for _, sub := range subs {
		// 只有 IsActive = false 的订阅才需要重置
		if sub.SubscriptionID == subscriptionID && !sub.IsActive {
			sub.IsActive = true
			sub.LastTriggeredPrice = 0 // 清除上次触发价格
			// 📢 实际应用中，这里必须触发数据库持久化更新
			log.Printf("INFO: 订阅 %s 已标记为已重置 (重新激活)。", subscriptionID)
			return
		}
	}
}
