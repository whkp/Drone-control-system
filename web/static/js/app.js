// 全局变量
const API_BASE = 'http://localhost:8080/api/v1';
const MONITOR_BASE = 'http://localhost:8083/api/monitoring';
let authToken = localStorage.getItem('authToken') || '';
let websocket = null;
let metricsChart = null;
let droneWebRTC = null; // WebRTC客户端实例

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

// 应用初始化
async function initializeApp() {
    console.log('初始化应用...');
    
    // 设置 axios 默认配置
    axios.defaults.baseURL = API_BASE;
    if (authToken) {
        axios.defaults.headers.common['Authorization'] = `Bearer ${authToken}`;
    }
    
    // 检查认证状态
    if (!authToken) {
        // 如果没有token，显示登录界面或使用演示模式
        console.log('使用演示模式');
        authToken = 'demo-token';
        localStorage.setItem('authToken', authToken);
    }
    
    // 初始化WebSocket连接
    initializeWebSocket();
    
    // 初始化WebRTC（等待客户端加载完成）
    setTimeout(() => {
        if (window.droneWebRTC) {
            droneWebRTC = window.droneWebRTC;
            console.log('WebRTC客户端已连接');
        }
    }, 1000);
    
    // 初始化图表
    initializeCharts();
    
    // 加载初始数据
    await loadDashboardData();
    await loadDrones();
    await loadTasks();
    await loadAlerts();
    
    // 设置定时刷新
    setInterval(loadDashboardData, 30000); // 30秒刷新一次概览数据
    setInterval(loadAlerts, 10000); // 10秒刷新一次告警
    
    console.log('应用初始化完成');
}

// 初始化WebSocket连接
function initializeWebSocket() {
    try {
        websocket = new WebSocket('ws://localhost:8083/ws/monitoring');
        
        websocket.onopen = function() {
            console.log('WebSocket连接已建立');
            updateConnectionStatus(true, 'WebSocket已连接');
        };
        
        websocket.onmessage = function(event) {
            const data = JSON.parse(event.data);
            handleWebSocketMessage(data);
        };
        
        websocket.onclose = function() {
            console.log('WebSocket连接已关闭');
            updateConnectionStatus(false, 'WebSocket已断开');
            // 5秒后尝试重连
            setTimeout(initializeWebSocket, 5000);
        };
        
        websocket.onerror = function(error) {
            console.error('WebSocket错误:', error);
            updateConnectionStatus(false, 'WebSocket连接错误');
        };
    } catch (error) {
        console.error('WebSocket初始化失败:', error);
        updateConnectionStatus(false, 'WebSocket不可用');
    }
}

// 处理WebSocket消息
function handleWebSocketMessage(data) {
    switch (data.type) {
        case 'drone_update':
            updateDroneRealTimeData(data.data);
            break;
        case 'alert_created':
            addNewAlert(data.data);
            break;
        case 'task_progress':
            updateTaskProgress(data.data);
            break;
        case 'initial_data':
            handleInitialData(data);
            break;
        default:
            console.log('收到未知类型的WebSocket消息:', data);
    }
}

// 更新连接状态
function updateConnectionStatus(connected, text) {
    const statusDot = document.getElementById('connectionStatus');
    const statusText = document.getElementById('connectionText');
    
    if (connected) {
        statusDot.className = 'status-dot status-online';
    } else {
        statusDot.className = 'status-dot status-offline';
    }
    statusText.textContent = text;
}

// 初始化图表
function initializeCharts() {
    const ctx = document.getElementById('metricsChart');
    if (!ctx) return;
    
    metricsChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [
                {
                    label: 'CPU使用率 (%)',
                    data: [],
                    borderColor: 'rgb(59, 130, 246)',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    tension: 0.4
                },
                {
                    label: '内存使用率 (%)',
                    data: [],
                    borderColor: 'rgb(16, 185, 129)',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    tension: 0.4
                },
                {
                    label: '活跃连接数',
                    data: [],
                    borderColor: 'rgb(245, 158, 11)',
                    backgroundColor: 'rgba(245, 158, 11, 0.1)',
                    tension: 0.4
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: '系统性能监控'
                }
            },
            scales: {
                x: {
                    display: true,
                    title: {
                        display: true,
                        text: '时间'
                    }
                },
                y: {
                    display: true,
                    title: {
                        display: true,
                        text: '数值'
                    },
                    beginAtZero: true
                }
            }
        }
    });
    
    // 模拟实时数据更新
    updateMetricsChart();
}

