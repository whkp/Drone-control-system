package services

import (
	"context"
	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"
)

// KafkaService Kafka服务接口
type KafkaService interface {
	// 发布事件
	PublishDroneEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error
	PublishTaskEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error
	PublishUserEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error
	PublishAlertEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error

	// 管理方法
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
}

// KafkaServiceImpl Kafka服务实现
type KafkaServiceImpl struct {
	manager *kafka.Manager
	logger  *logger.Logger
}

// NewKafkaService 创建Kafka服务
func NewKafkaService(config *kafka.Config, logger *logger.Logger) (KafkaService, error) {
	manager, err := kafka.NewManager(config, logger)
	if err != nil {
		return nil, err
	}

	return &KafkaServiceImpl{
		manager: manager,
		logger:  logger,
	}, nil
}

// PublishDroneEvent 发布无人机事件
func (s *KafkaServiceImpl) PublishDroneEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error {
	event := kafka.NewEvent(eventType, "mvc-server", data)
	return s.manager.PublishDroneEvent(ctx, event)
}

// PublishTaskEvent 发布任务事件
func (s *KafkaServiceImpl) PublishTaskEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error {
	event := kafka.NewEvent(eventType, "mvc-server", data)
	return s.manager.PublishTaskEvent(ctx, event)
}

// PublishUserEvent 发布用户事件
func (s *KafkaServiceImpl) PublishUserEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error {
	event := kafka.NewEvent(eventType, "mvc-server", data)
	return s.manager.PublishUserEvent(ctx, event)
}

// PublishAlertEvent 发布告警事件
func (s *KafkaServiceImpl) PublishAlertEvent(ctx context.Context, eventType kafka.EventType, data interface{}) error {
	event := kafka.NewEvent(eventType, "mvc-server", data)
	return s.manager.PublishAlertEvent(ctx, event)
}

// Start 启动Kafka服务
func (s *KafkaServiceImpl) Start(ctx context.Context) error {
	if err := s.manager.Initialize(ctx); err != nil {
		return err
	}
	return s.manager.Start(ctx)
}

// Stop 停止Kafka服务
func (s *KafkaServiceImpl) Stop() error {
	return s.manager.Stop()
}

// IsRunning 检查是否运行中
func (s *KafkaServiceImpl) IsRunning() bool {
	return s.manager.IsRunning()
}
