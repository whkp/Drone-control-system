package views

import (
	"time"

	"drone-control-system/internal/mvc/models"
)

// BaseResponse 基础响应格式
type BaseResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// PaginatedResponse 分页响应格式
type PaginatedResponse struct {
	BaseResponse
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo 分页信息
type PaginationInfo struct {
	Total  int64 `json:"total"`
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
	Pages  int   `json:"pages"`
}

// UserView 用户视图
type UserView struct {
	ID        uint              `json:"id"`
	Username  string            `json:"username"`
	Email     string            `json:"email"`
	Role      models.UserRole   `json:"role"`
	Status    models.UserStatus `json:"status"`
	Avatar    string            `json:"avatar,omitempty"`
	LastLogin *time.Time        `json:"last_login,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// DroneView 无人机视图
type DroneView struct {
	ID           uint               `json:"id"`
	SerialNo     string             `json:"serial_no"`
	Model        string             `json:"model"`
	Status       models.DroneStatus `json:"status"`
	Battery      int                `json:"battery"`
	Position     models.Position    `json:"position"`
	LastSeen     *time.Time         `json:"last_seen,omitempty"`
	Capabilities []string           `json:"capabilities,omitempty"`
	Firmware     string             `json:"firmware,omitempty"`
	Version      string             `json:"version,omitempty"`
	IsOnline     bool               `json:"is_online"`
	IsAvailable  bool               `json:"is_available"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// TaskView 任务视图
type TaskView struct {
	ID          uint                `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Type        models.TaskType     `json:"type"`
	Status      models.TaskStatus   `json:"status"`
	Priority    models.TaskPriority `json:"priority"`
	Progress    int                 `json:"progress"`
	User        *UserView           `json:"user,omitempty"`
	Drone       *DroneView          `json:"drone,omitempty"`
	Plan        models.TaskPlan     `json:"plan,omitempty"`
	Result      models.TaskResult   `json:"result,omitempty"`
	ScheduledAt *time.Time          `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// AlertView 告警视图
type AlertView struct {
	ID             uint               `json:"id"`
	Title          string             `json:"title"`
	Message        string             `json:"message"`
	Type           models.AlertType   `json:"type"`
	Level          models.AlertLevel  `json:"level"`
	Status         models.AlertStatus `json:"status"`
	Source         string             `json:"source,omitempty"`
	Code           string             `json:"code,omitempty"`
	Data           interface{}        `json:"data,omitempty"`
	Drone          *DroneView         `json:"drone,omitempty"`
	Task           *TaskView          `json:"task,omitempty"`
	User           *UserView          `json:"user,omitempty"`
	AcknowledgedAt *time.Time         `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *UserView          `json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time         `json:"resolved_at,omitempty"`
	ResolvedBy     *UserView          `json:"resolved_by,omitempty"`
	SeverityScore  int                `json:"severity_score"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// LoginView 登录响应视图
type LoginView struct {
	Token     string    `json:"token"`
	ExpiresIn int64     `json:"expires_in"`
	TokenType string    `json:"token_type"`
	User      *UserView `json:"user"`
}

// DashboardView 仪表盘视图
type DashboardView struct {
	OnlineDrones    int              `json:"online_drones"`
	TotalDrones     int              `json:"total_drones"`
	ActiveTasks     int              `json:"active_tasks"`
	CompletedTasks  int              `json:"completed_tasks"`
	ActiveAlerts    int              `json:"active_alerts"`
	CriticalAlerts  int              `json:"critical_alerts"`
	SystemStatus    string           `json:"system_status"`
	TotalFlights    int              `json:"total_flights"`
	FlightTimeHours float64          `json:"flight_time_hours"`
	RecentTasks     []*TaskView      `json:"recent_tasks,omitempty"`
	RecentAlerts    []*AlertView     `json:"recent_alerts,omitempty"`
	DroneStatistics *DroneStatistics `json:"drone_statistics,omitempty"`
	PerformanceData *PerformanceData `json:"performance_data,omitempty"`
}

// DroneStatistics 无人机统计
type DroneStatistics struct {
	StatusDistribution map[string]int `json:"status_distribution"`
	BatteryLevels      map[string]int `json:"battery_levels"`
	ModelDistribution  map[string]int `json:"model_distribution"`
}

// PerformanceData 性能数据
type PerformanceData struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIn   float64 `json:"network_in"`
	NetworkOut  float64 `json:"network_out"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}, requestID string) *BaseResponse {
	return &BaseResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code int, message string, requestID string) *BaseResponse {
	return &BaseResponse{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().Unix(),
		RequestID: requestID,
	}
}

// NewPaginatedResponse 创建分页响应
func NewPaginatedResponse(data interface{}, total int64, offset, limit int, requestID string) *PaginatedResponse {
	pages := int((total + int64(limit) - 1) / int64(limit))

	return &PaginatedResponse{
		BaseResponse: BaseResponse{
			Code:      0,
			Message:   "success",
			Data:      data,
			Timestamp: time.Now().Unix(),
			RequestID: requestID,
		},
		Pagination: &PaginationInfo{
			Total:  total,
			Offset: offset,
			Limit:  limit,
			Pages:  pages,
		},
	}
}
