package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型，包含公共字段
type BaseModel struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// User 用户模型
type User struct {
	BaseModel
	Username  string     `json:"username" gorm:"unique;not null;size:50"`
	Email     string     `json:"email" gorm:"unique;not null;size:100"`
	Password  string     `json:"-" gorm:"not null;size:255"`
	Role      UserRole   `json:"role" gorm:"default:operator;size:20"`
	Status    UserStatus `json:"status" gorm:"default:active;size:20"`
	Avatar    string     `json:"avatar" gorm:"size:255"`
	LastLogin *time.Time `json:"last_login"`

	// 关联关系 - 在需要时加载，避免循环引用
	// Tasks []Task `json:"tasks,omitempty" gorm:"foreignKey:UserID"`
}

// UserRole 用户角色
type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

// UserStatus 用户状态
type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusInactive UserStatus = "inactive"
	StatusBlocked  UserStatus = "blocked"
)

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// BeforeCreate 创建前钩子
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Role == "" {
		u.Role = RoleOperator
	}
	if u.Status == "" {
		u.Status = StatusActive
	}
	return nil
}
