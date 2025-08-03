package database

import (
	"drone-control-system/internal/domain"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// SeedData 初始化种子数据
func SeedData(db *gorm.DB) error {
	// 创建默认管理员用户
	if err := seedUsers(db); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	// 创建示例无人机
	if err := seedDrones(db); err != nil {
		return fmt.Errorf("failed to seed drones: %w", err)
	}

	// 创建示例任务
	if err := seedTasks(db); err != nil {
		return fmt.Errorf("failed to seed tasks: %w", err)
	}

	return nil
}

func seedUsers(db *gorm.DB) error {
	users := []domain.User{
		{
			Username: "admin",
			Email:    "admin@drone-control.com",
			Password: "$2a$10$8X5JQKvLzXyj1cZ1YGqXcOKx.aCJ2FYvNJZbBJzVvJJZbBJzVvJJZ", // "admin123"
			Role:     domain.RoleAdmin,
			Status:   domain.StatusActive,
		},
		{
			Username: "operator",
			Email:    "operator@drone-control.com",
			Password: "$2a$10$8X5JQKvLzXyj1cZ1YGqXcOKx.aCJ2FYvNJZbBJzVvJJZbBJzVvJJZ", // "operator123"
			Role:     domain.RoleOperator,
			Status:   domain.StatusActive,
		},
		{
			Username: "viewer",
			Email:    "viewer@drone-control.com",
			Password: "$2a$10$8X5JQKvLzXyj1cZ1YGqXcOKx.aCJ2FYvNJZbBJzVvJJZbBJzVvJJZ", // "viewer123"
			Role:     domain.RoleViewer,
			Status:   domain.StatusActive,
		},
	}

	for _, user := range users {
		var existingUser domain.User
		if err := db.Where("username = ?", user.Username).First(&existingUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				user.CreatedAt = time.Now()
				user.UpdatedAt = time.Now()
				if err := db.Create(&user).Error; err != nil {
					return fmt.Errorf("failed to create user %s: %w", user.Username, err)
				}
				fmt.Printf("Created user: %s\n", user.Username)
			} else {
				return fmt.Errorf("failed to check user %s: %w", user.Username, err)
			}
		}
	}

	return nil
}

func seedDrones(db *gorm.DB) error {
	drones := []domain.Drone{
		{
			SerialNo: "DRONE001",
			Model:    "DJI Mavic Pro",
			Status:   domain.DroneStatusOnline,
			Battery:  85,
			Position: domain.Position{
				Latitude:  40.7128,
				Longitude: -74.0060,
				Altitude:  0,
				Heading:   0,
			},
			Capabilities: []string{"camera", "gps", "lidar"},
			LastSeen:     time.Now(),
		},
		{
			SerialNo: "DRONE002",
			Model:    "DJI Air 2S",
			Status:   domain.DroneStatusOffline,
			Battery:  92,
			Position: domain.Position{
				Latitude:  40.7589,
				Longitude: -73.9851,
				Altitude:  0,
				Heading:   180,
			},
			Capabilities: []string{"camera", "gps"},
			LastSeen:     time.Now().Add(-1 * time.Hour),
		},
		{
			SerialNo: "DRONE003",
			Model:    "DJI Mini 3",
			Status:   domain.DroneStatusMaintenance,
			Battery:  0,
			Position: domain.Position{
				Latitude:  40.7505,
				Longitude: -73.9934,
				Altitude:  0,
				Heading:   90,
			},
			Capabilities: []string{"camera"},
			LastSeen:     time.Now().Add(-24 * time.Hour),
		},
	}

	for _, drone := range drones {
		var existingDrone domain.Drone
		if err := db.Where("serial_no = ?", drone.SerialNo).First(&existingDrone).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				drone.CreatedAt = time.Now()
				drone.UpdatedAt = time.Now()
				if err := db.Create(&drone).Error; err != nil {
					return fmt.Errorf("failed to create drone %s: %w", drone.SerialNo, err)
				}
				fmt.Printf("Created drone: %s\n", drone.SerialNo)
			} else {
				return fmt.Errorf("failed to check drone %s: %w", drone.SerialNo, err)
			}
		}
	}

	return nil
}

