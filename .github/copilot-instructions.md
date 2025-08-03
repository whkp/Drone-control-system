# Copilot 自定义指令

<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

## 项目概述
这是一个基于 Go 的无人机控制系统，采用微服务架构设计。

## 技术栈
- **框架**: Gin (Web框架)
- **数据库**: PostgreSQL + Redis
- **通信**: gRPC + RESTful API + WebSocket
- **大模型**: DeepSeek API 集成

## 代码规范
1. 使用 Go 标准命名规范 (驼峰命名)
2. 每个服务都应该有独立的包结构
3. 使用依赖注入模式
4. 错误处理要完整和明确
5. 添加适当的日志记录
6. 使用上下文 (context) 进行超时控制

## 架构模式
- 微服务架构
- DDD (领域驱动设计)
- CQRS (命令查询职责分离)
- 事件驱动架构

## 安全要求
- JWT 身份认证
- RBAC 权限控制
- 输入验证和清理
- 限流和防护机制
