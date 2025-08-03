package domain

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrDroneNotFound    = errors.New("drone not found")
	ErrDroneUnavailable = errors.New("drone unavailable")
	ErrInvalidPlan      = errors.New("invalid task plan")
	ErrBatteryLow       = errors.New("drone battery too low")
	ErrUnauthorized     = errors.New("unauthorized access")
)

// TaskDomainService 任务领域服务
type TaskDomainService struct {
	taskRepo  TaskRepository
	droneRepo DroneRepository
	alertRepo AlertRepository
}

func NewTaskDomainService(taskRepo TaskRepository, droneRepo DroneRepository, alertRepo AlertRepository) *TaskDomainService {
	return &TaskDomainService{
		taskRepo:  taskRepo,
		droneRepo: droneRepo,
		alertRepo: alertRepo,
	}
}

// AssignDroneToTask 为任务分配无人机
func (s *TaskDomainService) AssignDroneToTask(ctx context.Context, taskID uint, droneID uint) error {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ErrTaskNotFound
	}

	drone, err := s.droneRepo.GetByID(ctx, droneID)
	if err != nil {
		return ErrDroneNotFound
	}

	// 检查无人机状态
	if drone.Status != DroneStatusOnline {
		return ErrDroneUnavailable
	}

	// 检查电量
	if drone.Battery < 30 { // 至少需要30%电量
		return ErrBatteryLow
	}

	// 分配无人机
	task.DroneID = droneID
	task.Status = TaskStatusScheduled
	
	return s.taskRepo.Update(ctx, task)
}

// ValidateTaskPlan 验证任务规划
func (s *TaskDomainService) ValidateTaskPlan(ctx context.Context, plan *TaskPlan) error {
	if len(plan.Waypoints) == 0 {
		return ErrInvalidPlan
	}

	// 验证路径点顺序
	for i, waypoint := range plan.Waypoints {
		if waypoint.Order != i+1 {
			return errors.New("waypoint order invalid")
		}

		// 验证高度限制
		if waypoint.Position.Altitude > plan.MaxAltitude {
			return errors.New("waypoint altitude exceeds maximum")
		}

		// 验证禁飞区
		for _, zone := range plan.SafetyZones {
			if zone.Type == "no-fly" && s.isPointInZone(waypoint.Position, zone) {
				return errors.New("waypoint in no-fly zone")
			}
		}
	}

	return nil
}

// CalculateTaskDistance 计算任务总距离
func (s *TaskDomainService) CalculateTaskDistance(plan *TaskPlan) float64 {
	if len(plan.Waypoints) < 2 {
		return 0
	}

	totalDistance := 0.0
	for i := 1; i < len(plan.Waypoints); i++ {
		prev := plan.Waypoints[i-1].Position
		curr := plan.Waypoints[i].Position
		totalDistance += s.calculateDistance(prev, curr)
	}

	return totalDistance
}

// EstimateBatteryConsumption 估算电量消耗
func (s *TaskDomainService) EstimateBatteryConsumption(plan *TaskPlan) int {
	distance := s.CalculateTaskDistance(plan)
	
	// 基础消耗：每公里消耗10%电量
	baseBattery := distance / 1000 * 10
	
	// 悬停消耗：每分钟消耗1%电量
	hoverTime := 0
	for _, waypoint := range plan.Waypoints {
		hoverTime += waypoint.Duration
	}
	hoverBattery := float64(hoverTime) / 60.0 * 1
	
	// 高度消耗：每100米增加5%消耗
	maxAltitude := plan.MaxAltitude
	altitudeBattery := maxAltitude / 100 * 5
	
	total := baseBattery + hoverBattery + altitudeBattery
	return int(math.Ceil(total))
}

// StartTask 启动任务
func (s *TaskDomainService) StartTask(ctx context.Context, taskID uint) error {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusScheduled {
		return errors.New("task not in scheduled status")
	}

	// 检查无人机状态
	drone, err := s.droneRepo.GetByID(ctx, task.DroneID)
	if err != nil {
		return ErrDroneNotFound
	}

	if drone.Status != DroneStatusOnline {
		return ErrDroneUnavailable
	}

	// 更新任务状态
	now := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &now
	task.Progress = 0

	// 更新无人机状态
	drone.Status = DroneStatusFlying

	// 保存更新
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return err
	}

	return s.droneRepo.Update(ctx, drone)
}

