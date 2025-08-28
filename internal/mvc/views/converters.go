package views

import (
	"encoding/json"
	"strings"

	"drone-control-system/internal/mvc/models"
)

// ModelToUserView 将User模型转换为UserView
func ModelToUserView(user *models.User) *UserView {
	if user == nil {
		return nil
	}

	return &UserView{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		Status:    user.Status,
		Avatar:    user.Avatar,
		LastLogin: user.LastLogin,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// ModelToDroneView 将Drone模型转换为DroneView
func ModelToDroneView(drone *models.Drone) *DroneView {
	if drone == nil {
		return nil
	}

	// 解析capabilities JSON字符串
	var capabilities []string
	if drone.Capabilities != "" {
		json.Unmarshal([]byte(drone.Capabilities), &capabilities)
	}

	return &DroneView{
		ID:           drone.ID,
		SerialNo:     drone.SerialNo,
		Model:        drone.Model,
		Status:       drone.Status,
		Battery:      drone.Battery,
		Position:     drone.Position,
		LastSeen:     drone.LastSeen,
		Capabilities: capabilities,
		Firmware:     drone.Firmware,
		Version:      drone.Version,
		IsOnline:     drone.IsOnline(),
		IsAvailable:  drone.IsAvailable(),
		CreatedAt:    drone.CreatedAt,
		UpdatedAt:    drone.UpdatedAt,
	}
}

// ModelToTaskView 将Task模型转换为TaskView
func ModelToTaskView(task *models.Task) *TaskView {
	if task == nil {
		return nil
	}

	return &TaskView{
		ID:          task.ID,
		Name:        task.Name,
		Description: task.Description,
		Type:        task.Type,
		Status:      task.Status,
		Priority:    task.Priority,
		Progress:    task.Progress,
		User:        ModelToUserView(&task.User),
		Drone:       ModelToDroneView(&task.Drone),
		Plan:        task.Plan,
		Result:      task.Result,
		ScheduledAt: task.ScheduledAt,
		StartedAt:   task.StartedAt,
		CompletedAt: task.CompletedAt,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}

// ModelToAlertView 将Alert模型转换为AlertView
func ModelToAlertView(alert *models.Alert) *AlertView {
	if alert == nil {
		return nil
	}

	// 解析data JSON字符串
	var data interface{}
	if alert.Data != "" {
		json.Unmarshal([]byte(alert.Data), &data)
	}

	return &AlertView{
		ID:             alert.ID,
		Title:          alert.Title,
		Message:        alert.Message,
		Type:           alert.Type,
		Level:          alert.Level,
		Status:         alert.Status,
		Source:         alert.Source,
		Code:           alert.Code,
		Data:           data,
		Drone:          ModelToDroneView(alert.Drone),
		Task:           ModelToTaskView(alert.Task),
		User:           ModelToUserView(alert.User),
		AcknowledgedAt: alert.AcknowledgedAt,
		AcknowledgedBy: ModelToUserView(alert.AckUser),
		ResolvedAt:     alert.ResolvedAt,
		ResolvedBy:     ModelToUserView(alert.ResolveUser),
		SeverityScore:  alert.GetSeverityScore(),
		CreatedAt:      alert.CreatedAt,
		UpdatedAt:      alert.UpdatedAt,
	}
}

// ModelsToUserViews 批量转换User模型为UserView
func ModelsToUserViews(users []*models.User) []*UserView {
	views := make([]*UserView, len(users))
	for i, user := range users {
		views[i] = ModelToUserView(user)
	}
	return views
}

// ModelsToDroneViews 批量转换Drone模型为DroneView
func ModelsToDroneViews(drones []*models.Drone) []*DroneView {
	views := make([]*DroneView, len(drones))
	for i, drone := range drones {
		views[i] = ModelToDroneView(drone)
	}
	return views
}

// ModelsToTaskViews 批量转换Task模型为TaskView
func ModelsToTaskViews(tasks []*models.Task) []*TaskView {
	views := make([]*TaskView, len(tasks))
	for i, task := range tasks {
		views[i] = ModelToTaskView(task)
	}
	return views
}

// ModelsToAlertViews 批量转换Alert模型为AlertView
func ModelsToAlertViews(alerts []*models.Alert) []*AlertView {
	views := make([]*AlertView, len(alerts))
	for i, alert := range alerts {
		views[i] = ModelToAlertView(alert)
	}
	return views
}

// FormatUserRole 格式化用户角色显示
func FormatUserRole(role models.UserRole) string {
	switch role {
	case models.RoleAdmin:
		return "管理员"
	case models.RoleOperator:
		return "操作员"
	case models.RoleViewer:
		return "观察员"
	default:
		return "未知"
	}
}

// FormatUserStatus 格式化用户状态显示
func FormatUserStatus(status models.UserStatus) string {
	switch status {
	case models.StatusActive:
		return "活跃"
	case models.StatusInactive:
		return "非活跃"
	case models.StatusBlocked:
		return "已封禁"
	default:
		return "未知"
	}
}

// FormatDroneStatus 格式化无人机状态显示
func FormatDroneStatus(status models.DroneStatus) string {
	switch status {
	case models.DroneStatusOffline:
		return "离线"
	case models.DroneStatusOnline:
		return "在线"
	case models.DroneStatusFlying:
		return "飞行中"
	case models.DroneStatusCharging:
		return "充电中"
	case models.DroneStatusMaintenance:
		return "维护中"
	case models.DroneStatusError:
		return "故障"
	default:
		return "未知"
	}
}

// FormatTaskStatus 格式化任务状态显示
func FormatTaskStatus(status models.TaskStatus) string {
	switch status {
	case models.TaskStatusPending:
		return "待处理"
	case models.TaskStatusScheduled:
		return "已调度"
	case models.TaskStatusRunning:
		return "执行中"
	case models.TaskStatusCompleted:
		return "已完成"
	case models.TaskStatusFailed:
		return "失败"
	case models.TaskStatusCancelled:
		return "已取消"
	default:
		return "未知"
	}
}

// FormatAlertLevel 格式化告警级别显示
func FormatAlertLevel(level models.AlertLevel) string {
	switch level {
	case models.AlertLevelInfo:
		return "信息"
	case models.AlertLevelWarning:
		return "警告"
	case models.AlertLevelError:
		return "错误"
	case models.AlertLevelCritical:
		return "严重"
	default:
		return "未知"
	}
}

// SanitizeEmail 脱敏邮箱地址
func SanitizeEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		return email
	}

	// 保留前2位和后1位，中间用*替代
	sanitized := username[:2] + strings.Repeat("*", len(username)-3) + username[len(username)-1:]
	return sanitized + "@" + domain
}

// TruncateString 截断字符串
func TruncateString(str string, maxLength int) string {
	if len(str) <= maxLength {
		return str
	}
	return str[:maxLength-3] + "..."
}