func seedTasks(db *gorm.DB) error {
	// 先获取一个用户和无人机
	var user domain.User
	if err := db.First(&user).Error; err != nil {
		return fmt.Errorf("no users found for task seeding: %w", err)
	}

	var drone domain.Drone
	if err := db.First(&drone).Error; err != nil {
		return fmt.Errorf("no drones found for task seeding: %w", err)
	}

	// 辅助函数：创建时间指针
	timePtr := func(t time.Time) *time.Time { return &t }

	tasks := []domain.Task{
		{
			Name:        "仓库巡检任务",
			Description: "对仓库进行定期巡检，检查安全状况",
			Type:        domain.TaskTypeInspection,
			Status:      domain.TaskStatusCompleted,
			Priority:    domain.TaskPriorityNormal,
			UserID:      user.ID,
			DroneID:     drone.ID,
			Plan: domain.TaskPlan{
				Waypoints: []domain.Waypoint{
					{
						Order: 1,
						Position: domain.Position{
							Latitude:  40.7128,
							Longitude: -74.0060,
							Altitude:  50,
						},
						Action:   "takeoff",
						Duration: 30,
					},
					{
						Order: 2,
						Position: domain.Position{
							Latitude:  40.7150,
							Longitude: -74.0080,
							Altitude:  100,
						},
						Action:   "inspect",
						Duration: 300,
					},
					{
						Order: 3,
						Position: domain.Position{
							Latitude:  40.7128,
							Longitude: -74.0060,
							Altitude:  0,
						},
						Action:   "land",
						Duration: 60,
					},
				},
				Instructions:      []string{"起飞到50米", "飞行到巡检点", "执行巡检", "返回起飞点", "降落"},
				EstimatedDuration: 30,
				MaxAltitude:       120,
			},
			Progress:    100,
			ScheduledAt: time.Now().Add(-24 * time.Hour),
			StartedAt:   timePtr(time.Now().Add(-24 * time.Hour)),
			CompletedAt: timePtr(time.Now().Add(-23 * time.Hour)),
		},
		{
			Name:        "区域监控任务",
			Description: "对指定区域进行实时监控",
			Type:        domain.TaskTypePatrol,
			Status:      domain.TaskStatusPending,
			Priority:    domain.TaskPriorityHigh,
			UserID:      user.ID,
			DroneID:     0, // 未分配无人机
			Plan: domain.TaskPlan{
				Waypoints: []domain.Waypoint{
					{
						Order: 1,
						Position: domain.Position{
							Latitude:  40.7589,
							Longitude: -73.9851,
							Altitude:  80,
						},
						Action:   "monitor",
						Duration: 1800, // 30分钟
					},
				},
				Instructions:      []string{"起飞", "前往监控点", "执行监控", "返回"},
				EstimatedDuration: 45,
				MaxAltitude:       100,
			},
			ScheduledAt: time.Now().Add(1 * time.Hour),
		},
	}

	for _, task := range tasks {
		var existingTask domain.Task
		if err := db.Where("name = ?", task.Name).First(&existingTask).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				task.CreatedAt = time.Now()
				task.UpdatedAt = time.Now()
				if err := db.Create(&task).Error; err != nil {
					return fmt.Errorf("failed to create task %s: %w", task.Name, err)
				}
				fmt.Printf("Created task: %s\n", task.Name)
			} else {
				return fmt.Errorf("failed to check task %s: %w", task.Name, err)
			}
		}
	}

	return nil
}

// ClearData 清空所有数据（用于测试）
func ClearData(db *gorm.DB) error {
	// 注意：删除顺序很重要，要先删除外键引用的表
	tables := []interface{}{
		&domain.Alert{},
		&domain.Task{},
		&domain.Drone{},
		&domain.User{},
	}

	for _, table := range tables {
		if err := db.Unscoped().Where("1 = 1").Delete(table).Error; err != nil {
			return fmt.Errorf("failed to clear table %T: %w", table, err)
		}
	}

	fmt.Println("All data cleared successfully")
	return nil
}

// ResetDatabase 重置数据库（删除并重新创建所有数据）
func ResetDatabase(db *gorm.DB) error {
	// 清空现有数据
	if err := ClearData(db); err != nil {
		return fmt.Errorf("failed to clear data: %w", err)
	}

	// 重新迁移
	if err := Migrate(db); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	// 重新创建种子数据
	if err := SeedData(db); err != nil {
		return fmt.Errorf("failed to seed data: %w", err)
	}

	fmt.Println("Database reset successfully")
	return nil
}
