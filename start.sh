#!/bin/bash

# 无人机控制系统快速启动脚本
echo "🚁 启动无人机控制系统..."
echo "================================="

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go未安装，请先安装Go 1.21+"
    exit 1
fi

# 进入项目目录
cd "$(dirname "$0")"

# 构建所有服务
echo "📦 构建服务..."
make build

if [ $? -ne 0 ]; then
    echo "❌ 构建失败"
    exit 1
fi

echo "✅ 构建完成"

# 启动服务
echo ""
echo "🚀 启动服务..."
echo "================================="

# 启动API网关
echo "启动 API Gateway (端口 8080)..."
./build/api-gateway &
API_PID=$!

# 等待API网关启动
sleep 2

# 启动用户服务
echo "启动 User Service (端口 8081)..."
./build/user-service &
USER_PID=$!

# 启动任务服务
echo "启动 Task Service (端口 8084)..."
./build/task-service &
TASK_PID=$!

# 启动监控服务
echo "启动 Monitor Service (端口 8083)..."
./build/monitor-service &
MONITOR_PID=$!

# 启动无人机控制服务
echo "启动 Drone Control (端口 8082)..."
./build/drone-control &
DRONE_PID=$!

# 等待后端服务启动
sleep 3

# 启动Web前端服务
echo "启动 Web Server (端口 8888)..."
./build/web-server &
WEB_PID=$!

echo ""
echo "🎉 系统启动成功！"
echo "================================="
echo "📊 Web控制台: http://localhost:8888"
echo "🔗 API网关: http://localhost:8080"
echo "📈 监控服务: http://localhost:8083"
echo ""
echo "💡 提示:"
echo "  - 访问 http://localhost:8888 查看Web界面"
echo "  - 使用演示模式体验完整功能"
echo "  - 按 Ctrl+C 停止所有服务"

# 创建停止函数
cleanup() {
    echo ""
    echo "🛑 正在停止服务..."
    kill $API_PID $USER_PID $TASK_PID $MONITOR_PID $DRONE_PID $WEB_PID 2>/dev/null
    echo "✅ 所有服务已停止"
    exit 0
}

# 捕获Ctrl+C信号
trap cleanup SIGINT SIGTERM

# 保持脚本运行
echo "按 Ctrl+C 停止所有服务..."
wait
