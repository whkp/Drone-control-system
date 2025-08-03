# 数据库使用说明

## 📦 **数据库完善内容**

我已经完善了整个数据库层，现在包含：

### 🗄️ **MySQL 数据库**
- **完整连接管理**: 支持连接池、超时配置、日志级别
- **自动迁移**: 基于GORM的自动表结构迁移
- **健康检查**: 完整的数据库健康状态监控
- **统计信息**: 连接池使用情况和性能指标
- **数据库管理**: 创建、删除、重置数据库功能

### 🔴 **Redis 缓存**
- **多服务支持**: 缓存、发布订阅、队列、分布式锁
- **连接配置**: 完整的连接池和超时配置
- **健康检查**: Redis连接状态和性能监控
- **高级功能**: 分布式锁、消息队列、实时通信

### 🛠️ **数据库管理器**
- **统一管理**: 同时管理MySQL和Redis连接
- **事务支持**: 数据库事务操作
- **缓存集成**: 带缓存的数据库操作
- **优雅关闭**: 安全的资源清理

### 🌱 **种子数据**
- **初始用户**: 管理员、操作员、查看员角色
- **示例无人机**: 不同型号和状态的无人机
- **示例任务**: 巡检和监控任务
- **数据重置**: 开发和测试环境的数据重置

## 🚀 **使用方法**

### 1. 数据库工具使用

```bash
# 创建数据库
go run cmd/db-tool/main.go -action create

# 执行迁移
go run cmd/db-tool/main.go -action migrate

# 创建种子数据
go run cmd/db-tool/main.go -action seed

# 健康检查
go run cmd/db-tool/main.go -action health

# 重置数据库（谨慎使用）
go run cmd/db-tool/main.go -action reset -force

# 删除数据库（谨慎使用）
go run cmd/db-tool/main.go -action drop -force
```

### 2. 代码中使用

```go
package main

import (
    "drone-control-system/pkg/database"
    "log"
)

func main() {
    // MySQL配置
    pgConfig := database.Config{
        Host:     "localhost",
        Port:     5432,
        User:     "drone_user",
        Password: "drone_pass",
        DBName:   "drone_control",
        SSLMode:  "disable",
        // ... 其他配置
    }

    // Redis配置
    redisConfig := database.DefaultRedisConfig()

    // 创建数据库管理器
    dbManager, err := database.NewDatabaseManager(pgConfig, redisConfig)
    if err != nil {
        log.Fatal(err)
    }
    defer dbManager.Close()

    // 初始化数据库
    if err := dbManager.Initialize(); err != nil {
        log.Fatal(err)
    }

    // 使用MySQL
    var users []domain.User
    dbManager.PostgresDB.Find(&users)

    // 使用Redis缓存
    ctx := context.Background()
    dbManager.CacheService.Set(ctx, "key", "value", time.Hour)

    // 使用消息队列
    dbManager.QueueService.Push(ctx, "task_queue", "task_data")

    // 分布式锁
    locked, _ := dbManager.LockService.AcquireLock(ctx, "lock_key", "owner", time.Minute)
    if locked {
        // 执行临界区代码
        defer dbManager.LockService.ReleaseLock(ctx, "lock_key", "owner")
    }
}
```

### 3. 配置文件

```yaml
database:
  postgres:
    host: "localhost"
    port: 5432
    user: "drone_user"
    password: "drone_pass"
    dbname: "drone_control"
    sslmode: "disable"
    max_open_conns: 100
    max_idle_conns: 10
    conn_max_lifetime: 1h
    conn_max_idle_time: 30m
    log_level: "info"

  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
    pool_size: 10
    min_idle_conns: 5
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    pool_timeout: 4s
    idle_timeout: 5m
```

## 🔧 **技术特性**

### MySQL优势
- **ACID事务**: 保证数据一致性
- **JSON支持**: 存储复杂的传感器数据
- **地理数据**: 支持PostGIS扩展（可选）
- **并发性能**: 高并发读写支持
- **复杂查询**: 强大的SQL查询能力

### Redis功能
- **高性能缓存**: 亚毫秒级响应
- **实时通信**: 发布订阅模式
- **任务队列**: 异步任务处理
- **分布式锁**: 保证操作原子性
- **会话存储**: 用户会话管理

## 📊 **监控和运维**

### 健康检查
每个服务都提供数据库健康检查接口：
```bash
curl http://localhost:8080/api/health/database
```

### 性能监控
- 连接池使用情况
- 查询响应时间
- 缓存命中率
- 队列长度监控

### 日志记录
- 慢查询日志
- 连接错误日志
- 缓存操作日志
- 性能指标日志

## 🛡️ **安全考虑**

1. **连接安全**: 支持SSL连接
2. **访问控制**: 数据库用户权限控制
3. **密码安全**: 不在日志中暴露密码
4. **网络安全**: 防火墙规则配置
5. **备份策略**: 定期数据备份

## 🔄 **部署建议**

### 开发环境
```bash
# 使用Docker Compose
docker-compose up mysql redis

# 初始化数据库
go run cmd/db-tool/main.go -action migrate
go run cmd/db-tool/main.go -action seed
```

### 生产环境
1. 使用专用数据库服务器
2. 配置主从复制
3. 设置监控和告警
4. 定期备份策略
5. 连接池优化

现在数据库层已经完全完善，支持：
- ✅ MySQL完整功能
- ✅ Redis多服务支持  
- ✅ 统一管理和监控
- ✅ 种子数据和工具
- ✅ 健康检查和统计
- ✅ 优雅关闭和错误处理

整个无人机控制系统的数据基础设施已经就绪！
