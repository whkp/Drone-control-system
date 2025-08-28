package services

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"

	"github.com/gorilla/websocket"
)

// WebSocketMessage WebSocket消息结构
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// WebSocketClient WebSocket客户端
type WebSocketClient struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan WebSocketMessage
	UserID *uint // 可选，用于权限控制
}

// WebSocketService WebSocket服务接口
type WebSocketService interface {
	// 连接管理
	RegisterClient(client *WebSocketClient)
	UnregisterClient(clientID string)
	HandleWebSocketConnection(w http.ResponseWriter, r *http.Request, userID *uint) error

	// 消息广播
	BroadcastToAll(message WebSocketMessage)
	BroadcastToUser(userID uint, message WebSocketMessage)
	SendToClient(clientID string, message WebSocketMessage)

	// Kafka事件处理
	HandleKafkaEvent(event *kafka.Event)

	// 服务管理
	Start() error
	Stop() error
}

// WebSocketServiceImpl WebSocket服务实现
type WebSocketServiceImpl struct {
	clients    map[string]*WebSocketClient
	register   chan *WebSocketClient
	unregister chan string
	broadcast  chan WebSocketMessage
	logger     *logger.Logger
	upgrader   websocket.Upgrader
	mu         sync.RWMutex
	running    bool
}

// NewWebSocketService 创建WebSocket服务
func NewWebSocketService(logger *logger.Logger) WebSocketService {
	return &WebSocketServiceImpl{
		clients:    make(map[string]*WebSocketClient),
		register:   make(chan *WebSocketClient),
		unregister: make(chan string),
		broadcast:  make(chan WebSocketMessage),
		logger:     logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// 生产环境应该检查Origin
				return true
			},
		},
		running: false,
	}
}

// Start 启动WebSocket服务
func (ws *WebSocketServiceImpl) Start() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.running {
		return nil
	}

	ws.running = true
	go ws.run()

	ws.logger.Info("WebSocket service started", nil)
	return nil
}

// Stop 停止WebSocket服务
func (ws *WebSocketServiceImpl) Stop() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.running {
		return nil
	}

	ws.running = false

	// 关闭所有客户端连接
	for _, client := range ws.clients {
		close(client.Send)
		client.Conn.Close()
	}

	ws.logger.Info("WebSocket service stopped", nil)
	return nil
}

// run 运行主循环
func (ws *WebSocketServiceImpl) run() {
	for ws.running {
		select {
		case client := <-ws.register:
			ws.mu.Lock()
			ws.clients[client.ID] = client
			ws.mu.Unlock()

			ws.logger.Info("WebSocket client registered", map[string]interface{}{
				"client_id": client.ID,
				"user_id":   client.UserID,
			})

			// 发送欢迎消息
			ws.SendToClient(client.ID, WebSocketMessage{
				Type:      "connection",
				Data:      map[string]string{"status": "connected"},
				Timestamp: time.Now(),
			})

		case clientID := <-ws.unregister:
			ws.mu.Lock()
			if client, exists := ws.clients[clientID]; exists {
				delete(ws.clients, clientID)
				close(client.Send)
				client.Conn.Close()
			}
			ws.mu.Unlock()

			ws.logger.Info("WebSocket client unregistered", map[string]interface{}{
				"client_id": clientID,
			})

		case message := <-ws.broadcast:
			ws.mu.RLock()
			for clientID, client := range ws.clients {
				select {
				case client.Send <- message:
					// 成功发送
				default:
					// 发送失败，客户端可能已断开
					ws.logger.Warning("Failed to send message to client", map[string]interface{}{
						"client_id": clientID,
					})
					delete(ws.clients, clientID)
					close(client.Send)
					client.Conn.Close()
				}
			}
			ws.mu.RUnlock()
		}
	}
}

// RegisterClient 注册WebSocket客户端
func (ws *WebSocketServiceImpl) RegisterClient(client *WebSocketClient) {
	ws.register <- client
}

// UnregisterClient 注销WebSocket客户端
func (ws *WebSocketServiceImpl) UnregisterClient(clientID string) {
	ws.unregister <- clientID
}

// BroadcastToAll 广播消息给所有客户端
func (ws *WebSocketServiceImpl) BroadcastToAll(message WebSocketMessage) {
	ws.broadcast <- message
}

// BroadcastToUser 广播消息给指定用户
func (ws *WebSocketServiceImpl) BroadcastToUser(userID uint, message WebSocketMessage) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	for _, client := range ws.clients {
		if client.UserID != nil && *client.UserID == userID {
			select {
			case client.Send <- message:
				// 成功发送
			default:
				// 发送失败
				ws.logger.Warning("Failed to send message to user", map[string]interface{}{
					"user_id":   userID,
					"client_id": client.ID,
				})
			}
		}
	}
}

// SendToClient 发送消息给指定客户端
func (ws *WebSocketServiceImpl) SendToClient(clientID string, message WebSocketMessage) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if client, exists := ws.clients[clientID]; exists {
		select {
		case client.Send <- message:
			// 成功发送
		default:
			// 发送失败
			ws.logger.Warning("Failed to send message to client", map[string]interface{}{
				"client_id": clientID,
			})
		}
	}
}

// HandleKafkaEvent 处理Kafka事件，转换为WebSocket消息
func (ws *WebSocketServiceImpl) HandleKafkaEvent(event *kafka.Event) {
	message := WebSocketMessage{
		Type:      string(event.Type),
		Data:      event.Data,
		Timestamp: event.Timestamp,
	}

	// 根据事件类型决定广播策略
	switch event.Type {
	case kafka.DroneLocationUpdatedEvent, kafka.DroneStatusChangedEvent, kafka.DroneBatteryLowEvent:
		// 无人机相关事件广播给所有连接的客户端
		ws.BroadcastToAll(message)

	case kafka.AlertCreatedEvent:
		// 告警事件广播给所有客户端
		ws.BroadcastToAll(message)

	case kafka.TaskProgressEvent, kafka.TaskCompletedEvent, kafka.TaskFailedEvent:
		// 任务相关事件广播给所有客户端
		ws.BroadcastToAll(message)

	default:
		// 其他事件只记录日志
		ws.logger.Debug("Received kafka event", map[string]interface{}{
			"event_type": event.Type,
			"data":       event.Data,
		})
	}
}

// HandleWebSocketConnection 处理WebSocket连接升级
func (ws *WebSocketServiceImpl) HandleWebSocketConnection(w http.ResponseWriter, r *http.Request, userID *uint) error {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	// 生成客户端ID
	clientID := generateClientID()

	client := &WebSocketClient{
		ID:     clientID,
		Conn:   conn,
		Send:   make(chan WebSocketMessage, 256),
		UserID: userID,
	}

	// 注册客户端
	ws.RegisterClient(client)

	// 启动客户端消息处理协程
	go ws.handleClientMessages(client)
	go ws.handleClientWrites(client)

	return nil
}

// handleClientMessages 处理客户端发送的消息
func (ws *WebSocketServiceImpl) handleClientMessages(client *WebSocketClient) {
	defer func() {
		ws.UnregisterClient(client.ID)
	}()

	client.Conn.SetReadLimit(512)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.Error("WebSocket error", map[string]interface{}{
					"client_id": client.ID,
					"error":     err.Error(),
				})
			}
			break
		}

		// 处理客户端消息（如心跳包等）
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
				// 响应心跳包
				client.Send <- WebSocketMessage{
					Type:      "pong",
					Data:      map[string]string{"status": "ok"},
					Timestamp: time.Now(),
				}
			}
		}
	}
}

// handleClientWrites 处理向客户端写入消息
func (ws *WebSocketServiceImpl) handleClientWrites(client *WebSocketClient) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteJSON(message); err != nil {
				ws.logger.Error("Failed to write WebSocket message", map[string]interface{}{
					"client_id": client.ID,
					"error":     err.Error(),
				})
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// generateClientID 生成客户端ID
func generateClientID() string {
	return "client_" + time.Now().Format("20060102150405") + "_" + randomString(6)
}

// randomString 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
