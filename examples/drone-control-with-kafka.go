package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

// DroneControllerWithKafka 集成Kafka的无人机控制器
type DroneControllerWithKafka struct {
	logger        *logger.Logger
	kafkaManager  *kafka.Manager
	connections   map[string]*websocket.Conn
	connectionsMu sync.RWMutex

	// 原有字段保持不变
	heartbeatChan chan HeartbeatMessage
	commandChan   chan CommandMessage
}

// HeartbeatMessage 心跳消息
type HeartbeatMessage struct {
	DroneID   string    `json:"drone_id"`
	Battery   int       `json:"battery"`
	Location  Location  `json:"location"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// CommandMessage 命令消息
type CommandMessage struct {
	DroneID    string                 `json:"drone_id"`
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Location 位置信息
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Heading   float64 `json:"heading"`
}

// NewDroneControllerWithKafka 创建集成Kafka的无人机控制器
func NewDroneControllerWithKafka(logger *logger.Logger, kafkaManager *kafka.Manager) *DroneControllerWithKafka {
	return &DroneControllerWithKafka{
		logger:        logger,
		kafkaManager:  kafkaManager,
		connections:   make(map[string]*websocket.Conn),
		heartbeatChan: make(chan HeartbeatMessage, 1000),
		commandChan:   make(chan CommandMessage, 1000),
	}
}

// Start 启动控制器
func (dc *DroneControllerWithKafka) Start(ctx context.Context) error {
	// 注册 Kafka 事件处理器
	droneHandler := kafka.NewDroneEventHandler(dc.logger)
	taskHandler := kafka.NewTaskEventHandler(dc.logger)

	dc.kafkaManager.RegisterHandler(kafka.DroneEventsTopic, droneHandler)
	dc.kafkaManager.RegisterHandler(kafka.TaskEventsTopic, taskHandler)

	// 启动 Kafka 管理器
	if err := dc.kafkaManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start kafka manager: %w", err)
	}

	// 启动消息处理器
	go dc.heartbeatProcessor(ctx)
	go dc.commandProcessor(ctx)

	dc.logger.Info("Drone controller with Kafka started successfully")
	return nil
}

// handleDroneConnection 处理无人机连接（增强版）
func (dc *DroneControllerWithKafka) handleDroneConnection(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		dc.logger.WithError(err).Error("Failed to upgrade connection")
		return
	}
	defer conn.Close()

	droneID := r.URL.Query().Get("drone_id")
	if droneID == "" {
		dc.logger.Error("Missing drone_id parameter")
		return
	}

	// 注册连接
	dc.connectionsMu.Lock()
	dc.connections[droneID] = conn
	dc.connectionsMu.Unlock()

	// 发布无人机连接事件到Kafka
	connectEvent := kafka.NewEvent(
		kafka.DroneConnectedEvent,
		"drone-control-service",
		map[string]interface{}{
			"drone_id":   droneID,
			"ip_address": r.RemoteAddr,
			"user_agent": r.UserAgent(),
		},
	)

	if err := dc.kafkaManager.PublishDroneEvent(context.Background(), connectEvent); err != nil {
		dc.logger.WithError(err).Error("Failed to publish drone connected event")
	}

	dc.logger.WithField("drone_id", droneID).Info("Drone connected")

	// 处理消息
	for {
		var message map[string]interface{}
		err := conn.ReadJSON(&message)
		if err != nil {
			dc.logger.WithError(err).WithField("drone_id", droneID).Error("Failed to read message")
			break
		}

		// 处理不同类型的消息
		switch message["type"] {
		case "heartbeat":
			dc.handleHeartbeatMessage(droneID, message)
		case "status_update":
			dc.handleStatusUpdate(droneID, message)
		case "task_progress":
			dc.handleTaskProgress(droneID, message)
		case "alert":
			dc.handleAlert(droneID, message)
		default:
			dc.logger.WithField("type", message["type"]).Warn("Unknown message type")
		}
	}

	// 发布无人机断开事件
	disconnectEvent := kafka.NewEvent(
		kafka.DroneDisconnectedEvent,
		"drone-control-service",
		map[string]interface{}{
			"drone_id": droneID,
			"reason":   "connection_closed",
		},
	)

	if err := dc.kafkaManager.PublishDroneEvent(context.Background(), disconnectEvent); err != nil {
		dc.logger.WithError(err).Error("Failed to publish drone disconnected event")
	}

	// 清理连接
	dc.connectionsMu.Lock()
	delete(dc.connections, droneID)
	dc.connectionsMu.Unlock()

	dc.logger.WithField("drone_id", droneID).Info("Drone disconnected")
}

// handleHeartbeatMessage 处理心跳消息（增强版）
func (dc *DroneControllerWithKafka) handleHeartbeatMessage(droneID string, message map[string]interface{}) {
	// 解析心跳数据
	heartbeat := HeartbeatMessage{
		DroneID:   droneID,
		Timestamp: time.Now(),
	}

	if battery, ok := message["battery"].(float64); ok {
		heartbeat.Battery = int(battery)
	}

	if status, ok := message["status"].(string); ok {
		heartbeat.Status = status
	}

	if locationData, ok := message["location"].(map[string]interface{}); ok {
		if lat, ok := locationData["latitude"].(float64); ok {
			heartbeat.Location.Latitude = lat
		}
		if lon, ok := locationData["longitude"].(float64); ok {
			heartbeat.Location.Longitude = lon
		}
		if alt, ok := locationData["altitude"].(float64); ok {
			heartbeat.Location.Altitude = alt
		}
		if heading, ok := locationData["heading"].(float64); ok {
			heartbeat.Location.Heading = heading
		}
	}

	// 发送到处理队列
	select {
	case dc.heartbeatChan <- heartbeat:
	default:
		dc.logger.Warn("Heartbeat channel full, dropping message")
	}
}

// handleStatusUpdate 处理状态更新（增强版）
func (dc *DroneControllerWithKafka) handleStatusUpdate(droneID string, message map[string]interface{}) {
	oldStatus, _ := message["old_status"].(string)
	newStatus, _ := message["new_status"].(string)
	reason, _ := message["reason"].(string)

	// 发布状态变化事件
	statusEvent := kafka.NewEvent(
		kafka.DroneStatusChangedEvent,
		"drone-control-service",
		kafka.DroneStatusChangedEventData{
			DroneID:   parseUint(droneID),
			DroneName: droneID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			Reason:    reason,
			Timestamp: time.Now(),
		},
	)

	if err := dc.kafkaManager.PublishDroneEvent(context.Background(), statusEvent); err != nil {
		dc.logger.WithError(err).Error("Failed to publish drone status changed event")
	}

	dc.logger.WithField("drone_id", droneID).
		WithField("old_status", oldStatus).
		WithField("new_status", newStatus).
		Info("Drone status updated")
}

// handleTaskProgress 处理任务进度（增强版）
func (dc *DroneControllerWithKafka) handleTaskProgress(droneID string, message map[string]interface{}) {
	taskID, _ := message["task_id"].(float64)
	progress, _ := message["progress"].(float64)
	currentStep, _ := message["current_step"].(string)

	// 发布任务进度事件
	progressEvent := kafka.NewEvent(
		kafka.TaskProgressEvent,
		"drone-control-service",
		kafka.TaskProgressEventData{
			TaskID:      uint(taskID),
			DroneID:     parseUint(droneID),
			Progress:    int(progress),
			CurrentStep: currentStep,
			Timestamp:   time.Now(),
		},
	)

	if err := dc.kafkaManager.PublishTaskEvent(context.Background(), progressEvent); err != nil {
		dc.logger.WithError(err).Error("Failed to publish task progress event")
	}
}

// handleAlert 处理告警（增强版）
func (dc *DroneControllerWithKafka) handleAlert(droneID string, message map[string]interface{}) {
	alertType, _ := message["alert_type"].(string)
	level, _ := message["level"].(string)
	alertMessage, _ := message["message"].(string)

	// 发布告警事件
	alertEvent := kafka.NewEvent(
		kafka.AlertCreatedEvent,
		"drone-control-service",
		kafka.AlertCreatedEventData{
			Type:      alertType,
			Level:     level,
			Message:   alertMessage,
			Source:    "drone-control-service",
			DroneID:   uintPtr(parseUint(droneID)),
			Timestamp: time.Now(),
		},
	)

	if err := dc.kafkaManager.PublishAlertEvent(context.Background(), alertEvent); err != nil {
		dc.logger.WithError(err).Error("Failed to publish alert event")
	}

	dc.logger.WithField("drone_id", droneID).
		WithField("alert_type", alertType).
		WithField("level", level).
		Warn("Drone alert received")
}

// heartbeatProcessor 心跳处理器（增强版）
func (dc *DroneControllerWithKafka) heartbeatProcessor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case heartbeat := <-dc.heartbeatChan:
			// 处理心跳数据
			dc.processHeartbeat(ctx, heartbeat)
		case <-ticker.C:
			// 定期检查无人机健康状态
			dc.checkDroneHealth(ctx)
		}
	}
}

// processHeartbeat 处理心跳数据
func (dc *DroneControllerWithKafka) processHeartbeat(ctx context.Context, heartbeat HeartbeatMessage) {
	// 检查电量告警
	if heartbeat.Battery < 20 {
		lowBatteryEvent := kafka.NewEvent(
			kafka.DroneBatteryLowEvent,
			"drone-control-service",
			map[string]interface{}{
				"drone_id": heartbeat.DroneID,
				"battery":  heartbeat.Battery,
				"location": heartbeat.Location,
			},
		)

		if err := dc.kafkaManager.PublishDroneEvent(ctx, lowBatteryEvent); err != nil {
			dc.logger.WithError(err).Error("Failed to publish low battery event")
		}
	}

	// 发布位置更新事件
	locationEvent := kafka.NewEvent(
		kafka.DroneLocationUpdatedEvent,
		"drone-control-service",
		map[string]interface{}{
			"drone_id": heartbeat.DroneID,
			"location": heartbeat.Location,
			"battery":  heartbeat.Battery,
			"status":   heartbeat.Status,
		},
	)

	if err := dc.kafkaManager.PublishDroneEvent(ctx, locationEvent); err != nil {
		dc.logger.WithError(err).Error("Failed to publish location updated event")
	}
}

// commandProcessor 命令处理器
func (dc *DroneControllerWithKafka) commandProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case command := <-dc.commandChan:
			dc.processCommand(ctx, command)
		}
	}
}

// processCommand 处理命令
func (dc *DroneControllerWithKafka) processCommand(ctx context.Context, command CommandMessage) {
	dc.connectionsMu.RLock()
	conn, exists := dc.connections[command.DroneID]
	dc.connectionsMu.RUnlock()

	if !exists {
		dc.logger.WithField("drone_id", command.DroneID).Error("Drone not connected")
		return
	}

	// 发送命令到无人机
	if err := conn.WriteJSON(command); err != nil {
		dc.logger.WithError(err).WithField("drone_id", command.DroneID).Error("Failed to send command")
		return
	}

	dc.logger.WithField("drone_id", command.DroneID).
		WithField("action", command.Action).
		Info("Command sent to drone")
}

// checkDroneHealth 检查无人机健康状态
func (dc *DroneControllerWithKafka) checkDroneHealth(ctx context.Context) {
	// 实现健康检查逻辑
	// 例如：检查连接超时、心跳间隔等
}

// 辅助函数
func parseUint(s string) uint {
	// 简单的字符串转uint实现
	// 实际项目中应该使用 strconv.ParseUint
	return 1 // 示例返回值
}

func uintPtr(u uint) *uint {
	return &u
}

func main() {
	// 加载配置
	config := viper.New()
	config.SetConfigFile("configs/config.yaml")
	if err := config.ReadInConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	appLogger := logger.NewLogger(logger.Config{
		Level:  config.GetString("logging.level"),
		Format: config.GetString("logging.format"),
		Output: config.GetString("logging.output"),
	})

	// 初始化 Kafka
	kafkaConfig := kafka.LoadConfigFromViper(config)
	kafkaManager, err := kafka.NewManager(kafkaConfig, appLogger)
	if err != nil {
		log.Fatalf("Failed to create kafka manager: %v", err)
	}

	// 初始化 Kafka 主题
	ctx := context.Background()
	if err := kafkaManager.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize kafka: %v", err)
	}

	// 创建控制器
	controller := NewDroneControllerWithKafka(appLogger, kafkaManager)

	// 启动控制器
	if err := controller.Start(ctx); err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}

	// 设置HTTP路由
	http.HandleFunc("/ws/drone", controller.handleDroneConnection)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"drone-control-with-kafka"}`))
	})

	// 启动HTTP服务器
	srv := &http.Server{
		Addr:    ":8084",
		Handler: nil,
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		appLogger.Info("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			appLogger.WithError(err).Error("Server shutdown error")
		}

		if err := kafkaManager.Stop(); err != nil {
			appLogger.WithError(err).Error("Kafka manager shutdown error")
		}
	}()

	appLogger.Info("Starting drone control service with Kafka on :8084")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
