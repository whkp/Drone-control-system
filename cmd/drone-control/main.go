package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"drone-control-system/pkg/database"
	"drone-control-system/pkg/llm"
	"drone-control-system/pkg/logger"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

// DroneController 无人机控制器
type DroneController struct {
	llmClient     *llm.Client
	cacheService  *database.CacheService
	logger        *logger.Logger
	connections   map[string]*websocket.Conn
	connectionsMu sync.RWMutex
	commands      chan DroneCommand
	heartbeats    chan DroneHeartbeat
}

// DroneCommand 无人机指令
type DroneCommand struct {
	DroneID    string                 `json:"drone_id"`
	Type       string                 `json:"type"`
	Command    string                 `json:"command"`
	Parameters map[string]interface{} `json:"parameters"`
	Timestamp  time.Time              `json:"timestamp"`
}

// DroneHeartbeat 无人机心跳
type DroneHeartbeat struct {
	DroneID   string     `json:"drone_id"`
	Status    string     `json:"status"`
	Position  Position   `json:"position"`
	Battery   int        `json:"battery"`
	Sensors   SensorData `json:"sensors"`
	Timestamp time.Time  `json:"timestamp"`
}

// Position 位置信息
type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Heading   float64 `json:"heading"`
}

// SensorData 传感器数据
type SensorData struct {
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	Pressure    float64 `json:"pressure"`
	WindSpeed   float64 `json:"wind_speed"`
	Visibility  float64 `json:"visibility"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境中应该检查Origin
	},
}

func main() {
	// 加载配置
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	appLogger := logger.NewLogger(logger.Config{
		Level:  config.GetString("logging.level"),
		Format: config.GetString("logging.format"),
		Output: config.GetString("logging.output"),
	})

	// 初始化Redis连接
	redisClient, err := database.NewRedisConnection(database.RedisConfig{
		Addr:         config.GetString("database.redis.addr"),
		Password:     config.GetString("database.redis.password"),
		DB:           config.GetInt("database.redis.db"),
		PoolSize:     config.GetInt("database.redis.pool_size"),
		MinIdleConns: config.GetInt("database.redis.min_idle_conns"),
	})
	if err != nil {
		appLogger.WithError(err).Warn("Failed to connect to Redis, using mock cache")
		// 在没有Redis的情况下，使用模拟缓存服务
	}

	var cacheService *database.CacheService
	if redisClient != nil {
		cacheService = database.NewCacheService(redisClient)
	}

	// 初始化LLM客户端
	llmClient := llm.NewClient(llm.Config{
		APIKey:      config.GetString("llm.deepseek.api_key"),
		BaseURL:     config.GetString("llm.deepseek.base_url"),
		Model:       config.GetString("llm.deepseek.model"),
		MaxTokens:   config.GetInt("llm.deepseek.max_tokens"),
		Temperature: float32(config.GetFloat64("llm.deepseek.temperature")),
	})

	// 创建无人机控制器
	controller := &DroneController{
		llmClient:    llmClient,
		cacheService: cacheService,
		logger:       appLogger,
		connections:  make(map[string]*websocket.Conn),
		commands:     make(chan DroneCommand, 1000),
		heartbeats:   make(chan DroneHeartbeat, 1000),
	}

	// 启动控制器服务
	go controller.commandProcessor()
	go controller.heartbeatProcessor()
	go controller.healthCheck()

	// 创建HTTP服务器
	mux := http.NewServeMux()

	// WebSocket端点
	mux.HandleFunc("/ws/drone", controller.handleDroneConnection)

	// HTTP API端点
	mux.HandleFunc("/api/command", controller.handleCommand)
	mux.HandleFunc("/api/status", controller.handleStatus)
	mux.HandleFunc("/api/tasks/execute", controller.handleTaskExecution)
	mux.HandleFunc("/health", controller.handleHealth)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.GetInt("grpc.drone_control")),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.WithError(err).Fatal("Failed to start drone control server")
		}
	}()

	appLogger.WithField("port", config.GetInt("grpc.drone_control")).Info("Drone Control Service started")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down drone control service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.WithError(err).Fatal("Server forced to shutdown")
	}

	appLogger.Info("Drone control service exited")
}

func loadConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../../configs")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return v, nil
}

// handleDroneConnection 处理无人机WebSocket连接
func (dc *DroneController) handleDroneConnection(w http.ResponseWriter, r *http.Request) {
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

	dc.logger.WithField("drone_id", droneID).Info("Drone connected")

	// 发送欢迎消息
	welcomeMsg := map[string]interface{}{
		"type":     "welcome",
		"message":  "Connected to drone control service",
		"drone_id": droneID,
	}
	conn.WriteJSON(welcomeMsg)

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

	// 清理连接
	dc.connectionsMu.Lock()
	delete(dc.connections, droneID)
	dc.connectionsMu.Unlock()

	dc.logger.WithField("drone_id", droneID).Info("Drone disconnected")
}

// handleCommand 处理HTTP命令请求
func (dc *DroneController) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cmd DroneCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	cmd.Timestamp = time.Now()

	// 发送命令到处理队列
	select {
	case dc.commands <- cmd:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "accepted",
			"command_id": fmt.Sprintf("%s_%d", cmd.DroneID, cmd.Timestamp.Unix()),
		})
	default:
		http.Error(w, "Command queue full", http.StatusServiceUnavailable)
	}
}

// handleTaskExecution 处理任务执行请求
func (dc *DroneController) handleTaskExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskID  uint   `json:"task_id"`
		DroneID string `json:"drone_id"`
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 使用LLM生成任务规划
	ctx := context.Background()

	// 构建环境状态（从缓存或数据库获取）
	envState := llm.EnvironmentState{
		DronePosition: llm.Position{Latitude: 40.7128, Longitude: -74.0060, Altitude: 0},
		Battery:       80,
		Weather: llm.WeatherInfo{
			WindSpeed:     5.0,
			WindDirection: 180,
			Visibility:    10.0,
			Temperature:   20.0,
			Humidity:      60.0,
		},
	}

	constraints := llm.PlanningConstraints{
		MaxAltitude:    120,
		MaxDistance:    5000,
		MaxFlightTime:  30,
		MinBattery:     20,
		SafetyDistance: 5.0,
	}

	planReq := llm.PlanningRequest{
		Command:     req.Command,
		Environment: envState,
		Constraints: constraints,
	}

	plan, err := dc.llmClient.GenerateTaskPlan(ctx, planReq)
	if err != nil {
		dc.logger.WithError(err).Error("Failed to generate task plan")
		// 如果LLM失败，返回简化的计划
		plan = &llm.TaskPlan{
			PlanID: fmt.Sprintf("plan_%d", time.Now().Unix()),
			Steps: []llm.TaskStep{
				{
					Action: "fly_to",
					Parameters: map[string]interface{}{
						"target": []float64{40.7128, -74.0060, 100},
					},
					Order: 1,
				},
				{
					Action: "capture",
					Parameters: map[string]interface{}{
						"mode": "photo",
					},
					Order: 2,
				},
				{
					Action:     "return_home",
					Parameters: map[string]interface{}{},
					Order:      3,
				},
			},
		}
	}

	// 发送任务到无人机
	taskCmd := DroneCommand{
		DroneID: req.DroneID,
		Type:    "execute_task",
		Command: "start_mission",
		Parameters: map[string]interface{}{
			"task_id": req.TaskID,
			"plan":    plan,
		},
		Timestamp: time.Now(),
	}

	dc.sendCommandToDrone(req.DroneID, taskCmd)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "task_started",
		"task_id": req.TaskID,
		"plan":    plan,
	})
}

// commandProcessor 处理命令队列
func (dc *DroneController) commandProcessor() {
	for cmd := range dc.commands {
		dc.logger.DroneLogger(0, "command_received", 0).
			WithField("command", cmd.Command).
			WithField("drone_id", cmd.DroneID).
			Info("Processing drone command")

		// 发送命令到指定无人机
		dc.sendCommandToDrone(cmd.DroneID, cmd)

		// 记录命令到缓存（如果可用）
		if dc.cacheService != nil {
			cmdJSON, _ := json.Marshal(cmd)
			key := fmt.Sprintf("drone:%s:commands:%d", cmd.DroneID, cmd.Timestamp.Unix())
			dc.cacheService.Set(context.Background(), key, cmdJSON, 24*time.Hour)
		}
	}
}

// heartbeatProcessor 处理心跳队列
func (dc *DroneController) heartbeatProcessor() {
	for heartbeat := range dc.heartbeats {
		dc.logger.DroneLogger(0, heartbeat.Status, heartbeat.Battery).
			WithField("drone_id", heartbeat.DroneID).
			Debug("Processing drone heartbeat")

		// 更新无人机状态到缓存（如果可用）
		if dc.cacheService != nil {
			hbJSON, _ := json.Marshal(heartbeat)
			key := fmt.Sprintf("drone:%s:status", heartbeat.DroneID)
			dc.cacheService.Set(context.Background(), key, hbJSON, 5*time.Minute)
		}

		// 检查异常状态
		if heartbeat.Battery < 20 {
			dc.logger.AlertLogger("battery", "warning", "drone-control").
				WithField("drone_id", heartbeat.DroneID).
				WithField("battery", heartbeat.Battery).
				Warn("Low battery alert")
		}
	}
}

// healthCheck 定期检查无人机健康状态
func (dc *DroneController) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		dc.connectionsMu.RLock()
		connCount := len(dc.connections)
		dc.connectionsMu.RUnlock()

		dc.logger.WithField("active_connections", connCount).Info("Health check")

		// 发送心跳请求到所有连接的无人机
		dc.connectionsMu.RLock()
		for droneID, conn := range dc.connections {
			pingMsg := map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Unix(),
			}
			if err := conn.WriteJSON(pingMsg); err != nil {
				dc.logger.WithError(err).WithField("drone_id", droneID).Error("Failed to send ping")
			}
		}
		dc.connectionsMu.RUnlock()
	}
}

// 辅助方法

func (dc *DroneController) sendCommandToDrone(droneID string, cmd DroneCommand) {
	dc.connectionsMu.RLock()
	conn, exists := dc.connections[droneID]
	dc.connectionsMu.RUnlock()

	if !exists {
		dc.logger.WithField("drone_id", droneID).Error("Drone not connected")
		return
	}

	if err := conn.WriteJSON(cmd); err != nil {
		dc.logger.WithError(err).WithField("drone_id", droneID).Error("Failed to send command")
	}
}

func (dc *DroneController) handleHeartbeatMessage(droneID string, message map[string]interface{}) {
	// 解析心跳数据
	var heartbeat DroneHeartbeat
	heartbeat.DroneID = droneID
	heartbeat.Timestamp = time.Now()

	if status, ok := message["status"].(string); ok {
		heartbeat.Status = status
	}
	if battery, ok := message["battery"].(float64); ok {
		heartbeat.Battery = int(battery)
	}
	if position, ok := message["position"].(map[string]interface{}); ok {
		if lat, ok := position["latitude"].(float64); ok {
			heartbeat.Position.Latitude = lat
		}
		if lng, ok := position["longitude"].(float64); ok {
			heartbeat.Position.Longitude = lng
		}
		if alt, ok := position["altitude"].(float64); ok {
			heartbeat.Position.Altitude = alt
		}
	}

	// 发送到心跳处理队列
	select {
	case dc.heartbeats <- heartbeat:
	default:
		dc.logger.Warn("Heartbeat queue full")
	}
}

func (dc *DroneController) handleStatusUpdate(droneID string, message map[string]interface{}) {
	dc.logger.WithField("drone_id", droneID).WithField("status", message).Info("Status update received")
}

func (dc *DroneController) handleTaskProgress(droneID string, message map[string]interface{}) {
	dc.logger.WithField("drone_id", droneID).WithField("progress", message).Info("Task progress received")
}

func (dc *DroneController) handleAlert(droneID string, message map[string]interface{}) {
	dc.logger.WithField("drone_id", droneID).WithField("alert", message).Warn("Alert received from drone")
}

func (dc *DroneController) handleStatus(w http.ResponseWriter, r *http.Request) {
	dc.connectionsMu.RLock()
	connCount := len(dc.connections)
	connectedDrones := make([]string, 0, len(dc.connections))
	for droneID := range dc.connections {
		connectedDrones = append(connectedDrones, droneID)
	}
	dc.connectionsMu.RUnlock()

	status := map[string]interface{}{
		"service":          "drone-control",
		"status":           "running",
		"connected_drones": connCount,
		"drone_list":       connectedDrones,
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (dc *DroneController) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"service":   "drone-control",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
