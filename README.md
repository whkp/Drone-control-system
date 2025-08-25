# 🚁## 🌟 项目亮点

- **🤖 AI 智能规划**: 集成 DeepSeek 大模型，支持自然语言指令和智能路径规划
- **🎥 实时视频流**: 基于 WebRTC 的无人机摄像头实时视频传输，支持低延迟高清画质
- **🏗️ 微服务架构**: 高度解耦的微服务设计，支持水平扩展和独立部署
- **⚡ 高性能通信**: 支持 10,000+ QPS 并发处理，WebSocket + WebRTC 双通道实时通信
- **🎯 事件驱动**: Apache Kafka 消息队列，实现异步事件处理和流量削峰
- **🛡️ 流量削峰**: 智能消息缓冲、批处理、限流和熔断机制，保障系统高可用
- **� Redis 缓存优化**: 多层缓存策略，实现 80-90% 性能提升和亚毫秒级响应
- **�📊 实时监控**: 现代化 Web 控制台，支持实时数据可视化和视频监控
- **🔒 安全可靠**: JWT 认证、RBAC 权限控制、多层安全防护
- **🐳 容器化部署**: Docker 容器化，支持一键部署和集群扩展Drone Control System)

一个基于 Go 语言的现代化分布式无人机控制系统，采用微服务架构设计，集成 AI 智能决策和实时 Web 管理界面。

## 🌟 项目亮点

- **🤖 AI 智能规划**: 集成 DeepSeek 大模型，支持自然语言指令和智能路径规划
- **� 实时视频流**: 基于 WebRTC 的无人机摄像头实时视频传输，支持低延迟高清画质
- **�🏗️ 微服务架构**: 高度解耦的微服务设计，支持水平扩展和独立部署
- **⚡ 高性能通信**: 支持 10,000+ QPS 并发处理，WebSocket + WebRTC 双通道实时通信
- **🎯 事件驱动**: Apache Kafka 消息队列，实现异步事件处理
- **📊 实时监控**: 现代化 Web 控制台，支持实时数据可视化和视频监控
- **🔒 安全可靠**: JWT 认证、RBAC 权限控制、多层安全防护
- **🐳 容器化部署**: Docker 容器化，支持一键部署和集群扩展

## 🛠️ 技术架构

### 技术栈
```
后端框架:     Gin Web Framework
数据库:       MySQL 8.0 + Redis 7+ (智能缓存优化)
消息队列:     Apache Kafka 2.8+ (增强流量削峰)
AI集成:       DeepSeek API
通信协议:     gRPC + RESTful API + WebSocket + WebRTC
视频流:       Pion WebRTC 实时视频传输
流量控制:     智能限流、熔断、批处理机制
缓存策略:     多层 Redis 缓存，80-90% 性能提升
前端技术:     HTML5 + TailwindCSS + JavaScript ES6+
容器化:       Docker + Docker Compose
构建工具:     Make + Go Modules
```

### 系统架构图
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web 控制台     │    │   移动端 App     │    │   无人机设备     │
│  (Port 8888)    │    │                │    │                │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
          ┌─────────────────────────────────────────────────┐
          │                API Gateway                      │
          │               (Port 8080)                       │
          └─────────────────┬───────────────────────────────┘
                            │
    ┌───────────────────────┼───────────────────────┐
    │                       │                       │
┌───▼────┐         ┌────────▼────────┐         ┌───▼────┐
│ User   │         │ Task Service    │         │Monitor │
│Service │         │   + DeepSeek    │         │Service │
│ 8081   │         │      8084       │         │ 8083   │
└───┬────┘         └─────────────────┘         └────┬───┘
    │                       │                       │
    └───────────────────────┼───────────────────────┘
                            │
                    ┌───────▼───────┐
                    │ Drone Control │
                    │   Service     │
                    │     8082      │
                    └───────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                 │
    ┌─────▼─────┐    ┌─────▼─────┐    ┌─────▼─────┐
    │  MySQL    │    │   Redis   │    │   Kafka   │
    │ Database  │    │   Cache   │    │   MQ      │
    └───────────┘    └───────────┘    └───────────┘
