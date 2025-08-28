package services

import (
	"context"
	"time"

	"drone-control-system/internal/mvc/models"
)

// UserService 用户服务接口
type UserService interface {
	CreateUser(ctx context.Context, params *CreateUserParams) (*models.User, error)
	GetUserByID(ctx context.Context, id uint) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, id uint, params *UpdateUserParams) (*models.User, error)
	DeleteUser(ctx context.Context, id uint) error
	ListUsers(ctx context.Context, params *ListUsersParams) ([]*models.User, int64, error)
	Login(ctx context.Context, username, password string) (*LoginResult, error)
	ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error
	ValidateToken(ctx context.Context, token string) (*models.User, error)
	RefreshToken(ctx context.Context, token string) (*LoginResult, error)
}

// CreateUserParams 创建用户参数
type CreateUserParams struct {
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Role     models.UserRole `json:"role"`
}

// UpdateUserParams 更新用户参数
type UpdateUserParams struct {
	Username string            `json:"username"`
	Email    string            `json:"email"`
	Role     models.UserRole   `json:"role"`
	Status   models.UserStatus `json:"status"`
	Avatar   string            `json:"avatar"`
}

// ListUsersParams 用户列表参数
type ListUsersParams struct {
	Offset int               `json:"offset"`
	Limit  int               `json:"limit"`
	Role   models.UserRole   `json:"role"`
	Status models.UserStatus `json:"status"`
	Search string            `json:"search"`
}

// LoginResult 登录结果
type LoginResult struct {
	Token     string       `json:"token"`
	ExpiresIn int64        `json:"expires_in"`
	User      *models.User `json:"user"`
}

// DroneService 无人机服务接口
type DroneService interface {
	CreateDrone(ctx context.Context, params *CreateDroneParams) (*models.Drone, error)
	GetDroneByID(ctx context.Context, id uint) (*models.Drone, error)
	GetDroneBySerialNo(ctx context.Context, serialNo string) (*models.Drone, error)
	UpdateDrone(ctx context.Context, id uint, params *UpdateDroneParams) (*models.Drone, error)
	DeleteDrone(ctx context.Context, id uint) error
	ListDrones(ctx context.Context, params *ListDronesParams) ([]*models.Drone, int64, error)
	UpdateDroneStatus(ctx context.Context, id uint, status models.DroneStatus) error
	UpdateDronePosition(ctx context.Context, id uint, position models.Position) error
	UpdateDroneBattery(ctx context.Context, id uint, battery int) error
	GetAvailableDrones(ctx context.Context) ([]*models.Drone, error)
}

// CreateDroneParams 创建无人机参数
type CreateDroneParams struct {
	SerialNo     string   `json:"serial_no"`
	Model        string   `json:"model"`
	Capabilities []string `json:"capabilities"`
	Firmware     string   `json:"firmware"`
	Version      string   `json:"version"`
}

// UpdateDroneParams 更新无人机参数
type UpdateDroneParams struct {
	Model        string             `json:"model"`
	Status       models.DroneStatus `json:"status"`
	Position     *models.Position   `json:"position"`
	Battery      *int               `json:"battery"`
	Capabilities []string           `json:"capabilities"`
	Firmware     string             `json:"firmware"`
	Version      string             `json:"version"`
}

// ListDronesParams 无人机列表参数
type ListDronesParams struct {
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
	Status models.DroneStatus `json:"status"`
	Search string             `json:"search"`
}

// TaskService 任务服务接口
type TaskService interface {
	CreateTask(ctx context.Context, params *CreateTaskParams) (*models.Task, error)
	GetTaskByID(ctx context.Context, id uint) (*models.Task, error)
	UpdateTask(ctx context.Context, id uint, params *UpdateTaskParams) (*models.Task, error)
	DeleteTask(ctx context.Context, id uint) error
	ListTasks(ctx context.Context, params *ListTasksParams) ([]*models.Task, int64, error)
	StartTask(ctx context.Context, id uint) error
	StopTask(ctx context.Context, id uint) error
	UpdateTaskProgress(ctx context.Context, id uint, progress int) error
	CompleteTask(ctx context.Context, id uint, success bool, message string) error
	GetTasksByUser(ctx context.Context, userID uint, params *ListTasksParams) ([]*models.Task, int64, error)
	GetTasksByDrone(ctx context.Context, droneID uint, params *ListTasksParams) ([]*models.Task, int64, error)
}

// CreateTaskParams 创建任务参数
type CreateTaskParams struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        models.TaskType     `json:"type"`
	Priority    models.TaskPriority `json:"priority"`
	UserID      uint                `json:"user_id"`
	DroneID     uint                `json:"drone_id"`
	Plan        models.TaskPlan     `json:"plan"`
	ScheduledAt *time.Time          `json:"scheduled_at"`
}

// UpdateTaskParams 更新任务参数
type UpdateTaskParams struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        models.TaskType     `json:"type"`
	Status      models.TaskStatus   `json:"status"`
	Priority    models.TaskPriority `json:"priority"`
	DroneID     *uint               `json:"drone_id"`
	Plan        *models.TaskPlan    `json:"plan"`
	Progress    *int                `json:"progress"`
	ScheduledAt *time.Time          `json:"scheduled_at"`
}

// ListTasksParams 任务列表参数
type ListTasksParams struct {
	Offset  int               `json:"offset"`
	Limit   int               `json:"limit"`
	Status  models.TaskStatus `json:"status"`
	Type    models.TaskType   `json:"type"`
	UserID  uint              `json:"user_id"`
	DroneID uint              `json:"drone_id"`
	Search  string            `json:"search"`
}

// AlertService 告警服务接口
type AlertService interface {
	CreateAlert(ctx context.Context, params *CreateAlertParams) (*models.Alert, error)
	GetAlertByID(ctx context.Context, id uint) (*models.Alert, error)
	UpdateAlert(ctx context.Context, id uint, params *UpdateAlertParams) (*models.Alert, error)
	DeleteAlert(ctx context.Context, id uint) error
	ListAlerts(ctx context.Context, params *ListAlertsParams) ([]*models.Alert, int64, error)
	AcknowledgeAlert(ctx context.Context, id uint, userID uint) error
	ResolveAlert(ctx context.Context, id uint, userID uint) error
	GetActiveAlerts(ctx context.Context) ([]*models.Alert, error)
	GetAlertsByDrone(ctx context.Context, droneID uint) ([]*models.Alert, error)
}

// CreateAlertParams 创建告警参数
type CreateAlertParams struct {
	Title   string            `json:"title"`
	Message string            `json:"message"`
	Type    models.AlertType  `json:"type"`
	Level   models.AlertLevel `json:"level"`
	Source  string            `json:"source"`
	Code    string            `json:"code"`
	Data    string            `json:"data"`
	DroneID *uint             `json:"drone_id"`
	TaskID  *uint             `json:"task_id"`
	UserID  *uint             `json:"user_id"`
}

// UpdateAlertParams 更新告警参数
type UpdateAlertParams struct {
	Title   string             `json:"title"`
	Message string             `json:"message"`
	Status  models.AlertStatus `json:"status"`
	Data    string             `json:"data"`
}

// ListAlertsParams 告警列表参数
type ListAlertsParams struct {
	Offset  int                `json:"offset"`
	Limit   int                `json:"limit"`
	Type    models.AlertType   `json:"type"`
	Level   models.AlertLevel  `json:"level"`
	Status  models.AlertStatus `json:"status"`
	DroneID uint               `json:"drone_id"`
	TaskID  uint               `json:"task_id"`
	Search  string             `json:"search"`
}
