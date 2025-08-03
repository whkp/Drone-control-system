package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"drone-control-system/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func main() {
	// 加载配置
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	appLogger := logger.NewLogger(logger.Config{
		Level:  config.GetString("logging.level"),
		Format: config.GetString("logging.format"),
		Output: config.GetString("logging.output"),
	})

	// 创建 Gin 引擎
	if config.GetString("logging.level") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// 添加中间件
	r.Use(LoggerMiddleware(appLogger))
	r.Use(CORSMiddleware())
	r.Use(RecoveryMiddleware(appLogger))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"service":   "api-gateway",
			"version":   "1.0.0",
		})
	})

	// 根路径
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Drone Control System API Gateway",
			"version": "1.0.0",
			"docs":    "/api/v1/docs",
		})
	})

	// API 路由组
	v1 := r.Group("/api/v1")
	{
		// 用户认证路由
		auth := v1.Group("/auth")
		{
			auth.POST("/login", handleLogin)
			auth.POST("/logout", handleLogout)
			auth.POST("/refresh", handleRefresh)
			auth.GET("/profile", authMiddleware(), handleProfile)
		}

		// 用户管理路由
		users := v1.Group("/users")
		users.Use(authMiddleware())
		{
			users.GET("", handleListUsers)
			users.POST("", handleCreateUser)
			users.GET("/:id", handleGetUser)
			users.PUT("/:id", handleUpdateUser)
			users.DELETE("/:id", handleDeleteUser)
		}

		// 无人机管理路由
		drones := v1.Group("/drones")
		drones.Use(authMiddleware())
		{
			drones.GET("", handleListDrones)
			drones.POST("", handleCreateDrone)
			drones.GET("/:id", handleGetDrone)
			drones.PUT("/:id", handleUpdateDrone)
			drones.DELETE("/:id", handleDeleteDrone)
			drones.POST("/:id/command", handleDroneCommand)
			drones.GET("/:id/status", handleDroneStatus)
		}

		// 任务管理路由
		tasks := v1.Group("/tasks")
		tasks.Use(authMiddleware())
		{
			tasks.GET("", handleListTasks)
			tasks.POST("", handleCreateTask)
			tasks.GET("/:id", handleGetTask)
			tasks.PUT("/:id", handleUpdateTask)
			tasks.DELETE("/:id", handleDeleteTask)
			tasks.POST("/:id/start", handleStartTask)
			tasks.POST("/:id/pause", handlePauseTask)
			tasks.POST("/:id/stop", handleStopTask)
		}

		// 告警管理路由
		alerts := v1.Group("/alerts")
		alerts.Use(authMiddleware())
		{
			alerts.GET("", handleListAlerts)
			alerts.GET("/:id", handleGetAlert)
			alerts.POST("/:id/acknowledge", handleAcknowledgeAlert)
			alerts.POST("/:id/resolve", handleResolveAlert)
		}

		// 监控路由
		monitor := v1.Group("/monitor")
		monitor.Use(authMiddleware())
		{
			monitor.GET("/dashboard", handleDashboard)
			monitor.GET("/metrics", handleMetrics)
		}
	}

	// WebSocket 路由
	r.GET("/ws/monitor", handleWebSocketMonitor)

	// 启动服务器
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", config.GetInt("server.port")),
		Handler:        r,
		ReadTimeout:    config.GetDuration("server.read_timeout"),
		WriteTimeout:   config.GetDuration("server.write_timeout"),
		MaxHeaderBytes: 1 << 20,
	}

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.WithError(err).Fatal("Failed to start server")
		}
	}()

	appLogger.WithField("port", config.GetInt("server.port")).Info("API Gateway started")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.WithError(err).Fatal("Server forced to shutdown")
	}

	appLogger.Info("Server exited")
}

func loadConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../../configs")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return v, nil
}

// 中间件

func LoggerMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.RequestLogger(method, path, clientIP, statusCode, latency.String()).Info("Request completed")
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func RecoveryMiddleware(logger *logger.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.WithField("panic", recovered).Error("Panic recovered")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// JWT 验证逻辑
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization token",
			})
			c.Abort()
			return
		}

		// TODO: 验证 JWT token
		// 这里应该调用用户服务验证token

		c.Next()
	}
}

// 处理函数

func handleLogin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Login endpoint",
		"status":  "success",
		"data": gin.H{
			"token":      "example_jwt_token",
			"expires_in": 86400,
		},
	})
}

func handleLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

func handleRefresh(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed",
		"data": gin.H{
			"token": "new_jwt_token",
		},
	})
}

func handleProfile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "User profile",
		"data": gin.H{
			"id":       1,
			"username": "admin",
			"email":    "admin@drone-control.com",
			"role":     "admin",
		},
	})
}

func handleListUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Users list",
		"data": []gin.H{
			{"id": 1, "username": "admin", "email": "admin@example.com", "role": "admin"},
			{"id": 2, "username": "operator", "email": "operator@example.com", "role": "operator"},
		},
	})
}

func handleCreateUser(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"data": gin.H{
			"id":       3,
			"username": "new_user",
		},
	})
}

func handleGetUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "User details",
		"data": gin.H{
			"id":       id,
			"username": "user_" + id,
			"email":    "user" + id + "@example.com",
		},
	})
}

func handleUpdateUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"data": gin.H{
			"id": id,
		},
	})
}

func handleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
		"data": gin.H{
			"id": id,
		},
	})
}

func handleListDrones(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Drones list",
		"data": []gin.H{
			{
				"id":       1,
				"serial":   "DRONE001",
				"model":    "DJI Mavic Pro",
				"status":   "online",
				"battery":  85,
				"position": gin.H{"lat": 40.7128, "lng": -74.0060, "alt": 100},
			},
			{
				"id":       2,
				"serial":   "DRONE002",
				"model":    "DJI Air 2S",
				"status":   "flying",
				"battery":  92,
				"position": gin.H{"lat": 40.7589, "lng": -73.9851, "alt": 150},
			},
		},
	})
}

