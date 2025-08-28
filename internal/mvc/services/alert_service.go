package services

import (
	"fmt"
	"sync"
	"time"

	"drone-control-system/internal/mvc/models"
	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"
)

// AlertPattern 告警模式
type AlertPattern struct {
	DroneID   uint                   `json:"drone_id"`
	AlertType string                 `json:"alert_type"`
	Count     int                    `json:"count"`
	FirstSeen time.Time              `json:"first_seen"`
	LastSeen  time.Time              `json:"last_seen"`
	Severity  string                 `json:"severity"` // "low", "medium", "high", "critical"
	Metadata  map[string]interface{} `json:"metadata"`
}

// EventPattern 事件模式分析结果
type EventPattern struct {
	BatteryDrainRate  float64                  `json:"battery_drain_rate"`
	LocationAnomalies []LocationAnomaly        `json:"location_anomalies"`
	AlertPatterns     map[string]*AlertPattern `json:"alert_patterns"`
	SystemHealthScore float64                  `json:"system_health_score"`
	PredictedIssues   []PredictedIssue         `json:"predicted_issues"`
}

// LocationAnomaly 位置异常
type LocationAnomaly struct {
	DroneID     uint      `json:"drone_id"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	AnomalyType string    `json:"anomaly_type"` // "speed_anomaly", "zone_violation", "trajectory_deviation"
	Timestamp   time.Time `json:"timestamp"`
	Severity    string    `json:"severity"`
}

// PredictedIssue 预测问题
type PredictedIssue struct {
	Type        string        `json:"type"`
	DroneID     uint          `json:"drone_id"`
	Probability float64       `json:"probability"`
	TimeToIssue time.Duration `json:"time_to_issue"`
	Description string        `json:"description"`
}

// SmartAlertService 智能告警服务接口
type SmartAlertService interface {
	// 事件处理
	ProcessEvents(events []kafka.Event) (*EventPattern, error)
	AnalyzeEventPatterns(events []kafka.Event) (*EventPattern, error)

	// 预测性告警
	PredictBatteryDrain(droneID uint, events []kafka.Event) (*PredictedIssue, error)
	PredictMaintenanceNeeds(droneID uint, events []kafka.Event) (*PredictedIssue, error)

	// 告警聚合
	AggregateAlerts(alerts []models.Alert) ([]models.Alert, error)

	// 告警抑制（防止告警风暴）
	SuppressAlerts(alerts []models.Alert) ([]models.Alert, error)
}

// AlertServiceImpl 智能告警服务实现
type AlertServiceImpl struct {
	logger       *logger.Logger
	kafkaService KafkaService

	// 缓存和状态
	alertPatterns   map[string]*AlertPattern
	lastEventTime   map[uint]time.Time         // 每个无人机的最后事件时间
	batteryHistory  map[uint][]BatteryReading  // 电量历史
	locationHistory map[uint][]LocationReading // 位置历史

	mu sync.RWMutex
}

// BatteryReading 电量读数
type BatteryReading struct {
	DroneID   uint      `json:"drone_id"`
	Battery   int       `json:"battery"`
	Timestamp time.Time `json:"timestamp"`
}

// LocationReading 位置读数
type LocationReading struct {
	DroneID   uint      `json:"drone_id"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	Speed     float64   `json:"speed"`
	Timestamp time.Time `json:"timestamp"`
}

// NewAlertService 创建智能告警服务
func NewSmartAlertService(logger *logger.Logger, kafkaService KafkaService) SmartAlertService {
	return &AlertServiceImpl{
		logger:          logger,
		kafkaService:    kafkaService,
		alertPatterns:   make(map[string]*AlertPattern),
		lastEventTime:   make(map[uint]time.Time),
		batteryHistory:  make(map[uint][]BatteryReading),
		locationHistory: make(map[uint][]LocationReading),
	}
}

// ProcessEvents 处理事件批次
func (s *AlertServiceImpl) ProcessEvents(events []kafka.Event) (*EventPattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pattern := &EventPattern{
		AlertPatterns:   make(map[string]*AlertPattern),
		PredictedIssues: []PredictedIssue{},
	}

	// 分析事件
	for _, event := range events {
		switch event.Type {
		case kafka.DroneLocationUpdatedEvent:
			s.processLocationEvent(event, pattern)
		case kafka.DroneBatteryLowEvent:
			s.processBatteryEvent(event, pattern)
		case kafka.DroneStatusChangedEvent:
			s.processStatusEvent(event, pattern)
		case kafka.AlertCreatedEvent:
			s.processAlertEvent(event, pattern)
		}
	}

	// 计算系统健康分数
	pattern.SystemHealthScore = s.calculateSystemHealthScore(events)

	// 预测性分析
	s.performPredictiveAnalysis(pattern)

	return pattern, nil
}

