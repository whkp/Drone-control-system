package middleware

import (
	"net/http"
	"strings"

	"drone-control-system/internal/mvc/models"
	"drone-control-system/internal/mvc/services"
	"drone-control-system/pkg/logger"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware JWT认证中间件
type AuthMiddleware struct {
	userService services.UserService
	logger      *logger.Logger
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(userService services.UserService, logger *logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		userService: userService,
		logger:      logger,
	}
}

// RequireAuth 需要认证的中间件
func (am *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := am.extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "authentication required",
			})
			c.Abort()
			return
		}

		user, err := am.userService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			am.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
				"token": token[:10] + "...", // 只记录token前10位
			}).Warn("Token validation failed")

			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", user.ID)
		c.Set("user_role", user.Role)
		c.Set("user", user)

		c.Next()
	}
}

// RequireRole 需要特定角色的中间件
func (am *AuthMiddleware) RequireRole(requiredRole models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "authentication required",
			})
			c.Abort()
			return
		}

		role, ok := userRole.(models.UserRole)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    http.StatusInternalServerError,
				"message": "invalid user role",
			})
			c.Abort()
			return
		}

		// 权限级别检查
		roleLevel := map[models.UserRole]int{
			models.RoleAdmin:    3,
			models.RoleOperator: 2,
			models.RoleViewer:   1,
		}

		userLevel := roleLevel[role]
		requiredLevel := roleLevel[requiredRole]

		if userLevel < requiredLevel {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    http.StatusForbidden,
				"message": "insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuth 可选认证的中间件
func (am *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := am.extractToken(c)
		if token == "" {
			c.Next()
			return
		}

		user, err := am.userService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			// 可选认证失败时不阻止请求，但记录日志
			am.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Debug("Optional auth failed")
			c.Next()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", user.ID)
		c.Set("user_role", user.Role)
		c.Set("user", user)

		c.Next()
	}
}

// extractToken 从请求中提取token
func (am *AuthMiddleware) extractToken(c *gin.Context) string {
	// 从Authorization header提取
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// 从查询参数提取
	token := c.Query("token")
	if token != "" {
		return token
	}

	// 从Cookie提取
	cookie, err := c.Cookie("auth_token")
	if err == nil && cookie != "" {
		return cookie
	}

	return ""
}
