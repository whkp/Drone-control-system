package webrtc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"drone-control-system/pkg/logger"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// StreamServer WebRTC视频流服务器
type StreamServer struct {
	logger      *logger.Logger
	upgrader    websocket.Upgrader
	connections map[string]*DroneStreamConnection
	mu          sync.RWMutex
	api         *webrtc.API
}

// DroneStreamConnection 无人机流连接
type DroneStreamConnection struct {
	DroneID        string
	PeerConnection *webrtc.PeerConnection
	WebSocketConn  *websocket.Conn
	VideoTrack     *webrtc.TrackLocalStaticRTP
	AudioTrack     *webrtc.TrackLocalStaticRTP
	IsStreaming    bool
	LastSeen       time.Time
	mu             sync.Mutex
}

// StreamMessage WebSocket消息结构
type StreamMessage struct {
	Type    string          `json:"type"`
	DroneID string          `json:"drone_id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewStreamServer 创建新的流服务器
func NewStreamServer(logger *logger.Logger) *StreamServer {
	// 创建WebRTC API
	mediaEngine := &webrtc.MediaEngine{}

	// 支持VP8/VP9视频编解码器
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeVP8,
			ClockRate:    90000,
			RTCPFeedback: nil,
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		logger.WithError(err).Error("Failed to register VP8 codec")
	}

	// 支持H.264视频编解码器
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		},
		PayloadType: 102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		logger.WithError(err).Error("Failed to register H264 codec")
	}

	// 支持Opus音频编解码器
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		logger.WithError(err).Error("Failed to register Opus codec")
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	return &StreamServer{
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make(map[string]*DroneStreamConnection),
		api:         api,
	}
}

// HandleDroneStream 处理无人机视频流连接
func (s *StreamServer) HandleDroneStream(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}
	defer conn.Close()

	droneID := r.URL.Query().Get("drone_id")
	if droneID == "" {
		s.logger.Error("Missing drone_id parameter")
		return
	}

	s.logger.WithField("drone_id", droneID).Info("New drone stream connection")

	// 创建WebRTC连接
	peerConnection, err := s.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		s.logger.WithError(err).Error("Failed to create peer connection")
		return
	}
	defer peerConnection.Close()

	// 创建视频轨道
	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video",
		fmt.Sprintf("video-%s", droneID),
	)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create video track")
		return
	}

	// 创建音频轨道
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		fmt.Sprintf("audio-%s", droneID),
	)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create audio track")
		return
	}

	// 添加轨道到连接
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		s.logger.WithError(err).Error("Failed to add video track")
		return
	}

	if _, err = peerConnection.AddTrack(audioTrack); err != nil {
		s.logger.WithError(err).Error("Failed to add audio track")
		return
	}

	// 创建连接对象
	droneConn := &DroneStreamConnection{
		DroneID:        droneID,
		PeerConnection: peerConnection,
		WebSocketConn:  conn,
		VideoTrack:     videoTrack,
		AudioTrack:     audioTrack,
		IsStreaming:    false,
		LastSeen:       time.Now(),
	}

	// 注册连接
	s.mu.Lock()
	s.connections[droneID] = droneConn
	s.mu.Unlock()

	// 设置ICE连接状态回调
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		s.logger.WithField("drone_id", droneID).
			WithField("state", state.String()).
			Info("ICE connection state changed")

		if state == webrtc.ICEConnectionStateConnected {
			droneConn.mu.Lock()
			droneConn.IsStreaming = true
			droneConn.mu.Unlock()
		} else if state == webrtc.ICEConnectionStateDisconnected ||
			state == webrtc.ICEConnectionStateFailed {
			droneConn.mu.Lock()
			droneConn.IsStreaming = false
			droneConn.mu.Unlock()
		}
	})

	// 处理WebSocket消息
	for {
		var msg StreamMessage
		if err := conn.ReadJSON(&msg); err != nil {
			s.logger.WithError(err).WithField("drone_id", droneID).Error("Failed to read WebSocket message")
			break
		}

		if err := s.handleStreamMessage(droneConn, &msg); err != nil {
			s.logger.WithError(err).WithField("drone_id", droneID).Error("Failed to handle stream message")
		}
	}

	// 清理连接
	s.mu.Lock()
	delete(s.connections, droneID)
	s.mu.Unlock()

	s.logger.WithField("drone_id", droneID).Info("Drone stream connection closed")
}

// handleStreamMessage 处理流消息
func (s *StreamServer) handleStreamMessage(conn *DroneStreamConnection, msg *StreamMessage) error {
	switch msg.Type {
	case "offer":
		return s.handleOffer(conn, msg.Data)
	case "answer":
		return s.handleAnswer(conn, msg.Data)
	case "ice-candidate":
		return s.handleICECandidate(conn, msg.Data)
	case "video-frame":
		return s.handleVideoFrame(conn, msg.Data)
	case "audio-frame":
		return s.handleAudioFrame(conn, msg.Data)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleOffer 处理WebRTC Offer
func (s *StreamServer) handleOffer(conn *DroneStreamConnection, data json.RawMessage) error {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal(data, &offer); err != nil {
		return fmt.Errorf("failed to unmarshal offer: %w", err)
	}

	// 设置远程描述
	if err := conn.PeerConnection.SetRemoteDescription(offer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// 创建答案
	answer, err := conn.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// 设置本地描述
	if err := conn.PeerConnection.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	// 发送答案
	response := StreamMessage{
		Type: "answer",
		Data: mustMarshal(answer),
	}

	return conn.WebSocketConn.WriteJSON(response)
}

// handleAnswer 处理WebRTC Answer
func (s *StreamServer) handleAnswer(conn *DroneStreamConnection, data json.RawMessage) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal(data, &answer); err != nil {
		return fmt.Errorf("failed to unmarshal answer: %w", err)
	}

	return conn.PeerConnection.SetRemoteDescription(answer)
}

// handleICECandidate 处理ICE候选
func (s *StreamServer) handleICECandidate(conn *DroneStreamConnection, data json.RawMessage) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(data, &candidate); err != nil {
		return fmt.Errorf("failed to unmarshal ICE candidate: %w", err)
	}

	return conn.PeerConnection.AddICECandidate(candidate)
}

// handleVideoFrame 处理视频帧
func (s *StreamServer) handleVideoFrame(conn *DroneStreamConnection, data json.RawMessage) error {
	// 这里应该解码视频帧数据并写入视频轨道
	// 实际实现需要根据具体的视频编码格式来处理

	conn.mu.Lock()
	isStreaming := conn.IsStreaming
	conn.mu.Unlock()

	if !isStreaming {
		return nil // 连接未就绪，丢弃帧
	}

	// 写入视频轨道 (这里需要实际的RTP包数据)
	// conn.VideoTrack.WriteRTP(&rtp.Packet{...})

	return nil
}

// handleAudioFrame 处理音频帧
func (s *StreamServer) handleAudioFrame(conn *DroneStreamConnection, data json.RawMessage) error {
	// 类似视频帧处理
	conn.mu.Lock()
	isStreaming := conn.IsStreaming
	conn.mu.Unlock()

	if !isStreaming {
		return nil
	}

	// 写入音频轨道
	// conn.AudioTrack.WriteRTP(&rtp.Packet{...})

	return nil
}

// GetActiveStreams 获取活跃的流连接
func (s *StreamServer) GetActiveStreams() map[string]*DroneStreamConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	active := make(map[string]*DroneStreamConnection)
	for id, conn := range s.connections {
		conn.mu.Lock()
		if conn.IsStreaming {
			active[id] = conn
		}
		conn.mu.Unlock()
	}

	return active
}

// CloseConnection 关闭指定无人机的连接
func (s *StreamServer) CloseConnection(droneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, exists := s.connections[droneID]; exists {
		conn.PeerConnection.Close()
		conn.WebSocketConn.Close()
		delete(s.connections, droneID)
		return nil
	}

	return fmt.Errorf("drone connection not found: %s", droneID)
}

// 辅助函数
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("Failed to marshal: %v", err)
		return json.RawMessage("{}")
	}
	return data
}
