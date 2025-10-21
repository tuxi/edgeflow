package kafka

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"log"
)

// Kafka 生产者服务
// 定义接口，方便测试和替换
type ProducerService interface {
	Produce(ctx context.Context, topic string, key []byte, msg proto.Message) error
	Close()
}

type kafkaProducer struct {
	tickerWriter    *kafka.Writer // 用于高频 Ticker
	subscribeWriter *kafka.Writer // 用于低频 Subscribe
	systemWriter    *kafka.Writer // 用于系统/排序数据的 Writer
}

func NewKafkaProducer(brokerURL string) ProducerService {
	// 初始化 Ticker Writer
	tickerWriter := &kafka.Writer{
		Addr:     kafka.TCP(brokerURL),
		Topic:    "marketdata_ticker",
		Balancer: &kafka.LeastBytes{}, // 保证写入负载均衡
	}
	// 初始化 Subscribe Writer
	subscribeWriter := &kafka.Writer{
		Addr:     kafka.TCP(brokerURL),
		Topic:    "marketdata_subscribe",
		Balancer: &kafka.LeastBytes{},
	}

	// 初始化 System Writer (使用 marketdata_system)
	systemWriter := &kafka.Writer{
		Addr:     kafka.TCP(brokerURL),
		Topic:    "marketdata_system", // 正确指定 Topic
		Balancer: &kafka.LeastBytes{},
	}

	return &kafkaProducer{
		tickerWriter:    tickerWriter,
		subscribeWriter: subscribeWriter,
		systemWriter:    systemWriter,
	}
}

// Produce 通用方法：序列化 Protobuf 消息并写入 Kafka
func (p *kafkaProducer) Produce(ctx context.Context, topic string, key []byte, msg proto.Message) error {
	// 1. Protobuf 序列化
	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// 2. 选择正确的 Writer
	var writer *kafka.Writer
	switch topic {
	case "marketdata_ticker":
		writer = p.tickerWriter
	case "marketdata_subscribe":
		writer = p.subscribeWriter
	case "marketdata_system":
		writer = p.systemWriter
	default:
		return errors.New("invalid kafka topic")
	}

	// 3. 写入 Kafka
	return writer.WriteMessages(ctx, kafka.Message{
		Key:   key, // 使用 InstId 作为 Key，确保相同币种的数据进入同一个 Partition (有序性/关联性)
		Value: protoBytes,
	})
}

func (p *kafkaProducer) Close() {
	if err := p.tickerWriter.Close(); err != nil {
		log.Printf("Error closing Ticker writer: %v", err)
	}
	if err := p.subscribeWriter.Close(); err != nil {
		log.Printf("Error closing Subscribe writer: %v", err)
	}
	// 关闭 System Writer
	if err := p.systemWriter.Close(); err != nil {
		log.Printf("Error closing System writer: %v", err)
	}
}
