package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"drone-control-system/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// Producer Kafka生产者
type Producer struct {
	writer *kafka.Writer
	logger *logger.Logger
}

// NewProducer 创建新的生产者
func NewProducer(config *Config, logger *logger.Logger) *Producer {
	// 解析压缩算法
	var compression kafka.Compression
	switch config.CompressionCodec {
	case "gzip":
		compression = kafka.Gzip
	case "snappy":
		compression = kafka.Snappy
	case "lz4":
		compression = kafka.Lz4
	case "zstd":
		compression = kafka.Zstd
	default:
		compression = kafka.Snappy // 默认使用 snappy
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		Compression:  compression,
		BatchTimeout: config.CommitInterval,
		BatchSize:    100,
		Async:        false, // 同步写入确保可靠性
	}

	return &Producer{
		writer: writer,
		logger: logger,
	}
}

// SendMessage 发送消息
func (p *Producer) SendMessage(ctx context.Context, topic string, key string, value interface{}) error {
	// 序列化消息
	messageBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	message := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: messageBytes,
		Time:  time.Now(),
	}

	// 发送消息
	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		p.logger.WithError(err).WithField("topic", topic).Error("Failed to send kafka message")
		return fmt.Errorf("failed to send message to topic %s: %w", topic, err)
	}

	p.logger.WithField("topic", topic).WithField("key", key).Debug("Message sent successfully")
	return nil
}

// SendBatchMessages 批量发送消息
func (p *Producer) SendBatchMessages(ctx context.Context, topic string, messages []MessageData) error {
	kafkaMessages := make([]kafka.Message, len(messages))

	for i, msg := range messages {
		messageBytes, err := json.Marshal(msg.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal message %d: %w", i, err)
		}

		kafkaMessages[i] = kafka.Message{
			Topic: topic,
			Key:   []byte(msg.Key),
			Value: messageBytes,
			Time:  time.Now(),
		}
	}

	err := p.writer.WriteMessages(ctx, kafkaMessages...)
	if err != nil {
		p.logger.WithError(err).WithField("topic", topic).Error("Failed to send batch messages")
		return fmt.Errorf("failed to send batch messages to topic %s: %w", topic, err)
	}

	p.logger.WithField("topic", topic).WithField("count", len(messages)).Debug("Batch messages sent successfully")
	return nil
}

// Close 关闭生产者
func (p *Producer) Close() error {
	return p.writer.Close()
}

// Consumer Kafka消费者
type Consumer struct {
	reader *kafka.Reader
	logger *logger.Logger
}

// NewConsumer 创建新的消费者
func NewConsumer(config *Config, topic string, logger *logger.Logger) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          topic,
		GroupID:        config.GroupID,
		CommitInterval: config.CommitInterval,
		StartOffset:    kafka.FirstOffset,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
	})

	return &Consumer{
		reader: reader,
		logger: logger,
	}
}

// ConsumeMessages 消费消息
func (c *Consumer) ConsumeMessages(ctx context.Context, handler MessageHandler) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Consumer context cancelled, stopping consumption")
			return ctx.Err()
		default:
			// 读取消息
			message, err := c.reader.ReadMessage(ctx)
			if err != nil {
				c.logger.WithError(err).Error("Failed to read kafka message")
				continue
			}

			// 处理消息
			err = handler.HandleMessage(ctx, &Message{
				Topic:     message.Topic,
				Partition: message.Partition,
				Offset:    message.Offset,
				Key:       string(message.Key),
				Value:     message.Value,
				Time:      message.Time,
			})

			if err != nil {
				c.logger.WithError(err).
					WithField("topic", message.Topic).
					WithField("offset", message.Offset).
					Error("Failed to handle message")
				// 这里可以添加重试逻辑或错误消息处理
				continue
			}

			c.logger.WithField("topic", message.Topic).
				WithField("offset", message.Offset).
				Debug("Message processed successfully")
		}
	}
}

// Close 关闭消费者
func (c *Consumer) Close() error {
	return c.reader.Close()
}

// MessageData 消息数据结构
type MessageData struct {
	Key   string
	Value interface{}
}

// Message 接收到的消息
type Message struct {
	Topic     string
	Partition int
	Offset    int64
	Key       string
	Value     []byte
	Time      time.Time
}

// UnmarshalValue 反序列化消息值
func (m *Message) UnmarshalValue(v interface{}) error {
	return json.Unmarshal(m.Value, v)
}

// MessageHandler 消息处理接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, message *Message) error
}

// MessageHandlerFunc 函数式消息处理器
type MessageHandlerFunc func(ctx context.Context, message *Message) error

// HandleMessage 实现 MessageHandler 接口
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, message *Message) error {
	return f(ctx, message)
}
