package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"drone-control-system/pkg/database"
	"drone-control-system/pkg/logger"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

type MonitoringData struct {
	DroneID       string    `json:"drone_id"`
	Status        string    `json:"status"`
	Position      Position  `json:"position"`
	Battery       float64   `json:"battery"`
	Speed         float64   `json:"speed"`
	Temperature   float64   `json:"temperature"`
	Timestamp     time.Time `json:"timestamp"`
	HeartbeatTime time.Time `json:"heartbeat_time"`
}

type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

type AlertData struct {
	AlertID      string    `json:"alert_id"`
	DroneID      string    `json:"drone_id"`
	Level        string    `json:"level"` // INFO, WARNING, ERROR, CRITICAL
	Type         string    `json:"type"`  // BATTERY_LOW, CONNECTION_LOST, POSITION_DRIFT, etc.
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

type MonitorService struct {
	upgrader      websocket.Upgrader
	connections   map[string]*websocket.Conn
	mutex         sync.RWMutex
	logger        *logger.Logger
	droneData     map[string]*MonitoringData
	alerts        []AlertData
	cacheService  *database.CacheService
	pubSubService *database.PubSubService
	queueService  *database.QueueService
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
		DialTimeout:  config.GetDuration("database.redis.dial_timeout"),
		ReadTimeout:  config.GetDuration("database.redis.read_timeout"),
		WriteTimeout: config.GetDuration("database.redis.write_timeout"),
		PoolTimeout:  config.GetDuration("database.redis.pool_timeout"),
		IdleTimeout:  config.GetDuration("database.redis.idle_timeout"),
	})
	if err != nil {
		appLogger.WithError(err).Warn("Failed to connect to Redis, using in-memory cache")
	}

	var cacheService *database.CacheService
	var pubSubService *database.PubSubService
	var queueService *database.QueueService
	if redisClient != nil {
		cacheService = database.NewCacheService(redisClient)
		pubSubService = database.NewPubSubService(redisClient)
		queueService = database.NewQueueService(redisClient)
		appLogger.Info("Redis cache services initialized")
	}

	// 创建监控服务
	service := &MonitorService{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境需要限制
			},
		},
		connections:   make(map[string]*websocket.Conn),
		logger:        appLogger,
		droneData:     make(map[string]*MonitoringData),
		alerts:        make([]AlertData, 0),
		cacheService:  cacheService,
		pubSubService: pubSubService,
		queueService:  queueService,
	}

	// 创建HTTP服务器
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"monitor-service","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// 监控端点
	mux.HandleFunc("/api/monitoring/drones", service.handleDroneMonitoring)
	mux.HandleFunc("/api/monitoring/drone/", service.handleSingleDrone)
	mux.HandleFunc("/api/monitoring/alerts", service.handleAlerts)
	mux.HandleFunc("/api/monitoring/metrics", service.handleMetrics)
	mux.HandleFunc("/ws/monitoring", service.handleWebSocket)

	srv := &http.Server{
		Addr:         ":50053", // 直接使用端口50053
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 启动数据收集器
	go service.startDataCollector()

	// 启动警报检查器
	go service.startAlertChecker()

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.WithError(err).Fatal("Failed to start monitor service")
		}
	}()

	appLogger.WithField("port", 50053).Info("Monitor Service started")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down monitor service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭Redis连接
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			appLogger.WithError(err).Error("Failed to close Redis connection")
		} else {
			appLogger.Info("Redis connection closed")
		}
	}

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.WithError(err).Fatal("Server forced to shutdown")
	}

	appLogger.Info("Monitor service exited")
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