// 更新指标图表
function updateMetricsChart() {
    if (!metricsChart) return;
    
    const now = new Date().toLocaleTimeString();
    const data = metricsChart.data;
    
    // 添加新数据点
    data.labels.push(now);
    data.datasets[0].data.push(Math.random() * 100); // CPU
    data.datasets[1].data.push(Math.random() * 100); // 内存
    data.datasets[2].data.push(Math.floor(Math.random() * 50)); // 连接数
    
    // 保持最多20个数据点
    if (data.labels.length > 20) {
        data.labels.shift();
        data.datasets.forEach(dataset => dataset.data.shift());
    }
    
    metricsChart.update('none');
    
    // 5秒后更新
    setTimeout(updateMetricsChart, 5000);
}

// 加载仪表板数据
async function loadDashboardData() {
    try {
        // 模拟API调用，实际项目中应该调用真实API
        const metrics = {
            total_drones: 12,
            active_drones: 8,
            running_tasks: 3,
            unacknowledged_alerts: 2
        };
        
        document.getElementById('totalDrones').textContent = metrics.total_drones;
        document.getElementById('activeDrones').textContent = metrics.active_drones;
        document.getElementById('runningTasks').textContent = metrics.running_tasks;
        document.getElementById('unacknowledgedAlerts').textContent = metrics.unacknowledged_alerts;
        
    } catch (error) {
        console.error('加载仪表板数据失败:', error);
        showNotification('加载数据失败', 'error');
    }
}

// 加载无人机列表
async function loadDrones() {
    try {
        // 模拟无人机数据
        const drones = [
            {
                id: 1,
                name: 'Drone-001',
                model: 'DJI-Mini3',
                status: 'online',
                battery: 85,
                position: { latitude: 40.7128, longitude: -74.0060, altitude: 120 },
                last_heartbeat: new Date().toISOString()
            },
            {
                id: 2,
                name: 'Drone-002',
                model: 'DJI-Air3',
                status: 'flying',
                battery: 72,
                position: { latitude: 40.7589, longitude: -73.9851, altitude: 95 },
                last_heartbeat: new Date().toISOString()
            },
            {
                id: 3,
                name: 'Drone-003',
                model: 'DJI-Mavic3',
                status: 'offline',
                battery: 15,
                position: { latitude: 40.7505, longitude: -73.9934, altitude: 0 },
                last_heartbeat: new Date(Date.now() - 300000).toISOString() // 5分钟前
            }
        ];
        
        renderDrones(drones);
        updateDroneSelectOptions(drones.filter(d => d.status !== 'offline'));
        
    } catch (error) {
        console.error('加载无人机数据失败:', error);
        showNotification('加载无人机数据失败', 'error');
    }
}

