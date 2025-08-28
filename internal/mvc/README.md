# MVC 分层架构设计

本项目已重构为标准的 MVC (Model-View-Controller) 分层架构，提供清晰的代码组织结构和职责分离。

## 🏗️ 架构概览

```
internal/mvc/
├── models/          # 数据模型层 (Model)
├── views/           # 视图层 (View) 
├── controllers/     # 控制器层 (Controller)
├── services/        # 业务逻辑层 (Service)
├── middleware/      # 中间件层 (Middleware)
└── routes/          # 路由层 (Routes)
```

## 📋 各层详细说明

### 1. Models 层 (数据模型)
**位置**: `internal/mvc/models/`

**职责**:
- 定义数据结构和数据库模型
- 包含数据验证逻辑
- 提供模型间的关联关系
- 包含业务相关的模型方法

**文件结构**:
```
models/
├── user.go         # 用户模型
├── drone.go        # 无人机模型
├── task.go         # 任务模型
└── alert.go        # 告警模型
```

**特性**:
- 使用 GORM 标签进行数据库映射
- 实现软删除和时间戳自动管理
- 包含模型验证和业务方法
- 支持 JSON 序列化和反序列化

### 2. Controllers 层 (控制器)
**位置**: `internal/mvc/controllers/`

**职责**:
- 处理 HTTP 请求和响应
- 参数验证和数据绑定
- 调用 Service 层执行业务逻辑
- 统一错误处理和日志记录

**文件结构**:
```
controllers/
├── base_controller.go    # 基础控制器
├── user_controller.go    # 用户控制器
├── drone_controller.go   # 无人机控制器
└── errors.go            # 错误定义
```

**特性**:
- 继承基础控制器提供通用功能
- 统一的响应格式和错误处理
- 权限验证和用户身份检查
- 请求参数验证和分页处理

### 3. Services 层 (业务逻辑)
**位置**: `internal/mvc/services/`

**职责**:
- 实现核心业务逻辑
- 数据处理和转换
- 第三方服务集成
- 事务管理

**文件结构**:
```
services/
├── interfaces.go    # 服务接口定义
├── errors.go        # 业务错误定义
├── user_service.go  # 用户服务实现
└── drone_service.go # 无人机服务实现
```

**特性**:
- 接口驱动的设计便于测试和扩展
- 完整的错误处理机制
- 支持上下文传递和取消
- 业务逻辑与数据访问分离

### 4. Views 层 (视图)
**位置**: `internal/mvc/views/`

**职责**:
- 定义 API 响应格式
- 数据转换和序列化
- 响应数据的格式化和脱敏

**文件结构**:
```
views/
├── responses.go     # 响应格式定义
└── converters.go    # 模型转换器
```

**特性**:
- 统一的响应格式标准
- 模型到视图的转换器
- 数据脱敏和格式化
- 分页响应支持

### 5. Middleware 层 (中间件)
**位置**: `internal/mvc/middleware/`

**职责**:
- 请求预处理和后处理
- 身份认证和权限控制
- 日志记录和监控
- 跨域和安全处理

**文件结构**:
```
middleware/
├── auth.go          # 认证中间件
└── common.go        # 通用中间件
```

**特性**:
- JWT 认证和权限控制
- 统一的日志和错误恢复
- CORS 和安全头设置
- 请求限流和防护

### 6. Routes 层 (路由)
**位置**: `internal/mvc/routes/`

**职责**:
- HTTP 路由定义和管理
- 中间件组合和应用
- API 版本控制

**文件结构**:
```
routes/
└── routes.go        # 路由定义
```

**特性**:
- RESTful API 设计
- 路由分组和权限控制
- 版本化 API 支持
- 中间件链组合

## 🚀 启动服务

### 使用新的 MVC 服务器
```bash
# 构建并运行 MVC 服务器
go run cmd/mvc-server/main.go
```

### 配置文件
服务器会读取 `configs/config.yaml` 配置文件：
```yaml
server:
  port: "8080"
  
logging:
  level: "info"     # debug, info, warn, error
  format: "json"    # json, text
  output: "stdout"  # stdout, stderr, or file path

database:
  host: "localhost"
  port: 3306
  user: "root"
  password: ""
  dbname: "drone_control"
```

## 📚 API 接口

### 认证接口
```
POST /api/v1/public/login           # 用户登录
```

### 用户管理接口
```
GET    /api/v1/users/profile        # 获取当前用户信息
PUT    /api/v1/users/profile        # 更新用户信息
POST   /api/v1/users/change-password # 修改密码

# 管理员接口
POST   /api/v1/users               # 创建用户
GET    /api/v1/users               # 用户列表
GET    /api/v1/users/:id           # 获取用户信息
PUT    /api/v1/users/:id           # 更新用户信息
DELETE /api/v1/users/:id           # 删除用户
```

### 无人机管理接口
```
GET    /api/v1/drones              # 无人机列表
GET    /api/v1/drones/available    # 可用无人机列表
GET    /api/v1/drones/:id          # 获取无人机信息

# 操作员接口
POST   /api/v1/drones              # 创建无人机
PUT    /api/v1/drones/:id          # 更新无人机信息
PUT    /api/v1/drones/:id/status   # 更新状态
PUT    /api/v1/drones/:id/position # 更新位置
PUT    /api/v1/drones/:id/battery  # 更新电量

# 管理员接口
DELETE /api/v1/drones/:id          # 删除无人机
```

## 🔐 权限系统

### 角色定义
- **admin**: 管理员 - 具有所有权限
- **operator**: 操作员 - 可以操作无人机和任务
- **viewer**: 观察员 - 只能查看数据

### 权限级别
```
admin (3) > operator (2) > viewer (1)
```

权限检查在路由和控制器层面都有实现，确保数据安全。

## 🧪 测试

### 健康检查
```bash
curl http://localhost:8080/health
curl http://localhost:8080/ping
```

### API 测试示例
```bash
# 登录获取 token (需要实现具体的用户服务)
curl -X POST http://localhost:8080/api/v1/public/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# 使用 token 访问受保护的接口
curl -X GET http://localhost:8080/api/v1/users/profile \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 🔄 与现有系统的关系

### 迁移策略
1. **渐进式迁移**: 新功能使用 MVC 架构开发
2. **接口兼容**: 保持与现有前端的接口兼容性
3. **数据库共享**: 共享现有的数据库和数据模型
4. **服务集成**: 逐步将现有服务迁移到新架构

### 配置调整
- 新的 MVC 服务器可以运行在不同端口 (如 8080)
- 现有的 web-server 继续运行在 8888 端口提供前端页面
- API Gateway 可以将请求路由到不同的服务

## 📈 扩展建议

### 即将实现的功能
1. **Task Controller**: 任务管理控制器
2. **Alert Controller**: 告警管理控制器
3. **Dashboard Controller**: 仪表盘数据聚合
4. **WebSocket Handler**: 实时数据推送

### 架构优化
1. **依赖注入**: 使用 DI 容器管理依赖关系
2. **配置管理**: 增强配置文件和环境变量支持
3. **缓存层**: 添加 Redis 缓存支持
4. **消息队列**: 集成 Kafka 消息处理

### 监控和运维
1. **指标监控**: 添加 Prometheus 指标
2. **链路追踪**: 集成 Jaeger 分布式追踪
3. **健康检查**: 完善健康检查机制
4. **优雅关闭**: 实现服务优雅停止

这个 MVC 架构为项目提供了清晰的代码组织结构，便于维护、测试和扩展。每个层次都有明确的职责分工，遵循了软件工程的最佳实践。
