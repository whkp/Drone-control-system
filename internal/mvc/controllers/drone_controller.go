package controllers

import (
	"drone-control-system/internal/mvc/models"
	"drone-control-system/internal/mvc/services"
	"drone-control-system/pkg/kafka"
	"drone-control-system/pkg/logger"

	"github.com/gin-gonic/gin"
)

// DroneController 无人机控制器
type DroneController struct {
	*BaseController
	droneService services.DroneService
	kafkaService services.KafkaService // 添加Kafka服务
}

// NewDroneController 创建无人机控制器
func NewDroneController(logger *logger.Logger, droneService services.DroneService, kafkaService services.KafkaService) *DroneController {
	return &DroneController{
		BaseController: NewBaseController(logger),
		droneService:   droneService,
		kafkaService:   kafkaService,
	}
}

// CreateDroneRequest 创建无人机请求
type CreateDroneRequest struct {
	SerialNo     string   `json:"serial_no" binding:"required,min=3,max=50"`
	Model        string   `json:"model" binding:"required,min=2,max=100"`
	Capabilities []string `json:"capabilities"`
	Firmware     string   `json:"firmware" binding:"omitempty,max=50"`
	Version      string   `json:"version" binding:"omitempty,max=20"`
}

// UpdateDroneRequest 更新无人机请求
type UpdateDroneRequest struct {
	Model        string             `json:"model" binding:"omitempty,min=2,max=100"`
	Status       models.DroneStatus `json:"status" binding:"omitempty,oneof=offline online flying charging maintenance error"`
	Position     *models.Position   `json:"position"`
	Battery      *int               `json:"battery" binding:"omitempty,min=0,max=100"`
	Capabilities []string           `json:"capabilities"`
	Firmware     string             `json:"firmware" binding:"omitempty,max=50"`
	Version      string             `json:"version" binding:"omitempty,max=20"`
}

// UpdatePositionRequest 更新位置请求
type UpdatePositionRequest struct {
	Latitude  float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude float64 `json:"longitude" binding:"required,min=-180,max=180"`
	Altitude  float64 `json:"altitude" binding:"min=0"`
	Heading   float64 `json:"heading" binding:"min=0,max=360"`
}

// CreateDrone 创建无人机
func (dc *DroneController) CreateDrone(c *gin.Context) {
	// 检查权限 - 只有管理员和操作员可以创建无人机
	if !dc.CheckPermission(c, models.RoleOperator) {
		return
	}

	var req CreateDroneRequest
	if err := dc.BindJSON(c, &req); err != nil {
		return
	}

	drone, err := dc.droneService.CreateDrone(c.Request.Context(), &services.CreateDroneParams{
		SerialNo:     req.SerialNo,
		Model:        req.Model,
		Capabilities: req.Capabilities,
		Firmware:     req.Firmware,
		Version:      req.Version,
	})
	if err != nil {
		if err == services.ErrDroneExists {
			dc.BadRequest(c, "drone with this serial number already exists")
			return
		}
		dc.LogError("CreateDrone", err, map[string]interface{}{
			"serial_no": req.SerialNo,
		})
		dc.InternalError(c, "failed to create drone")
		return
	}

	dc.LogInfo("CreateDrone", map[string]interface{}{
		"drone_id":  drone.ID,
		"serial_no": drone.SerialNo,
	})

	dc.Success(c, drone)
}

// GetDrone 获取无人机信息
func (dc *DroneController) GetDrone(c *gin.Context) {
	id, err := dc.ParseID(c, "id")
	if err != nil {
		dc.BadRequest(c, "invalid drone ID")
		return
	}

	drone, err := dc.droneService.GetDroneByID(c.Request.Context(), id)
	if err != nil {
		if err == services.ErrDroneNotFound {
			dc.NotFound(c, "drone not found")
			return
		}
		dc.LogError("GetDrone", err, map[string]interface{}{"drone_id": id})
		dc.InternalError(c, "failed to get drone")
		return
	}

	dc.Success(c, drone)
}

// UpdateDrone 更新无人机信息
func (dc *DroneController) UpdateDrone(c *gin.Context) {
	// 检查权限
	if !dc.CheckPermission(c, models.RoleOperator) {
		return
	}

	id, err := dc.ParseID(c, "id")
	if err != nil {
		dc.BadRequest(c, "invalid drone ID")
		return
	}

	var req UpdateDroneRequest
	if err := dc.BindJSON(c, &req); err != nil {
		return
	}

	drone, err := dc.droneService.UpdateDrone(c.Request.Context(), id, &services.UpdateDroneParams{
		Model:        req.Model,
		Status:       req.Status,
		Position:     req.Position,
		Battery:      req.Battery,
		Capabilities: req.Capabilities,
		Firmware:     req.Firmware,
		Version:      req.Version,
	})
	if err != nil {
		if err == services.ErrDroneNotFound {
			dc.NotFound(c, "drone not found")
			return
		}
		dc.LogError("UpdateDrone", err, map[string]interface{}{"drone_id": id})
		dc.InternalError(c, "failed to update drone")
		return
	}

	dc.LogInfo("UpdateDrone", map[string]interface{}{
		"drone_id": drone.ID,
	})

	dc.Success(c, drone)
}

