package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
)

// Config Kafka配置
type Config struct {
	Brokers          []string      `yaml:"brokers"`
	GroupID          string        `yaml:"group_id"`
	AutoOffsetReset  string        `yaml:"auto_offset_reset"`
	SessionTimeout   time.Duration `yaml:"session_timeout"`
	CommitInterval   time.Duration `yaml:"commit_interval"`
	RetryAttempts    int           `yaml:"retry_attempts"`
	RetryBackoff     time.Duration `yaml:"retry_backoff"`
	CompressionCodec string        `yaml:"compression_codec"`
	SecurityProtocol string        `yaml:"security_protocol"`
	SASLMechanism    string        `yaml:"sasl_mechanism"`
	SASLUsername     string        `yaml:"sasl_username"`
	SASLPassword     string        `yaml:"sasl_password"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Brokers:          []string{"localhost:9092"},
		GroupID:          "drone-control-system",
		AutoOffsetReset:  "earliest",
		SessionTimeout:   10 * time.Second,
		CommitInterval:   1 * time.Second,
		RetryAttempts:    3,
		RetryBackoff:     100 * time.Millisecond,
		CompressionCodec: "snappy",
		SecurityProtocol: "PLAINTEXT",
	}
}

// LoadConfigFromViper 从 Viper 加载配置
func LoadConfigFromViper(v *viper.Viper) *Config {
	config := DefaultConfig()
	
	if v.IsSet("kafka.brokers") {
		config.Brokers = v.GetStringSlice("kafka.brokers")
	}
	if v.IsSet("kafka.group_id") {
		config.GroupID = v.GetString("kafka.group_id")
	}
	if v.IsSet("kafka.auto_offset_reset") {
		config.AutoOffsetReset = v.GetString("kafka.auto_offset_reset")
	}
	if v.IsSet("kafka.session_timeout") {
		config.SessionTimeout = v.GetDuration("kafka.session_timeout")
	}
	if v.IsSet("kafka.commit_interval") {
		config.CommitInterval = v.GetDuration("kafka.commit_interval")
	}
	if v.IsSet("kafka.retry_attempts") {
		config.RetryAttempts = v.GetInt("kafka.retry_attempts")
	}
	if v.IsSet("kafka.retry_backoff") {
		config.RetryBackoff = v.GetDuration("kafka.retry_backoff")
	}
	if v.IsSet("kafka.compression_codec") {
		config.CompressionCodec = v.GetString("kafka.compression_codec")
	}
	
	return config
}

// Validate 验证配置
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("kafka brokers cannot be empty")
	}
	
	if c.GroupID == "" {
		return fmt.Errorf("kafka group_id cannot be empty")
	}
	
	if c.SessionTimeout <= 0 {
		return fmt.Errorf("kafka session_timeout must be positive")
	}
	
	if c.CommitInterval <= 0 {
		return fmt.Errorf("kafka commit_interval must be positive")
	}
	
	return nil
}

// GetDialer 获取 Kafka 连接器
func (c *Config) GetDialer() *kafka.Dialer {
	dialer := &kafka.Dialer{
		Timeout:   c.SessionTimeout,
		DualStack: true,
	}
	
	// 如果配置了 SASL 认证
	if c.SecurityProtocol == "SASL_PLAINTEXT" || c.SecurityProtocol == "SASL_SSL" {
		// TODO: 添加 SASL 配置支持
		// 这里可以根据需要添加 SASL 认证配置
	}
	
	return dialer
}

// CreateTopicsIfNotExist 创建主题（如果不存在）
func (c *Config) CreateTopicsIfNotExist(ctx context.Context, topics []string) error {
	conn, err := kafka.DialContext(ctx, "tcp", c.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to kafka: %w", err)
	}
	defer conn.Close()
	
	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}
	
	controllerConn, err := kafka.DialContext(ctx, "tcp", controller.Host+":"+fmt.Sprint(controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer controllerConn.Close()
	
	topicConfigs := make([]kafka.TopicConfig, len(topics))
	for i, topic := range topics {
		topicConfigs[i] = kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     3,  // 3个分区
			ReplicationFactor: 1,  // 单机部署用1，集群建议3
		}
	}
	
	return controllerConn.CreateTopics(topicConfigs...)
}
