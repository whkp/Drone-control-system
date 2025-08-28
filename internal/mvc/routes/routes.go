package routes

import (
	"drone-control-system/internal/mvc/controllers"
	"drone-control-system/internal/mvc/middleware"
	"drone-control-system/internal/mvc/services"
	"drone-control-system/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Router 路由管理器
type Router struct {
	engine           *gin.Engine
	logger           *logger.Logger
	authMiddleware   *middleware.AuthMiddleware
	userController   *controllers.UserController
	droneController  *controllers.DroneController
	websocketService services.WebSocketService
	// taskController   *controllers.TaskController
	// alertController  *controllers.AlertController
}

// NewRouter 创建路由管理器
func NewRouter(
	logger *logger.Logger,
	authMiddleware *middleware.AuthMiddleware,
	userController *controllers.UserController,
	droneController *controllers.DroneController,
	websocketService services.WebSocketService,
) *Router {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	return &Router{
		engine:           engine,
		logger:           logger,
		authMiddleware:   authMiddleware,
		userController:   userController,
		droneController:  droneController,
		websocketService: websocketService,
	}
}

// SetupRoutes 设置路由
func (r *Router) SetupRoutes() {
	// 添加全局中间件
	r.engine.Use(middleware.LoggerMiddleware(r.logger))
	r.engine.Use(middleware.CORSMiddleware())
	r.engine.Use(middleware.RecoveryMiddleware(r.logger))
	r.engine.Use(middleware.RequestIDMiddleware())
	r.engine.Use(middleware.SecurityMiddleware())

	// 健康检查
	r.engine.GET("/health", r.healthCheck)
	r.engine.GET("/ping", r.ping)

	// API版本分组
	v1 := r.engine.Group("/api/v1")
	{
		// 公开路由（无需认证）
		public := v1.Group("/public")
		{
			public.POST("/login", r.userController.Login)
			// public.POST("/register", r.userController.Register) // 如果需要公开注册
		}

		// 需要认证的路由
		protected := v1.Group("/")
		protected.Use(r.authMiddleware.RequireAuth())
		{
			// 用户相关路由
			r.setupUserRoutes(protected)

			// 无人机相关路由
			r.setupDroneRoutes(protected)

			// 任务相关路由
			// r.setupTaskRoutes(protected)

			// 告警相关路由
			// r.setupAlertRoutes(protected)
		}
	}

	// WebSocket路由（如果需要）
	r.engine.GET("/ws", r.handleWebSocket)
}

// setupUserRoutes 设置用户路由
func (r *Router) setupUserRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	{
		// 当前用户相关
		users.GET("/profile", r.userController.GetProfile)
		users.PUT("/profile", r.userController.UpdateUser)
		users.POST("/change-password", r.userController.ChangePassword)

		// 用户管理（需要管理员权限）
		adminUsers := users.Use(r.authMiddleware.RequireRole("admin"))
		{
			adminUsers.POST("/", r.userController.CreateUser)
			adminUsers.GET("/", r.userController.ListUsers)
			adminUsers.GET("/:id", r.userController.GetUser)
			adminUsers.PUT("/:id", r.userController.UpdateUser)
			adminUsers.DELETE("/:id", r.userController.DeleteUser)
		}
	}
}

// setupDroneRoutes 设置无人机路由
func (r *Router) setupDroneRoutes(rg *gin.RouterGroup) {
	drones := rg.Group("/drones")
	{
		// 查看无人机（所有用户）
		drones.GET("/", r.droneController.ListDrones)
		drones.GET("/available", r.droneController.GetAvailableDrones)
		drones.GET("/:id", r.droneController.GetDrone)

		// 更新无人机状态和位置（操作员及以上）
		operatorDrones := drones.Use(r.authMiddleware.RequireRole("operator"))
		{
			operatorDrones.POST("/", r.droneController.CreateDrone)
			operatorDrones.PUT("/:id", r.droneController.UpdateDrone)
			operatorDrones.PUT("/:id/status", r.droneController.UpdateDroneStatus)
			operatorDrones.PUT("/:id/position", r.droneController.UpdateDronePosition)
			operatorDrones.PUT("/:id/battery", r.droneController.UpdateDroneBattery)
		}

		// 删除无人机（仅管理员）
		adminDrones := drones.Use(r.authMiddleware.RequireRole("admin"))
		{
			adminDrones.DELETE("/:id", r.droneController.DeleteDrone)
		}
	}
}

// setupTaskRoutes 设置任务路由
/*
func (r *Router) setupTaskRoutes(rg *gin.RouterGroup) {
	tasks := rg.Group("/tasks")
	{
		// 查看任务（所有用户）
		tasks.GET("/", r.taskController.ListTasks)
		tasks.GET("/my", r.taskController.GetMyTasks)
		tasks.GET("/:id", r.taskController.GetTask)

		// 操作任务（操作员及以上）
		operatorTasks := tasks.Use(r.authMiddleware.RequireRole("operator"))
		{
			operatorTasks.POST("/", r.taskController.CreateTask)
			operatorTasks.PUT("/:id", r.taskController.UpdateTask)
			operatorTasks.POST("/:id/start", r.taskController.StartTask)
			operatorTasks.POST("/:id/stop", r.taskController.StopTask)
			operatorTasks.PUT("/:id/progress", r.taskController.UpdateTaskProgress)
		}

		// 删除任务（仅管理员）
		adminTasks := tasks.Use(r.authMiddleware.RequireRole("admin"))
		{
			adminTasks.DELETE("/:id", r.taskController.DeleteTask)
		}
	}
}
*/

// setupAlertRoutes 设置告警路由
/*
func (r *Router) setupAlertRoutes(rg *gin.RouterGroup) {
	alerts := rg.Group("/alerts")
	{
		// 查看告警（所有用户）
		alerts.GET("/", r.alertController.ListAlerts)
		alerts.GET("/active", r.alertController.GetActiveAlerts)
		alerts.GET("/:id", r.alertController.GetAlert)

		// 处理告警（操作员及以上）
		operatorAlerts := alerts.Use(r.authMiddleware.RequireRole("operator"))
		{
			operatorAlerts.POST("/:id/acknowledge", r.alertController.AcknowledgeAlert)
			operatorAlerts.POST("/:id/resolve", r.alertController.ResolveAlert)
		}

		// 管理告警（仅管理员）
		adminAlerts := alerts.Use(r.authMiddleware.RequireRole("admin"))
		{
			adminAlerts.POST("/", r.alertController.CreateAlert)
			adminAlerts.PUT("/:id", r.alertController.UpdateAlert)
			adminAlerts.DELETE("/:id", r.alertController.DeleteAlert)
		}
	}
}
*/

// healthCheck 健康检查
func (r *Router) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "ok",
		"service":   "drone-control-system",
		"version":   "1.0.0",
		"timestamp": "2024-01-01T00:00:00Z",
	})
}

// ping 简单ping
func (r *Router) ping(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

// handleWebSocket WebSocket处理
func (r *Router) handleWebSocket(c *gin.Context) {
	// 从JWT中获取用户ID（可选）
	var userID *uint
	if userIDInterface, exists := c.Get("userID"); exists {
		if uid, ok := userIDInterface.(uint); ok {
			userID = &uid
		}
	}

	// 升级HTTP连接为WebSocket
	err := r.websocketService.HandleWebSocketConnection(c.Writer, c.Request, userID)
	if err != nil {
		r.logger.Error("Failed to upgrade WebSocket connection", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(400, gin.H{
			"error": "Failed to upgrade connection",
		})
		return
	}
}

// GetEngine 获取Gin引擎
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}