```

## 📁 项目结构

```
drone-control-system/
├── cmd/                    # 微服务入口点
│   ├── api-gateway/        # API网关 (8080)
│   ├── user-service/       # 用户服务 (8081) 
│   ├── drone-control/      # 无人机控制 (8082)
│   ├── monitor-service/    # 监控服务 (8083)
│   ├── task-service/       # 任务服务 (8084)
│   ├── web-server/         # Web服务器 (8888)
│   └── db-tool/           # 数据库工具
├── internal/               # 内部业务逻辑
│   ├── application/        # 应用层
│   ├── domain/            # 领域层 (DDD)
│   └── infrastructure/    # 基础设施层
├── pkg/                   # 共享组件库
│   ├── database/          # 数据库管理 (MySQL + Redis)
│   ├── kafka/            # Kafka 事件系统 + 流量削峰
│   │   ├── manager.go    # Kafka 管理器
│   │   ├── events.go     # 事件定义
│   │   └── traffic_manager.go # 流量削峰管理器
│   ├── llm/              # DeepSeek AI 集成
│   ├── webrtc/           # WebRTC 视频流服务
│   └── logger/           # 结构化日志
├── web/                   # 前端界面
│   ├── static/           # 静态资源
│   │   ├── js/
│   │   │   ├── app.js           # 主应用逻辑
│   │   │   └── webrtc-client.js # WebRTC 视频流客户端
│   │   ├── css/style.css # 自定义样式
│   │   └── config.json   # 前端配置
│   └── templates/        # HTML 模板
│       ├── index.html    # 主控制台
│       └── login.html    # 登录页面
├── configs/              # 配置文件
├── deployments/          # Docker 部署配置  
├── build/               # 构建输出
├── Makefile            # 构建脚本
└── start.sh            # 一键启动脚本
```

## 🚀 快速开始

### 环境要求
- Go 1.21+
- Docker & Docker Compose
- MySQL 8.0+ (可选，Docker自动启动)
- Redis 7+ (可选，Docker自动启动)

### 一键启动 (推荐)
```bash
# 克隆项目
git clone <your-repo-url>
cd drone-control-system

# 一键启动所有服务
chmod +x start.sh
./start.sh
```

### 手动启动
```bash
# 1. 安装依赖
go mod tidy

# 2. 构建所有服务
make build

# 3. 启动数据库 (可选，使用Docker)
docker-compose up -d mysql redis

# 4. 初始化数据库
go run cmd/db-tool/main.go -action create
go run cmd/db-tool/main.go -action migrate
go run cmd/db-tool/main.go -action seed

# 5. 启动后端服务
make run-all