// 渲染无人机列表
function renderDrones(drones) {
    const container = document.getElementById('dronesContainer');
    
    container.innerHTML = drones.map(drone => `
        <div class="drone-card bg-white rounded-lg shadow-md p-6 drone-${drone.status}">
            <div class="flex justify-between items-start mb-4">
                <div>
                    <h3 class="text-lg font-semibold text-gray-900">${drone.name}</h3>
                    <p class="text-sm text-gray-500">${drone.model}</p>
                </div>
                <div class="flex items-center">
                    <span class="status-dot status-${drone.status}"></span>
                    <span class="text-sm font-medium ${getStatusTextColor(drone.status)}">${getStatusText(drone.status)}</span>
                </div>
            </div>
            
            <div class="space-y-2 mb-4">
                <div class="flex justify-between">
                    <span class="text-sm text-gray-500">电量:</span>
                    <div class="flex items-center">
                        <div class="w-16 bg-gray-200 rounded-full h-2 mr-2">
                            <div class="bg-${getBatteryColor(drone.battery)} h-2 rounded-full" style="width: ${drone.battery}%"></div>
                        </div>
                        <span class="text-sm font-medium">${drone.battery}%</span>
                    </div>
                </div>
                
                <div class="flex justify-between">
                    <span class="text-sm text-gray-500">位置:</span>
                    <span class="text-sm">${drone.position.latitude.toFixed(4)}, ${drone.position.longitude.toFixed(4)}</span>
                </div>
                
                <div class="flex justify-between">
                    <span class="text-sm text-gray-500">高度:</span>
                    <span class="text-sm">${drone.position.altitude}m</span>
                </div>
                
                <div class="flex justify-between">
                    <span class="text-sm text-gray-500">最后心跳:</span>
                    <span class="text-sm">${formatTime(drone.last_heartbeat)}</span>
                </div>
            </div>
            
            <div class="flex space-x-2">
                <button class="flex-1 bg-blue-500 hover:bg-blue-600 text-white py-2 px-3 rounded text-sm" onclick="viewDroneDetails(${drone.id})">
                    <i class="fas fa-eye mr-1"></i>详情
                </button>
                <button class="flex-1 bg-green-500 hover:bg-green-600 text-white py-2 px-3 rounded text-sm" onclick="sendCommand(${drone.id})" ${drone.status === 'offline' ? 'disabled' : ''}>
                    <i class="fas fa-paper-plane mr-1"></i>控制
                </button>
                <button class="flex-1 bg-purple-500 hover:bg-purple-600 text-white py-2 px-3 rounded text-sm" onclick="startDroneStream(${drone.id})" ${drone.status === 'offline' ? 'disabled' : ''}>
                    <i class="fas fa-video mr-1"></i>视频
                </button>
            </div>
        </div>
    `).join('');
}

// 加载任务列表
async function loadTasks() {
    try {
        // 模拟任务数据
        const tasks = [
            {
                id: 1,
                name: '区域巡检任务A',
                drone_id: 1,
                drone_name: 'Drone-001',
                status: 'running',
                progress: 65,
                type: 'inspection',
                priority: 'high',
                created_at: new Date(Date.now() - 7200000).toISOString(), // 2小时前
                estimated_duration: 120 // 分钟
            },
            {
                id: 2,
                name: '监控任务B',
                drone_id: 2,
                drone_name: 'Drone-002',
                status: 'pending',
                progress: 0,
                type: 'monitoring',
                priority: 'normal',
                created_at: new Date(Date.now() - 1800000).toISOString(), // 30分钟前
                estimated_duration: 180
            },
            {
                id: 3,
                name: '紧急配送任务',
                drone_id: null,
                drone_name: '未分配',
                status: 'scheduled',
                progress: 0,
                type: 'delivery',
                priority: 'urgent',
                created_at: new Date().toISOString(),
                estimated_duration: 60
            }
        ];
        
        renderTasks(tasks);
        
    } catch (error) {
        console.error('加载任务数据失败:', error);
        showNotification('加载任务数据失败', 'error');
    }
}

// 渲染任务列表
function renderTasks(tasks) {
    const tbody = document.getElementById('tasksTableBody');
    
    tbody.innerHTML = tasks.map(task => `
        <tr>
            <td class="px-6 py-4 whitespace-nowrap">
                <div class="text-sm font-medium text-gray-900">${task.name}</div>
                <div class="text-sm text-gray-500">${getTaskTypeText(task.type)}</div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">${task.drone_name}</td>
            <td class="px-6 py-4 whitespace-nowrap">
                <span class="px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full ${getTaskStatusClass(task.status)}">
                    ${getTaskStatusText(task.status)}
                </span>
                <span class="px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full ${getPriorityClass(task.priority)} ml-1">
                    ${getPriorityText(task.priority)}
                </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center">
                    <div class="w-16 bg-gray-200 rounded-full h-2 mr-2">
                        <div class="bg-blue-500 h-2 rounded-full" style="width: ${task.progress}%"></div>
                    </div>
                    <span class="text-sm text-gray-900">${task.progress}%</span>
                </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">${formatTime(task.created_at)}</td>
            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                <button class="text-blue-600 hover:text-blue-900 mr-3" onclick="viewTaskDetails(${task.id})">详情</button>
                <button class="text-green-600 hover:text-green-900 mr-3" onclick="startTask(${task.id})" ${task.status === 'running' ? 'disabled' : ''}>
                    ${task.status === 'running' ? '运行中' : '启动'}
                </button>
                <button class="text-red-600 hover:text-red-900" onclick="deleteTask(${task.id})">删除</button>
            </td>
        </tr>
    `).join('');
}

