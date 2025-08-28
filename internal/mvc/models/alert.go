package models

import (
	"time"
)

// Alert 告警模型
type Alert struct {
	BaseModel
	Title   string      `json:"title" gorm:"not null;size:200"`
	Message string      `json:"message" gorm:"type:text"`
	Type    AlertType   `json:"type" gorm:"not null;size:20"`
	Level   AlertLevel  `json:"level" gorm:"not null;size:20"`
	Status  AlertStatus `json:"status" gorm:"default:active;size:20"`
	Source  string      `json:"source" gorm:"size:100"` // 告警来源
	Code    string      `json:"code" gorm:"size:50"`    // 告警代码
	Data    string      `json:"data" gorm:"type:text"`  // JSON格式的附加数据

	// 外键关联
	DroneID *uint `json:"drone_id" gorm:"index"` // 可选：关联的无人机
	TaskID  *uint `json:"task_id" gorm:"index"`  // 可选：关联的任务
	UserID  *uint `json:"user_id" gorm:"index"`  // 可选：关联的用户

	// 关联关系
	Drone *Drone `json:"drone,omitempty" gorm:"foreignKey:DroneID"`
	Task  *Task  `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	User  *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`

	// 处理信息
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	AcknowledgedBy *uint      `json:"acknowledged_by"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	ResolvedBy     *uint      `json:"resolved_by"`
	AckUser        *User      `json:"ack_user,omitempty" gorm:"foreignKey:AcknowledgedBy"`
	ResolveUser    *User      `json:"resolve_user,omitempty" gorm:"foreignKey:ResolvedBy"`
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeSystem      AlertType = "system"
	AlertTypeDrone       AlertType = "drone"
	AlertTypeTask        AlertType = "task"
	AlertTypeSecurity    AlertType = "security"
	AlertTypePerformance AlertType = "performance"
	AlertTypeNetwork     AlertType = "network"
	AlertTypeBattery     AlertType = "battery"
	AlertTypeWeather     AlertType = "weather"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusClosed       AlertStatus = "closed"
)

// TableName 指定表名
func (Alert) TableName() string {
	return "alerts"
}

// IsActive 检查告警是否活跃
func (a *Alert) IsActive() bool {
	return a.Status == AlertStatusActive
}

// IsAcknowledged 检查告警是否已确认
func (a *Alert) IsAcknowledged() bool {
	return a.Status == AlertStatusAcknowledged || a.Status == AlertStatusResolved
}

// IsResolved 检查告警是否已解决
func (a *Alert) IsResolved() bool {
	return a.Status == AlertStatusResolved || a.Status == AlertStatusClosed
}

// Acknowledge 确认告警
func (a *Alert) Acknowledge(userID uint) {
	if a.IsActive() {
		a.Status = AlertStatusAcknowledged
		now := time.Now()
		a.AcknowledgedAt = &now
		a.AcknowledgedBy = &userID
	}
}

// Resolve 解决告警
func (a *Alert) Resolve(userID uint) {
	if a.IsActive() || a.IsAcknowledged() {
		a.Status = AlertStatusResolved
		now := time.Now()
		a.ResolvedAt = &now
		a.ResolvedBy = &userID
	}
}

// GetSeverityScore 获取严重程度分数（用于排序）
func (a *Alert) GetSeverityScore() int {
	switch a.Level {
	case AlertLevelCritical:
		return 4
	case AlertLevelError:
		return 3
	case AlertLevelWarning:
		return 2
	case AlertLevelInfo:
		return 1
	default:
		return 0
	}
}