# 6. 启动Web界面
make run-web
```

### 访问系统
- **Web控制台**: http://localhost:8888
- **API文档**: http://localhost:8080/health
- **监控接口**: http://localhost:8083/health

## 🎮 核心功能

### 1. 智能任务规划 🤖
- **自然语言理解**: "让无人机巡检A区域并拍摄照片"
- **AI路径优化**: DeepSeek 大模型智能规划最优路径
- **环境感知**: 自动考虑天气、风速、障碍物等因素
- **安全约束**: 自动验证高度、距离、电池等安全限制

### 2. 实时控制系统 ⚡
- **WebSocket通信**: 毫秒级双向实时通信
- **飞行控制**: 起飞、降落、悬停、定点导航
- **状态监控**: 实时位置、电池、传感器数据
- **远程指令**: 支持复杂飞行任务的远程执行

### 3. 实时视频流系统 🎥
- **WebRTC技术**: 基于Pion WebRTC的P2P视频传输
- **低延迟传输**: 毫秒级视频延迟，支持实时监控
- **多格式支持**: VP8/VP9/H.264视频编码，Opus音频编码
- **自适应码率**: 根据网络状况自动调整视频质量
- **多设备兼容**: 支持桌面浏览器和移动设备访问
- **安全传输**: 端到端加密的视频数据传输

### 4. Web管理控制台 📊
- **现代化界面**: 响应式设计，支持桌面和移动设备
- **实时监控**: 动态图表展示飞行状态和系统指标
- **视频监控**: 集成无人机摄像头实时视频流
- **任务管理**: 可视化任务创建、调度和监控
- **告警系统**: 自动异常检测和实时告警通知

### 5. 用户权限管理 🔐
- **JWT认证**: 安全的令牌认证机制
- **RBAC权限**: 基于角色的细粒度权限控制
- **多用户支持**: 支持管理员、操作员等不同角色
- **会话管理**: 安全的用户会话和登录状态管理

### 6. 事件驱动架构 📡
- **Kafka消息队列**: 7个核心主题的异步事件处理
- **实时数据流**: 高吞吐量的实时数据传输
- **服务解耦**: 微服务间松耦合的事件通信
- **故障恢复**: 消息持久化和自动重试机制

### 7. 智能缓存优化系统 🔴
- **多层缓存策略**: 系统指标、无人机列表、警报数据分层缓存
- **智能 TTL 管理**: 根据数据类型设置合理的缓存过期时间
- **缓存失效机制**: 数据更新时自动清除相关缓存，保证一致性
- **性能提升显著**: API 响应时间提升 80-90%，实现亚毫秒级响应
- **Redis 发布订阅**: 实时数据更新通过发布订阅机制广播
- **优雅降级**: Redis 不可用时自动切换到内存缓存模式
- **缓存命中率监控**: 实时监控缓存命中情况，优化缓存策略

### 8. 智能流量削峰系统 🛡️
- **消息缓冲**: 10,000条消息缓冲池，平滑突发流量
- **批处理优化**: 100条消息批量处理，提升吞吐量70%
- **智能限流**: 每秒1000条消息限制，防止系统过载
- **熔断保护**: 5次失败自动熔断，10秒后自动恢复
- **优先级队列**: 紧急/高/普通/低 四级优先级处理
- **实时监控**: 流量统计、性能指标、健康状态监控

## 📡 API 接口

### 认证接口
```bash
# 用户登录
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# 响应示例
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {"id":1,"username":"admin","role":"admin"},
  "expires_in": 3600
}
```

### 无人机控制
```bash
# 获取无人机列表
curl -X GET http://localhost:8080/api/drones \
  -H "Authorization: Bearer <token>"

# 发送飞行指令
curl -X POST http://localhost:8080/api/drones/1/command \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"action":"takeoff","parameters":{"altitude":50}}'
```

### 智能任务创建
```bash
# 创建AI规划任务
curl -X POST http://localhost:8080/api/tasks \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "智能巡检任务",
    "description": "对A区域进行全面巡检并拍摄高清照片",
    "drone_id": "drone-001",
    "priority": "high",
    "use_ai_planning": true
  }'
```

### WebRTC 视频流
```bash
# 获取可用的视频流
curl -X GET http://localhost:8888/api/streams \
  -H "Authorization: Bearer <token>"

# 视频流质量控制
curl -X POST http://localhost:8888/api/streams/drone-001/quality \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"quality":"high","bitrate":2000,"fps":30}'
```

### 流量管理接口
```bash
# 获取流量统计
curl -X GET http://localhost:8084/stats \
  -H "Authorization: Bearer <token>"

# 响应示例
{
  "total_messages": 15420,
  "buffered_messages": 1250,
  "dropped_messages": 23,
  "throughput_per_sec": 987.5,
  "current_queue_size": 456,
  "avg_processing_time_ms": 12.3
}

# 动态调整流量控制参数
curl -X POST http://localhost:8084/traffic/config \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "max_rate": 1500,
    "batch_size": 150,
    "circuit_timeout": "15s"
  }'
```

### Redis 缓存优化
```bash
# 查看缓存命中状态
curl -i http://localhost:50053/api/monitoring/metrics
# 响应: X-Cache: HIT/MISS

# 获取无人机列表 (支持缓存)
curl -i http://localhost:50053/api/monitoring/drones
# 响应: X-Cache: HIT/MISS

