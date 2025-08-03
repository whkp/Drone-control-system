package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Config 日志配置
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output string // stdout, stderr, file path
}

// Logger 包装的日志实例
type Logger struct {
	*logrus.Logger
}

// NewLogger 创建新的日志实例
func NewLogger(config Config) *Logger {
	log := logrus.New()

	// 设置日志级别
	switch config.Level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}

	// 设置日志格式
	if config.Format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置输出
	switch config.Output {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	default:
		if config.Output != "" {
			if file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
				log.SetOutput(file)
			} else {
				log.SetOutput(os.Stdout)
				log.WithError(err).Warn("Failed to open log file, using stdout")
			}
		} else {
			log.SetOutput(os.Stdout)
		}
	}

	return &Logger{Logger: log}
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithError 添加错误字段
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

// RequestLogger 请求日志中间件使用的字段
func (l *Logger) RequestLogger(method, path, clientIP string, statusCode int, latency string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"client_ip":   clientIP,
		"status_code": statusCode,
		"latency":     latency,
		"type":        "request",
	})
}

// TaskLogger 任务相关日志
func (l *Logger) TaskLogger(taskID uint, droneID uint, action string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"task_id":  taskID,
		"drone_id": droneID,
		"action":   action,
		"type":     "task",
	})
}

// DroneLogger 无人机相关日志
func (l *Logger) DroneLogger(droneID uint, status string, battery int) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"drone_id": droneID,
		"status":   status,
		"battery":  battery,
		"type":     "drone",
	})
}

// SecurityLogger 安全相关日志
func (l *Logger) SecurityLogger(userID uint, action string, resource string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"user_id":  userID,
		"action":   action,
		"resource": resource,
		"type":     "security",
	})
}

// AlertLogger 告警相关日志
func (l *Logger) AlertLogger(alertType string, level string, source string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"alert_type": alertType,
		"level":      level,
		"source":     source,
		"type":       "alert",
	})
}

// PerformanceLogger 性能相关日志
func (l *Logger) PerformanceLogger(operation string, duration string, success bool) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"operation": operation,
		"duration":  duration,
		"success":   success,
		"type":      "performance",
	})
}
