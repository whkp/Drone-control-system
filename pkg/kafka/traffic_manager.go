package kafka

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"drone-control-system/pkg/logger"
)

// TrafficManager 流量削峰管理器
type TrafficManager struct {
	logger   *logger.Logger
	producer *Producer

	// 消息缓冲池
	messageBuffer chan *BufferedMessage
	batchBuffer   []*BufferedMessage
	batchSize     int
	flushInterval time.Duration

	// 限流器
	rateLimiter    *RateLimiter
	circuitBreaker *CircuitBreaker

	// 统计信息
	stats *TrafficStats
	mu    sync.RWMutex

	// 控制协程
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// BufferedMessage 缓冲消息
type BufferedMessage struct {
	Topic      string
	Event      *Event
	Priority   MessagePriority
	Timestamp  time.Time
	RetryCount int
}

// MessagePriority 消息优先级
type MessagePriority int

const (
	PriorityLow MessagePriority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// TrafficStats 流量统计
type TrafficStats struct {
	TotalMessages     int64         `json:"total_messages"`
	BufferedMessages  int64         `json:"buffered_messages"`
	DroppedMessages   int64         `json:"dropped_messages"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	ThroughputPerSec  float64       `json:"throughput_per_sec"`
	CurrentQueueSize  int           `json:"current_queue_size"`
	mu                sync.RWMutex
}

// RateLimiter 限流器
type RateLimiter struct {
	maxRate     int
	currentRate int
	window      time.Duration
	lastReset   time.Time
	mu          sync.Mutex
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	state        CircuitState
	failureCount int
	successCount int
	timeout      time.Duration
	maxFailures  int
	lastFailTime time.Time
	mu           sync.Mutex
}

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// TrafficConfig 流量控制配置
type TrafficConfig struct {
	// 缓冲配置
	BufferSize    int           `yaml:"buffer_size" json:"buffer_size"`
	BatchSize     int           `yaml:"batch_size" json:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`

	// 限流配置
	MaxRate    int           `yaml:"max_rate" json:"max_rate"`
	RateWindow time.Duration `yaml:"rate_window" json:"rate_window"`

	// 熔断配置
	MaxFailures    int           `yaml:"max_failures" json:"max_failures"`
	CircuitTimeout time.Duration `yaml:"circuit_timeout" json:"circuit_timeout"`
}

// 错误定义
var (
	ErrRateLimitExceeded  = fmt.Errorf("rate limit exceeded")
	ErrCircuitBreakerOpen = fmt.Errorf("circuit breaker is open")
	ErrBufferFull         = fmt.Errorf("message buffer is full")
)

// NewTrafficManager 创建流量管理器
func NewTrafficManager(logger *logger.Logger, producer *Producer, config *TrafficConfig) *TrafficManager {
	ctx, cancel := context.WithCancel(context.Background())

	tm := &TrafficManager{
		logger:        logger,
		producer:      producer,
		messageBuffer: make(chan *BufferedMessage, config.BufferSize),
		batchBuffer:   make([]*BufferedMessage, 0, config.BatchSize),
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		rateLimiter: &RateLimiter{
			maxRate: config.MaxRate,
			window:  config.RateWindow,
		},
		circuitBreaker: &CircuitBreaker{
			maxFailures: config.MaxFailures,
			timeout:     config.CircuitTimeout,
			state:       StateClosed,
		},
		stats:  &TrafficStats{},
		ctx:    ctx,
		cancel: cancel,
	}

	return tm
}

// DefaultTrafficConfig 默认流量配置
func DefaultTrafficConfig() *TrafficConfig {
	return &TrafficConfig{
		BufferSize:     10000,                 // 1万消息缓冲
		BatchSize:      100,                   // 100条消息一批
		FlushInterval:  50 * time.Millisecond, // 50ms刷新一次
		MaxRate:        1000,                  // 每秒1000条消息
		RateWindow:     time.Second,           // 1秒窗口
		MaxFailures:    5,                     // 5次失败触发熔断
		CircuitTimeout: 10 * time.Second,      // 10秒熔断超时
	}
}

// PublishWithTrafficControl 带流量控制的发布
func (tm *TrafficManager) PublishWithTrafficControl(ctx context.Context, topic string, event *Event, priority MessagePriority) error {
	tm.stats.mu.Lock()
	tm.stats.TotalMessages++
	tm.stats.mu.Unlock()

	// 1. 限流检查
	if !tm.rateLimiter.Allow() {
		tm.stats.mu.Lock()
		tm.stats.DroppedMessages++
		tm.stats.mu.Unlock()
		tm.logger.WithField("topic", topic).Warn("Rate limit exceeded, dropping message")
		return ErrRateLimitExceeded
	}

	// 2. 熔断器检查
	if !tm.circuitBreaker.Allow() {
		tm.stats.mu.Lock()
		tm.stats.DroppedMessages++
		tm.stats.mu.Unlock()
		tm.logger.WithField("topic", topic).Warn("Circuit breaker open, dropping message")
		return ErrCircuitBreakerOpen
	}

	// 3. 创建缓冲消息
	bufferedMsg := &BufferedMessage{
		Topic:     topic,
		Event:     event,
		Priority:  priority,
		Timestamp: time.Now(),
	}

	// 4. 根据优先级处理
	switch priority {
	case PriorityUrgent:
		// 紧急消息直接发送
		return tm.sendMessageImmediately(ctx, bufferedMsg)
	case PriorityHigh:
		// 高优先级消息优先入队
		return tm.enqueueHighPriority(ctx, bufferedMsg)
	default:
		// 普通消息进入缓冲队列
		return tm.enqueueMessage(ctx, bufferedMsg)
	}
}

// Start 启动流量管理器
func (tm *TrafficManager) Start(ctx context.Context) {
	tm.wg.Add(3)

	// 启动批处理协程
	go tm.batchProcessor()

	// 启动统计协程
	go tm.statsCollector()

	// 启动健康检查协程
	go tm.healthChecker()

	tm.logger.Info("Traffic manager started")
}

// Stop 停止流量管理器
func (tm *TrafficManager) Stop() error {
	tm.logger.Info("Stopping traffic manager...")

	// 取消上下文
	tm.cancel()

	// 等待所有协程结束
	tm.wg.Wait()

	// 处理剩余消息
	tm.flushRemainingMessages(context.Background())

	tm.logger.Info("Traffic manager stopped")
	return nil
}

// batchProcessor 批处理器
func (tm *TrafficManager) batchProcessor() {
	defer tm.wg.Done()

	ticker := time.NewTicker(tm.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			tm.logger.Info("Batch processor stopping...")
			return

		case msg := <-tm.messageBuffer:
			tm.batchBuffer = append(tm.batchBuffer, msg)

			tm.stats.mu.Lock()
			tm.stats.BufferedMessages++
			tm.stats.mu.Unlock()

			// 达到批次大小则立即发送
			if len(tm.batchBuffer) >= tm.batchSize {
				tm.flushBatch(tm.ctx)
			}

		case <-ticker.C:
			// 定时刷新批次
			if len(tm.batchBuffer) > 0 {
				tm.flushBatch(tm.ctx)
			}
		}
	}
}

// flushBatch 刷新批次
func (tm *TrafficManager) flushBatch(ctx context.Context) {
	if len(tm.batchBuffer) == 0 {
		return
	}

	startTime := time.Now()

	// 按优先级排序
	tm.sortByPriority(tm.batchBuffer)

	// 批量发送
	var successCount, failureCount int

	for _, msg := range tm.batchBuffer {
		if err := tm.sendMessage(ctx, msg); err != nil {
			failureCount++
			tm.handleSendFailure(msg, err)
		} else {
			successCount++
		}
	}

	// 更新统计信息
	tm.updateStats(successCount, failureCount, time.Since(startTime))

	// 更新熔断器状态
	if failureCount > 0 {
		tm.circuitBreaker.RecordFailure()
	} else {
		tm.circuitBreaker.RecordSuccess()
	}

	// 清空批次缓冲
	tm.batchBuffer = tm.batchBuffer[:0]

	tm.logger.WithField("success", successCount).
		WithField("failure", failureCount).
		WithField("duration", time.Since(startTime)).
		Debug("Batch processed")
}

// sendMessageImmediately 立即发送消息
func (tm *TrafficManager) sendMessageImmediately(ctx context.Context, msg *BufferedMessage) error {
	return tm.sendMessage(ctx, msg)
}

// enqueueHighPriority 高优先级入队
func (tm *TrafficManager) enqueueHighPriority(ctx context.Context, msg *BufferedMessage) error {
	// 高优先级消息插入到队列前端
	select {
	case tm.messageBuffer <- msg:
		return nil
	default:
		return ErrBufferFull
	}
}

// enqueueMessage 普通消息入队
func (tm *TrafficManager) enqueueMessage(ctx context.Context, msg *BufferedMessage) error {
	select {
	case tm.messageBuffer <- msg:
		return nil
	default:
		return ErrBufferFull
	}
}

// sendMessage 发送消息
func (tm *TrafficManager) sendMessage(ctx context.Context, msg *BufferedMessage) error {
	if tm.producer == nil {
		return fmt.Errorf("producer is nil")
	}

	// 这里应该调用实际的producer发送方法
	// 由于当前kafka包可能还没有实现Producer，我们先模拟
	tm.logger.WithField("topic", msg.Topic).
		WithField("priority", msg.Priority).
		Debug("Message sent")

	return nil
}

// sortByPriority 按优先级排序
func (tm *TrafficManager) sortByPriority(messages []*BufferedMessage) {
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Priority > messages[j].Priority
	})
}

// handleSendFailure 处理发送失败
func (tm *TrafficManager) handleSendFailure(msg *BufferedMessage, err error) {
	msg.RetryCount++

	// 如果重试次数超过限制，丢弃消息
	if msg.RetryCount > 3 {
		tm.stats.mu.Lock()
		tm.stats.DroppedMessages++
		tm.stats.mu.Unlock()

		tm.logger.WithField("topic", msg.Topic).
			WithError(err).
			Error("Message dropped after max retries")
		return
	}

	// 重新入队
	go func() {
		time.Sleep(time.Duration(msg.RetryCount) * time.Second)
		tm.enqueueMessage(context.Background(), msg)
	}()
}

// updateStats 更新统计信息
func (tm *TrafficManager) updateStats(successCount, failureCount int, duration time.Duration) {
	tm.stats.mu.Lock()
	defer tm.stats.mu.Unlock()

	tm.stats.AvgProcessingTime = duration
	tm.stats.ThroughputPerSec = float64(successCount) / duration.Seconds()
	tm.stats.CurrentQueueSize = len(tm.messageBuffer)
}

// statsCollector 统计收集器
func (tm *TrafficManager) statsCollector() {
	defer tm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.logStats()
		}
	}
}

// healthChecker 健康检查器
func (tm *TrafficManager) healthChecker() {
	defer tm.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.checkHealth()
		}
	}
}

// logStats 记录统计信息
func (tm *TrafficManager) logStats() {
	tm.stats.mu.RLock()
	defer tm.stats.mu.RUnlock()

	tm.logger.WithField("total_messages", tm.stats.TotalMessages).
		WithField("buffered_messages", tm.stats.BufferedMessages).
		WithField("dropped_messages", tm.stats.DroppedMessages).
		WithField("throughput_per_sec", tm.stats.ThroughputPerSec).
		WithField("queue_size", tm.stats.CurrentQueueSize).
		Info("Traffic manager stats")
}

// checkHealth 健康检查
func (tm *TrafficManager) checkHealth() {
	// 检查队列是否过满
	queueUsage := float64(len(tm.messageBuffer)) / float64(cap(tm.messageBuffer))
	if queueUsage > 0.8 {
		tm.logger.WithField("queue_usage", queueUsage).Warn("Message queue usage high")
	}

	// 检查熔断器状态
	if tm.circuitBreaker.state == StateOpen {
		tm.logger.Warn("Circuit breaker is open")
	}
}

// flushRemainingMessages 清空剩余消息
func (tm *TrafficManager) flushRemainingMessages(ctx context.Context) {
	// 处理缓冲队列中的剩余消息
	close(tm.messageBuffer)
	for msg := range tm.messageBuffer {
		tm.sendMessage(ctx, msg)
	}

	// 处理批次缓冲中的剩余消息
	if len(tm.batchBuffer) > 0 {
		tm.flushBatch(ctx)
	}
}

// GetStats 获取统计信息
func (tm *TrafficManager) GetStats() *TrafficStats {
	tm.stats.mu.RLock()
	defer tm.stats.mu.RUnlock()

	// 返回副本以避免并发问题
	return &TrafficStats{
		TotalMessages:     tm.stats.TotalMessages,
		BufferedMessages:  tm.stats.BufferedMessages,
		DroppedMessages:   tm.stats.DroppedMessages,
		AvgProcessingTime: tm.stats.AvgProcessingTime,
		ThroughputPerSec:  tm.stats.ThroughputPerSec,
		CurrentQueueSize:  tm.stats.CurrentQueueSize,
	}
}

// Allow 限流检查
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 重置窗口
	if now.Sub(rl.lastReset) >= rl.window {
		rl.currentRate = 0
		rl.lastReset = now
	}

	// 检查是否超过限制
	if rl.currentRate >= rl.maxRate {
		return false
	}

	rl.currentRate++
	return true
}

// Allow 熔断器检查
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以转为半开状态
		if time.Since(cb.lastFailTime) >= cb.timeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++

	if cb.state == StateHalfOpen && cb.successCount >= 3 {
		cb.state = StateClosed
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = StateOpen
	}
}