// CompleteTask 完成任务
func (s *TaskDomainService) CompleteTask(ctx context.Context, taskID uint, result *TaskResult) error {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusRunning {
		return errors.New("task not in running status")
	}

	// 更新任务状态
	now := time.Now()
	task.Status = TaskStatusCompleted
	task.CompletedAt = &now
	task.Progress = 100
	task.Result = result

	// 更新无人机状态
	drone, err := s.droneRepo.GetByID(ctx, task.DroneID)
	if err != nil {
		return ErrDroneNotFound
	}
	drone.Status = DroneStatusOnline

	// 保存更新
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return err
	}

	return s.droneRepo.Update(ctx, drone)
}

// 辅助方法

func (s *TaskDomainService) isPointInZone(point Position, zone Zone) bool {
	// 简化的点在多边形内判断算法
	// 实际项目中应使用更精确的地理空间算法
	return false
}

func (s *TaskDomainService) calculateDistance(p1, p2 Position) float64 {
	// 使用 Haversine 公式计算地球表面两点间距离
	const R = 6371000 // 地球半径（米）

	lat1Rad := p1.Latitude * math.Pi / 180
	lat2Rad := p2.Latitude * math.Pi / 180
	deltaLat := (p2.Latitude - p1.Latitude) * math.Pi / 180
	deltaLon := (p2.Longitude - p1.Longitude) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// DroneDomainService 无人机领域服务
type DroneDomainService struct {
	droneRepo DroneRepository
	alertRepo AlertRepository
}

func NewDroneDomainService(droneRepo DroneRepository, alertRepo AlertRepository) *DroneDomainService {
	return &DroneDomainService{
		droneRepo: droneRepo,
		alertRepo: alertRepo,
	}
}

// UpdateDroneHeartbeat 更新无人机心跳
func (s *DroneDomainService) UpdateDroneHeartbeat(ctx context.Context, droneID uint, position Position, battery int) error {
	drone, err := s.droneRepo.GetByID(ctx, droneID)
	if err != nil {
		return ErrDroneNotFound
	}

	// 更新位置和电量
	drone.Position = position
	drone.Battery = battery
	drone.LastSeen = time.Now()

	// 检查电量告警
	if battery <= 20 && battery > 10 {
		s.createAlert(ctx, droneID, AlertTypeBattery, AlertLevelWarning, "无人机电量低于20%")
	} else if battery <= 10 {
		s.createAlert(ctx, droneID, AlertTypeBattery, AlertLevelCritical, "无人机电量极低，需要立即返航")
	}

	// 检查连接状态
	if drone.Status == DroneStatusOffline {
		drone.Status = DroneStatusOnline
	}

	return s.droneRepo.Update(ctx, drone)
}

// CheckDroneHealth 检查无人机健康状态
func (s *DroneDomainService) CheckDroneHealth(ctx context.Context) error {
	drones, err := s.droneRepo.List(ctx, 0, 1000) // 获取所有无人机
	if err != nil {
		return err
	}

	for _, drone := range drones {
		// 检查是否超过5分钟未收到心跳
		if time.Since(drone.LastSeen) > 5*time.Minute && drone.Status != DroneStatusOffline {
			drone.Status = DroneStatusOffline
			s.droneRepo.Update(ctx, drone)
			s.createAlert(ctx, drone.ID, AlertTypeConnection, AlertLevelError, "无人机失去连接")
		}
	}

	return nil
}

func (s *DroneDomainService) createAlert(ctx context.Context, droneID uint, alertType AlertType, level AlertLevel, message string) {
	alert := &Alert{
		Type:     alertType,
		Level:    level,
		Message:  message,
		Source:   "drone-service",
		DroneID:  &droneID,
	}
	s.alertRepo.Create(ctx, alert)
}