// DeleteDrone 删除无人机
func (dc *DroneController) DeleteDrone(c *gin.Context) {
	// 只有管理员可以删除无人机
	if !dc.CheckPermission(c, models.RoleAdmin) {
		return
	}

	id, err := dc.ParseID(c, "id")
	if err != nil {
		dc.BadRequest(c, "invalid drone ID")
		return
	}

	err = dc.droneService.DeleteDrone(c.Request.Context(), id)
	if err != nil {
		if err == services.ErrDroneNotFound {
			dc.NotFound(c, "drone not found")
			return
		}
		if err == services.ErrDroneInUse {
			dc.BadRequest(c, "drone is currently in use and cannot be deleted")
			return
		}
		dc.LogError("DeleteDrone", err, map[string]interface{}{"drone_id": id})
		dc.InternalError(c, "failed to delete drone")
		return
	}

	dc.LogInfo("DeleteDrone", map[string]interface{}{"drone_id": id})
	dc.Success(c, gin.H{"message": "drone deleted successfully"})
}

// ListDrones 获取无人机列表
func (dc *DroneController) ListDrones(c *gin.Context) {
	offset, limit := dc.ParsePagination(c)

	// 筛选参数
	status := c.Query("status")
	search := c.Query("search")

	drones, total, err := dc.droneService.ListDrones(c.Request.Context(), &services.ListDronesParams{
		Offset: offset,
		Limit:  limit,
		Status: models.DroneStatus(status),
		Search: search,
	})
	if err != nil {
		dc.LogError("ListDrones", err, map[string]interface{}{
			"offset": offset,
			"limit":  limit,
		})
		dc.InternalError(c, "failed to list drones")
		return
	}

	dc.Success(c, gin.H{
		"drones": drones,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

// UpdateDroneStatus 更新无人机状态
func (dc *DroneController) UpdateDroneStatus(c *gin.Context) {
	// 检查权限
	if !dc.CheckPermission(c, models.RoleOperator) {
		return
	}

	id, err := dc.ParseID(c, "id")
	if err != nil {
		dc.BadRequest(c, "invalid drone ID")
		return
	}

	var req struct {
		Status models.DroneStatus `json:"status" binding:"required,oneof=offline online flying charging maintenance error"`
	}
	if err := dc.BindJSON(c, &req); err != nil {
		return
	}

	err = dc.droneService.UpdateDroneStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		if err == services.ErrDroneNotFound {
			dc.NotFound(c, "drone not found")
			return
		}
		dc.LogError("UpdateDroneStatus", err, map[string]interface{}{
			"drone_id": id,
			"status":   req.Status,
		})
		dc.InternalError(c, "failed to update drone status")
		return
	}

	dc.LogInfo("UpdateDroneStatus", map[string]interface{}{
		"drone_id": id,
		"status":   req.Status,
	})

	dc.Success(c, gin.H{"message": "drone status updated successfully"})
}

// UpdateDronePosition 更新无人机位置
func (dc *DroneController) UpdateDronePosition(c *gin.Context) {
	id, err := dc.ParseID(c, "id")
	if err != nil {
		dc.BadRequest(c, "invalid drone ID")
		return
	}

	var req UpdatePositionRequest
	if err := dc.BindJSON(c, &req); err != nil {
		return
	}

	position := models.Position{
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Altitude:  req.Altitude,
		Heading:   req.Heading,
	}

	err = dc.droneService.UpdateDronePosition(c.Request.Context(), id, position)
	if err != nil {
		if err == services.ErrDroneNotFound {
			dc.NotFound(c, "drone not found")
			return
		}
		dc.LogError("UpdateDronePosition", err, map[string]interface{}{
			"drone_id": id,
			"position": position,
		})
		dc.InternalError(c, "failed to update drone position")
		return
	}

	// 🚀 发布位置更新事件到Kafka（异步处理，不阻塞响应）
	if dc.kafkaService != nil {
		eventData := map[string]interface{}{
			"drone_id":  id,
			"position":  position,
			"timestamp": c.Request.Context().Value("timestamp"),
		}

		// 异步发布事件，避免阻塞HTTP响应
		go func() {
			if err := dc.kafkaService.PublishDroneEvent(c.Request.Context(), kafka.DroneLocationUpdatedEvent, eventData); err != nil {
				dc.Logger.Error("Failed to publish drone location event", map[string]interface{}{
					"drone_id": id,
					"error":    err.Error(),
				})
			}
		}()
	}

	dc.Success(c, gin.H{"message": "drone position updated successfully"})
}

// UpdateDroneBattery 更新无人机电量
func (dc *DroneController) UpdateDroneBattery(c *gin.Context) {
	id, err := dc.ParseID(c, "id")
	if err != nil {
		dc.BadRequest(c, "invalid drone ID")
		return
	}

	var req struct {
		Battery int `json:"battery" binding:"required,min=0,max=100"`
	}
	if err := dc.BindJSON(c, &req); err != nil {
		return
	}

	err = dc.droneService.UpdateDroneBattery(c.Request.Context(), id, req.Battery)
	if err != nil {
		if err == services.ErrDroneNotFound {
			dc.NotFound(c, "drone not found")
			return
		}
		dc.LogError("UpdateDroneBattery", err, map[string]interface{}{
			"drone_id": id,
			"battery":  req.Battery,
		})
		dc.InternalError(c, "failed to update drone battery")
		return
	}

	dc.Success(c, gin.H{"message": "drone battery updated successfully"})
}

// GetAvailableDrones 获取可用无人机列表
func (dc *DroneController) GetAvailableDrones(c *gin.Context) {
	drones, err := dc.droneService.GetAvailableDrones(c.Request.Context())
	if err != nil {
		dc.LogError("GetAvailableDrones", err, map[string]interface{}{})
		dc.InternalError(c, "failed to get available drones")
		return
	}

	dc.Success(c, gin.H{
		"drones": drones,
		"count":  len(drones),
	})
}
