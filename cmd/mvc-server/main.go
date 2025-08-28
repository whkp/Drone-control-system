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

	"drone-control-system/internal/mvc/controllers"
	"drone-control-system/internal/mvc/handlers"
	"drone-control-system/internal/mvc/middleware"
	"drone-control-system/internal/mvc/models"
	"drone-control-system/internal/mvc/routes"
	"drone-control-system/internal/mvc/services"
	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"

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

	// 初始化数据库（示例，需要根据实际情况实现）
	// db, err := initDatabase(config)
	// if err != nil {
	//     appLogger.WithFields(map[string]interface{}{"error": err}).Fatal("Failed to initialize database")
	//     return
	// }

	// 初始化服务层（示例，需要根据实际情况实现）
	// userService := services.NewUserService(db, appLogger)
	// droneService := services.NewDroneService(db, appLogger)

	// 为了演示，创建mock服务
	userService := &MockUserService{}
	droneService := &MockDroneService{}

	// 🚀 初始化Kafka服务
	kafkaConfig := &kafka.Config{
		Brokers:          []string{config.GetString("kafka.brokers")},
		GroupID:          config.GetString("kafka.group_id"),
		AutoOffsetReset:  config.GetString("kafka.auto_offset_reset"),
		SessionTimeout:   config.GetDuration("kafka.session_timeout"),
		CommitInterval:   config.GetDuration("kafka.commit_interval"),
		CompressionCodec: config.GetString("kafka.compression_codec"),
	}

	kafkaService, err := services.NewKafkaService(kafkaConfig, appLogger)
	if err != nil {
		appLogger.Error("Failed to create kafka service", map[string]interface{}{"error": err.Error()})
		log.Fatalf("Failed to create kafka service: %v", err)
	}

	// 🌐 初始化WebSocket服务
	websocketService := services.NewWebSocketService(appLogger)

	// 🧠 初始化智能告警服务
	smartAlertService := services.NewSmartAlertService(appLogger, kafkaService)

	// 🔗 初始化事件处理器
	eventHandler := handlers.NewEventHandler(appLogger, websocketService, smartAlertService)

	// 初始化控制器
	userController := controllers.NewUserController(appLogger, userService)
	droneController := controllers.NewDroneController(appLogger, droneService, kafkaService)

	// 初始化中间件
	authMiddleware := middleware.NewAuthMiddleware(userService, appLogger)

	// 初始化路由
	router := routes.NewRouter(
		appLogger,
		authMiddleware,
		userController,
		droneController,
		websocketService,
	)

	// 设置路由
	router.SetupRoutes()

	// 🚀 启动WebSocket服务
	if err := websocketService.Start(); err != nil {
		appLogger.Error("Failed to start WebSocket service", map[string]interface{}{"error": err.Error()})
		log.Fatalf("Failed to start WebSocket service: %v", err)
	}

	// 🚀 启动Kafka服务并注册事件处理器
	if err := kafkaService.Start(context.Background()); err != nil {
		appLogger.Error("Failed to start Kafka service", map[string]interface{}{"error": err.Error()})
		log.Fatalf("Failed to start Kafka service: %v", err)
	}

	// 注册Kafka事件处理器（如果Kafka管理器支持）
	// 这里需要根据实际的Kafka管理器API来注册
	appLogger.Info("Event handler initialized", map[string]interface{}{
		"handler":             "event_handler",
		"smart_alert_enabled": true,
	})

	// 使用事件处理器（示例用法）
	_ = eventHandler

	// 创建HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.GetString("server.port")),
		Handler: router.GetEngine(),
	}

	// 启动服务器
	go func() {
		appLogger.WithFields(map[string]interface{}{
			"port": config.GetString("server.port"),
		}).Info("Starting MVC API server")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.WithFields(map[string]interface{}{
				"error": err,
			}).Fatal("Failed to start server")
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 🛑 停止Kafka服务
	if err := kafkaService.Stop(); err != nil {
		appLogger.Error("Error stopping Kafka service", map[string]interface{}{"error": err.Error()})
	} else {
		appLogger.Info("Kafka service stopped")
	}

	// 🛑 停止WebSocket服务
	if err := websocketService.Stop(); err != nil {
		appLogger.Error("Error stopping WebSocket service", map[string]interface{}{"error": err.Error()})
	} else {
		appLogger.Info("WebSocket service stopped")
	}

	// 🛑 停止HTTP服务器
	if err := server.Shutdown(ctx); err != nil {
		appLogger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("Server forced to shutdown")
	} else {
		appLogger.Info("Server shutdown completed")
	}
}

// loadConfig 加载配置
func loadConfig() (*viper.Viper, error) {
	config := viper.New()

	// 设置默认值
	config.SetDefault("server.port", "8080")
	config.SetDefault("logging.level", "info")
	config.SetDefault("logging.format", "json")
	config.SetDefault("logging.output", "stdout")

	// 设置配置文件
	config.SetConfigName("config")
	config.SetConfigType("yaml")
	config.AddConfigPath("./configs")
	config.AddConfigPath(".")

	// 读取环境变量
	config.AutomaticEnv()

	// 尝试读取配置文件
	if err := config.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// 配置文件不存在时使用默认值
	}

	return config, nil
}

// Mock服务实现（示例）
type MockUserService struct{}

func (m *MockUserService) CreateUser(ctx context.Context, params *services.CreateUserParams) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) GetUserByID(ctx context.Context, id uint) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) UpdateUser(ctx context.Context, id uint, params *services.UpdateUserParams) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) DeleteUser(ctx context.Context, id uint) error {
	return fmt.Errorf("not implemented")
}

func (m *MockUserService) ListUsers(ctx context.Context, params *services.ListUsersParams) ([]*models.User, int64, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (m *MockUserService) Login(ctx context.Context, username, password string) (*services.LoginResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockUserService) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockUserService) RefreshToken(ctx context.Context, token string) (*services.LoginResult, error) {
	return nil, fmt.Errorf("not implemented")
}

type MockDroneService struct{}

func (m *MockDroneService) CreateDrone(ctx context.Context, params *services.CreateDroneParams) (*models.Drone, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockDroneService) GetDroneByID(ctx context.Context, id uint) (*models.Drone, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockDroneService) GetDroneBySerialNo(ctx context.Context, serialNo string) (*models.Drone, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockDroneService) UpdateDrone(ctx context.Context, id uint, params *services.UpdateDroneParams) (*models.Drone, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockDroneService) DeleteDrone(ctx context.Context, id uint) error {
	return fmt.Errorf("not implemented")
}

func (m *MockDroneService) ListDrones(ctx context.Context, params *services.ListDronesParams) ([]*models.Drone, int64, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (m *MockDroneService) UpdateDroneStatus(ctx context.Context, id uint, status models.DroneStatus) error {
	return fmt.Errorf("not implemented")
}

func (m *MockDroneService) UpdateDronePosition(ctx context.Context, id uint, position models.Position) error {
	return fmt.Errorf("not implemented")
}

func (m *MockDroneService) UpdateDroneBattery(ctx context.Context, id uint, battery int) error {
	return fmt.Errorf("not implemented")
}

func (m *MockDroneService) GetAvailableDrones(ctx context.Context) ([]*models.Drone, error) {
	return nil, fmt.Errorf("not implemented")
}
