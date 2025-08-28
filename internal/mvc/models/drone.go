package models

import (
	"time"
)

// Drone 无人机模型
type Drone struct {
	BaseModel
	SerialNo     string      `json:"serial_no" gorm:"unique;not null;size:50"`
	Model        string      `json:"model" gorm:"not null;size:100"`
	Status       DroneStatus `json:"status" gorm:"default:offline;size:20"`
	Battery      int         `json:"battery" gorm:"default:0;check:battery >= 0 AND battery <= 100"`
	Position     Position    `json:"position" gorm:"embedded;embeddedPrefix:pos_"`
	LastSeen     *time.Time  `json:"last_seen"`
	Capabilities string      `json:"capabilities" gorm:"type:text"` // JSON字符串存储能力列表
	Firmware     string      `json:"firmware" gorm:"size:50"`
	Version      string      `json:"version" gorm:"size:20"`

	// 关联关系 - 在需要时加载，避免循环引用
	// Tasks []Task `json:"tasks,omitempty" gorm:"foreignKey:DroneID"`
	// Alerts []Alert `json:"alerts,omitempty" gorm:"foreignKey:DroneID"`
}

// DroneStatus 无人机状态
type DroneStatus string

const (
	DroneStatusOffline     DroneStatus = "offline"
	DroneStatusOnline      DroneStatus = "online"
	DroneStatusFlying      DroneStatus = "flying"
	DroneStatusCharging    DroneStatus = "charging"
	DroneStatusMaintenance DroneStatus = "maintenance"
	DroneStatusError       DroneStatus = "error"
)

// Position 位置信息
type Position struct {
	Latitude  float64 `json:"latitude" gorm:"type:decimal(10,8)"`
	Longitude float64 `json:"longitude" gorm:"type:decimal(11,8)"`
	Altitude  float64 `json:"altitude" gorm:"type:decimal(8,2)"`
	Heading   float64 `json:"heading" gorm:"type:decimal(5,2)"`
}

// TableName 指定表名
func (Drone) TableName() string {
	return "drones"
}

// IsOnline 检查无人机是否在线
func (d *Drone) IsOnline() bool {
	return d.Status == DroneStatusOnline || d.Status == DroneStatusFlying
}

// IsAvailable 检查无人机是否可用（在线且电量充足）
func (d *Drone) IsAvailable() bool {
	return d.IsOnline() && d.Battery > 20
}

// UpdateLastSeen 更新最后在线时间
func (d *Drone) UpdateLastSeen() {
	now := time.Now()
	d.LastSeen = &now
}
