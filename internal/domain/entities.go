package domain

import (
	"time"
)

// User 用户实体
type User struct {
	ID       uint      `json:"id" gorm:"primaryKey"`
	Username string    `json:"username" gorm:"unique;not null"`
	Email    string    `json:"email" gorm:"unique;not null"`
	Password string    `json:"-" gorm:"not null"`
	Role     UserRole  `json:"role" gorm:"default:operator"`
	Status   UserStatus `json:"status" gorm:"default:active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusInactive UserStatus = "inactive"
	StatusBlocked  UserStatus = "blocked"
)

// Drone 无人机实体
type Drone struct {
	ID          uint         `json:"id" gorm:"primaryKey"`
	SerialNo    string       `json:"serial_no" gorm:"unique;not null"`
	Model       string       `json:"model" gorm:"not null"`
	Status      DroneStatus  `json:"status" gorm:"default:offline"`
	Battery     int          `json:"battery" gorm:"default:0"`
	Position    Position     `json:"position" gorm:"embedded"`
	LastSeen    time.Time    `json:"last_seen"`
	Capabilities []string    `json:"capabilities" gorm:"type:text[]"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type DroneStatus string

const (
	DroneStatusOffline   DroneStatus = "offline"
	DroneStatusOnline    DroneStatus = "online"
	DroneStatusFlying    DroneStatus = "flying"
	DroneStatusCharging  DroneStatus = "charging"
	DroneStatusMaintenance DroneStatus = "maintenance"
	DroneStatusError     DroneStatus = "error"
)

// Position 位置信息
type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Heading   float64 `json:"heading"`
}

// Task 任务实体
type Task struct {
	ID          uint        `json:"id" gorm:"primaryKey"`
	Name        string      `json:"name" gorm:"not null"`
	Description string      `json:"description"`
	Type        TaskType    `json:"type" gorm:"not null"`
	Status      TaskStatus  `json:"status" gorm:"default:pending"`
	Priority    TaskPriority `json:"priority" gorm:"default:normal"`
	DroneID     uint        `json:"drone_id"`
	Drone       Drone       `json:"drone" gorm:"foreignKey:DroneID"`
	UserID      uint        `json:"user_id"`
	User        User        `json:"user" gorm:"foreignKey:UserID"`
	Plan        TaskPlan    `json:"plan" gorm:"embedded"`
	Progress    int         `json:"progress" gorm:"default:0"`
	Result      *TaskResult `json:"result,omitempty" gorm:"embedded"`
	ScheduledAt time.Time   `json:"scheduled_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type TaskType string

const (
	TaskTypeInspection  TaskType = "inspection"
	TaskTypeDelivery    TaskType = "delivery"
	TaskTypeMapping     TaskType = "mapping"
	TaskTypePatrol      TaskType = "patrol"
	TaskTypeEmergency   TaskType = "emergency"
)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusScheduled  TaskStatus = "scheduled"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusPaused     TaskStatus = "paused"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityNormal   TaskPriority = "normal"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// TaskPlan 任务规划
type TaskPlan struct {
	Waypoints    []Waypoint `json:"waypoints" gorm:"type:jsonb"`
	Instructions []string   `json:"instructions" gorm:"type:text[]"`
	EstimatedDuration int   `json:"estimated_duration"` // 分钟
	MaxAltitude  float64    `json:"max_altitude"`
	SafetyZones  []Zone     `json:"safety_zones" gorm:"type:jsonb"`
}

// Waypoint 路径点
type Waypoint struct {
	Order    int      `json:"order"`
	Position Position `json:"position"`
	Action   string   `json:"action"`
	Duration int      `json:"duration"` // 秒
	Params   map[string]interface{} `json:"params"`
}

// Zone 区域定义
type Zone struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"` // no-fly, restricted, safe
	Boundary  []Position `json:"boundary"`
	MinAlt    float64   `json:"min_altitude"`
	MaxAlt    float64   `json:"max_altitude"`
}

// TaskResult 任务结果
type TaskResult struct {
	Success    bool              `json:"success"`
	Message    string            `json:"message"`
	Data       map[string]interface{} `json:"data" gorm:"type:jsonb"`
	Files      []string          `json:"files" gorm:"type:text[]"`
	Statistics TaskStatistics    `json:"statistics" gorm:"embedded"`
}

// TaskStatistics 任务统计
type TaskStatistics struct {
	ActualDuration   int     `json:"actual_duration"` // 秒
	DistanceTraveled float64 `json:"distance_traveled"` // 米
	BatteryConsumed  int     `json:"battery_consumed"` // 百分比
	PhotosTaken      int     `json:"photos_taken"`
	VideoRecorded    int     `json:"video_recorded"` // 秒
}

// Alert 告警实体
type Alert struct {
	ID          uint        `json:"id" gorm:"primaryKey"`
	Type        AlertType   `json:"type" gorm:"not null"`
	Level       AlertLevel  `json:"level" gorm:"not null"`
	Message     string      `json:"message" gorm:"not null"`
	Source      string      `json:"source"`
	DroneID     *uint       `json:"drone_id,omitempty"`
	Drone       *Drone      `json:"drone,omitempty" gorm:"foreignKey:DroneID"`
	TaskID      *uint       `json:"task_id,omitempty"`
	Task        *Task       `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	Acknowledged bool       `json:"acknowledged" gorm:"default:false"`
	AcknowledgedBy *uint    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type AlertType string

const (
	AlertTypeBattery    AlertType = "battery"
	AlertTypeWeather    AlertType = "weather"
	AlertTypeObstacle   AlertType = "obstacle"
	AlertTypeSystem     AlertType = "system"
	AlertTypeSecurity   AlertType = "security"
	AlertTypeConnection AlertType = "connection"
)

type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)
