package handlers

import (
	"encoding/json"

	"drone-control-system/internal/mvc/services"
	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"
)

// EventHandler 事件处理器
type EventHandler struct {
	logger            *logger.Logger
	websocketService  services.WebSocketService
	smartAlertService services.SmartAlertService
	eventBuffer       []kafka.Event
	bufferSize        int
}

// NewEventHandler 创建事件处理器
func NewEventHandler(logger *logger.Logger, websocketService services.WebSocketService, smartAlertService services.SmartAlertService) *EventHandler {
	return &EventHandler{
		logger:            logger,
		websocketService:  websocketService,
		smartAlertService: smartAlertService,
		eventBuffer:       make([]kafka.Event, 0, 100),
		bufferSize:        100,
	}
}

// HandleDroneEvent 处理无人机事件
func (h *EventHandler) HandleDroneEvent(message *kafka.Message) error {
	h.logger.Debug("Handling drone event", map[string]interface{}{
		"topic":   message.Topic,
		"message": string(message.Value),
	})

	var event kafka.Event
	if err := json.Unmarshal(message.Value, &event); err != nil {
		h.logger.Error("Failed to unmarshal drone event", map[string]interface{}{
			"error":   err.Error(),
			"message": string(message.Value),
		})
		return err
	}

	// 转发事件到WebSocket
	h.websocketService.HandleKafkaEvent(&event)

	// 添加到事件缓冲区用于批量分析
	h.addToEventBuffer(event)

	// 根据事件类型进行特定处理
	switch event.Type {
	case kafka.DroneBatteryLowEvent:
		h.handleBatteryLowEvent(&event)
	case kafka.DroneLocationUpdatedEvent:
		h.handleLocationUpdateEvent(&event)
	case kafka.DroneStatusChangedEvent:
		h.handleStatusChangeEvent(&event)
	}

	return nil
}

// addToEventBuffer 添加事件到缓冲区
func (h *EventHandler) addToEventBuffer(event kafka.Event) {
	h.eventBuffer = append(h.eventBuffer, event)

	// 当缓冲区满时，进行批量分析
	if len(h.eventBuffer) >= h.bufferSize {
		h.processBatchEvents()
	}
}

// processBatchEvents 批量处理事件
func (h *EventHandler) processBatchEvents() {
	if len(h.eventBuffer) == 0 {
		return
	}

	// 使用智能告警服务分析事件模式
	pattern, err := h.smartAlertService.ProcessEvents(h.eventBuffer)
	if err != nil {
		h.logger.Error("Failed to process batch events", map[string]interface{}{
			"error": err.Error(),
			"count": len(h.eventBuffer),
		})
		return
	}

	// 处理分析结果
	h.handleEventPattern(pattern)

	// 清空缓冲区
	h.eventBuffer = h.eventBuffer[:0]
}

// handleEventPattern 处理事件模式分析结果
func (h *EventHandler) handleEventPattern(pattern *services.EventPattern) {
	// 发送系统健康分数更新
	if pattern.SystemHealthScore < 80 {
		h.websocketService.BroadcastToAll(services.WebSocketMessage{
			Type: "system_health_warning",
			Data: map[string]interface{}{
				"health_score": pattern.SystemHealthScore,
				"timestamp":    pattern,
			},
		})
	}

	// 发送预测性告警
	for _, issue := range pattern.PredictedIssues {
		h.websocketService.BroadcastToAll(services.WebSocketMessage{
			Type: "predictive_alert",
			Data: issue,
		})
	}

	// 发送位置异常警告
	for _, anomaly := range pattern.LocationAnomalies {
		h.websocketService.BroadcastToAll(services.WebSocketMessage{
			Type: "location_anomaly",
			Data: anomaly,
		})
	}
}

// HandleTaskEvent 处理任务事件
func (h *EventHandler) HandleTaskEvent(message *kafka.Message) error {
	h.logger.Debug("Handling task event", map[string]interface{}{
		"topic":   message.Topic,
		"message": string(message.Value),
	})

	var event kafka.Event
	if err := json.Unmarshal(message.Value, &event); err != nil {
		h.logger.Error("Failed to unmarshal task event", map[string]interface{}{
			"error":   err.Error(),
			"message": string(message.Value),
		})
		return err
	}

	// 转发事件到WebSocket
	h.websocketService.HandleKafkaEvent(&event)

	// 根据事件类型进行特定处理
	switch event.Type {
	case kafka.TaskFailedEvent:
		h.handleTaskFailedEvent(&event)
	case kafka.TaskCompletedEvent:
		h.handleTaskCompletedEvent(&event)
	}

	return nil
}

// HandleAlertEvent 处理告警事件
func (h *EventHandler) HandleAlertEvent(message *kafka.Message) error {
	h.logger.Debug("Handling alert event", map[string]interface{}{
		"topic":   message.Topic,
		"message": string(message.Value),
	})

	var event kafka.Event
	if err := json.Unmarshal(message.Value, &event); err != nil {
		h.logger.Error("Failed to unmarshal alert event", map[string]interface{}{
			"error":   err.Error(),
			"message": string(message.Value),
		})
		return err
	}

	// 转发事件到WebSocket
	h.websocketService.HandleKafkaEvent(&event)

	return nil
}

// handleBatteryLowEvent 处理电量低告警
func (h *EventHandler) handleBatteryLowEvent(event *kafka.Event) {
	h.logger.Warning("Drone battery low detected", map[string]interface{}{
		"event_data": event.Data,
	})

	// 这里可以添加额外的处理逻辑：
	// 1. 发送紧急通知
	// 2. 自动触发返航
	// 3. 记录告警日志
}

// handleLocationUpdateEvent 处理位置更新事件
func (h *EventHandler) handleLocationUpdateEvent(event *kafka.Event) {
	// 可以添加额外的处理逻辑：
	// 1. 检查是否进入禁飞区
	// 2. 更新轨迹缓存
	// 3. 地理围栏检查

	h.logger.Debug("Drone location updated", map[string]interface{}{
		"event_data": event.Data,
	})
}

// handleStatusChangeEvent 处理状态变化事件
func (h *EventHandler) handleStatusChangeEvent(event *kafka.Event) {
	h.logger.Info("Drone status changed", map[string]interface{}{
		"event_data": event.Data,
	})

	// 可以添加额外的处理逻辑：
	// 1. 状态变化通知
	// 2. 统计数据更新
	// 3. 自动化响应
}

// handleTaskFailedEvent 处理任务失败事件
func (h *EventHandler) handleTaskFailedEvent(event *kafka.Event) {
	h.logger.Error("Task failed", map[string]interface{}{
		"event_data": event.Data,
	})

	// 可以添加额外的处理逻辑：
	// 1. 发送失败通知
	// 2. 自动重试逻辑
	// 3. 故障分析
}

// handleTaskCompletedEvent 处理任务完成事件
func (h *EventHandler) handleTaskCompletedEvent(event *kafka.Event) {
	h.logger.Info("Task completed", map[string]interface{}{
		"event_data": event.Data,
	})

	// 可以添加额外的处理逻辑：
	// 1. 发送完成通知
	// 2. 结果统计
	// 3. 后续任务调度
}
