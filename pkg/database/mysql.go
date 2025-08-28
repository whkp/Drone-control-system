package database

import (
	"context"
	"fmt"
	"time"

	"drone-control-system/internal/mvc/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	User            string        `yaml:"user" json:"user"`
	Password        string        `yaml:"password" json:"password"`
	DBName          string        `yaml:"dbname" json:"dbname"`
	Charset         string        `yaml:"charset" json:"charset"`
	ParseTime       bool          `yaml:"parse_time" json:"parse_time"`
	Loc             string        `yaml:"loc" json:"loc"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
	LogLevel        string        `yaml:"log_level" json:"log_level"`
}

// DefaultConfig 返回默认的数据库配置
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            3306,
		User:            "root",
		Password:        "password",
		DBName:          "drone_control",
		Charset:         "utf8mb4",
		ParseTime:       true,
		Loc:             "Local",
		MaxOpenConns:    100,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 30,
		LogLevel:        "info",
	}
}

func NewMySQLConnection(config Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		config.User, config.Password, config.Host, config.Port, config.DBName,
		config.Charset, config.ParseTime, config.Loc)

	// 配置GORM日志级别
	var logLevel logger.LogLevel
	switch config.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Info
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	return db, nil
}

func Migrate(db *gorm.DB) error {
	// 自动迁移所有表结构
	err := db.AutoMigrate(
		&models.User{},
		&models.Drone{},
		&models.Task{},
		&models.Alert{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

// TestConnection 测试数据库连接
func TestConnection(db *gorm.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// GetDBStats 获取数据库连接池统计信息
func GetDBStats(db *gorm.DB) (map[string]interface{}, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	stats := sqlDB.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}, nil
}

// CreateDatabase 创建数据库（如果不存在）
func CreateDatabase(config Config) error {
	// 连接到默认数据库 mysql 来创建目标数据库
	tempConfig := config
	tempConfig.DBName = "mysql"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		tempConfig.User, tempConfig.Password, tempConfig.Host, tempConfig.Port,
		tempConfig.DBName, tempConfig.Charset, tempConfig.ParseTime, tempConfig.Loc)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to mysql database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	defer sqlDB.Close()

	// 检查数据库是否存在
	var count int64
	err = db.Raw("SELECT COUNT(*) FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", config.DBName).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("failed to check database existence: %w", err)
	}

	if count == 0 {
		// 创建数据库
		err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", config.DBName)).Error
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
	}

	return nil
}

// DropDatabase 删除数据库（谨慎使用）
func DropDatabase(config Config) error {
	// 连接到默认数据库 mysql 来删除目标数据库
	tempConfig := config
	tempConfig.DBName = "mysql"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		tempConfig.User, tempConfig.Password, tempConfig.Host, tempConfig.Port,
		tempConfig.DBName, tempConfig.Charset, tempConfig.ParseTime, tempConfig.Loc)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to mysql database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	defer sqlDB.Close()

	// 删除数据库
	err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", config.DBName)).Error
	if err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	return nil
}

// HealthCheck 数据库健康检查
func HealthCheck(db *gorm.DB) map[string]interface{} {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	}

	// 测试连接
	if err := TestConnection(db); err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()
		return health
	}

	// 获取统计信息
	stats, err := GetDBStats(db)
	if err != nil {
		health["stats_error"] = err.Error()
	} else {
		health["stats"] = stats
	}

	// 测试简单查询
	var version string
	err = db.Raw("SELECT VERSION()").Scan(&version).Error
	if err != nil {
		health["query_error"] = err.Error()
	} else {
		health["mysql_version"] = version
	}

	return health
}
