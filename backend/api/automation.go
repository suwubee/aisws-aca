package api

import (
	"github.com/gofiber/fiber/v2"
)

// AutomationController 自动化控制器（预留接口）
type AutomationController struct{}

func NewAutomationController() *AutomationController {
	return &AutomationController{}
}

// AnalyzeRequest 分析请求
type AnalyzeRequest struct {
	TerminalID string                 `json:"terminal_id"`
	RecentLogs string                 `json:"recent_logs"`
	Context    map[string]interface{} `json:"context"`
}

// AnalyzeResponse 分析响应
type AnalyzeResponse struct {
	Action      string  `json:"action"`      // approve, reject, input, wait
	InputCmd    string  `json:"input_cmd"`   // 输入命令
	Confidence  float64 `json:"confidence"`  // 置信度
	Reasoning   string  `json:"reasoning"`   // 推理说明
}

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	TerminalID string `json:"terminal_id"`
	Action     string `json:"action"`
	Input      string `json:"input"`
}

// AutomationConfig 自动化配置
type AutomationConfig struct {
	Enabled         bool     `json:"enabled"`
	AutoApprove     bool     `json:"auto_approve"`
	ApprovePatterns []string `json:"approve_patterns"`
	RejectPatterns  []string `json:"reject_patterns"`
	AIProvider      string   `json:"ai_provider"` // openai, anthropic, etc.
	AIModel         string   `json:"ai_model"`
	AIAPIKey        string   `json:"ai_api_key,omitempty"`
}

var automationConfig = AutomationConfig{
	Enabled:     false,
	AutoApprove: false,
	ApprovePatterns: []string{
		"allow read",
		"allow write",
		"create file",
	},
	RejectPatterns: []string{
		"delete",
		"remove",
		"rm -rf",
	},
	AIProvider: "openai",
	AIModel:    "gpt-4",
}

// Analyze 分析日志并返回建议（预留接口）
func (ctrl *AutomationController) Analyze(c *fiber.Ctx) error {
	var req AnalyzeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// TODO: 实现AI分析逻辑
	// 目前返回默认响应
	response := AnalyzeResponse{
		Action:     "wait",
		InputCmd:   "",
		Confidence: 0.0,
		Reasoning:  "AI analysis not implemented yet. Please configure AI provider.",
	}

	// 简单的模式匹配（示例）
	for _, pattern := range automationConfig.ApprovePatterns {
		if containsPattern(req.RecentLogs, pattern) {
			response.Action = "approve"
			response.InputCmd = "yes"
			response.Confidence = 0.7
			response.Reasoning = "Matched approve pattern: " + pattern
			break
		}
	}

	for _, pattern := range automationConfig.RejectPatterns {
		if containsPattern(req.RecentLogs, pattern) {
			response.Action = "reject"
			response.InputCmd = "no"
			response.Confidence = 0.8
			response.Reasoning = "Matched reject pattern: " + pattern
			break
		}
	}

	return c.JSON(response)
}

// Execute 执行自动化命令（预留接口）
func (ctrl *AutomationController) Execute(c *fiber.Ctx) error {
	var req ExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// TODO: 实现执行逻辑，将命令发送到终端
	return c.JSON(fiber.Map{
		"message": "Execution requested",
		"action":  req.Action,
		"input":   req.Input,
	})
}

// GetConfig 获取自动化配置
func (ctrl *AutomationController) GetConfig(c *fiber.Ctx) error {
	// 不返回API Key
	config := automationConfig
	config.AIAPIKey = ""
	if automationConfig.AIAPIKey != "" {
		config.AIAPIKey = "********"
	}
	return c.JSON(config)
}

// UpdateConfig 更新自动化配置
func (ctrl *AutomationController) UpdateConfig(c *fiber.Ctx) error {
	var req AutomationConfig
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 更新配置（保留原有API Key如果没有提供新的）
	if req.AIAPIKey == "" || req.AIAPIKey == "********" {
		req.AIAPIKey = automationConfig.AIAPIKey
	}
	automationConfig = req

	return c.JSON(fiber.Map{"message": "Configuration updated"})
}

func containsPattern(s, pattern string) bool {
	return len(s) >= len(pattern) && findSubstring(s, pattern)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RegisterRoutes 注册路由
func (ctrl *AutomationController) RegisterRoutes(app *fiber.App) {
	automation := app.Group("/api/automation")
	automation.Post("/analyze", ctrl.Analyze)
	automation.Post("/execute", ctrl.Execute)
	automation.Get("/config", ctrl.GetConfig)
	automation.Put("/config", ctrl.UpdateConfig)
}
