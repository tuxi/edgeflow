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
func (s *AlertService) PublishToDevice(deviceId string, msg *pb.AlertMessage) {
	// 1. 构造消息
	protoMsg := kafka.Message{
		// Kafka Key 必须是 deviceId
		Key: deviceId,
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
		log.Printf("ERROR: AlertService 定向推送写入 Kafka失败 (Device: %s): %v", deviceId, err)
	}
	// 注意：AlertService 不关心客户端是否在线，只负责写入 Kafka。
	// 在线判断和推送由 AlertGateway 负责。
}
