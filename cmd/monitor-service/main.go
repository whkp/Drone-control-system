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
	AlertID     string    `json:"alert_id"`
	DroneID     string    `json:"drone_id"`
	Level       string    `json:"level"`       // INFO, WARNING, ERROR, CRITICAL
	Type        string    `json:"type"`        // BATTERY_LOW, CONNECTION_LOST, POSITION_DRIFT, etc.
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Acknowledged bool     `json:"acknowledged"`
}

type MonitorService struct {
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mutex       sync.RWMutex
	logger      *logger.Logger
	droneData   map[string]*MonitoringData
	alerts      []AlertData
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

	// 创建监控服务
	service := &MonitorService{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境需要限制
			},
		},
		connections: make(map[string]*websocket.Conn),
		logger:      appLogger,
		droneData:   make(map[string]*MonitoringData),
		alerts:      make([]AlertData, 0),
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
	mux.HandleFunc("/api/monitoring/alerts", service.handleAlerts)
	mux.HandleFunc("/api/monitoring/metrics", service.handleMetrics)
	mux.HandleFunc("/ws/monitoring", service.handleWebSocket)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.GetInt("grpc.monitor_service")),
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

	appLogger.WithField("port", config.GetInt("grpc.monitor_service")).Info("Monitor Service started")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down monitor service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
		s.mutex.RLock()
		droneList := make([]*MonitoringData, 0, len(s.droneData))
		for _, data := range s.droneData {
			droneList = append(droneList, data)
		}
		s.mutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "无人机监控数据",
			"drones":  droneList,
			"count":   len(droneList),
		})
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

		// 检查是否需要生成警报
		s.checkForAlerts(&data)

		// 广播更新到所有WebSocket连接
		s.broadcastUpdate(&data)

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
		s.mutex.RLock()
		alertsCopy := make([]AlertData, len(s.alerts))
		copy(alertsCopy, s.alerts)
		s.mutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "警报列表",
			"alerts":  alertsCopy,
			"count":   len(alertsCopy),
		})
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
		for i := range s.alerts {
			if s.alerts[i].AlertID == req.AlertID {
				s.alerts[i].Acknowledged = true
				break
			}
		}
		s.mutex.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"警报已确认"}`))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *MonitorService) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
		"total_drones":            totalDrones,
		"active_drones":           activeDrones,
		"total_alerts":            totalAlerts,
		"unacknowledged_alerts":   unacknowledgedAlerts,
		"system_health":           "good",
		"timestamp":               time.Now(),
		"uptime_seconds":          time.Since(time.Now().Add(-24 * time.Hour)).Seconds(),
		"average_battery_level":   s.calculateAverageBattery(),
		"connection_count":        len(s.connections),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "系统指标",
		"metrics": metrics,
	})
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
			}
		}
		s.mutex.Unlock()
	}
}

func (s *MonitorService) startAlertChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()
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
				s.alerts = append(s.alerts, alert)
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
				s.alerts = append(s.alerts, alert)
			}
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