// 加载告警列表
async function loadAlerts() {
    try {
        // 获取监控告警数据
        const response = await fetch(`${MONITOR_BASE}/alerts`);
        let alerts = [];
        
        if (response.ok) {
            const data = await response.json();
            alerts = data.alerts || [];
        } else {
            // 如果API不可用，使用模拟数据
            alerts = [
                {
                    alert_id: 'battery_001_' + Date.now(),
                    drone_id: 'Drone-001',
                    level: 'WARNING',
                    type: 'BATTERY_LOW',
                    message: '无人机 Drone-001 电池电量低于20%',
                    timestamp: new Date(Date.now() - 300000).toISOString(),
                    acknowledged: false
                },
                {
                    alert_id: 'connection_003_' + Date.now(),
                    drone_id: 'Drone-003',
                    level: 'ERROR',
                    type: 'CONNECTION_LOST',
                    message: '无人机 Drone-003 连接丢失',
                    timestamp: new Date(Date.now() - 600000).toISOString(),
                    acknowledged: false
                }
            ];
        }
        
        renderAlerts(alerts);
        
    } catch (error) {
        console.error('加载告警数据失败:', error);
        // 使用模拟数据
        const alerts = [
            {
                alert_id: 'demo_001',
                drone_id: 'Demo-Drone',
                level: 'INFO',
                type: 'SYSTEM_INFO',
                message: '系统运行正常，演示模式',
                timestamp: new Date().toISOString(),
                acknowledged: false
            }
        ];
        renderAlerts(alerts);
    }
}

// 渲染告警列表
function renderAlerts(alerts) {
    const container = document.getElementById('alertsContainer');
    
    if (alerts.length === 0) {
        container.innerHTML = '<div class="text-center py-8 text-gray-500">暂无告警信息</div>';
        return;
    }
    
    container.innerHTML = alerts.map(alert => `
        <div class="alert-card bg-white rounded-lg shadow-md p-4 mb-4 border-l-4 ${getAlertBorderColor(alert.level)}">
            <div class="flex justify-between items-start">
                <div class="flex-1">
                    <div class="flex items-center mb-2">
                        <i class="fas ${getAlertIcon(alert.type)} ${getAlertIconColor(alert.level)} mr-2"></i>
                        <span class="font-semibold text-gray-900">${alert.type}</span>
                        <span class="px-2 py-1 ml-2 text-xs font-medium rounded-full ${getAlertLevelClass(alert.level)}">
                            ${alert.level}
                        </span>
                    </div>
                    <p class="text-gray-700 mb-2">${alert.message}</p>
                    <div class="flex items-center text-sm text-gray-500">
                        <i class="fas fa-helicopter mr-1"></i>
                        <span class="mr-4">${alert.drone_id}</span>
                        <i class="fas fa-clock mr-1"></i>
                        <span>${formatTime(alert.timestamp)}</span>
                    </div>
                </div>
                <div class="flex space-x-2">
                    ${!alert.acknowledged ? `
                        <button class="bg-green-500 hover:bg-green-600 text-white px-3 py-1 rounded text-sm" onclick="acknowledgeAlert('${alert.alert_id}')">
                            <i class="fas fa-check mr-1"></i>确认
                        </button>
                    ` : `
                        <span class="text-green-600 text-sm">
                            <i class="fas fa-check-circle mr-1"></i>已确认
                        </span>
                    `}
                </div>
            </div>
        </div>
    `).join('');
}

// 标签页切换
function switchTab(tabName) {
    // 隐藏所有面板
    document.querySelectorAll('.tab-content').forEach(panel => {
        panel.classList.add('hidden');
    });
    
    // 重置所有标签按钮样式
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.className = 'tab-btn border-b-2 border-transparent py-4 px-1 text-sm font-medium text-gray-500 hover:text-gray-700';
    });
    
    // 显示选中的面板
    document.getElementById(`${tabName}-panel`).classList.remove('hidden');
    
    // 激活选中的标签按钮
    const activeBtn = document.getElementById(`${tabName}-tab`);
    activeBtn.className = 'tab-btn border-b-2 border-blue-500 py-4 px-1 text-sm font-medium text-blue-600';
    
    // 如果切换到监控面板，更新实时数据
    if (tabName === 'monitoring') {
        updateMonitoringPanel();
    }
}

