package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"drone-control-system/pkg/llm"
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

	// 初始化LLM客户端
	llmClient := llm.NewClient(llm.Config{
		APIKey:      config.GetString("llm.deepseek.api_key"),
		BaseURL:     config.GetString("llm.deepseek.base_url"),
		Model:       config.GetString("llm.deepseek.model"),
		MaxTokens:   config.GetInt("llm.deepseek.max_tokens"),
		Temperature: float32(config.GetFloat64("llm.deepseek.temperature")),
	})

	// 创建HTTP服务器
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"task-service","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// 任务管理端点
	mux.HandleFunc("/api/tasks", handleTasks)
	mux.HandleFunc("/api/tasks/plan", func(w http.ResponseWriter, r *http.Request) {
		handleTaskPlanning(w, r, llmClient, appLogger)
	})
	mux.HandleFunc("/api/tasks/schedule", handleScheduleTasks)
	mux.HandleFunc("/api/tasks/execute", handleExecuteTasks)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.GetInt("grpc.task_service")),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 优雅关闭
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.WithError(err).Fatal("Failed to start task service")
		}
	}()

	appLogger.WithField("port", config.GetInt("grpc.task_service")).Info("Task Service started")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down task service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.WithError(err).Fatal("Server forced to shutdown")
	}

	appLogger.Info("Task service exited")
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

func handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"message":"任务列表",
			"tasks":[
				{"id":1,"name":"仓库巡检","status":"completed","drone_id":1},
				{"id":2,"name":"区域监控","status":"running","drone_id":2}
			]
		}`))
	case http.MethodPost:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"message":"任务创建成功","task_id":3}`))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleTaskPlanning(w http.ResponseWriter, r *http.Request, llmClient *llm.Client, logger *logger.Logger) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command     string `json:"command"`
		DroneID     string `json:"drone_id"`
		Environment struct {
			Weather    string  `json:"weather"`
			WindSpeed  float64 `json:"wind_speed"`
			Visibility float64 `json:"visibility"`
		} `json:"environment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 构建规划请求
	planReq := llm.PlanningRequest{
		Command: req.Command,
		Environment: llm.EnvironmentState{
			DronePosition: llm.Position{Latitude: 40.7128, Longitude: -74.0060, Altitude: 0},
			Battery:       85,
			Weather: llm.WeatherInfo{
				WindSpeed:   req.Environment.WindSpeed,
				Visibility:  req.Environment.Visibility,
				Temperature: 20.0,
				Humidity:    60.0,
			},
		},
		Constraints: llm.PlanningConstraints{
			MaxAltitude:    120,
			MaxDistance:    5000,
			MaxFlightTime:  30,
			MinBattery:     20,
			SafetyDistance: 5.0,
		},
	}

	ctx := context.Background()
	plan, err := llmClient.GenerateTaskPlan(ctx, planReq)
	if err != nil {
		logger.WithError(err).Error("Failed to generate task plan")
		// 返回备用计划
		plan = &llm.TaskPlan{
			PlanID: fmt.Sprintf("backup_plan_%d", time.Now().Unix()),
			Steps: []llm.TaskStep{
				{
					Action: "takeoff",
					Parameters: map[string]interface{}{
						"altitude": 50,
					},
					Order: 1,
				},
				{
					Action: "fly_to",
					Parameters: map[string]interface{}{
						"target": []float64{40.7150, -74.0080, 100},
					},
					Order: 2,
				},
				{
					Action: "inspect",
					Parameters: map[string]interface{}{
						"duration": 300,
						"mode":     "visual",
					},
					Order: 3,
				},
				{
					Action: "return_home",
					Parameters: map[string]interface{}{},
					Order:      4,
				},
			},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "任务规划生成成功",
		"plan":    plan,
	})
}

func handleScheduleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message":"任务调度成功","scheduled_tasks":3}`))
}

func handleExecuteTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message":"任务执行中","executing_tasks":2}`))
}