// processLocationEvent 处理位置事件
func (s *AlertServiceImpl) processLocationEvent(event kafka.Event, pattern *EventPattern) {
	data := event.Data
	if data == nil {
		return
	}

	droneIDFloat, ok := data["drone_id"].(float64)
	if !ok {
		return
	}
	droneID := uint(droneIDFloat)

	position, ok := data["position"].(map[string]interface{})
	if !ok {
		return
	}

	reading := LocationReading{
		DroneID:   droneID,
		Latitude:  position["latitude"].(float64),
		Longitude: position["longitude"].(float64),
		Altitude:  position["altitude"].(float64),
		Timestamp: event.Timestamp,
	}

	// 计算速度（如果有历史数据）
	if history, exists := s.locationHistory[droneID]; exists && len(history) > 0 {
		lastReading := history[len(history)-1]
		timeDiff := reading.Timestamp.Sub(lastReading.Timestamp).Seconds()
		if timeDiff > 0 {
			distance := s.calculateDistance(lastReading.Latitude, lastReading.Longitude, reading.Latitude, reading.Longitude)
			reading.Speed = distance / timeDiff // m/s
		}
	}

	// 存储历史数据
	s.locationHistory[droneID] = append(s.locationHistory[droneID], reading)

	// 保持历史数据在合理范围内（最近100个点）
	if len(s.locationHistory[droneID]) > 100 {
		s.locationHistory[droneID] = s.locationHistory[droneID][1:]
	}

	// 检查异常
	s.checkLocationAnomalies(reading, pattern)
}

// processBatteryEvent 处理电量事件
func (s *AlertServiceImpl) processBatteryEvent(event kafka.Event, pattern *EventPattern) {
	data := event.Data
	if data == nil {
		return
	}

	droneIDFloat, ok := data["drone_id"].(float64)
	if !ok {
		return
	}
	droneID := uint(droneIDFloat)

	batteryFloat, ok := data["battery"].(float64)
	if !ok {
		return
	}
	battery := int(batteryFloat)

	reading := BatteryReading{
		DroneID:   droneID,
		Battery:   battery,
		Timestamp: event.Timestamp,
	}

	// 存储电量历史
	s.batteryHistory[droneID] = append(s.batteryHistory[droneID], reading)

	// 保持历史数据在合理范围内
	if len(s.batteryHistory[droneID]) > 50 {
		s.batteryHistory[droneID] = s.batteryHistory[droneID][1:]
	}

	// 计算电量消耗率
	if len(s.batteryHistory[droneID]) >= 2 {
		pattern.BatteryDrainRate = s.calculateBatteryDrainRate(droneID)
	}
}

// processStatusEvent 处理状态事件
func (s *AlertServiceImpl) processStatusEvent(event kafka.Event, pattern *EventPattern) {
	// 处理无人机状态变化事件
	s.logger.Debug("Processing status event", map[string]interface{}{
		"event": event,
	})
}

// processAlertEvent 处理告警事件
func (s *AlertServiceImpl) processAlertEvent(event kafka.Event, pattern *EventPattern) {
	data := event.Data
	if data == nil {
		return
	}

	alertType, ok := data["type"].(string)
	if !ok {
		return
	}

	droneIDFloat, ok := data["drone_id"].(float64)
	if !ok {
		return
	}
	droneID := uint(droneIDFloat)

	patternKey := s.generatePatternKey(droneID, alertType)

	if existingPattern, exists := pattern.AlertPatterns[patternKey]; exists {
		existingPattern.Count++
		existingPattern.LastSeen = event.Timestamp
	} else {
		pattern.AlertPatterns[patternKey] = &AlertPattern{
			DroneID:   droneID,
			AlertType: alertType,
			Count:     1,
			FirstSeen: event.Timestamp,
			LastSeen:  event.Timestamp,
			Severity:  "low",
			Metadata:  make(map[string]interface{}),
		}
	}
}

// checkLocationAnomalies 检查位置异常
func (s *AlertServiceImpl) checkLocationAnomalies(reading LocationReading, pattern *EventPattern) {
	// 检查速度异常
	if reading.Speed > 50 { // 假设最大速度为50m/s
		anomaly := LocationAnomaly{
			DroneID:     reading.DroneID,
			Latitude:    reading.Latitude,
			Longitude:   reading.Longitude,
			AnomalyType: "speed_anomaly",
			Timestamp:   reading.Timestamp,
			Severity:    "high",
		}
		pattern.LocationAnomalies = append(pattern.LocationAnomalies, anomaly)
	}

	// 可以添加更多异常检查：
	// - 禁飞区检查
	// - 轨迹偏差检查
	// - 异常停留检查
}

// calculateDistance 计算两点间距离（简化版）
func (s *AlertServiceImpl) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// 简化的距离计算，实际应该使用Haversine公式
	const earthRadius = 6371000 // 地球半径（米）

	dlat := (lat2 - lat1) * 3.14159 / 180
	dlon := (lon2 - lon1) * 3.14159 / 180

	return earthRadius * (dlat*dlat + dlon*dlon)
}

