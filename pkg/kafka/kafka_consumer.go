package kafka

import (
	"context"
	"github.com/segmentio/kafka-go"
	"log"
	"time"
)

// ConsumerService 定义了消费 Kafka 消息的通用接口
type ConsumerService interface {
	// Consume 启动一个协程消费指定主题，将消息发送到返回的通道
	Consume(ctx context.Context, topic string, groupID string) (<-chan kafka.Message, error)
	Close()
}

type kafkaConsumer struct {
	brokerURL string
	// 可以添加 map 来管理多个 Reader
}

func NewKafkaConsumer(brokerURL string) ConsumerService {
	return &kafkaConsumer{
		brokerURL: brokerURL,
	}
}

// Consume 方法的核心逻辑
func (c *kafkaConsumer) Consume(ctx context.Context, topic string, groupID string) (<-chan kafka.Message, error) {
	// 1. 创建 kafka.Reader
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{c.brokerURL},
		Topic:    topic,
		GroupID:  groupID, // 不同的 Gateway 使用不同的 GroupID
		MinBytes: 10e3,    // 10KB
		MaxBytes: 10e6,    // 10MB
		// 从最新的 offset 开始消费 (通常对于实时推送是最佳选择)
		StartOffset:    kafka.LastOffset,
		CommitInterval: time.Second, // 启动自动提交，每秒提交一次
		MaxAttempts:    3,
		// 注意：如果使用自动提交，就不能在循环中手动调用CommitMessages
	})

	// 2. 创建输出通道
	outputCh := make(chan kafka.Message, 100) // 缓冲区用于平滑流量

	// 3. 启动消费协程
	go func() {
		defer close(outputCh)
		for {
			// 阻塞读取消息
			m, err := r.FetchMessage(ctx)
			if err != nil {
				// 如果是 Context 被取消（服务关闭），正常退出
				if ctx.Err() != nil {
					break
				}
				log.Printf("ERROR: Kafka read error on topic %s: %v", topic, err)
				time.Sleep(time.Second) // 短暂等待后重试
				continue
			}

			// 尝试将消息发送到输出通道
			select {
			case outputCh <- m:
				// 成功发送
			case <-ctx.Done():
				// 上下文结束，退出循环
				break
			}

			// 注意：这里手动提交严重影响客户端接收数据的频率，导致延迟，我们设置CommitInterval自动提交数据
			// 提交 Offset (重要：确保消息被处理后才提交)
			//if err := r.CommitMessages(ctx, m); err != nil {
			//	log.Printf("ERROR: Failed to commit offset: %v", err)
			//}
		}
		r.Close() // 退出时关闭 Reader
		log.Printf("Kafka Consumer for topic %s finished.", topic)
	}()

	return outputCh, nil
}

func (c *kafkaConsumer) Close() {
	// 由于 Reader 在其消费协程退出时自动关闭，
	// 这里的 Close 主要是用于清理任何全局资源，目前可以留空或仅记录日志。
	log.Println("Kafka Consumer Service closing...")
}
