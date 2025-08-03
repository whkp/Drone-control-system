package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// DatabaseManager 数据库管理器
type DatabaseManager struct {
	MySQLDB       *gorm.DB
	RedisClient   *redis.Client
	CacheService  *CacheService
	PubSubService *PubSubService
	QueueService  *QueueService
	LockService   *LockService
}

// NewDatabaseManager 创建数据库管理器
func NewDatabaseManager(mysqlConfig Config, redisConfig RedisConfig) (*DatabaseManager, error) {
	// 初始化MySQL
	mysqlDB, err := NewMySQLConnection(mysqlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// 初始化Redis
	redisClient, err := NewRedisConnection(redisConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// 初始化Redis服务
	cacheService := NewCacheService(redisClient)
	pubSubService := NewPubSubService(redisClient)
	queueService := NewQueueService(redisClient)
	lockService := NewLockService(redisClient)

	return &DatabaseManager{
		MySQLDB:       mysqlDB,
		RedisClient:   redisClient,
		CacheService:  cacheService,
		PubSubService: pubSubService,
		QueueService:  queueService,
		LockService:   lockService,
	}, nil
}

// Initialize 初始化数据库
func (dm *DatabaseManager) Initialize() error {
	// 测试连接
	if err := TestConnection(dm.MySQLDB); err != nil {
		return fmt.Errorf("MySQL connection test failed: %w", err)
	}

	if err := TestRedisConnection(dm.RedisClient); err != nil {
		return fmt.Errorf("Redis connection test failed: %w", err)
	}

	// 执行数据库迁移
	if err := Migrate(dm.MySQLDB); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	return nil
}

// Shutdown 关闭数据库连接
func (dm *DatabaseManager) Shutdown() error {
	var errors []error

	// 关闭MySQL
	if dm.MySQLDB != nil {
		sqlDB, err := dm.MySQLDB.DB()
		if err == nil {
			if closeErr := sqlDB.Close(); closeErr != nil {
				errors = append(errors, fmt.Errorf("failed to close MySQL: %w", closeErr))
			}
		}
	}

	// 关闭Redis
	if dm.RedisClient != nil {
		if closeErr := dm.RedisClient.Close(); closeErr != nil {
			errors = append(errors, fmt.Errorf("failed to close Redis: %w", closeErr))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// HealthCheck 健康检查
func (dm *DatabaseManager) HealthCheck() map[string]interface{} {
	health := map[string]interface{}{
		"timestamp": time.Now().UTC(),
	}

	// MySQL健康检查
	if dm.MySQLDB != nil {
		health["mysql"] = HealthCheck(dm.MySQLDB)
	}

	// Redis健康检查
	if dm.RedisClient != nil {
		if err := TestRedisConnection(dm.RedisClient); err != nil {
			health["redis"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
		} else {
			health["redis"] = map[string]interface{}{
				"status": "healthy",
			}
		}
	}

	return health
}

// GetStats 获取统计信息
func (dm *DatabaseManager) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"timestamp": time.Now().UTC(),
	}

	// MySQL统计
	if dm.MySQLDB != nil {
		mysqlStats, err := GetDBStats(dm.MySQLDB)
		if err != nil {
			stats["mysql_error"] = err.Error()
		} else {
			stats["mysql"] = mysqlStats
		}
	}

	// Redis统计
	if dm.RedisClient != nil {
		redisStats, err := GetRedisStats(dm.RedisClient)
		if err != nil {
			stats["redis_error"] = err.Error()
		} else {
			stats["redis"] = redisStats
		}
	}

	return stats
}

// Transaction 执行数据库事务
func (dm *DatabaseManager) Transaction(fn func(tx *gorm.DB) error) error {
	return dm.MySQLDB.Transaction(fn)
}

// WithCache 带缓存的数据库操作
func (dm *DatabaseManager) WithCache(ctx context.Context, cacheKey string, expiration time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	// 先尝试从缓存获取
	cached, err := dm.CacheService.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		return cached, nil
	}

	// 缓存未命中，执行函数
	result, err := fn()
	if err != nil {
		return nil, err
	}

	// 将结果存入缓存
	if err := dm.CacheService.Set(ctx, cacheKey, result, expiration); err != nil {
		// 缓存失败不影响主流程，只记录错误
		fmt.Printf("Cache set error: %v\n", err)
	}

	return result, nil
}

// Ping 测试所有连接
func (dm *DatabaseManager) Ping() error {
	// 测试MySQL连接
	if dm.MySQLDB != nil {
		if err := TestConnection(dm.MySQLDB); err != nil {
			return fmt.Errorf("MySQL ping failed: %w", err)
		}
	}

	// 测试Redis连接
	if dm.RedisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := dm.RedisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("Redis ping failed: %w", err)
		}
	}

	return nil
}

// GetDB 获取MySQL数据库连接
func (dm *DatabaseManager) GetDB() *gorm.DB {
	return dm.MySQLDB
}

// GetRedis 获取Redis客户端
func (dm *DatabaseManager) GetRedis() *redis.Client {
	return dm.RedisClient
}

// GetCache 获取缓存服务
func (dm *DatabaseManager) GetCache() *CacheService {
	return dm.CacheService
}

// GetPubSub 获取发布订阅服务
func (dm *DatabaseManager) GetPubSub() *PubSubService {
	return dm.PubSubService
}

// GetQueue 获取队列服务
func (dm *DatabaseManager) GetQueue() *QueueService {
	return dm.QueueService
}

// GetLock 获取锁服务
func (dm *DatabaseManager) GetLock() *LockService {
	return dm.LockService
}
