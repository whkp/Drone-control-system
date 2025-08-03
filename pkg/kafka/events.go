package kafka

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// EventType 事件类型
type EventType string

const (
	// 无人机事件
	DroneConnectedEvent       EventType = "drone.connected"
	DroneDisconnectedEvent    EventType = "drone.disconnected"
	DroneStatusChangedEvent   EventType = "drone.status.changed"
	DroneBatteryLowEvent      EventType = "drone.battery.low"
	DroneLocationUpdatedEvent EventType = "drone.location.updated"

	// 任务事件
	TaskCreatedEvent   EventType = "task.created"
	TaskScheduledEvent EventType = "task.scheduled"
	TaskStartedEvent   EventType = "task.started"
	TaskProgressEvent  EventType = "task.progress"
	TaskCompletedEvent EventType = "task.completed"
	TaskFailedEvent    EventType = "task.failed"
	TaskCancelledEvent EventType = "task.cancelled"

	// 用户事件
	UserLoggedInEvent  EventType = "user.logged.in"
	UserLoggedOutEvent EventType = "user.logged.out"
	UserCreatedEvent   EventType = "user.created"
	UserUpdatedEvent   EventType = "user.updated"
	UserDeletedEvent   EventType = "user.deleted"

	// 告警事件
	AlertCreatedEvent      EventType = "alert.created"
	AlertAcknowledgedEvent EventType = "alert.acknowledged"
	AlertResolvedEvent     EventType = "alert.resolved"

	// 系统事件
	SystemHealthCheckEvent EventType = "system.health.check"
	SystemMetricsEvent     EventType = "system.metrics"
)

// Topics Kafka主题定义
const (
	DroneEventsTopic  = "drone-events"
	TaskEventsTopic   = "task-events"
	UserEventsTopic   = "user-events"
	AlertEventsTopic  = "alert-events"
	SystemEventsTopic = "system-events"
	MonitoringTopic   = "monitoring-data"
	LogsTopic         = "application-logs"
)

// Event 基础事件结构
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// DroneStatusChangedEventData 无人机状态变化事件数据
type DroneStatusChangedEventData struct {
	DroneID   uint      `json:"drone_id"`
	DroneName string    `json:"drone_name"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	Reason    string    `json:"reason,omitempty"`
	Location  *Location `json:"location,omitempty"`
	Battery   int       `json:"battery"`
	Timestamp time.Time `json:"timestamp"`
}

// Location 位置信息
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Heading   float64 `json:"heading"`
}

// TaskProgressEventData 任务进度事件数据
type TaskProgressEventData struct {
	TaskID      uint      `json:"task_id"`
	TaskName    string    `json:"task_name"`
	DroneID     uint      `json:"drone_id"`
	Progress    int       `json:"progress"`
	Status      string    `json:"status"`
	CurrentStep string    `json:"current_step,omitempty"`
	Location    *Location `json:"location,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// AlertCreatedEventData 告警创建事件数据
type AlertCreatedEventData struct {
	AlertID   uint      `json:"alert_id"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	DroneID   *uint     `json:"drone_id,omitempty"`
	TaskID    *uint     `json:"task_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// UserActionEventData 用户操作事件数据
type UserActionEventData struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// SystemMetricsEventData 系统指标事件数据
type SystemMetricsEventData struct {
	Service   string             `json:"service"`
	Metrics   map[string]float64 `json:"metrics"`
	Labels    map[string]string  `json:"labels,omitempty"`
	Timestamp time.Time          `json:"timestamp"`
}

// LogEventData 日志事件数据
type LogEventData struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Service   string                 `json:"service"`
	TraceID   string                 `json:"trace_id,omitempty"`
	UserID    *uint                  `json:"user_id,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, source string, data interface{}) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Version:   "1.0",
		Data:      structToMap(data),
		Metadata:  make(map[string]string),
	}
}

// AddMetadata 添加元数据
func (e *Event) AddMetadata(key, value string) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
}

// generateEventID 生成事件ID
func generateEventID() string {
	// 使用时间戳 + 随机数生成事件ID
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}

// structToMap 将结构体转换为map
func structToMap(data interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 使用JSON进行序列化和反序列化来转换
	bytes, err := json.Marshal(data)
	if err != nil {
		return result
	}

	json.Unmarshal(bytes, &result)
	return result
}
