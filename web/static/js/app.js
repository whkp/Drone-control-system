// 全局变量
const API_BASE = 'http://localhost:8080/api/v1';
const MONITOR_BASE = 'http://localhost:50053/api/monitoring';
let authToken = localStorage.getItem('authToken') || '';
let ws = null; // 统一使用 ws 变量表示 WebSocket 实例
let metricsChart = null;
let droneWebRTC = null; // WebRTC客户端实例

// 页面加载完成后执行
document.addEventListener('DOMContentLoaded', async () => {
    // 首先检查认证状态（在登录页不抛错，直接返回）
    const authed = checkAuthStatus();
    if (!authed) return; // 未认证则不进行后续初始化

    // 为所有请求设置 Authorization 头
    const token = localStorage.getItem('authToken');
    if (token && window.axios) {
        axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
    }

    // 认证通过后，继续执行初始化
    console.log("认证通过，开始初始化应用...");
    updateConnectionStatus('connecting', '连接中...');

    try {
        // 并行执行所有初始化任务（WebSocket 连接为即发型，不阻塞初始化完成）
        await Promise.all([
            initializeUI(),
            initializeCharts(),
            loadDashboardData(),
            loadDrones(),
            loadTasks(),
            loadAlerts(),
            connectWebSocket()
        ]);

        // 初始化完成，此处不强制设为已连接，等待 WebSocket onopen 决定
        console.log("应用初始化完成，等待WebSocket建立连接...");

    } catch (error) {
        console.error('初始化过程中发生错误:', error);
        updateConnectionStatus('disconnected', `初始化失败: ${error.message}`);
    }
});


/**
 * 检查用户认证状态
 * 返回是否已认证；未认证时：
 *  - 如果不在登录页，则重定向到 /login
 *  - 如果在登录页，则不抛错，仅返回 false
 */
function checkAuthStatus() {
    const token = localStorage.getItem('authToken');
    const path = window.location.pathname || '';
    const onLoginPage = path === '/login' || path.startsWith('/login');

    if (!token) {
        console.log('未找到authToken');
        if (!onLoginPage) {
            console.log('重定向到登录页面');
            window.location.href = '/login';
        }
        return false; // 不抛错，交由调用方决定是否继续
    }
    console.log('用户已认证');
    return true;
}

/**
 * 初始化UI组件和事件监听器
 */
async function initializeUI() {
    console.log("初始化UI组件...");
    // 可以在这里添加其他的UI初始化逻辑，例如事件监听器
    const currentUser = localStorage.getItem('currentUser') || '用户';
    const loginMode = localStorage.getItem('loginMode');

    const userDisplay = document.getElementById('currentUserDisplay');
    if (userDisplay) {
        userDisplay.textContent = `${currentUser} (${loginMode === 'demo' ? '演示模式' : '测试模式'})`;
    }

    const logoutButton = document.getElementById('logoutButton');
    if (logoutButton) {
        logoutButton.addEventListener('click', () => {
            localStorage.removeItem('authToken');
            localStorage.removeItem('currentUser');
            localStorage.removeItem('loginMode');
            window.location.href = '/login';
        });
    }
    console.log("UI组件初始化完成。");
}


/**
 * 连接到WebSocket服务器
 */
function connectWebSocket() {
    console.log('开始连接WebSocket...');
    const wsUrl = 'ws://localhost:50053/ws/monitoring';
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log('WebSocket 连接已建立');
        updateConnectionStatus('connected', '已连接');
    };

    ws.onclose = (event) => {
        console.log('WebSocket 连接关闭:', event);
        updateConnectionStatus('disconnected', '连接已断开');
        // 尝试重连
        setTimeout(connectWebSocket, 5000);
    };

    ws.onerror = (error) => {
        console.error('WebSocket 错误:', error);
        updateConnectionStatus('disconnected', '连接错误');
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            // console.log('收到WebSocket消息:', data);
            handleWebSocketMessage(data);
        } catch (e) {
            console.error('解析WebSocket消息失败:', e);
        }
    };
}


