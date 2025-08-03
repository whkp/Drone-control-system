package domain

import (
	"context"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uint) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, offset, limit int) ([]*User, error)
}

// DroneRepository 无人机仓储接口
type DroneRepository interface {
	Create(ctx context.Context, drone *Drone) error
	GetByID(ctx context.Context, id uint) (*Drone, error)
	GetBySerialNo(ctx context.Context, serialNo string) (*Drone, error)
	Update(ctx context.Context, drone *Drone) error
	UpdateStatus(ctx context.Context, id uint, status DroneStatus) error
	UpdatePosition(ctx context.Context, id uint, position Position) error
	UpdateBattery(ctx context.Context, id uint, battery int) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, offset, limit int) ([]*Drone, error)
	GetByStatus(ctx context.Context, status DroneStatus) ([]*Drone, error)
	GetAvailable(ctx context.Context) ([]*Drone, error)
}

// TaskRepository 任务仓储接口
type TaskRepository interface {
	Create(ctx context.Context, task *Task) error
	GetByID(ctx context.Context, id uint) (*Task, error)
	Update(ctx context.Context, task *Task) error
	UpdateStatus(ctx context.Context, id uint, status TaskStatus) error
	UpdateProgress(ctx context.Context, id uint, progress int) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, offset, limit int) ([]*Task, error)
	GetByUser(ctx context.Context, userID uint, offset, limit int) ([]*Task, error)
	GetByDrone(ctx context.Context, droneID uint, offset, limit int) ([]*Task, error)
	GetByStatus(ctx context.Context, status TaskStatus) ([]*Task, error)
	GetScheduled(ctx context.Context) ([]*Task, error)
	GetRunning(ctx context.Context) ([]*Task, error)
}

// AlertRepository 告警仓储接口
type AlertRepository interface {
	Create(ctx context.Context, alert *Alert) error
	GetByID(ctx context.Context, id uint) (*Alert, error)
	Update(ctx context.Context, alert *Alert) error
	Acknowledge(ctx context.Context, id uint, userID uint) error
	Resolve(ctx context.Context, id uint) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, offset, limit int) ([]*Alert, error)
	GetByType(ctx context.Context, alertType AlertType) ([]*Alert, error)
	GetByLevel(ctx context.Context, level AlertLevel) ([]*Alert, error)
	GetUnacknowledged(ctx context.Context) ([]*Alert, error)
	GetByDrone(ctx context.Context, droneID uint) ([]*Alert, error)
}