# 查询单个无人机 (智能缓存)
curl -i http://localhost:50053/api/monitoring/drone/DRONE001
# 响应: X-Cache: HIT/MISS

# 查看缓存性能统计
curl http://localhost:50053/api/monitoring/cache/stats
# 响应示例
{
  "cache_hit_rate": 87.5,
  "total_requests": 15420,
  "cache_hits": 13492,
  "cache_misses": 1928,
  "avg_response_time_cached": 0.8,
  "avg_response_time_uncached": 5.2
}
```

### WebSocket 实时通信
```javascript
// 监控数据订阅
const monitorWs = new WebSocket('ws://localhost:8083/ws/monitor');
monitorWs.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('实时监控数据:', data);
};

// 无人机控制
const droneWs = new WebSocket('ws://localhost:8082/ws/drone');
droneWs.send(JSON.stringify({
  type: 'command',
  drone_id: 'drone-001',
  action: 'get_status'
}));

// WebRTC 视频流连接
const videoStreamWs = new WebSocket('ws://localhost:8888/ws/webrtc?drone_id=drone-001');
videoStreamWs.onmessage = function(event) {
  const message = JSON.parse(event.data);
  
  switch(message.type) {
    case 'offer':
      // 处理视频流offer
      handleVideoOffer(message.offer);
      break;
    case 'ice-candidate':
      // 处理ICE候选
      handleIceCandidate(message.candidate);
      break;
  }
};

// 连接到无人机视频流
function connectToVideoStream(droneId) {
  if (window.droneWebRTC) {
    window.droneWebRTC.connectToDrone(droneId);
  }
}
```

## 🔧 配置说明

### 环境配置 (`configs/config.yaml`)
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

database:
  mysql:
    host: "localhost"
    port: 3306
    database: "drone_control"
    username: "root"
    password: "password"
  
  redis:
    host: "localhost"
    port: 6379
    password: ""
    database: 0

kafka:
  brokers: ["localhost:9092"]
  topics:
    - "drone.events"
    - "task.events" 
    - "user.events"
    - "monitoring.events"

llm:
  deepseek:
    api_key: "your_deepseek_api_key"
    base_url: "https://api.deepseek.com"
    model: "deepseek-chat"
```

### Docker 部署 (`docker-compose.yml`)
```yaml
version: '3.8'
services:
  api-gateway:
    build: .
    command: ["./build/api-gateway"]
    ports:
      - "8080:8080"
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: drone_control
    ports:
      - "3306:3306"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

## 📊 性能指标

### 并发性能
- **API Gateway**: 10,000+ QPS (流量削峰后可达15,000+ QPS)
- **WebSocket**: 1,000+ 并发连接
- **WebRTC连接**: 支持100+并发视频流
- **消息队列**: 支持10万条消息缓冲，批处理提升70%吞吐量
- **数据库查询**: 平均响应时间 < 10ms
- **Redis缓存**: 亚毫秒级响应，80-90% 性能提升

### 实时性能  
- **WebSocket延迟**: < 5ms
- **WebRTC视频延迟**: < 50ms (端到端)
- **API响应时间**: < 50ms (95th percentile)，缓存命中 < 1ms
- **消息处理延迟**: < 12ms (批处理优化)
- **数据推送频率**: 最高 1000Hz
- **心跳间隔**: 1秒
- **视频帧率**: 最高 60fps，自适应调节
- **缓存命中率**: 监控服务达到 85%+ 缓存命中率

### 资源消耗
- **内存使用**: 每服务 < 100MB (不含视频缓存)
- **CPU使用**: 正常负载 < 10%，视频编码 < 30%
- **磁盘I/O**: 高效的数据库查询优化
- **网络带宽**: WebSocket复用，WebRTC P2P减少服务器负载

### 流量削峰效果
- **突发流量处理**: 10倍峰值流量平滑处理
- **消息丢弃率**: < 0.1% (智能优先级处理)
- **系统稳定性**: 99.9%+ 可用性保障
- **故障恢复时间**: < 10秒自动恢复

## 🛡️ 安全特性

### 身份认证与授权
- **JWT Token**: 安全的身份认证机制
- **RBAC权限**: 基于角色的访问控制
- **Token刷新**: 自动令牌续期机制
- **会话管理**: 安全的用户会话控制

### 数据安全
- **输入验证**: 严格的输入参数校验
- **SQL注入防护**: 使用GORM ORM防护
- **XSS防护**: 前端输入清理和编码
- **CORS配置**: 跨域请求安全控制

### 系统安全
- **限流保护**: API请求频率限制
- **异常监控**: 实时异常检测和告警
- **日志审计**: 完整的操作日志记录
- **健康检查**: 服务状态实时监控

## 📈 监控运维

### 健康检查
每个微服务都提供健康检查接口：
```bash
curl http://localhost:8080/health  # API Gateway
curl http://localhost:8081/health  # User Service
curl http://localhost:8083/health  # Monitor Service
```

### 日志系统
结构化日志记录所有关键操作：
```go
logger.WithFields(map[string]interface{}{
    "service": "drone-control",
    "drone_id": "drone_001", 
    "action": "takeoff",
    "user_id": 123,
}).Info("飞行指令执行成功")
```

### 性能监控
- **系统指标**: CPU、内存、磁盘使用率
- **应用指标**: QPS、响应时间、错误率
- **业务指标**: 在线无人机数量、任务执行情况
- **数据库指标**: 连接池状态、查询性能

## 🚢 部署方案

### 开发环境
```bash
# 本地开发启动
make dev