/**
 * 更新连接状态显示
 * @param {string} status - 'connected', 'connecting', 'disconnected'
 * @param {string} text - 显示的文本
 */
function updateConnectionStatus(status, text) {
    const statusIndicator = document.getElementById('connectionStatus');
    const statusText = document.getElementById('connectionText');

    if (statusIndicator && statusText) {
        // 移除所有状态类
        statusIndicator.classList.remove('connected', 'connecting', 'disconnected', 'text-green-500', 'text-yellow-500', 'text-red-500');
        
        // 添加当前状态对应的类
        statusIndicator.classList.add(status);
        statusText.textContent = text;

        switch (status) {
            case 'connected':
                statusText.className = 'text-sm text-green-500';
                break;
            case 'connecting':
                statusText.className = 'text-sm text-yellow-500';
                break;
            case 'disconnected':
                statusText.className = 'text-sm text-red-500';
                break;
        }
    } else {
        console.warn('无法找到连接状态元素');
    }
}


/**
 * 处理从WebSocket收到的消息
 * @param {object} data - 解析后的JSON数据
 */
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

/**
 * 初始化所有图表
 */
function initializeCharts() {
    console.log("初始化图表...");
    const commonOptions = {
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
    };

    const ctxCpu = document.getElementById('cpuChart').getContext('2d');
    cpuChart = new Chart(ctxCpu, {
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
                }
            ]
        },
        options: commonOptions
    });
    
    const ctxMemory = document.getElementById('memoryChart').getContext('2d');
    memoryChart = new Chart(ctxMemory, {
        type: 'line',
        data: {
            labels: [],
            datasets: [
                {
                    label: '内存使用率 (%)',
                    data: [],
                    borderColor: 'rgb(16, 185, 129)',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    tension: 0.4
                }
            ]
        },
        options: commonOptions
    });
    
    const ctxDisk = document.getElementById('diskChart').getContext('2d');
    diskChart = new Chart(ctxDisk, {
        type: 'line',
        data: {
            labels: [],
            datasets: [
                {
                    label: '磁盘使用率 (%)',
                    data: [],
                    borderColor: 'rgb(245, 158, 11)',
                    backgroundColor: 'rgba(245, 158, 11, 0.1)',
                    tension: 0.4
                }
            ]
        },
        options: commonOptions
    });
    
    const ctxNetwork = document.getElementById('networkChart').getContext('2d');
    networkChart = new Chart(ctxNetwork, {
        type: 'line',
        data: {
            labels: [],
            datasets: [
                {
                    label: '网络流入 (KB/s)',
                    data: [],
                    borderColor: 'rgb(220, 38, 38)',
                    backgroundColor: 'rgba(220, 38, 38, 0.1)',
                    tension: 0.4
                },
                {
                    label: '网络流出 (KB/s)',
                    data: [],
                    borderColor: 'rgb(34, 197, 94)',
                    backgroundColor: 'rgba(34, 197, 94, 0.1)',
                    tension: 0.4
                }
            ]
        },
        options: commonOptions
    });
    console.log("图表初始化完成。");
}

/**
 * 更新图表数据
 * @param {string} chartName - 'cpu', 'memory', 'disk', 'network'
 * @param {object} data - 新的数据点 { label: string, value: number }
 */
function updateChart(chart, data) {
    if (!chart) return;
    
    chart.data.labels.push(data.label);
    chart.data.datasets[0].data.push(data.value);
    
    // 保持最多20个数据点
    if (chart.data.labels.length > 20) {
        chart.data.labels.shift();
        chart.data.datasets.forEach(dataset => dataset.data.shift());
    }
    
    chart.update('none');
}

/**
 * 从API加载仪表盘数据
 */
