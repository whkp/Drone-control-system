package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Config LLM配置
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float32
}

// Client LLM客户端
type Client struct {
	client *openai.Client
	config Config
}

// NewClient 创建LLM客户端
func NewClient(config Config) *Client {
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}
	
	return &Client{
		client: openai.NewClientWithConfig(clientConfig),
		config: config,
	}
}

// TaskPlan 任务规划结构
type TaskPlan struct {
	PlanID string     `json:"plan_id"`
	Steps  []TaskStep `json:"steps"`
}

// TaskStep 任务步骤
type TaskStep struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Order      int                    `json:"order"`
	Safety     string                 `json:"safety,omitempty"`
}

// PlanningRequest 规划请求
type PlanningRequest struct {
	Command     string            `json:"command"`
	Environment EnvironmentState  `json:"environment"`
	Constraints PlanningConstraints `json:"constraints"`
}

// EnvironmentState 环境状态
type EnvironmentState struct {
	DronePosition  Position      `json:"drone_position"`
	Battery        int           `json:"battery"`
	Weather        WeatherInfo   `json:"weather"`
	Obstacles      []Obstacle    `json:"obstacles"`
	NoFlyZones     []Zone        `json:"no_fly_zones"`
}

// Position 位置信息
type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Heading   float64 `json:"heading"`
}

// WeatherInfo 天气信息
type WeatherInfo struct {
	WindSpeed    float64 `json:"wind_speed"`    // 风速 m/s
	WindDirection float64 `json:"wind_direction"` // 风向 度
	Visibility   float64 `json:"visibility"`    // 能见度 km
	Temperature  float64 `json:"temperature"`   // 温度 °C
	Humidity     float64 `json:"humidity"`      // 湿度 %
}

// Obstacle 障碍物
type Obstacle struct {
	ID       string   `json:"id"`
	Position Position `json:"position"`
	Size     Size     `json:"size"`
	Type     string   `json:"type"`
}

// Size 尺寸
type Size struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Length float64 `json:"length"`
}

// Zone 区域
type Zone struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"` // no-fly, restricted, safe
	Boundary []Position `json:"boundary"`
	MinAlt   float64    `json:"min_altitude"`
	MaxAlt   float64    `json:"max_altitude"`
}

// PlanningConstraints 规划约束
type PlanningConstraints struct {
	MaxAltitude    float64 `json:"max_altitude"`
	MaxDistance    float64 `json:"max_distance"`
	MaxFlightTime  int     `json:"max_flight_time"` // 分钟
	MinBattery     int     `json:"min_battery"`     // 百分比
	SafetyDistance float64 `json:"safety_distance"` // 与障碍物的安全距离
}

// GenerateTaskPlan 生成任务规划
func (c *Client) GenerateTaskPlan(ctx context.Context, request PlanningRequest) (*TaskPlan, error) {
	prompt := c.buildPlanningPrompt(request)
	
	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.config.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: c.getSystemPrompt(),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens:   c.config.MaxTokens,
			Temperature: c.config.Temperature,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// 解析响应
	content := resp.Choices[0].Message.Content
	plan, err := c.parsePlanResponse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 验证规划
	if err := c.validatePlan(plan, request.Constraints); err != nil {
		return nil, fmt.Errorf("invalid plan: %w", err)
	}

	return plan, nil
}

// AnalyzeCommand 分析用户指令
func (c *Client) AnalyzeCommand(ctx context.Context, command string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`
分析以下无人机指令并提取关键信息：

指令: %s

请分析指令的：
1. 任务类型 (inspection, delivery, mapping, patrol, emergency)
2. 目标位置或区域
3. 具体要求或动作
4. 优先级
5. 时间要求

以JSON格式返回分析结果。
`, command)

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.config.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens:   c.config.MaxTokens,
			Temperature: c.config.Temperature,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze command: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// 解析JSON响应
	content := resp.Choices[0].Message.Content
	var analysis map[string]interface{}
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		// 如果无法解析JSON，返回原始内容
		return map[string]interface{}{
			"raw_response": content,
			"parsed":       false,
		}, nil
	}

	return analysis, nil
}

// OptimizePath 路径优化
func (c *Client) OptimizePath(ctx context.Context, waypoints []Position, constraints PlanningConstraints) ([]Position, error) {
	waypointsJSON, _ := json.Marshal(waypoints)
	constraintsJSON, _ := json.Marshal(constraints)

	prompt := fmt.Sprintf(`
优化以下无人机路径，考虑以下约束条件：

路径点: %s
约束条件: %s

请优化路径以：
1. 最小化总飞行距离
2. 避开障碍物
3. 满足安全距离要求
4. 考虑电量消耗

返回优化后的路径点JSON数组。
`, string(waypointsJSON), string(constraintsJSON))

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.config.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens:   c.config.MaxTokens,
			Temperature: c.config.Temperature,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to optimize path: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// 解析优化后的路径
	content := resp.Choices[0].Message.Content
	var optimizedPath []Position
	if err := json.Unmarshal([]byte(content), &optimizedPath); err != nil {
		return waypoints, nil // 返回原始路径
	}

	return optimizedPath, nil
}

// 私有方法

func (c *Client) getSystemPrompt() string {
	return `你是一个专业的无人机任务规划专家。你需要根据用户指令和环境状态，生成安全、高效的无人机任务规划。

规划原则：
1. 安全第一：严格遵守禁飞区、安全距离等约束
2. 效率优化：最短路径、最少电量消耗
3. 任务完成：确保指令要求得到满足
4. 应急处理：考虑异常情况的处理方案

支持的动作类型：
- fly_to: 飞往指定坐标 {target: [x,y,z], speed: float}
- capture: 拍摄照片/视频 {mode: "photo/video", duration: int}
- inspect: 检查目标 {target_id: string, detail_level: string}
- hover: 悬停 {duration: int}
- return_home: 返回起飞点
- land: 降落 {location: [x,y,z]}

请始终以JSON格式返回规划结果。`
}

func (c *Client) buildPlanningPrompt(request PlanningRequest) string {
	envJSON, _ := json.Marshal(request.Environment)
	constJSON, _ := json.Marshal(request.Constraints)

	return fmt.Sprintf(`
用户指令: %s

当前环境状态:
%s

规划约束:
%s

请生成详细的任务规划，包含步骤序列和安全检查。
`, request.Command, string(envJSON), string(constJSON))
}

func (c *Client) parsePlanResponse(content string) (*TaskPlan, error) {
	// 提取JSON部分
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}") + 1
	
	if start == -1 || end == 0 || start >= end {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := content[start:end]
	
	var plan TaskPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	return &plan, nil
}

func (c *Client) validatePlan(plan *TaskPlan, constraints PlanningConstraints) error {
	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan contains no steps")
	}

	// 验证步骤顺序
	for i, step := range plan.Steps {
		if step.Order != i+1 {
			return fmt.Errorf("invalid step order at index %d", i)
		}

		// 验证必要参数
		if step.Action == "" {
			return fmt.Errorf("step %d missing action", i+1)
		}
	}

	return nil
}
