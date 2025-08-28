package models

import (
	"time"
)

// Task 任务模型
type Task struct {
	BaseModel
	Name        string       `json:"name" gorm:"not null;size:100"`
	Description string       `json:"description" gorm:"type:text"`
	Type        TaskType     `json:"type" gorm:"not null;size:20"`
	Status      TaskStatus   `json:"status" gorm:"default:pending;size:20"`
	Priority    TaskPriority `json:"priority" gorm:"default:normal;size:20"`
	Progress    int          `json:"progress" gorm:"default:0;check:progress >= 0 AND progress <= 100"`

	// 外键关联
	UserID  uint `json:"user_id" gorm:"not null"`
	DroneID uint `json:"drone_id" gorm:"not null"`

	// 关联关系
	User  User  `json:"user" gorm:"foreignKey:UserID"`
	Drone Drone `json:"drone" gorm:"foreignKey:DroneID"`

	// 任务计划和结果
	Plan   TaskPlan   `json:"plan" gorm:"embedded;embeddedPrefix:plan_"`
	Result TaskResult `json:"result" gorm:"embedded;embeddedPrefix:result_"`

	// 时间字段
	ScheduledAt *time.Time `json:"scheduled_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// TaskType 任务类型
type TaskType string

const (
	TaskTypeInspection TaskType = "inspection"
	TaskTypeDelivery   TaskType = "delivery"
	TaskTypeMapping    TaskType = "mapping"
	TaskTypePatrol     TaskType = "patrol"
	TaskTypeEmergency  TaskType = "emergency"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusScheduled TaskStatus = "scheduled"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityNormal TaskPriority = "normal"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

// TaskPlan 任务计划
type TaskPlan struct {
	Route       string  `json:"route" gorm:"type:text"`     // JSON格式的路径点
	Waypoints   string  `json:"waypoints" gorm:"type:text"` // JSON格式的航点
	MaxAltitude float64 `json:"max_altitude" gorm:"type:decimal(8,2)"`
	MaxSpeed    float64 `json:"max_speed" gorm:"type:decimal(5,2)"`
	Duration    int     `json:"duration"`                 // 预计执行时间（分钟）
	Payload     string  `json:"payload" gorm:"type:text"` // JSON格式的载荷配置
}

// TaskResult 任务结果
type TaskResult struct {
	Success     bool   `json:"success" gorm:"default:false"`
	Message     string `json:"message" gorm:"type:text"`
	Data        string `json:"data" gorm:"type:text"`       // JSON格式的结果数据
	Files       string `json:"files" gorm:"type:text"`      // JSON格式的文件列表
	Statistics  string `json:"statistics" gorm:"type:text"` // JSON格式的统计信息
	ErrorCode   string `json:"error_code" gorm:"size:50"`
	ErrorDetail string `json:"error_detail" gorm:"type:text"`
}

// TableName 指定表名
func (Task) TableName() string {
	return "tasks"
}

// IsRunning 检查任务是否正在运行
func (t *Task) IsRunning() bool {
	return t.Status == TaskStatusRunning
}

// IsCompleted 检查任务是否已完成
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed || t.Status == TaskStatusCancelled
}

// CanStart 检查任务是否可以开始
func (t *Task) CanStart() bool {
	return t.Status == TaskStatusPending || t.Status == TaskStatusScheduled
}

// Start 启动任务
func (t *Task) Start() {
	if t.CanStart() {
		t.Status = TaskStatusRunning
		now := time.Now()
		t.StartedAt = &now
	}
}

// Complete 完成任务
func (t *Task) Complete(success bool, message string) {
	if t.IsRunning() {
		if success {
			t.Status = TaskStatusCompleted
			t.Progress = 100
		} else {
			t.Status = TaskStatusFailed
		}
		now := time.Now()
		t.CompletedAt = &now
		t.Result.Success = success
		t.Result.Message = message
	}
}