// 模态框相关功能
function openAddDroneModal() {
    document.getElementById('addDroneModal').classList.remove('hidden');
}

function closeAddDroneModal() {
    document.getElementById('addDroneModal').classList.add('hidden');
    document.getElementById('addDroneForm').reset();
}

function openAddTaskModal() {
    document.getElementById('addTaskModal').classList.remove('hidden');
}

function closeAddTaskModal() {
    document.getElementById('addTaskModal').classList.add('hidden');
    document.getElementById('addTaskForm').reset();
}

// 表单提交处理
document.getElementById('addDroneForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const droneData = {
        name: document.getElementById('droneName').value,
        model: document.getElementById('droneModel').value,
        description: document.getElementById('droneDescription').value,
        status: 'offline' // 新添加的无人机默认离线状态
    };
    
    try {
        // 这里应该调用真实的API
        console.log('创建无人机:', droneData);
        showNotification('无人机添加成功', 'success');
        closeAddDroneModal();
        loadDrones(); // 重新加载无人机列表
    } catch (error) {
        console.error('添加无人机失败:', error);
        showNotification('添加无人机失败', 'error');
    }
});

document.getElementById('addTaskForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const taskData = {
        name: document.getElementById('taskName').value,
        drone_id: document.getElementById('taskDrone').value,
        type: document.getElementById('taskType').value,
        priority: document.getElementById('taskPriority').value,
        description: document.getElementById('taskDescription').value
    };
    
    try {
        // 这里应该调用真实的API
        console.log('创建任务:', taskData);
        showNotification('任务创建成功', 'success');
        closeAddTaskModal();
        loadTasks(); // 重新加载任务列表
    } catch (error) {
        console.error('创建任务失败:', error);
        showNotification('创建任务失败', 'error');
    }
});

// 辅助函数
function getStatusText(status) {
    const statusMap = {
        'online': '在线',
        'flying': '飞行中',
        'offline': '离线',
        'maintenance': '维护中'
    };
    return statusMap[status] || status;
}

function getStatusTextColor(status) {
    const colorMap = {
        'online': 'text-green-600',
        'flying': 'text-blue-600',
        'offline': 'text-red-600',
        'maintenance': 'text-yellow-600'
    };
    return colorMap[status] || 'text-gray-600';
}

function getBatteryColor(battery) {
    if (battery > 50) return 'green-500';
    if (battery > 20) return 'yellow-500';
    return 'red-500';
}

function getTaskTypeText(type) {
    const typeMap = {
        'inspection': '巡检任务',
        'monitoring': '监控任务',
        'survey': '测绘任务',
        'delivery': '配送任务'
    };
    return typeMap[type] || type;
}

function getTaskStatusText(status) {
    const statusMap = {
        'pending': '等待中',
        'scheduled': '已调度',
        'running': '执行中',
        'completed': '已完成',
        'failed': '失败',
        'cancelled': '已取消'
    };
    return statusMap[status] || status;
}

function getTaskStatusClass(status) {
    const classMap = {
        'pending': 'bg-gray-100 text-gray-800',
        'scheduled': 'bg-blue-100 text-blue-800',
        'running': 'bg-green-100 text-green-800',
        'completed': 'bg-green-100 text-green-800',
        'failed': 'bg-red-100 text-red-800',
        'cancelled': 'bg-red-100 text-red-800'
    };
    return classMap[status] || 'bg-gray-100 text-gray-800';
}

function getPriorityText(priority) {
    const priorityMap = {
        'low': '低',
        'normal': '普通',
        'high': '高',
        'urgent': '紧急'
    };
    return priorityMap[priority] || priority;
}

function getPriorityClass(priority) {
    const classMap = {
        'low': 'bg-gray-100 text-gray-800',
        'normal': 'bg-blue-100 text-blue-800',
        'high': 'bg-yellow-100 text-yellow-800',
        'urgent': 'bg-red-100 text-red-800'
    };
    return classMap[priority] || 'bg-gray-100 text-gray-800';
}