func (s *MonitorService) handleDroneMonitoring(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 尝试从缓存获取无人机列表
		ctx := context.Background()
		cacheKey := "monitor:drones:list"

		if s.cacheService != nil {
			if cachedDrones, err := s.cacheService.Get(ctx, cacheKey); err == nil && cachedDrones != "" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.Write([]byte(cachedDrones))
				return
			}
		}

		// 缓存未命中，从内存获取
		s.mutex.RLock()
		droneList := make([]*MonitoringData, 0, len(s.droneData))
		for _, data := range s.droneData {
			droneList = append(droneList, data)
		}
		s.mutex.RUnlock()

		response := map[string]interface{}{
			"message": "无人机监控数据",
			"drones":  droneList,
			"count":   len(droneList),
		}

		// 将结果存入缓存
		if s.cacheService != nil {
			if responseBytes, err := json.Marshal(response); err == nil {
				s.cacheService.Set(ctx, cacheKey, string(responseBytes), 10*time.Second)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "MISS")
		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		var data MonitoringData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		data.Timestamp = time.Now()
		data.HeartbeatTime = time.Now()

		s.mutex.Lock()
		s.droneData[data.DroneID] = &data
		s.mutex.Unlock()

		// 更新单个无人机的缓存
		s.cacheDroneData(&data)

		// 清除列表缓存（因为数据已更新）
		s.invalidateDroneListCache()

		// 检查是否需要生成警报
		s.checkForAlerts(&data)

		// 广播更新到所有WebSocket连接
		s.broadcastUpdate(&data)

		// 发布实时更新事件
		s.publishDroneUpdate(&data)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"监控数据已更新"}`))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *MonitorService) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 尝试从缓存获取警报列表
		ctx := context.Background()
		cacheKey := "monitor:alerts:list"

		if s.cacheService != nil {
			if cachedAlerts, err := s.cacheService.Get(ctx, cacheKey); err == nil && cachedAlerts != "" {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.Write([]byte(cachedAlerts))
				return
			}
		}

		// 缓存未命中，从内存获取
		s.mutex.RLock()
		alertsCopy := make([]AlertData, len(s.alerts))
		copy(alertsCopy, s.alerts)
		s.mutex.RUnlock()

		response := map[string]interface{}{
			"message": "警报列表",
			"alerts":  alertsCopy,
			"count":   len(alertsCopy),
		}

		// 将结果存入缓存
		if s.cacheService != nil {
			if responseBytes, err := json.Marshal(response); err == nil {
				s.cacheService.Set(ctx, cacheKey, string(responseBytes), 30*time.Second)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "MISS")
		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		// 确认警报
		var req struct {
			AlertID string `json:"alert_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		s.mutex.Lock()
		alertFound := false
		for i := range s.alerts {
			if s.alerts[i].AlertID == req.AlertID {
				s.alerts[i].Acknowledged = true
				alertFound = true
				break
			}
		}
		s.mutex.Unlock()

		if alertFound {
			// 清除警报列表缓存
			s.invalidateAlertsCache()

			// 发布警报确认事件
			s.publishAlertAcknowledged(req.AlertID)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"警报已确认"}`))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSingleDrone 处理单个无人机数据请求
func (s *MonitorService) handleSingleDrone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取无人机ID
	droneID := r.URL.Path[len("/api/monitoring/drone/"):]
	if droneID == "" {
		http.Error(w, "Drone ID is required", http.StatusBadRequest)
		return
	}

	// 首先尝试从缓存获取
	if cachedData, err := s.getDroneFromCache(droneID); err == nil {
		response := map[string]interface{}{
			"message": "无人机监控数据",
			"drone":   cachedData,
			"source":  "cache",
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(response)
		return
	}

	// 缓存未命中，从内存获取
	s.mutex.RLock()
	data, exists := s.droneData[droneID]
	s.mutex.RUnlock()

	if !exists {
		http.Error(w, "Drone not found", http.StatusNotFound)
		return
	}

	// 更新缓存
	s.cacheDroneData(data)

	response := map[string]interface{}{
		"message": "无人机监控数据",
		"drone":   data,
		"source":  "memory",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(response)
}

func (s *MonitorService) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 尝试从缓存获取指标
	ctx := context.Background()
	cacheKey := "monitor:metrics:system"

	if s.cacheService != nil {
		if cachedMetrics, err := s.cacheService.Get(ctx, cacheKey); err == nil && cachedMetrics != "" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cachedMetrics))
			return
		}
	}

	// 缓存未命中，计算指标
	s.mutex.RLock()
	totalDrones := len(s.droneData)
	activeDrones := 0
	totalAlerts := len(s.alerts)
	unacknowledgedAlerts := 0

	for _, data := range s.droneData {
		if data.Status == "flying" || data.Status == "hovering" {
			activeDrones++
		}
	}

	for _, alert := range s.alerts {
		if !alert.Acknowledged {
			unacknowledgedAlerts++
		}
	}
	s.mutex.RUnlock()

	metrics := map[string]interface{}{
		"total_drones":          totalDrones,
		"active_drones":         activeDrones,
		"total_alerts":          totalAlerts,
		"unacknowledged_alerts": unacknowledgedAlerts,
		"system_health":         s.calculateSystemHealth(),
		"timestamp":             time.Now(),
		"uptime_seconds":        time.Since(time.Now().Add(-24 * time.Hour)).Seconds(),
		"average_battery_level": s.calculateAverageBattery(),
		"connection_count":      len(s.connections),
	}

	response := map[string]interface{}{
		"message": "系统指标",
		"metrics": metrics,
	}

	// 将结果存入缓存
	if s.cacheService != nil {
		if responseBytes, err := json.Marshal(response); err == nil {
			s.cacheService.Set(ctx, cacheKey, string(responseBytes), 30*time.Second)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(response)
}

func (s *MonitorService) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	s.mutex.Lock()
	s.connections[clientID] = conn
	s.mutex.Unlock()

	s.logger.WithField("client_id", clientID).Info("WebSocket client connected")

	// 发送当前数据
	s.sendCurrentData(conn)

	// 处理WebSocket消息
	go func() {
		defer func() {
			s.mutex.Lock()
			delete(s.connections, clientID)
			s.mutex.Unlock()
			conn.Close()
			s.logger.WithField("client_id", clientID).Info("WebSocket client disconnected")
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.WithError(err).Error("WebSocket error")
				}
				break
			}
		}
	}()
}

func (s *MonitorService) startDataCollector() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 模拟数据收集
		s.mutex.Lock()
		updatedDrones := []string{}

		for droneID, data := range s.droneData {
			// 更新心跳时间
			if time.Since(data.HeartbeatTime) < 30*time.Second {
				// 模拟数据变化
				if data.Status == "flying" {
					data.Battery -= 0.1
					data.Position.Altitude += float64((time.Now().Unix() % 3) - 1)
				}
				data.Timestamp = time.Now()
				s.droneData[droneID] = data
				updatedDrones = append(updatedDrones, droneID)

				// 更新单个无人机缓存
				s.cacheDroneData(data)
			}
		}
		s.mutex.Unlock()

		// 如果有数据更新，清除列表缓存
		if len(updatedDrones) > 0 {
			s.invalidateDroneListCache()
		}
	}
}

func (s *MonitorService) startAlertChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()
		newAlerts := []AlertData{}

		for droneID, data := range s.droneData {
			// 检查电池电量
			if data.Battery < 20 {
				alert := AlertData{
					AlertID:   fmt.Sprintf("battery_%s_%d", droneID, time.Now().Unix()),
					DroneID:   droneID,
					Level:     "WARNING",
					Type:      "BATTERY_LOW",
					Message:   fmt.Sprintf("无人机 %s 电池电量低: %.1f%%", droneID, data.Battery),
					Timestamp: time.Now(),
				}
				newAlerts = append(newAlerts, alert)
				s.queueAlert(alert) // 加入队列处理
			}

			// 检查连接状态
			if time.Since(data.HeartbeatTime) > 30*time.Second {
				alert := AlertData{
					AlertID:   fmt.Sprintf("connection_%s_%d", droneID, time.Now().Unix()),
					DroneID:   droneID,
					Level:     "ERROR",
					Type:      "CONNECTION_LOST",
					Message:   fmt.Sprintf("无人机 %s 连接丢失", droneID),
					Timestamp: time.Now(),
				}
				newAlerts = append(newAlerts, alert)
				s.queueAlert(alert) // 加入队列处理
			}
		}

		// 添加新警报
		if len(newAlerts) > 0 {
			s.alerts = append(s.alerts, newAlerts...)
			s.invalidateAlertsCache() // 清除警报缓存
			s.cacheAlertCounters()    // 更新警报计数器缓存
		}

		s.mutex.Unlock()
	}
}

func (s *MonitorService) checkForAlerts(data *MonitoringData) {
	// 实时警报检查逻辑已在 startAlertChecker 中实现
}

func (s *MonitorService) broadcastUpdate(data *MonitoringData) {
	message := map[string]interface{}{
		"type": "drone_update",
		"data": data,
	}

	s.mutex.RLock()
	for clientID, conn := range s.connections {
		if err := conn.WriteJSON(message); err != nil {
			s.logger.WithError(err).WithField("client_id", clientID).Error("Failed to send update")
		}
	}
	s.mutex.RUnlock()
}

func (s *MonitorService) sendCurrentData(conn *websocket.Conn) {
	s.mutex.RLock()
	droneList := make([]*MonitoringData, 0, len(s.droneData))
	for _, data := range s.droneData {
		droneList = append(droneList, data)
	}
	s.mutex.RUnlock()

	message := map[string]interface{}{
		"type":   "initial_data",
		"drones": droneList,
	}

	if err := conn.WriteJSON(message); err != nil {
		s.logger.WithError(err).Error("Failed to send initial data")
	}
}

func (s *MonitorService) calculateAverageBattery() float64 {
	if len(s.droneData) == 0 {
		return 0
	}

	total := 0.0
	for _, data := range s.droneData {
		total += data.Battery
	}
	return total / float64(len(s.droneData))
}

// calculateSystemHealth 计算系统健康状态
func (s *MonitorService) calculateSystemHealth() string {
	totalDrones := len(s.droneData)
	if totalDrones == 0 {
		return "unknown"
	}

	criticalAlerts := 0
	errorAlerts := 0
	lowBatteryCount := 0
	disconnectedCount := 0

	for _, alert := range s.alerts {
		if !alert.Acknowledged {
			switch alert.Level {
			case "CRITICAL":
				criticalAlerts++
			case "ERROR":
				errorAlerts++
			}
		}
	}

	for _, data := range s.droneData {
		if data.Battery < 20 {
			lowBatteryCount++
		}
		if time.Since(data.HeartbeatTime) > 30*time.Second {
			disconnectedCount++
		}
	}

	// 健康状态评估
	if criticalAlerts > 0 || disconnectedCount > totalDrones/2 {
		return "critical"
	}
	if errorAlerts > 2 || lowBatteryCount > totalDrones/3 {
		return "warning"
	}
	if errorAlerts > 0 || lowBatteryCount > 0 {
		return "good"
	}
	return "excellent"
}

// cacheDroneData 缓存单个无人机数据
func (s *MonitorService) cacheDroneData(data *MonitoringData) {
	if s.cacheService == nil {
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("monitor:drone:%s:data", data.DroneID)

	if dataBytes, err := json.Marshal(data); err == nil {
		s.cacheService.Set(ctx, key, string(dataBytes), 5*time.Minute)
	}
}

// invalidateDroneListCache 清除无人机列表缓存
func (s *MonitorService) invalidateDroneListCache() {
	if s.cacheService == nil {
		return
	}

	ctx := context.Background()
	s.cacheService.Delete(ctx, "monitor:drones:list")
	s.cacheService.Delete(ctx, "monitor:metrics:system") // 系统指标也需要更新
}

// invalidateAlertsCache 清除警报缓存
func (s *MonitorService) invalidateAlertsCache() {
	if s.cacheService == nil {
		return
	}

	ctx := context.Background()
	s.cacheService.Delete(ctx, "monitor:alerts:list")
	s.cacheService.Delete(ctx, "monitor:metrics:system") // 系统指标也需要更新
}

// publishDroneUpdate 发布无人机更新事件
func (s *MonitorService) publishDroneUpdate(data *MonitoringData) {
	if s.pubSubService == nil {
		return
	}

	ctx := context.Background()
	message := map[string]interface{}{
		"type":      "drone_update",
		"drone_id":  data.DroneID,
		"status":    data.Status,
		"battery":   data.Battery,
		"position":  data.Position,
		"timestamp": data.Timestamp,
	}

	if messageBytes, err := json.Marshal(message); err == nil {
		s.pubSubService.Publish(ctx, "drone:updates", string(messageBytes))
	}
}

// publishAlertAcknowledged 发布警报确认事件
func (s *MonitorService) publishAlertAcknowledged(alertID string) {
	if s.pubSubService == nil {
		return
	}

	ctx := context.Background()
	message := map[string]interface{}{
		"type":      "alert_acknowledged",
		"alert_id":  alertID,
		"timestamp": time.Now(),
	}

	if messageBytes, err := json.Marshal(message); err == nil {
		s.pubSubService.Publish(ctx, "alerts:updates", string(messageBytes))
	}
}

// cacheAlertCounters 缓存警报计数器
func (s *MonitorService) cacheAlertCounters() {
	if s.cacheService == nil {
		return
	}

	ctx := context.Background()

	// 统计不同类型的警报
	alertCounters := make(map[string]int)
	for _, alert := range s.alerts {
		if !alert.Acknowledged {
			alertCounters[alert.Type]++
		}
	}

	// 缓存各种计数器
	for alertType, count := range alertCounters {
		key := fmt.Sprintf("monitor:alerts:counter:%s", alertType)
		s.cacheService.Set(ctx, key, strconv.Itoa(count), 1*time.Hour)
	}
}

// getDroneFromCache 从缓存获取无人机数据
func (s *MonitorService) getDroneFromCache(droneID string) (*MonitoringData, error) {
	if s.cacheService == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	ctx := context.Background()
	key := fmt.Sprintf("monitor:drone:%s:data", droneID)

	cachedData, err := s.cacheService.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var data MonitoringData
	if err := json.Unmarshal([]byte(cachedData), &data); err != nil {
		return nil, err
	}

	return &data, nil
}

// queueAlert 将警报加入队列进行处理
func (s *MonitorService) queueAlert(alert AlertData) {
	if s.queueService == nil {
		return
	}

	ctx := context.Background()
	if alertBytes, err := json.Marshal(alert); err == nil {
		s.queueService.Push(ctx, "monitor:alerts:queue", string(alertBytes))
	}
}
