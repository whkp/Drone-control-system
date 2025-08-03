package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisConfig struct {
	Addr         string        `yaml:"addr" json:"addr"`
	Password     string        `yaml:"password" json:"password"`
	DB           int           `yaml:"db" json:"db"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	PoolTimeout  time.Duration `yaml:"pool_timeout" json:"pool_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
}

// DefaultRedisConfig 返回默认的Redis配置
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
	}
}

func NewRedisConnection(config RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
		IdleTimeout:  config.IdleTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return rdb, nil
}

// CacheService Redis缓存服务
type CacheService struct {
	client *redis.Client
}

func NewCacheService(client *redis.Client) *CacheService {
	return &CacheService{client: client}
}

func (s *CacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return s.client.Set(ctx, key, value, expiration).Err()
}

func (s *CacheService) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s *CacheService) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	result, err := s.client.Exists(ctx, key).Result()
	return result > 0, err
}

func (s *CacheService) SetHash(ctx context.Context, key string, fields map[string]interface{}, expiration time.Duration) error {
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, key, fields)
	if expiration > 0 {
		pipe.Expire(ctx, key, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *CacheService) GetHash(ctx context.Context, key string) (map[string]string, error) {
	return s.client.HGetAll(ctx, key).Result()
}

// PubSubService Redis发布订阅服务
type PubSubService struct {
	client *redis.Client
}

func NewPubSubService(client *redis.Client) *PubSubService {
	return &PubSubService{client: client}
}

func (s *PubSubService) Publish(ctx context.Context, channel string, message interface{}) error {
	return s.client.Publish(ctx, channel, message).Err()
}

func (s *PubSubService) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return s.client.Subscribe(ctx, channels...)
}

// QueueService Redis队列服务
type QueueService struct {
	client *redis.Client
}

func NewQueueService(client *redis.Client) *QueueService {
	return &QueueService{client: client}
}

func (s *QueueService) Push(ctx context.Context, queue string, message interface{}) error {
	return s.client.LPush(ctx, queue, message).Err()
}

func (s *QueueService) Pop(ctx context.Context, queue string, timeout time.Duration) (string, error) {
	result, err := s.client.BRPop(ctx, timeout, queue).Result()
	if err != nil {
		return "", err
	}
	if len(result) < 2 {
		return "", fmt.Errorf("invalid queue result")
	}
	return result[1], nil
}

func (s *QueueService) Length(ctx context.Context, queue string) (int64, error) {
	return s.client.LLen(ctx, queue).Result()
}

// TestRedisConnection 测试Redis连接
func TestRedisConnection(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	return err
}

// GetRedisStats 获取Redis统计信息
func GetRedisStats(client *redis.Client) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get redis info: %w", err)
	}

	poolStats := client.PoolStats()

	return map[string]interface{}{
		"redis_info":  info,
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
	}, nil
}

// RedisHealthCheck Redis健康检查
func RedisHealthCheck(client *redis.Client) map[string]interface{} {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	}

	// 测试连接
	if err := TestRedisConnection(client); err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()
		return health
	}

	// 获取统计信息
	stats, err := GetRedisStats(client)
	if err != nil {
		health["stats_error"] = err.Error()
	} else {
		health["stats"] = stats
	}

	// 测试基本操作
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	testKey := "health_check_test"
	testValue := "ok"

	// 测试SET
	err = client.Set(ctx, testKey, testValue, time.Minute).Err()
	if err != nil {
		health["set_error"] = err.Error()
		return health
	}

	// 测试GET
	val, err := client.Get(ctx, testKey).Result()
	if err != nil {
		health["get_error"] = err.Error()
		return health
	}

	if val != testValue {
		health["value_mismatch"] = fmt.Sprintf("expected %s, got %s", testValue, val)
		return health
	}

	// 清理测试数据
	client.Del(ctx, testKey)

	health["test_operations"] = "success"
	return health
}

// 分布式锁服务
type LockService struct {
	client *redis.Client
}

func NewLockService(client *redis.Client) *LockService {
	return &LockService{client: client}
}

// AcquireLock 获取分布式锁
func (s *LockService) AcquireLock(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	result, err := s.client.SetNX(ctx, key, value, expiration).Result()
	return result, err
}

// ReleaseLock 释放分布式锁
func (s *LockService) ReleaseLock(ctx context.Context, key string, value string) error {
	luaScript := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	_, err := s.client.Eval(ctx, luaScript, []string{key}, value).Result()
	return err
}

// ExtendLock 延长锁的过期时间
func (s *LockService) ExtendLock(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	luaScript := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	result, err := s.client.Eval(ctx, luaScript, []string{key}, value, int(expiration.Seconds())).Result()
	if err != nil {
		return false, err
	}
	return result.(int64) == 1, nil
}
