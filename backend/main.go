package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ai-coding-assistant/api"
	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	sshservice "github.com/ai-coding-assistant/service/ssh"
	"github.com/ai-coding-assistant/service/task"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/service/workflow"
	"github.com/ai-coding-assistant/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
	"go.uber.org/zap"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志
	if err := utils.InitLogger(cfg.Log.Level, cfg.Log.File); err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	// 确保数据目录存在
	dataDir := filepath.Dir(cfg.Database.DSN)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		utils.Error("Failed to create data directory", zap.Error(err))
		log.Fatal(err)
	}

	// 初始化数据库
	if err := model.InitDB(cfg.Database.DSN); err != nil {
		utils.Error("Failed to initialize database", zap.Error(err))
		log.Fatal(err)
	}
	utils.Info("Database initialized", zap.String("dsn", cfg.Database.DSN))

	// 创建终端管理器
	terminalManager := terminal.NewManager(&cfg.Terminal)

	// 创建Fiber应用
	app := fiber.New(fiber.Config{
		AppName:      "AI Coding Assistant",
		ServerHeader: "ACA",
	})

	// 中间件
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// WebSocket升级检查
	app.Use("/api/terminal/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// API路由组
	apiGroup := app.Group("/api")

	// 健康检查（不需要认证）
	apiGroup.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "1.0.0",
		})
	})

	// 认证API - login不需要认证
	authController := api.NewAuthController(&cfg.Auth)
	authController.SetTerminalManager(terminalManager)
	app.Post("/api/auth/login", authController.Login)

	// 需要认证的API
	apiGroup.Use(middleware.AuthMiddleware(&cfg.Auth))

	// 认证相关的需要认证的路由
	apiGroup.Post("/auth/logout", authController.Logout)
	apiGroup.Get("/auth/me", authController.Me)
	apiGroup.Post("/auth/change-password", authController.ChangePassword)
	apiGroup.Post("/auth/reset-data", middleware.RequireRole("admin"), authController.ResetData)
	apiGroup.Post("/auth/register", middleware.RequireRole("admin"), authController.Register)

	// 用户管理API（仅管理员）
	userController := api.NewUserController()
	userController.RegisterRoutes(apiGroup.Group("", middleware.RequireRole("admin")))

	// 终端API (注册在apiGroup上，继承auth中间件)
	terminalController := api.NewTerminalController(terminalManager)
	terminalController.RegisterRoutes(apiGroup)
	terminalController.RegisterWebSocket(app) // WebSocket单独注册，不需要auth

	// 任务API
	taskController := api.NewTaskController(terminalManager)
	taskController.RegisterRoutes(apiGroup)

	// 项目API
	projectController := api.NewProjectController()
	projectController.RegisterRoutes(apiGroup)

	// 工作流API
	workflowController := api.NewWorkflowController(cfg.Auth.JWTSecret, terminalManager)
	workflowController.RegisterRoutes(apiGroup)

	// 工作流模板API
	workflowTemplateController := api.NewWorkflowTemplateController()
	workflowTemplateController.RegisterRoutes(apiGroup)

	// 评论API
	commentController := api.NewCommentController()
	commentController.RegisterRoutes(apiGroup)

	// 自动化API
	automationController := api.NewAutomationController(terminalManager)
	automationController.RegisterRoutes(apiGroup)

	// 日志导出API
	logExportController := api.NewLogExportController()
	logExportController.RegisterRoutes(apiGroup)

	// 凭据管理API（仅管理员）
	secretController := api.NewSecretController(cfg.Auth.JWTSecret)
	secretController.RegisterRoutes(apiGroup.Group("", middleware.RequireRole("admin")))

	// SSH服务器管理API（仅管理员）
	sshServerController := api.NewSSHServerController(cfg.Auth.JWTSecret, terminalManager)
	sshServerController.RegisterRoutes(apiGroup.Group("", middleware.RequireRole("admin")))

	// 提示词模板配置（系统设置：各模块读取提示词模板，不再硬编码）
	promptTemplateController := api.NewPromptTemplateController()
	promptTemplateController.RegisterRoutes(apiGroup)

	// AI工作流API
	sshManager := sshservice.NewSSHManager(cfg.Auth.JWTSecret)
	automationService := task.NewAutomationService(terminalManager)
	toolExecutor := workflow.NewToolExecutor(sshManager, automationService, workflow.NewTerminalManagerAdapter(terminalManager))
	api.InitAIWorkflowEngine(toolExecutor)
	aiWorkflowGroup := apiGroup.Group("/ai-workflow")
	aiWorkflowGroup.Post("/start", api.StartAIWorkflow)
	aiWorkflowGroup.Get("/session/:id", api.GetAIWorkflowSession)
	aiWorkflowGroup.Post("/session/:id/message", api.PostAIWorkflowMessage)
	aiWorkflowGroup.Get("/sessions", api.ListAIWorkflowSessions)

	// 静态文件服务
	staticFS, err := fs.Sub(staticFiles, "static")
	if err == nil {
		app.Use("/", filesystem.New(filesystem.Config{
			Root:         http.FS(staticFS),
			Index:        "index.html",
			NotFoundFile: "index.html",
		}))
	}

	// 启动服务器
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	utils.Info("Starting server", zap.String("address", addr))
	log.Printf("Server starting at http://%s\n", addr)

	if err := app.Listen(addr); err != nil {
		utils.Error("Server error", zap.Error(err))
		log.Fatal(err)
	}
}
