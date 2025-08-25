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
	logger         *logger.Logger
	kafkaManager   *kafka.Manager
	trafficManager *kafka.TrafficManager
	connections    map[string]*websocket.Conn
	connectionsMu  sync.RWMutex

	// 消息队列，实现解耦
	incomingMessages chan *IncomingMessage
	outgoingMessages chan *OutgoingMessage

	// 原有字段保持不变
	heartbeatChan chan HeartbeatMessage
	commandChan   chan CommandMessage
}

// IncomingMessage 入站消息
type IncomingMessage struct {
	DroneID     string
	MessageType string
	Data        map[string]interface{}
	Timestamp   time.Time
	ClientIP    string
}

// OutgoingMessage 出站消息
type OutgoingMessage struct {
	DroneID    string
	Command    string
	Parameters map[string]interface{}
	Priority   kafka.MessagePriority
	Timestamp  time.Time
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
	// 创建流量管理器
	trafficConfig := kafka.DefaultTrafficConfig()
	trafficManager := kafka.NewTrafficManager(logger, nil, trafficConfig) // producer暂时为nil

	return &DroneControllerWithKafka{
		logger:           logger,
		kafkaManager:     kafkaManager,
		trafficManager:   trafficManager,
		connections:      make(map[string]*websocket.Conn),
		incomingMessages: make(chan *IncomingMessage, 10000), // 1万消息缓冲
		outgoingMessages: make(chan *OutgoingMessage, 5000),  // 5千命令缓冲
		heartbeatChan:    make(chan HeartbeatMessage, 1000),
		commandChan:      make(chan CommandMessage, 1000),
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

	// 启动流量管理器
	dc.trafficManager.Start(ctx)

	// 启动消息处理器
	go dc.heartbeatProcessor(ctx)
	go dc.commandProcessor(ctx)
	go dc.incomingMessageProcessor(ctx)
	go dc.outgoingMessageProcessor(ctx)

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

	// 异步发布连接事件（不阻塞连接处理）
	go func() {
		if err := dc.trafficManager.PublishWithTrafficControl(
			context.Background(),
			kafka.DroneEventsTopic,
			connectEvent,
			kafka.PriorityNormal,
		); err != nil {
			dc.logger.WithError(err).Error("Failed to publish connect event")
		}
	}()

	dc.logger.WithField("drone_id", droneID).Info("Drone connected")

	// 处理消息 - 异步处理，避免阻塞
	for {
		var message map[string]interface{}
		err := conn.ReadJSON(&message)
		if err != nil {
			dc.logger.WithError(err).WithField("drone_id", droneID).Error("Failed to read message")
			break
		}

		// 将消息放入缓冲队列，实现解耦
		incomingMsg := &IncomingMessage{
			DroneID:     droneID,
			MessageType: getMessageType(message),
			Data:        message,
			Timestamp:   time.Now(),
			ClientIP:    r.RemoteAddr,
		}

		select {
		case dc.incomingMessages <- incomingMsg:
			// 成功入队
		default:
			// 队列满，记录丢弃
			dc.logger.WithField("drone_id", droneID).Warn("Message queue full, dropping message")
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

// incomingMessageProcessor 入站消息处理器（批量处理）
func (dc *DroneControllerWithKafka) incomingMessageProcessor(ctx context.Context) {
	batch := make([]*IncomingMessage, 0, 100)
	ticker := time.NewTicker(50 * time.Millisecond) // 50ms处理一批
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 处理剩余消息
			dc.processBatch(ctx, batch)
			return

		case msg := <-dc.incomingMessages:
			batch = append(batch, msg)

			// 批次满了立即处理
			if len(batch) >= 100 {
				dc.processBatch(ctx, batch)
				batch = batch[:0] // 清空批次
			}

		case <-ticker.C:
			// 定时处理批次
			if len(batch) > 0 {
				dc.processBatch(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

// processBatch 批量处理消息
func (dc *DroneControllerWithKafka) processBatch(ctx context.Context, batch []*IncomingMessage) {
	if len(batch) == 0 {
		return
	}

	startTime := time.Now()

	// 按消息类型分组处理
	messageGroups := dc.groupMessagesByType(batch)

	for msgType, messages := range messageGroups {
		switch msgType {
		case "heartbeat":
			dc.batchProcessHeartbeats(ctx, messages)
		case "status_update":
			dc.batchProcessStatusUpdates(ctx, messages)
		case "alert":
			dc.batchProcessAlerts(ctx, messages)
		default:
			dc.processGenericMessages(ctx, messages)
		}
	}

	dc.logger.WithField("batch_size", len(batch)).
		WithField("processing_time", time.Since(startTime)).
		Debug("Batch processed")
}

// batchProcessHeartbeats 批量处理心跳消息
func (dc *DroneControllerWithKafka) batchProcessHeartbeats(ctx context.Context, messages []*IncomingMessage) {
	heartbeats := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		heartbeats = append(heartbeats, map[string]interface{}{
			"drone_id":  msg.DroneID,
			"data":      msg.Data,
			"timestamp": msg.Timestamp,
		})
	}

	// 批量发布心跳事件
	batchEvent := kafka.NewEvent(
		"drone.heartbeat.batch",
		"drone-control-service",
		map[string]interface{}{
			"heartbeats": heartbeats,
			"count":      len(heartbeats),
		},
	)

	// 心跳是普通优先级
	if err := dc.trafficManager.PublishWithTrafficControl(
		ctx,
		kafka.DroneEventsTopic,
		batchEvent,
		kafka.PriorityNormal,
	); err != nil {
		dc.logger.WithError(err).Error("Failed to publish batch heartbeats")
	}
}

// batchProcessStatusUpdates 批量处理状态更新
func (dc *DroneControllerWithKafka) batchProcessStatusUpdates(ctx context.Context, messages []*IncomingMessage) {
	for _, msg := range messages {
		dc.handleStatusUpdate(msg.DroneID, msg.Data)
	}
}

// batchProcessAlerts 批量处理告警消息
func (dc *DroneControllerWithKafka) batchProcessAlerts(ctx context.Context, messages []*IncomingMessage) {
	for _, msg := range messages {
		alertEvent := kafka.NewEvent(
			kafka.AlertCreatedEvent,
			"drone-control-service",
			map[string]interface{}{
				"drone_id":  msg.DroneID,
				"alert":     msg.Data,
				"client_ip": msg.ClientIP,
			},
		)

		// 告警是高优先级
		if err := dc.trafficManager.PublishWithTrafficControl(
			ctx,
			kafka.AlertEventsTopic,
			alertEvent,
			kafka.PriorityHigh,
		); err != nil {
			dc.logger.WithError(err).Error("Failed to publish alert")
		}
	}
}

// processGenericMessages 处理通用消息
func (dc *DroneControllerWithKafka) processGenericMessages(ctx context.Context, messages []*IncomingMessage) {
	for _, msg := range messages {
		switch msg.MessageType {
		case "task_progress":
			dc.handleTaskProgress(msg.DroneID, msg.Data)
		default:
			dc.logger.WithField("type", msg.MessageType).Warn("Unknown message type")
		}
	}
}

// outgoingMessageProcessor 出站消息处理器
func (dc *DroneControllerWithKafka) outgoingMessageProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-dc.outgoingMessages:
			dc.sendCommandToDrone(ctx, msg)
		}
	}
}

// SendCommand 发送命令（异步）
func (dc *DroneControllerWithKafka) SendCommand(droneID, command string, params map[string]interface{}, priority kafka.MessagePriority) error {
	outgoingMsg := &OutgoingMessage{
		DroneID:    droneID,
		Command:    command,
		Parameters: params,
		Priority:   priority,
		Timestamp:  time.Now(),
	}

	select {
	case dc.outgoingMessages <- outgoingMsg:
		return nil
	default:
		return fmt.Errorf("outgoing message queue full")
	}
}

// GetTrafficStats 获取流量统计
func (dc *DroneControllerWithKafka) GetTrafficStats() *kafka.TrafficStats {
	return dc.trafficManager.GetStats()
}

// 辅助函数
func parseUint(s string) uint {
	// 简单的字符串转uint实现
	// 实际项目中应该使用 strconv.ParseUint
	return 1 // 示例返回值
}

func getMessageType(message map[string]interface{}) string {
	if msgType, ok := message["type"].(string); ok {
		return msgType
	}
	return "unknown"
}

func (dc *DroneControllerWithKafka) groupMessagesByType(messages []*IncomingMessage) map[string][]*IncomingMessage {
	groups := make(map[string][]*IncomingMessage)

	for _, msg := range messages {
		msgType := msg.MessageType
		groups[msgType] = append(groups[msgType], msg)
	}

	return groups
}

func (dc *DroneControllerWithKafka) sendCommandToDrone(ctx context.Context, msg *OutgoingMessage) {
	dc.connectionsMu.RLock()
	conn, exists := dc.connections[msg.DroneID]
	dc.connectionsMu.RUnlock()

	if !exists {
		dc.logger.WithField("drone_id", msg.DroneID).Error("Drone not connected")
		return
	}

	command := map[string]interface{}{
		"command":    msg.Command,
		"parameters": msg.Parameters,
		"timestamp":  msg.Timestamp,
	}

	if err := conn.WriteJSON(command); err != nil {
		dc.logger.WithError(err).WithField("drone_id", msg.DroneID).Error("Failed to send command")
	}
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

	// 添加流量统计API
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := controller.GetTrafficStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		statsJSON := fmt.Sprintf(`{
			"total_messages": %d,
			"buffered_messages": %d,
			"dropped_messages": %d,
			"throughput_per_sec": %.2f,
			"current_queue_size": %d,
			"avg_processing_time_ms": %.2f
		}`,
			stats.TotalMessages,
			stats.BufferedMessages,
			stats.DroppedMessages,
			stats.ThroughputPerSec,
			stats.CurrentQueueSize,
			float64(stats.AvgProcessingTime.Nanoseconds())/1000000,
		)
		w.Write([]byte(statsJSON))
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