function getAlertBorderColor(level) {
    const colorMap = {
        'INFO': 'border-blue-500',
        'WARNING': 'border-yellow-500',
        'ERROR': 'border-red-500',
        'CRITICAL': 'border-red-700'
    };
    return colorMap[level] || 'border-gray-500';
}

function getAlertIcon(type) {
    const iconMap = {
        'BATTERY_LOW': 'fa-battery-quarter',
        'CONNECTION_LOST': 'fa-wifi-slash',
        'POSITION_DRIFT': 'fa-location-arrow',
        'SYSTEM_INFO': 'fa-info-circle'
    };
    return iconMap[type] || 'fa-exclamation-triangle';
}

function getAlertIconColor(level) {
    const colorMap = {
        'INFO': 'text-blue-500',
        'WARNING': 'text-yellow-500',
        'ERROR': 'text-red-500',
        'CRITICAL': 'text-red-700'
    };
    return colorMap[level] || 'text-gray-500';
}

function getAlertLevelClass(level) {
    const classMap = {
        'INFO': 'bg-blue-100 text-blue-800',
        'WARNING': 'bg-yellow-100 text-yellow-800',
        'ERROR': 'bg-red-100 text-red-800',
        'CRITICAL': 'bg-red-100 text-red-800'
    };
    return classMap[level] || 'bg-gray-100 text-gray-800';
}

function formatTime(isoString) {
    const date = new Date(isoString);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    
    if (diffMins < 1) return '刚刚';
    if (diffMins < 60) return `${diffMins}分钟前`;
    if (diffHours < 24) return `${diffHours}小时前`;
    return date.toLocaleDateString();
}

// 交互功能
function viewDroneDetails(droneId) {
    showNotification(`查看无人机 ${droneId} 的详细信息`, 'info');
    // 这里可以打开详情模态框或跳转到详情页
}

function sendCommand(droneId) {
    showNotification(`向无人机 ${droneId} 发送控制指令`, 'info');
    // 这里可以打开命令发送界面
}

function viewTaskDetails(taskId) {
    showNotification(`查看任务 ${taskId} 的详细信息`, 'info');
    // 这里可以打开任务详情模态框
}

function startTask(taskId) {
    if (confirm('确定要启动这个任务吗？')) {
        showNotification(`任务 ${taskId} 已启动`, 'success');
        loadTasks(); // 重新加载任务列表
    }
}

function deleteTask(taskId) {
    if (confirm('确定要删除这个任务吗？此操作不可恢复。')) {
        showNotification(`任务 ${taskId} 已删除`, 'success');
        loadTasks(); // 重新加载任务列表
    }
}

async function acknowledgeAlert(alertId) {
    try {
        // 调用确认告警API
        const response = await fetch(`${MONITOR_BASE}/alerts`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ alert_id: alertId })
        });
        
        if (response.ok) {
            showNotification('告警已确认', 'success');
            loadAlerts(); // 重新加载告警列表
        } else {
            throw new Error('确认告警失败');
        }
    } catch (error) {
        console.error('确认告警失败:', error);
        showNotification('确认告警失败', 'error');
    }
}

function clearAllAlerts() {
    if (confirm('确定要确认所有告警吗？')) {
        showNotification('所有告警已确认', 'success');
        loadAlerts(); // 重新加载告警列表
    }
}

function logout() {
    if (confirm('确定要退出登录吗？')) {
        localStorage.removeItem('authToken');
        window.location.reload();
    }
}

// 通知功能
function showNotification(message, type = 'info') {
    const colors = {
        'success': 'bg-green-500',
        'error': 'bg-red-500',
        'warning': 'bg-yellow-500',
        'info': 'bg-blue-500'
    };
    
    const notification = document.createElement('div');
    notification.className = `fixed top-20 right-4 ${colors[type]} text-white px-6 py-3 rounded-lg shadow-lg z-50 transition-all duration-300 transform translate-x-full`;
    notification.textContent = message;
    
    document.body.appendChild(notification);
    
    // 显示动画
    setTimeout(() => {
        notification.classList.remove('translate-x-full');
    }, 100);
    
    // 3秒后隐藏
    setTimeout(() => {
        notification.classList.add('translate-x-full');
        setTimeout(() => {
            document.body.removeChild(notification);
        }, 300);
    }, 3000);
}