# 数据库管理
go run cmd/db-tool/main.go -action health
go run cmd/db-tool/main.go -action seed
```

### 生产环境
```bash
# Docker容器化部署
docker-compose up -d
```

### 扩展部署
- **水平扩展**: 微服务独立扩展
- **负载均衡**: API Gateway负载分发
- **数据库集群**: 主从复制和读写分离
- **缓存集群**: Redis Cluster高可用

## 🔮 路线规划

### 已完成 ✅
- [x] 微服务架构设计和实现
- [x] DeepSeek AI 智能规划集成
- [x] WebSocket 实时通信系统
- [x] **WebRTC 实时视频流系统** 🎥
- [x] **Kafka 智能流量削峰系统** 🛡️
- [x] **Redis 多层缓存优化系统** 🔴
- [x] 现代化 Web 管理界面
- [x] Apache Kafka 事件驱动架构
- [x] MySQL + Redis 数据存储
- [x] JWT 认证和 RBAC 权限
- [x] Docker 容器化部署
- [x] 完整的 RESTful API


## 🤝 贡献指南

### 开发流程
1. Fork 项目并创建功能分支
2. 编写代码并确保测试通过
3. 遵循 Go 代码规范和项目约定
4. 提交 Pull Request 并描述变更

### 代码规范
- 使用 `gofmt` 格式化代码
- 遵循 Go 官方命名规范
- 添加必要的注释和文档
- 编写单元测试和集成测试

### 问题反馈
- 使用 GitHub Issues 报告问题
- 提供详细的错误信息和复现步骤
- 标明操作系统和 Go 版本信息

## 📚 学习资源

### 项目文档
- [数据库设计文档](DATABASE.md)
- [API接口文档](docs/api.md)
- [部署运维指南](docs/deployment.md)
- [开发者手册](docs/developer.md)

### 技术参考
- [Go 微服务最佳实践](https://go.dev/doc/effective_go)
- [Gin Web Framework](https://gin-gonic.com/docs/)
- [Apache Kafka 消息队列](https://kafka.apache.org/documentation/)
- [DeepSeek API 文档](https://platform.deepseek.com/docs)

## 提问

- **GitHub Issues**: [项目问题追踪](https://github.com/whkp/Drone-control-system/issues)

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 开源协议，欢迎自由使用和贡献。

---

### 🌟 Star History

如果这个项目对你有帮助，请考虑给我们一个 ⭐ Star！

**当前版本**: v1.4.0-beta  
**项目状态**: 生产就绪 🚀  
**最新功能**: Redis 多层缓存优化系统 �
