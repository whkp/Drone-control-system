package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// 设置静态文件目录
	r.Static("/static", "./web/static")

	// 加载模板
	r.LoadHTMLGlob("web/templates/*")

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 模板路由
	r.GET("/", func(c *gin.Context) {
		// 对于演示模式，不需要严格的认证检查
		// 前端JavaScript会处理token验证和重定向
		c.HTML(200, "index.html", gin.H{
			"title": "无人机控制系统",
		})
	})

	// 登录页面
	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.html", gin.H{
			"title": "用户登录 - 无人机控制系统",
		})
	})

	// 演示模式直接进入主页
	r.GET("/demo", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "无人机控制系统 - 演示模式",
		})
	})

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "web-frontend",
			"version": "1.0.0",
		})
	})

	// API 代理路由（可选，用于开发环境）
	api := r.Group("/proxy")
	{
		// 代理到 API Gateway
		api.Any("/api/*path", func(c *gin.Context) {
			// 这里可以实现简单的代理功能
			c.JSON(http.StatusOK, gin.H{
				"message": "API代理功能",
				"path":    c.Param("path"),
				"method":  c.Request.Method,
			})
		})

		// 代理到监控服务
		api.Any("/monitoring/*path", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "监控API代理功能",
				"path":    c.Param("path"),
				"method":  c.Request.Method,
			})
		})
	}

	// 获取绝对路径用于日志
	absPath, _ := filepath.Abs("./web")

	fmt.Printf(`
🚁 无人机控制系统 Web 界面启动成功！

📊 控制台地址: http://localhost:8888
📁 静态文件目录: %s/static
📄 模板目录: %s/templates

🔗 后端服务地址:
   API Gateway: http://localhost:8080
   监控服务: http://localhost:8083
   
💡 使用说明:
   1. 确保后端服务已启动
   2. 访问 http://localhost:8888 查看控制台
   3. 使用演示模式或配置真实API连接

`, absPath, absPath)

	// 启动服务
	log.Printf("Web前端服务启动在端口 8888")
	if err := r.Run(":8888"); err != nil {
		log.Fatalf("启动Web服务失败: %v", err)
	}
}
