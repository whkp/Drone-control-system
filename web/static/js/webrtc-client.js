/**
 * WebRTC客户端 - 处理无人机视频流
 */
class DroneWebRTCClient {
    constructor() {
        this.peerConnection = null;
        this.websocket = null;
        this.localVideo = null;
        this.remoteVideo = null;
        this.currentDroneId = null;
        this.isConnected = false;
        
        // WebRTC配置
        this.rtcConfiguration = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' }
            ]
        };
        
        this.init();
    }
    
    /**
     * 初始化WebRTC客户端
     */
    init() {
        console.log('初始化 WebRTC 客户端');
        this.setupVideoElements();
        this.connectWebSocket();
    }
    
    /**
     * 设置视频元素
     */
    setupVideoElements() {
        // 创建远程视频元素
        if (!document.getElementById('remoteVideo')) {
            const videoContainer = document.getElementById('video-container') || document.body;
            this.remoteVideo = document.createElement('video');
            this.remoteVideo.id = 'remoteVideo';
            this.remoteVideo.autoplay = true;
            this.remoteVideo.playsInline = true;
            this.remoteVideo.controls = true;
            this.remoteVideo.style.cssText = `
                width: 100%;
                max-width: 640px;
                height: auto;
                border: 2px solid #3b82f6;
                border-radius: 8px;
                background: #000;
            `;
            videoContainer.appendChild(this.remoteVideo);
        } else {
            this.remoteVideo = document.getElementById('remoteVideo');
        }
    }
    
    /**
     * 连接WebSocket信令服务器
     */
    connectWebSocket() {
        const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${wsProtocol}//${window.location.host}/ws/webrtc`;
        
        this.websocket = new WebSocket(wsUrl);
        
        this.websocket.onopen = () => {
            console.log('WebSocket连接已建立');
            this.isConnected = true;
        };
        
        this.websocket.onmessage = async (event) => {
            try {
                const message = JSON.parse(event.data);
                await this.handleSignalingMessage(message);
            } catch (error) {
                console.error('处理信令消息失败:', error);
            }
        };
        
        this.websocket.onclose = () => {
            console.log('WebSocket连接已关闭');
            this.isConnected = false;
            // 自动重连
            setTimeout(() => this.connectWebSocket(), 3000);
        };
        
        this.websocket.onerror = (error) => {
            console.error('WebSocket错误:', error);
        };
    }
    
    /**
     * 连接到指定无人机的视频流
     */
    async connectToDrone(droneId) {
        if (!this.isConnected) {
            console.error('WebSocket未连接');
            return;
        }
        
        try {
            this.currentDroneId = droneId;
            console.log(`连接到无人机 ${droneId} 的视频流`);
            
            // 创建新的PeerConnection
            await this.createPeerConnection();
            
            // 发送连接请求
            this.sendSignalingMessage({
                type: 'connect',
                droneId: droneId
            });
            
        } catch (error) {
            console.error('连接无人机失败:', error);
        }
    }
    
    /**
     * 创建PeerConnection
     */
    async createPeerConnection() {
        try {
            // 关闭现有连接
            if (this.peerConnection) {
                this.peerConnection.close();
            }
            
            this.peerConnection = new RTCPeerConnection(this.rtcConfiguration);
            
            // 监听ICE候选
            this.peerConnection.onicecandidate = (event) => {
                if (event.candidate) {
                    this.sendSignalingMessage({
                        type: 'ice-candidate',
                        candidate: event.candidate,
                        droneId: this.currentDroneId
                    });
                }
            };
            
            // 监听远程流
            this.peerConnection.ontrack = (event) => {
                console.log('接收到远程流');
                if (this.remoteVideo && event.streams && event.streams[0]) {
                    this.remoteVideo.srcObject = event.streams[0];
                }
            };
            
            // 监听连接状态变化
            this.peerConnection.onconnectionstatechange = () => {
                console.log('连接状态:', this.peerConnection.connectionState);
                this.updateConnectionStatus(this.peerConnection.connectionState);
            };
            
            // 监听ICE连接状态变化
            this.peerConnection.oniceconnectionstatechange = () => {
                console.log('ICE连接状态:', this.peerConnection.iceConnectionState);
            };
            
        } catch (error) {
            console.error('创建PeerConnection失败:', error);
            throw error;
        }
    }
    
    /**
     * 处理信令消息
     */
    async handleSignalingMessage(message) {
        try {
            switch (message.type) {
                case 'offer':
                    await this.handleOffer(message);
                    break;
                    
                case 'answer':
                    await this.handleAnswer(message);
                    break;
                    
                case 'ice-candidate':
                    await this.handleIceCandidate(message);
                    break;
                    
                case 'error':
                    console.error('服务器错误:', message.error);
                    break;
                    
                case 'connected':
                    console.log('成功连接到无人机:', message.droneId);
                    break;
                    
                case 'disconnected':
                    console.log('无人机连接断开:', message.droneId);
                    this.handleDisconnection();
                    break;
                    
                default:
                    console.warn('未知消息类型:', message.type);
            }
        } catch (error) {
            console.error('处理信令消息失败:', error);
        }
    }
    
    /**
     * 处理Offer
     */
    async handleOffer(message) {
        if (!this.peerConnection) {
            await this.createPeerConnection();
        }
        
        await this.peerConnection.setRemoteDescription(message.offer);
        
        // 创建Answer
        const answer = await this.peerConnection.createAnswer();
        await this.peerConnection.setLocalDescription(answer);
        
        // 发送Answer
        this.sendSignalingMessage({
            type: 'answer',
            answer: answer,
            droneId: this.currentDroneId
        });
    }
    
    /**
     * 处理Answer
     */
    async handleAnswer(message) {
        if (this.peerConnection) {
            await this.peerConnection.setRemoteDescription(message.answer);
        }
    }
    
    /**
     * 处理ICE候选
     */
    async handleIceCandidate(message) {
        if (this.peerConnection && message.candidate) {
            await this.peerConnection.addIceCandidate(message.candidate);
        }
    }
    
    /**
     * 发送信令消息
     */
    sendSignalingMessage(message) {
        if (this.websocket && this.websocket.readyState === WebSocket.OPEN) {
            this.websocket.send(JSON.stringify(message));
        } else {
            console.error('WebSocket未连接，无法发送消息');
        }
    }
    
    /**
     * 断开连接
     */
    disconnect() {
        console.log('断开WebRTC连接');
        
        if (this.peerConnection) {
            this.peerConnection.close();
            this.peerConnection = null;
        }
        
        if (this.remoteVideo) {
            this.remoteVideo.srcObject = null;
        }
        
        this.currentDroneId = null;
        
        if (this.websocket) {
            this.sendSignalingMessage({
                type: 'disconnect',
                droneId: this.currentDroneId
            });
        }
    }
    
    /**
     * 处理断开连接
     */
    handleDisconnection() {
        if (this.remoteVideo) {
            this.remoteVideo.srcObject = null;
        }
        
        this.currentDroneId = null;
        this.updateConnectionStatus('disconnected');
    }
    
    /**
     * 更新连接状态显示
     */
    updateConnectionStatus(status) {
        const statusElement = document.getElementById('connection-status');
        if (statusElement) {
            statusElement.textContent = `连接状态: ${status}`;
            statusElement.className = `connection-status ${status}`;
        }
        
        // 触发自定义事件
        window.dispatchEvent(new CustomEvent('webrtc-status-change', {
            detail: { status, droneId: this.currentDroneId }
        }));
    }
    
    /**
     * 获取当前连接状态
     */
    getConnectionState() {
        return {
            isConnected: this.isConnected,
            droneId: this.currentDroneId,
            peerConnectionState: this.peerConnection ? this.peerConnection.connectionState : 'closed',
            iceConnectionState: this.peerConnection ? this.peerConnection.iceConnectionState : 'closed'
        };
    }
    
    /**
     * 切换视频显示/隐藏
     */
    toggleVideo() {
        if (this.remoteVideo) {
            this.remoteVideo.style.display = this.remoteVideo.style.display === 'none' ? 'block' : 'none';
        }
    }
    
    /**
     * 设置视频质量
     */
    setVideoQuality(quality) {
        // 可以向服务器发送质量控制消息
        this.sendSignalingMessage({
            type: 'video-quality',
            quality: quality,
            droneId: this.currentDroneId
        });
    }
}

// 全局WebRTC客户端实例
let droneWebRTC = null;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    droneWebRTC = new DroneWebRTCClient();
    
    // 添加到全局作用域供其他脚本使用
    window.droneWebRTC = droneWebRTC;
    
    console.log('无人机WebRTC客户端已初始化');
});

// 导出供模块化使用
if (typeof module !== 'undefined' && module.exports) {
    module.exports = DroneWebRTCClient;
}
