package service

import (
	"context"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"log"
	"time"
)

// AlertService 用于消费上游告警来源并提供订阅通道给 gateway。
type AlertService struct {
	producer kafka.ProducerService
}

type AlertPublisher interface {
	PublishToDevice(alert *pb.AlertMessage)
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
}

func NewAlertService(producer kafka.ProducerService) *AlertService {
	return &AlertService{
		producer: producer,
	}
}

// 写入全量推送 Topic
func (s *AlertService) PublishBroadcast(msg *pb.AlertMessage) {
	//msg := &pb.AlertMessage{
	//	Id:        uuid.NewString(),
	//	Title:     "BTC暴涨预警",
	//	Content:   "BTC价格1小时内上涨5.2%",
	//	Symbol:    "BTCUSDT",
	//	Level:     pb.AlertLevel_ALERT_LEVEL_WARNING,
	//	AlertType: pb.AlertType_ALERT_TYPE_PRICE,
	//	Timestamp: time.Now().UnixMilli(),
	//}
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