func handleCreateDrone(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "Drone registered successfully",
		"data": gin.H{
			"id":     3,
			"serial": "DRONE003",
			"status": "offline",
		},
	})
}

func handleGetDrone(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Drone details",
		"data": gin.H{
			"id":       id,
			"serial":   "DRONE" + id,
			"model":    "DJI Mavic Pro",
			"status":   "online",
			"battery":  78,
			"position": gin.H{"lat": 40.7128, "lng": -74.0060, "alt": 120},
		},
	})
}

func handleUpdateDrone(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Drone updated successfully",
		"data": gin.H{
			"id": id,
		},
	})
}

func handleDeleteDrone(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Drone removed successfully",
		"data": gin.H{
			"id": id,
		},
	})
}

func handleDroneCommand(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message":    "Command sent to drone",
		"drone_id":   id,
		"command_id": "CMD_" + fmt.Sprintf("%d", time.Now().Unix()),
		"status":     "accepted",
	})
}

func handleDroneStatus(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Drone status",
		"data": gin.H{
			"drone_id": id,
			"status":   "online",
			"battery":  85,
			"position": gin.H{"lat": 40.7128, "lng": -74.0060, "alt": 100},
			"sensors": gin.H{
				"temperature": 25.5,
				"humidity":    60.0,
				"wind_speed":  5.2,
			},
		},
	})
}

func handleListTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Tasks list",
		"data": []gin.H{
			{
				"id":           1,
				"name":         "仓库巡检",
				"status":       "completed",
				"drone_id":     1,
				"progress":     100,
				"created_at":   "2025-07-26T10:00:00Z",
				"completed_at": "2025-07-26T10:30:00Z",
			},
			{
				"id":         2,
				"name":       "区域监控",
				"status":     "running",
				"drone_id":   2,
				"progress":   65,
				"created_at": "2025-07-26T11:00:00Z",
			},
		},
	})
}

func handleCreateTask(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "Task created successfully",
		"data": gin.H{
			"id":         3,
			"name":       "新任务",
			"status":     "pending",
			"created_at": time.Now().Format(time.RFC3339),
		},
	})
}

func handleGetTask(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Task details",
		"data": gin.H{
			"id":          id,
			"name":        "任务 " + id,
			"description": "详细的任务描述",
			"status":      "running",
			"progress":    45,
			"drone_id":    1,
			"waypoints": []gin.H{
				{"lat": 40.7128, "lng": -74.0060, "alt": 100, "action": "capture"},
				{"lat": 40.7150, "lng": -74.0080, "alt": 120, "action": "inspect"},
			},
		},
	})
}

func handleUpdateTask(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Task updated successfully",
		"data": gin.H{
			"id": id,
		},
	})
}

func handleDeleteTask(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
		"data": gin.H{
			"id": id,
		},
	})
}

func handleStartTask(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Task started successfully",
		"data": gin.H{
			"task_id":    id,
			"status":     "running",
			"started_at": time.Now().Format(time.RFC3339),
		},
	})
}

func handlePauseTask(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Task paused",
		"data": gin.H{
			"task_id": id,
			"status":  "paused",
		},
	})
}

func handleStopTask(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Task stopped",
		"data": gin.H{
			"task_id": id,
			"status":  "stopped",
		},
	})
}

func handleListAlerts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Alerts list",
		"data": []gin.H{
			{
				"id":           1,
				"type":         "battery",
				"level":        "warning",
				"message":      "无人机电量低于30%",
				"drone_id":     1,
				"acknowledged": false,
				"created_at":   "2025-07-26T12:00:00Z",
			},
			{
				"id":           2,
				"type":         "weather",
				"level":        "info",
				"message":      "风速增强，建议谨慎飞行",
				"acknowledged": true,
				"created_at":   "2025-07-26T11:30:00Z",
			},
		},
	})
}

func handleGetAlert(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Alert details",
		"data": gin.H{
			"id":           id,
			"type":         "battery",
			"level":        "warning",
			"message":      "电量告警详情",
			"drone_id":     1,
			"acknowledged": false,
		},
	})
}

func handleAcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Alert acknowledged",
		"data": gin.H{
			"alert_id":        id,
			"acknowledged":    true,
			"acknowledged_at": time.Now().Format(time.RFC3339),
		},
	})
}

func handleResolveAlert(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Alert resolved",
		"data": gin.H{
			"alert_id":    id,
			"status":      "resolved",
			"resolved_at": time.Now().Format(time.RFC3339),
		},
	})
}

func handleDashboard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Dashboard data",
		"data": gin.H{
			"summary": gin.H{
				"total_drones":   3,
				"active_drones":  2,
				"running_tasks":  1,
				"pending_alerts": 2,
			},
			"recent_activities": []gin.H{
				{"time": "12:30", "event": "任务完成", "drone": "DRONE001"},
				{"time": "12:15", "event": "电量告警", "drone": "DRONE002"},
				{"time": "12:00", "event": "任务开始", "drone": "DRONE001"},
			},
		},
	})
}

func handleMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "System metrics",
		"data": gin.H{
			"system": gin.H{
				"cpu_usage":    "45%",
				"memory_usage": "62%",
				"disk_usage":   "38%",
			},
			"service": gin.H{
				"requests_per_second": 120,
				"response_time_avg":   "45ms",
				"error_rate":          "0.1%",
			},
		},
	})
}

func handleWebSocketMonitor(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "WebSocket monitor endpoint",
		"note":    "Use WebSocket client to connect for real-time updates",
	})
}
