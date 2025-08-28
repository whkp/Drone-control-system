package controllers

import (
	"net/http"
	"strconv"
	"time"

	"drone-control-system/internal/mvc/models"
	"drone-control-system/pkg/logger"

	"github.com/gin-gonic/gin"
)

// BaseController 基础控制器
type BaseController struct {
	Logger *logger.Logger
}

// NewBaseController 创建基础控制器
func NewBaseController(logger *logger.Logger) *BaseController {
	return &BaseController{
		Logger: logger,
	}
}

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Time    int64       `json:"time"`
}

// Success 成功响应
func (bc *BaseController) Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
		Time:    time.Now().Unix(),
	})
}

// Error 错误响应
func (bc *BaseController) Error(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Code:    code,
		Message: message,
		Time:    time.Now().Unix(),
	})
}

// BadRequest 400错误
func (bc *BaseController) BadRequest(c *gin.Context, message string) {
	bc.Error(c, http.StatusBadRequest, message)
}

// Unauthorized 401错误
func (bc *BaseController) Unauthorized(c *gin.Context, message string) {
	bc.Error(c, http.StatusUnauthorized, message)
}

// Forbidden 403错误
func (bc *BaseController) Forbidden(c *gin.Context, message string) {
	bc.Error(c, http.StatusForbidden, message)
}

// NotFound 404错误
func (bc *BaseController) NotFound(c *gin.Context, message string) {
	bc.Error(c, http.StatusNotFound, message)
}

// InternalError 500错误
func (bc *BaseController) InternalError(c *gin.Context, message string) {
	bc.Error(c, http.StatusInternalServerError, message)
}

// GetUserID 从上下文获取用户ID
func (bc *BaseController) GetUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, ErrUserIDNotFound
	}

	switch v := userID.(type) {
	case uint:
		return v, nil
	case int:
		return uint(v), nil
	case string:
		id, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint(id), nil
	default:
		return 0, ErrInvalidUserID
	}
}

// GetUserRole 从上下文获取用户角色
func (bc *BaseController) GetUserRole(c *gin.Context) (models.UserRole, error) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", ErrUserRoleNotFound
	}

	if userRole, ok := role.(models.UserRole); ok {
		return userRole, nil
	}

	if roleStr, ok := role.(string); ok {
		return models.UserRole(roleStr), nil
	}

	return "", ErrInvalidUserRole
}

// ParseID 解析路径参数中的ID
func (bc *BaseController) ParseID(c *gin.Context, param string) (uint, error) {
	idStr := c.Param(param)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

// ParsePagination 解析分页参数
func (bc *BaseController) ParsePagination(c *gin.Context) (offset, limit int) {
	pageStr := c.DefaultQuery("page", "1")
	sizeStr := c.DefaultQuery("size", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		size = 20
	}

	offset = (page - 1) * size
	limit = size

	return offset, limit
}

// BindJSON 绑定JSON数据
func (bc *BaseController) BindJSON(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		bc.BadRequest(c, "invalid request data: "+err.Error())
		return err
	}
	return nil
}

// ValidateRequired 验证必填字段
func (bc *BaseController) ValidateRequired(c *gin.Context, fieldName string, value interface{}) bool {
	if value == nil {
		bc.BadRequest(c, fieldName+" is required")
		return false
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			bc.BadRequest(c, fieldName+" is required")
			return false
		}
	case *string:
		if v == nil || *v == "" {
			bc.BadRequest(c, fieldName+" is required")
			return false
		}
	}

	return true
}

// CheckPermission 检查权限
func (bc *BaseController) CheckPermission(c *gin.Context, requiredRole models.UserRole) bool {
	userRole, err := bc.GetUserRole(c)
	if err != nil {
		bc.Unauthorized(c, "authentication required")
		return false
	}

	// 权限级别: admin > operator > viewer
	roleLevel := map[models.UserRole]int{
		models.RoleAdmin:    3,
		models.RoleOperator: 2,
		models.RoleViewer:   1,
	}

	userLevel := roleLevel[userRole]
	requiredLevel := roleLevel[requiredRole]

	if userLevel < requiredLevel {
		bc.Forbidden(c, "insufficient permissions")
		return false
	}

	return true
}

// LogError 记录错误日志
func (bc *BaseController) LogError(operation string, err error, context map[string]interface{}) {
	fields := map[string]interface{}{
		"operation": operation,
		"error":     err.Error(),
	}

	for k, v := range context {
		fields[k] = v
	}

	bc.Logger.WithFields(fields).Error("Controller operation failed")
}

// LogInfo 记录信息日志
func (bc *BaseController) LogInfo(operation string, context map[string]interface{}) {
	fields := map[string]interface{}{
		"operation": operation,
	}

	for k, v := range context {
		fields[k] = v
	}

	bc.Logger.WithFields(fields).Info("Controller operation completed")
}
