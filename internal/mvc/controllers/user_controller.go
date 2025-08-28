package controllers

import (
	"drone-control-system/internal/mvc/models"
	"drone-control-system/internal/mvc/services"
	"drone-control-system/pkg/logger"

	"github.com/gin-gonic/gin"
)

// UserController 用户控制器
type UserController struct {
	*BaseController
	userService services.UserService
}

// NewUserController 创建用户控制器
func NewUserController(logger *logger.Logger, userService services.UserService) *UserController {
	return &UserController{
		BaseController: NewBaseController(logger),
		userService:    userService,
	}
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string          `json:"username" binding:"required,min=3,max=50"`
	Email    string          `json:"email" binding:"required,email"`
	Password string          `json:"password" binding:"required,min=6"`
	Role     models.UserRole `json:"role" binding:"omitempty,oneof=admin operator viewer"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Username string            `json:"username" binding:"omitempty,min=3,max=50"`
	Email    string            `json:"email" binding:"omitempty,email"`
	Role     models.UserRole   `json:"role" binding:"omitempty,oneof=admin operator viewer"`
	Status   models.UserStatus `json:"status" binding:"omitempty,oneof=active inactive blocked"`
	Avatar   string            `json:"avatar" binding:"omitempty,url"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string       `json:"token"`
	ExpiresIn int64        `json:"expires_in"`
	User      *models.User `json:"user"`
}

// CreateUser 创建用户
func (uc *UserController) CreateUser(c *gin.Context) {
	// 检查权限 - 只有管理员可以创建用户
	if !uc.CheckPermission(c, models.RoleAdmin) {
		return
	}

	var req CreateUserRequest
	if err := uc.BindJSON(c, &req); err != nil {
		return
	}

	// 创建用户
	user, err := uc.userService.CreateUser(c.Request.Context(), &services.CreateUserParams{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		uc.LogError("CreateUser", err, map[string]interface{}{
			"username": req.Username,
			"email":    req.Email,
		})
		uc.InternalError(c, "failed to create user")
		return
	}

	uc.LogInfo("CreateUser", map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
	})

	uc.Success(c, user)
}

// GetUser 获取用户信息
func (uc *UserController) GetUser(c *gin.Context) {
	id, err := uc.ParseID(c, "id")
	if err != nil {
		uc.BadRequest(c, "invalid user ID")
		return
	}

	// 权限检查：用户只能查看自己的信息，管理员可以查看所有用户
	currentUserID, _ := uc.GetUserID(c)
	currentUserRole, _ := uc.GetUserRole(c)

	if currentUserRole != models.RoleAdmin && currentUserID != id {
		uc.Forbidden(c, "access denied")
		return
	}

	user, err := uc.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if err == services.ErrUserNotFound {
			uc.NotFound(c, "user not found")
			return
		}
		uc.LogError("GetUser", err, map[string]interface{}{"user_id": id})
		uc.InternalError(c, "failed to get user")
		return
	}

	uc.Success(c, user)
}

// UpdateUser 更新用户信息
func (uc *UserController) UpdateUser(c *gin.Context) {
	id, err := uc.ParseID(c, "id")
	if err != nil {
		uc.BadRequest(c, "invalid user ID")
		return
	}

	// 权限检查
	currentUserID, _ := uc.GetUserID(c)
	currentUserRole, _ := uc.GetUserRole(c)

	if currentUserRole != models.RoleAdmin && currentUserID != id {
		uc.Forbidden(c, "access denied")
		return
	}

	var req UpdateUserRequest
	if err := uc.BindJSON(c, &req); err != nil {
		return
	}

	// 非管理员不能修改角色和状态
	if currentUserRole != models.RoleAdmin {
		req.Role = ""
		req.Status = ""
	}

	user, err := uc.userService.UpdateUser(c.Request.Context(), id, &services.UpdateUserParams{
		Username: req.Username,
		Email:    req.Email,
		Role:     req.Role,
		Status:   req.Status,
		Avatar:   req.Avatar,
	})
	if err != nil {
		if err == services.ErrUserNotFound {
			uc.NotFound(c, "user not found")
			return
		}
		uc.LogError("UpdateUser", err, map[string]interface{}{"user_id": id})
		uc.InternalError(c, "failed to update user")
		return
	}

	uc.LogInfo("UpdateUser", map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
	})

	uc.Success(c, user)
}