// calculateBatteryDrainRate 计算电量消耗率
func (s *AlertServiceImpl) calculateBatteryDrainRate(droneID uint) float64 {
	history := s.batteryHistory[droneID]
	if len(history) < 2 {
		return 0
	}

	first := history[0]
	last := history[len(history)-1]

	timeDiff := last.Timestamp.Sub(first.Timestamp).Hours()
	if timeDiff <= 0 {
		return 0
	}

	batteryDiff := float64(first.Battery - last.Battery)
	return batteryDiff / timeDiff // %/hour
}

// calculateSystemHealthScore 计算系统健康分数
func (s *AlertServiceImpl) calculateSystemHealthScore(events []kafka.Event) float64 {
	// 简化的健康分数计算
	score := 100.0

	for _, event := range events {
		switch event.Type {
		case kafka.DroneBatteryLowEvent:
			score -= 5
		case kafka.AlertCreatedEvent:
			score -= 3
		case kafka.TaskFailedEvent:
			score -= 10
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// performPredictiveAnalysis 执行预测性分析
func (s *AlertServiceImpl) performPredictiveAnalysis(pattern *EventPattern) {
	// 预测电量耗尽
	for droneID, history := range s.batteryHistory {
		if len(history) >= 3 {
			if issue, err := s.PredictBatteryDrain(droneID, []kafka.Event{}); err == nil && issue != nil {
				pattern.PredictedIssues = append(pattern.PredictedIssues, *issue)
			}
		}
	}
}

// PredictBatteryDrain 预测电量耗尽
func (s *AlertServiceImpl) PredictBatteryDrain(droneID uint, events []kafka.Event) (*PredictedIssue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := s.batteryHistory[droneID]
	if len(history) < 3 {
		return nil, nil
	}

	drainRate := s.calculateBatteryDrainRate(droneID)
	if drainRate <= 0 {
		return nil, nil
	}

	currentBattery := history[len(history)-1].Battery
	hoursToEmpty := float64(currentBattery) / drainRate

	if hoursToEmpty < 1 { // 1小时内耗尽
		return &PredictedIssue{
			Type:        "battery_drain",
			DroneID:     droneID,
			Probability: 0.9,
			TimeToIssue: time.Duration(hoursToEmpty * float64(time.Hour)),
			Description: fmt.Sprintf("无人机 %d 预计在 %.1f 小时内电量耗尽", droneID, hoursToEmpty),
		}, nil
	}

	return nil, nil
}

// PredictMaintenanceNeeds 预测维护需求
func (s *AlertServiceImpl) PredictMaintenanceNeeds(droneID uint, events []kafka.Event) (*PredictedIssue, error) {
	// 简化的维护预测
	// 实际实现会考虑飞行时间、故障历史等因素
	return nil, nil
}

// AggregateAlerts 聚合告警
func (s *AlertServiceImpl) AggregateAlerts(alerts []models.Alert) ([]models.Alert, error) {
	// 按类型和时间窗口聚合相似告警
	aggregated := make([]models.Alert, 0)
	alertMap := make(map[string][]models.Alert)

	// 按类型分组
	for _, alert := range alerts {
		key := fmt.Sprintf("%s_%d", alert.Type, alert.DroneID)
		alertMap[key] = append(alertMap[key], alert)
	}

	// 创建聚合告警
	for _, groupedAlerts := range alertMap {
		if len(groupedAlerts) > 1 {
			// 创建聚合告警
			aggregatedAlert := groupedAlerts[0]
			aggregatedAlert.Message = fmt.Sprintf("%s (聚合了%d个相似告警)",
				aggregatedAlert.Message, len(groupedAlerts))
			aggregated = append(aggregated, aggregatedAlert)
		} else {
			aggregated = append(aggregated, groupedAlerts[0])
		}
	}

	return aggregated, nil
}

// SuppressAlerts 抑制告警
func (s *AlertServiceImpl) SuppressAlerts(alerts []models.Alert) ([]models.Alert, error) {
	// 简单的抑制逻辑：同类型告警在5分钟内只保留一个
	suppressed := make([]models.Alert, 0)
	lastAlert := make(map[string]time.Time)

	for _, alert := range alerts {
		key := fmt.Sprintf("%s_%d", alert.Type, alert.DroneID)
		if lastTime, exists := lastAlert[key]; !exists ||
			time.Since(lastTime) > 5*time.Minute {
			suppressed = append(suppressed, alert)
			lastAlert[key] = alert.CreatedAt
		}
	}

	return suppressed, nil
}

// AnalyzeEventPatterns 分析事件模式
func (s *AlertServiceImpl) AnalyzeEventPatterns(events []kafka.Event) (*EventPattern, error) {
	return s.ProcessEvents(events)
}

// generatePatternKey 生成模式键
func (s *AlertServiceImpl) generatePatternKey(droneID uint, alertType string) string {
	return fmt.Sprintf("%d_%s", droneID, alertType)
}