// 更新无人机选择选项
function updateDroneSelectOptions(availableDrones) {
    const select = document.getElementById('taskDrone');
    select.innerHTML = '<option value="">请选择无人机</option>' + 
        availableDrones.map(drone => 
            `<option value="${drone.id}">${drone.name} (${getStatusText(drone.status)})</option>`
        ).join('');
}

// 实时数据更新相关函数
function updateDroneRealTimeData(droneData) {
    // 更新实时数据显示
    const realTimeContainer = document.getElementById('realTimeData');
    const timestamp = new Date().toLocaleTimeString();
    
    const dataItem = document.createElement('div');
    dataItem.className = 'border-b border-gray-200 py-2';
    dataItem.innerHTML = `
        <div class="flex justify-between items-center">
            <span class="text-sm font-medium">${droneData.drone_id}</span>
            <span class="text-xs text-gray-500">${timestamp}</span>
        </div>
        <div class="text-sm text-gray-600">
            状态: ${getStatusText(droneData.status)} | 
            电量: ${droneData.battery}% | 
            位置: ${droneData.position.latitude.toFixed(4)}, ${droneData.position.longitude.toFixed(4)}
        </div>
    `;
    
    realTimeContainer.insertBefore(dataItem, realTimeContainer.firstChild);
    
    // 保持最多50条记录
    const items = realTimeContainer.children;
    if (items.length > 50) {
        realTimeContainer.removeChild(items[items.length - 1]);
    }
}

function addNewAlert(alertData) {
    // 新告警通知
    showNotification(`新告警: ${alertData.message}`, 'warning');
    
    // 重新加载告警列表
    loadAlerts();
}

function updateTaskProgress(taskData) {
    showNotification(`任务 ${taskData.task_id} 进度更新: ${taskData.progress}%`, 'info');
    loadTasks(); // 重新加载任务列表
}

function handleInitialData(data) {
    console.log('收到初始监控数据:', data);
    
    if (data.drones && data.drones.length > 0) {
        data.drones.forEach(drone => {
            updateDroneRealTimeData({
                drone_id: drone.drone_id,
                status: drone.status,
                battery: drone.battery,
                position: drone.position
            });
        });
    }
}

function updateMonitoringPanel() {
    // 更新地图显示
    const mapContainer = document.getElementById('droneMap');
    mapContainer.innerHTML = `
        <div class="w-full h-full bg-blue-50 rounded-lg flex items-center justify-center">
            <div class="text-center">
                <i class="fas fa-map-marked-alt text-4xl text-blue-500 mb-2"></i>
                <p class="text-blue-600">实时位置地图</p>
                <p class="text-sm text-gray-500 mt-2">显示所有无人机的实时位置</p>
            </div>
        </div>
    `;
}

// ======================= WebRTC 视频流功能 =======================

/**
 * 启动无人机视频流
 */
async function startDroneStream(droneId) {
    try {
        if (!droneWebRTC) {
            console.error('WebRTC客户端未初始化');
            showNotification('WebRTC客户端未就绪', 'error');
            return;
        }
        
        console.log(`启动无人机 ${droneId} 的视频流`);
        
        // 显示视频流模态框
        showVideoStreamModal(droneId);
        
        // 连接到无人机视频流
        await droneWebRTC.connectToDrone(droneId.toString());
        
        showNotification(`正在连接无人机 ${droneId} 的视频流...`, 'info');
        
    } catch (error) {
        console.error('启动视频流失败:', error);
        showNotification('启动视频流失败', 'error');
    }
}

/**
 * 显示视频流模态框
 */
function showVideoStreamModal(droneId) {
    const modal = document.getElementById('videoStreamModal');
    if (!modal) {
        createVideoStreamModal();
    }
    
    // 更新模态框标题
    const title = document.getElementById('videoStreamTitle');
    if (title) {
        title.textContent = `无人机 ${droneId} - 实时视频流`;
    }
    
    // 显示模态框
    document.getElementById('videoStreamModal').classList.remove('hidden');
}