// DeleteUser 删除用户
func (uc *UserController) DeleteUser(c *gin.Context) {
	// 只有管理员可以删除用户
	if !uc.CheckPermission(c, models.RoleAdmin) {
		return
	}

	id, err := uc.ParseID(c, "id")
	if err != nil {
		uc.BadRequest(c, "invalid user ID")
		return
	}

	err = uc.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		if err == services.ErrUserNotFound {
			uc.NotFound(c, "user not found")
			return
		}
		uc.LogError("DeleteUser", err, map[string]interface{}{"user_id": id})
		uc.InternalError(c, "failed to delete user")
		return
	}

	uc.LogInfo("DeleteUser", map[string]interface{}{"user_id": id})
	uc.Success(c, gin.H{"message": "user deleted successfully"})
}

// ListUsers 获取用户列表
func (uc *UserController) ListUsers(c *gin.Context) {
	// 只有管理员可以查看用户列表
	if !uc.CheckPermission(c, models.RoleAdmin) {
		return
	}

	offset, limit := uc.ParsePagination(c)

	// 可选的筛选参数
	role := c.Query("role")
	status := c.Query("status")
	search := c.Query("search")

	users, total, err := uc.userService.ListUsers(c.Request.Context(), &services.ListUsersParams{
		Offset: offset,
		Limit:  limit,
		Role:   models.UserRole(role),
		Status: models.UserStatus(status),
		Search: search,
	})
	if err != nil {
		uc.LogError("ListUsers", err, map[string]interface{}{
			"offset": offset,
			"limit":  limit,
		})
		uc.InternalError(c, "failed to list users")
		return
	}

	uc.Success(c, gin.H{
		"users":  users,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

// Login 用户登录
func (uc *UserController) Login(c *gin.Context) {
	var req LoginRequest
	if err := uc.BindJSON(c, &req); err != nil {
		return
	}

	result, err := uc.userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			uc.Unauthorized(c, "invalid username or password")
			return
		}
		uc.LogError("Login", err, map[string]interface{}{
			"username": req.Username,
		})
		uc.InternalError(c, "login failed")
		return
	}

	uc.LogInfo("Login", map[string]interface{}{
		"user_id":  result.User.ID,
		"username": result.User.Username,
	})

	uc.Success(c, LoginResponse{
		Token:     result.Token,
		ExpiresIn: result.ExpiresIn,
		User:      result.User,
	})
}

// GetProfile 获取当前用户信息
func (uc *UserController) GetProfile(c *gin.Context) {
	userID, err := uc.GetUserID(c)
	if err != nil {
		uc.Unauthorized(c, "authentication required")
		return
	}

	user, err := uc.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		uc.LogError("GetProfile", err, map[string]interface{}{"user_id": userID})
		uc.InternalError(c, "failed to get profile")
		return
	}

	uc.Success(c, user)
}

// ChangePassword 修改密码
func (uc *UserController) ChangePassword(c *gin.Context) {
	userID, err := uc.GetUserID(c)
	if err != nil {
		uc.Unauthorized(c, "authentication required")
		return
	}

	var req ChangePasswordRequest
	if err := uc.BindJSON(c, &req); err != nil {
		return
	}

	err = uc.userService.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			uc.BadRequest(c, "invalid old password")
			return
		}
		uc.LogError("ChangePassword", err, map[string]interface{}{"user_id": userID})
		uc.InternalError(c, "failed to change password")
		return
	}

	uc.LogInfo("ChangePassword", map[string]interface{}{"user_id": userID})
	uc.Success(c, gin.H{"message": "password changed successfully"})
}
