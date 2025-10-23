package kafka

import (
	"context"
	"errors"
	"sync" // 引入 sync 包用于并发控制
	"time"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"log"
)

type Message struct {
	Key  string
	Data proto.Message
}

// Kafka 生产者服务
// 定义接口，方便测试和替换
type ProducerService interface {
	// 支持批量写入消息，每个消息可以对应不同的key，保证时间顺序
	Produce(ctx context.Context, topic string, messages ...Message) error
	Close()
}

// 定义所有合法的 Topic 名称
const (
	TopicTicker    = "marketdata_ticker"
	TopicSubscribe = "marketdata_subscribe"
	TopicSystem    = "marketdata_system"
)

// kafkaProducer 结构体修改为支持懒加载和并发安全
type kafkaProducer struct {
	brokerURL string
	writers   map[string]*kafka.Writer // 使用 map 存储已创建的 Writer
	mu        sync.RWMutex             // 读写锁，保护 writers map 的并发访问
}

// NewKafkaProducer 只存储 Broker 地址，不初始化 Writer
func NewKafkaProducer(brokerURL string) ProducerService {
	p := &kafkaProducer{
		brokerURL: brokerURL,
		writers:   make(map[string]*kafka.Writer),
	}
	p.getWriter(TopicTicker) // 启动时只创建优先级最高的topic，其他的懒加载
	return p
}

// getWriter 获取或创建指定 Topic 的 Writer (实现懒加载)
func (p *kafkaProducer) getWriter(topic string) (*kafka.Writer, error) {
	// 1. 读锁：尝试从 map 中获取已存在的 Writer
	p.mu.RLock()
	writer, ok := p.writers[topic]
	p.mu.RUnlock()

	if ok {
		// 找到了，直接返回
		return writer, nil
	}

	// 2. 写锁：如果 Writer 不存在，则需要创建
	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查：在获取写锁后，再次检查是否已被其他 Goroutine 创建 (避免重复创建)
	if writer, ok := p.writers[topic]; ok {
		return writer, nil
	}

	// 3. 验证 Topic 合法性并创建新的 Writer
	switch topic {
	case TopicSubscribe, TopicSystem, TopicTicker:
		// Topic 合法，创建 Writer
		newWriter := &kafka.Writer{
			Addr:     kafka.TCP(p.brokerURL),
			Topic:    topic,
			Balancer: &kafka.Hash{}, // 保证时间顺序
			// 建议：为 Writer 设置超时，防止长时间阻塞
			// WriteTimeout: 10 * time.Second,
		}
		// 存入 map
		p.writers[topic] = newWriter
		log.Printf("Kafka Producer: Lazily created writer for topic: %s", topic)
		return newWriter, nil
	default:
		// Topic 不合法
		return nil, errors.New("invalid kafka topic")
	}
}

// Produce 通用方法：序列化 Protobuf 消息并写入 Kafka
func (p *kafkaProducer) Produce(ctx context.Context, topic string, messages ...Message) error {
	// 1. 获取 (或懒加载创建) 对应的 Writer
	writer, err := p.getWriter(topic)
	if err != nil {
		return err
	}
	msgs := make([]kafka.Message, 0, len(messages))
	for _, msg := range messages {
		// 2. Protobuf 序列化
		protoBytes, err := proto.Marshal(msg.Data)
		if err != nil {
			return err
		}

		msgs = append(msgs, kafka.Message{
			Key:   []byte(msg.Key), // 每个币种单独 Key
			Value: protoBytes,
			Time:  time.Now(),
		})
	}

	// 3. 写入 Kafka
	return writer.WriteMessages(ctx, msgs...)
}

// Close 关闭所有已创建的 Writer
func (p *kafkaProducer) Close() {
	p.mu.Lock() // 获取写锁，确保在关闭过程中 map 不会被修改
	defer p.mu.Unlock()

	// 遍历所有已创建的 Writer 并关闭
	for topic, writer := range p.writers {
		if err := writer.Close(); err != nil {
			log.Printf("Error closing writer for topic %s: %v", topic, err)
		} else {
			log.Printf("Successfully closed writer for topic: %s", topic)
		}
	}
	// 清空 map (可选)
	p.writers = make(map[string]*kafka.Writer)
}