async function loadDashboardData() {
    try {
        console.log("加载仪表盘数据...");
        const response = await axios.get('/api/dashboard');
        const data = response.data;
        document.getElementById('onlineDrones').textContent = data.online_drones;
        document.getElementById('activeTasks').textContent = data.active_tasks;
        document.getElementById('systemStatus').textContent = data.system_status;
        document.getElementById('systemStatus').className = data.system_status === '正常' ? 'text-green-500 font-bold' : 'text-red-500 font-bold';
        document.getElementById('totalFlights').textContent = data.total_flights;
        document.getElementById('totalFlightTime').textContent = `${data.total_flight_time_hours.toFixed(2)} 小时`;
        console.log("仪表盘数据加载完成。");
    } catch (error) {
        console.error('加载仪表盘数据失败:', error);
        // 即使失败也要显示一些东西
        document.getElementById('systemStatus').textContent = '数据加载失败';
        document.getElementById('systemStatus').className = 'text-red-500 font-bold';
        throw error; // 抛出错误，让Promise.all知道
    }
}

/**
 * 从API加载无人机列表
 */
async function loadDrones() {
    try {
        console.log("加载无人机列表...");
        const response = await axios.get('/api/drones');
        const drones = response.data;
        const listElement = document.getElementById('droneList');
        listElement.innerHTML = ''; // 清空列表
        drones.forEach(drone => {
            const item = document.createElement('li');
            item.className = 'flex justify-between items-center p-2 hover:bg-gray-700 rounded';
            item.innerHTML = `
                <span><i class="fas fa-drone mr-2"></i>${drone.name}</span>
                <span class="text-xs ${drone.status === '在线' ? 'text-green-400' : 'text-gray-400'}">${drone.status}</span>
            `;
            listElement.appendChild(item);
        });
        console.log("无人机列表加载完成。");
    } catch (error) {
        console.error('加载无人机列表失败:', error);
        throw error;
    }
}

/**
 * 从API加载任务列表
 */
async function loadTasks() {
    try {
        console.log("加载任务列表...");
        const response = await axios.get('/api/tasks');
        const tasks = response.data;
        const listElement = document.getElementById('taskList');
        listElement.innerHTML = ''; // 清空列表
        tasks.forEach(task => {
            const item = document.createElement('li');
            item.className = 'flex justify-between items-center p-2 hover:bg-gray-700 rounded';
            item.innerHTML = `
                <span><i class="fas fa-tasks mr-2"></i>${task.name}</span>
                <span class="text-xs ${getTaskStatusClass(task.status)}">${task.status}</span>
            `;
            listElement.appendChild(item);
        });
        console.log("任务列表加载完成。");
    } catch (error) {
        console.error('加载任务列表失败:', error);
        throw error;
    }
}

/**
 * 从API加载警报信息
 */
async function loadAlerts() {
    try {
        console.log("加载警报信息...");
        const response = await axios.get('/api/alerts');
        const alerts = response.data;
        const listElement = document.getElementById('alertList');
        listElement.innerHTML = ''; // 清空列表
        alerts.forEach(alert => {
            const item = document.createElement('div');
            item.className = 'bg-red-800 p-2 rounded mb-2';
            item.innerHTML = `
                <p class="font-bold"><i class="fas fa-exclamation-triangle mr-2"></i>${alert.title}</p>
                <p class="text-sm">${alert.message}</p>
            `;
            listElement.appendChild(item);
        });
        console.log("警报信息加载完成。");
    } catch (error) {
        console.error('加载警报信息失败:', error);
        throw error;
    }
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

// 移除旧的、未被调用的initializeApp函数
/*
async function initializeApp() {
    console.log("Initializing application...");
    updateConnectionStatus('connecting', '连接中...');

    try {
        // Setup UI and connect WebSocket
        initializeUI();
        connectWebSocket();

        // Load initial data
        await Promise.all([
            loadDashboardData(),
            loadDrones(),
            loadTasks(),
            loadAlerts()
        ]);

        // Initialize charts after data loading (or in parallel if no dependency)
        initializeCharts();
        
        console.log("Application initialized successfully.");
        // The final status will be set by WebSocket 'onopen'
    } catch (error) {
        console.error('Failed to initialize application:', error);
        updateConnectionStatus('disconnected', '初始化失败');
    }
}
*/