/**
 * 创建视频流模态框
 */
function createVideoStreamModal() {
    const modalHtml = `
        <div id="videoStreamModal" class="fixed inset-0 z-50 hidden">
            <div class="fixed inset-0 bg-black bg-opacity-50"></div>
            <div class="fixed inset-0 flex items-center justify-center p-4">
                <div class="bg-white rounded-lg max-w-4xl w-full max-h-full overflow-auto">
                    <div class="flex justify-between items-center p-6 border-b">
                        <h2 id="videoStreamTitle" class="text-xl font-semibold">无人机视频流</h2>
                        <button onclick="closeVideoStreamModal()" class="text-gray-400 hover:text-gray-600">
                            <i class="fas fa-times text-xl"></i>
                        </button>
                    </div>
                    
                    <div class="p-6">
                        <!-- 连接状态 -->
                        <div class="mb-4">
                            <div id="connection-status" class="connection-status disconnected">
                                连接状态: 未连接
                            </div>
                        </div>
                        
                        <!-- 视频容器 -->
                        <div id="video-container" class="bg-black rounded-lg mb-4" style="min-height: 400px;">
                            <div class="w-full h-full flex items-center justify-center text-white">
                                <div class="text-center">
                                    <i class="fas fa-video text-4xl mb-2"></i>
                                    <p>等待视频流连接...</p>
                                </div>
                            </div>
                        </div>
                        
                        <!-- 控制按钮 -->
                        <div class="flex space-x-4">
                            <button onclick="disconnectDroneStream()" class="bg-red-500 hover:bg-red-600 text-white px-4 py-2 rounded">
                                <i class="fas fa-stop mr-2"></i>断开连接
                            </button>
                            <button onclick="toggleVideoDisplay()" class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded">
                                <i class="fas fa-eye mr-2"></i>切换显示
                            </button>
                            <select id="videoQualitySelect" onchange="changeVideoQuality(this.value)" class="border border-gray-300 rounded px-3 py-2">
                                <option value="high">高清</option>
                                <option value="medium" selected>标清</option>
                                <option value="low">低清</option>
                            </select>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `;
    
    document.body.insertAdjacentHTML('beforeend', modalHtml);
    
    // 添加样式
    const style = document.createElement('style');
    style.textContent = `
        .connection-status {
            padding: 8px 16px;
            border-radius: 4px;
            font-weight: medium;
            text-align: center;
        }
        .connection-status.connected {
            background-color: #10b981;
            color: white;
        }
        .connection-status.connecting {
            background-color: #f59e0b;
            color: white;
        }
        .connection-status.disconnected {
            background-color: #ef4444;
            color: white;
        }
    `;
    document.head.appendChild(style);
}

/**
 * 关闭视频流模态框
 */
function closeVideoStreamModal() {
    document.getElementById('videoStreamModal').classList.add('hidden');
    
    // 断开视频流连接
    if (droneWebRTC) {
        droneWebRTC.disconnect();
    }
}

/**
 * 断开无人机视频流
 */
function disconnectDroneStream() {
    if (droneWebRTC) {
        droneWebRTC.disconnect();
        showNotification('视频流已断开', 'info');
    }
}

/**
 * 切换视频显示
 */
function toggleVideoDisplay() {
    if (droneWebRTC) {
        droneWebRTC.toggleVideo();
    }
}

/**
 * 更改视频质量
 */
function changeVideoQuality(quality) {
    if (droneWebRTC) {
        droneWebRTC.setVideoQuality(quality);
        showNotification(`视频质量已调整为: ${quality}`, 'info');
    }
}

// 监听WebRTC状态变化
window.addEventListener('webrtc-status-change', (event) => {
    const { status, droneId } = event.detail;
    console.log(`WebRTC状态变化: ${status}, 无人机: ${droneId}`);
    
    // 根据状态显示不同的通知
    switch (status) {
        case 'connected':
            showNotification(`无人机 ${droneId} 视频流已连接`, 'success');
            break;
        case 'disconnected':
            showNotification(`无人机 ${droneId} 视频流已断开`, 'info');
            break;
        case 'failed':
            showNotification(`无人机 ${droneId} 视频流连接失败`, 'error');
            break;
    }
});
