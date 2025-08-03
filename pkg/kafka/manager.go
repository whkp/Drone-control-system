package kafka

import (
	"context"
	"fmt"
	"sync"

	"drone-control-system/pkg/logger"
)

// Manager Kafka管理器
type Manager struct {
	config    *Config
	logger    *logger.Logger
	producer  *Producer
	consumers map[string]*Consumer
	handlers  map[string]MessageHandler
	mu        sync.RWMutex
	running   bool
}

// NewManager 创建新的Kafka管理器
func NewManager(config *Config, logger *logger.Logger) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid kafka config: %w", err)
	}

	producer := NewProducer(config, logger)

	return &Manager{
		config:    config,
		logger:    logger,
		producer:  producer,
		consumers: make(map[string]*Consumer),
		handlers:  make(map[string]MessageHandler),
		running:   false,
	}, nil
}

// Initialize 初始化Kafka管理器
func (m *Manager) Initialize(ctx context.Context) error {
	m.logger.Info("Initializing Kafka manager")

	// 创建所需的主题
	topics := []string{
		DroneEventsTopic,
		TaskEventsTopic,
		UserEventsTopic,
		AlertEventsTopic,
		SystemEventsTopic,
		MonitoringTopic,
		LogsTopic,
	}

	if err := m.config.CreateTopicsIfNotExist(ctx, topics); err != nil {
		m.logger.WithError(err).Error("Failed to create kafka topics")
		return fmt.Errorf("failed to create kafka topics: %w", err)
	}

	m.logger.Info("Kafka manager initialized successfully")
	return nil
}

// PublishEvent 发布事件
func (m *Manager) PublishEvent(ctx context.Context, topic string, event *Event) error {
	key := fmt.Sprintf("%s-%s", event.Type, event.Source)
	return m.producer.SendMessage(ctx, topic, key, event)
}

// PublishDroneEvent 发布无人机事件
func (m *Manager) PublishDroneEvent(ctx context.Context, event *Event) error {
	return m.PublishEvent(ctx, DroneEventsTopic, event)
}

// PublishTaskEvent 发布任务事件
func (m *Manager) PublishTaskEvent(ctx context.Context, event *Event) error {
	return m.PublishEvent(ctx, TaskEventsTopic, event)
}

// PublishUserEvent 发布用户事件
func (m *Manager) PublishUserEvent(ctx context.Context, event *Event) error {
	return m.PublishEvent(ctx, UserEventsTopic, event)
}

// PublishAlertEvent 发布告警事件
func (m *Manager) PublishAlertEvent(ctx context.Context, event *Event) error {
	return m.PublishEvent(ctx, AlertEventsTopic, event)
}

// PublishSystemEvent 发布系统事件
func (m *Manager) PublishSystemEvent(ctx context.Context, event *Event) error {
	return m.PublishEvent(ctx, SystemEventsTopic, event)
}

// PublishMonitoringData 发布监控数据
func (m *Manager) PublishMonitoringData(ctx context.Context, data interface{}) error {
	event := NewEvent(SystemMetricsEvent, "system", data)
	return m.PublishEvent(ctx, MonitoringTopic, event)
}

// RegisterHandler 注册消息处理器
func (m *Manager) RegisterHandler(topic string, handler MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[topic] = handler
}

// Subscribe 订阅主题
func (m *Manager) Subscribe(ctx context.Context, topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.consumers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	handler, exists := m.handlers[topic]
	if !exists {
		return fmt.Errorf("no handler registered for topic: %s", topic)
	}

	consumer := NewConsumer(m.config, topic, m.logger)
	m.consumers[topic] = consumer

	// 启动消费者
	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.consumers, topic)
			m.mu.Unlock()
		}()

		if err := consumer.ConsumeMessages(ctx, handler); err != nil {
			m.logger.WithError(err).WithField("topic", topic).Error("Consumer stopped with error")
		}
	}()

	m.logger.WithField("topic", topic).Info("Subscribed to topic")
	return nil
}

// Start 启动所有已注册的消费者
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("kafka manager is already running")
	}

	m.logger.Info("Starting Kafka manager")

	// 启动所有已注册的处理器对应的消费者
	for topic := range m.handlers {
		if err := m.subscribe(ctx, topic); err != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
		}
	}

	m.running = true
	m.logger.Info("Kafka manager started successfully")
	return nil
}

// subscribe 内部订阅方法（不加锁）
func (m *Manager) subscribe(ctx context.Context, topic string) error {
	if _, exists := m.consumers[topic]; exists {
		return nil // 已经订阅
	}

	handler := m.handlers[topic]
	consumer := NewConsumer(m.config, topic, m.logger)
	m.consumers[topic] = consumer

	// 启动消费者
	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.consumers, topic)
			m.mu.Unlock()
		}()

		if err := consumer.ConsumeMessages(ctx, handler); err != nil {
			m.logger.WithError(err).WithField("topic", topic).Error("Consumer stopped with error")
		}
	}()

	return nil
}

// Stop 停止Kafka管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.logger.Info("Stopping Kafka manager")

	// 关闭生产者
	if err := m.producer.Close(); err != nil {
		m.logger.WithError(err).Error("Failed to close producer")
	}

	// 关闭所有消费者
	for topic, consumer := range m.consumers {
		if err := consumer.Close(); err != nil {
			m.logger.WithError(err).WithField("topic", topic).Error("Failed to close consumer")
		}
	}

	m.consumers = make(map[string]*Consumer)
	m.running = false

	m.logger.Info("Kafka manager stopped")
	return nil
}

// IsRunning 检查管理器是否正在运行
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	consumerTopics := make([]string, 0, len(m.consumers))
	for topic := range m.consumers {
		consumerTopics = append(consumerTopics, topic)
	}

	handlerTopics := make([]string, 0, len(m.handlers))
	for topic := range m.handlers {
		handlerTopics = append(handlerTopics, topic)
	}

	return map[string]interface{}{
		"running":         m.running,
		"consumer_topics": consumerTopics,
		"handler_topics":  handlerTopics,
		"consumer_count":  len(m.consumers),
		"handler_count":   len(m.handlers),
	}
}
