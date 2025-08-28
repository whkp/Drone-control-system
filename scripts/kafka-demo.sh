#!/bin/bash

# Kafka集成演示脚本
# 用于测试无人机控制系统的Kafka集成功能

echo "🚀 启动无人机控制系统 Kafka 集成演示"
echo "========================================"

# 检查Docker是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker未运行，请先启动Docker"
    exit 1
fi

echo "📦 启动基础设施服务..."

# 启动Kafka和依赖服务
cd deployments
docker-compose up -d zookeeper kafka redis mysql

echo "⏳ 等待服务启动..."
sleep 10

# 检查服务状态
echo "🔍 检查服务状态..."
docker-compose ps

echo "📊 创建Kafka主题..."

# 等待Kafka完全启动
sleep 5

# 创建必要的主题
docker exec drone_kafka kafka-topics --create --topic drone-events --partitions 3 --replication-factor 1 --if-not-exists --bootstrap-server localhost:9092
docker exec drone_kafka kafka-topics --create --topic task-events --partitions 3 --replication-factor 1 --if-not-exists --bootstrap-server localhost:9092
docker exec drone_kafka kafka-topics --create --topic alert-events --partitions 3 --replication-factor 1 --if-not-exists --bootstrap-server localhost:9092
docker exec drone_kafka kafka-topics --create --topic user-events --partitions 3 --replication-factor 1 --if-not-exists --bootstrap-server localhost:9092
docker exec drone_kafka kafka-topics --create --topic system-events --partitions 3 --replication-factor 1 --if-not-exists --bootstrap-server localhost:9092

echo "📋 列出已创建的主题:"
docker exec drone_kafka kafka-topics --list --bootstrap-server localhost:9092

echo ""
echo "🎯 演示场景说明:"
echo "================="
echo "1. 📍 无人机位置更新 - 测试实时位置追踪和异常检测"
echo "2. 🔋 电量监控 - 测试电量低告警和预测性维护"
echo "3. 📋 任务管理 - 测试任务状态事件和失败处理"
echo "4. 🚨 智能告警 - 测试告警聚合和抑制"
echo "5. 📡 WebSocket推送 - 测试实时数据推送到前端"

echo ""
echo "🌐 访问地址:"
echo "============="
echo "• API服务器: http://localhost:8080"
echo "• WebSocket: ws://localhost:8080/ws"
echo "• Kafka UI: http://localhost:8090 (如果启用)"
echo "• 前端页面: http://localhost:8080/static/"

echo ""
echo "🔧 测试命令:"
echo "============="
echo "# 1. 启动API服务器"
echo "cd .."
echo "go run cmd/mvc-server/main.go"
echo ""
echo "# 2. 测试位置更新 (在另一个终端)"
echo "curl -X PUT http://localhost:8080/api/v1/drones/1/position \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{\"latitude\": 40.7128, \"longitude\": -74.0060, \"altitude\": 100.5, \"heading\": 45}'"
echo ""
echo "# 3. 测试电量更新"
echo "curl -X PUT http://localhost:8080/api/v1/drones/1/battery \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{\"battery\": 15}'"
echo ""
echo "# 4. 监控Kafka消息"
echo "docker exec -it drone_kafka kafka-console-consumer \\"
echo "  --bootstrap-server localhost:9092 \\"
echo "  --topic drone-events \\"
echo "  --from-beginning"

echo ""
echo "⚡ 性能特性:"
echo "============="
echo "• 🚀 异步事件处理 - HTTP请求立即返回，后台处理事件"
echo "• 📊 智能告警聚合 - 防止告警风暴，提供智能分析"
echo "• 🔄 事件重放 - 支持历史数据回溯和调试"
echo "• 📈 削峰填谷 - 平滑处理高并发请求"
echo "• 🌐 实时推送 - WebSocket实时推送事件到前端"

echo ""
echo "✅ 基础设施已启动！现在可以启动API服务器进行测试。"
echo "💡 提示: 使用 'docker-compose logs -f kafka' 查看Kafka日志"
echo "🛑 停止服务: docker-compose down"
