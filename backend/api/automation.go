package api

import (
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/approval"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// AutomationController 自动化控制器
type AutomationController struct {
	approvalEngine *approval.Engine
}

func NewAutomationController() *AutomationController {
	return &AutomationController{
		approvalEngine: approval.NewEngine(),
	}
}

// ===== AI Provider Config APIs =====

// AIProviderConfigRequest AI配置请求
type AIProviderConfigRequest struct {
	Name        string  `json:"name"`
	Provider    string  `json:"provider"`
	BaseURL     string  `json:"base_url"`
	APIKey      string  `json:"api_key"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	IsDefault   bool    `json:"is_default"`
	Enabled     bool    `json:"enabled"`
}

// ListAIProviders 获取AI配置列表
func (ctrl *AutomationController) ListAIProviders(c *fiber.Ctx) error {
	var configs []model.AIProviderConfig
	model.DB.Order("created_at desc").Find(&configs)

	// 不返回API Key
	items := make([]fiber.Map, len(configs))
	for i, cfg := range configs {
		items[i] = fiber.Map{
			"id":          cfg.ID,
			"name":        cfg.Name,
			"provider":    cfg.Provider,
			"base_url":    cfg.BaseURL,
			"api_key":     maskAPIKey(cfg.APIKey),
			"model":       cfg.Model,
			"temperature": cfg.Temperature,
			"max_tokens":  cfg.MaxTokens,
			"is_default":  cfg.IsDefault,
			"enabled":     cfg.Enabled,
			"created_at":  cfg.CreatedAt,
			"updated_at":  cfg.UpdatedAt,
		}
	}

	return c.JSON(fiber.Map{"items": items})
}

// GetAIProvider 获取单个AI配置
func (ctrl *AutomationController) GetAIProvider(c *fiber.Ctx) error {
	id := c.Params("id")
	var config model.AIProviderConfig
	if err := model.DB.First(&config, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "AI provider not found"})
	}

	return c.JSON(fiber.Map{
		"item": fiber.Map{
			"id":          config.ID,
			"name":        config.Name,
			"provider":    config.Provider,
			"base_url":    config.BaseURL,
			"api_key":     maskAPIKey(config.APIKey),
			"model":       config.Model,
			"temperature": config.Temperature,
			"max_tokens":  config.MaxTokens,
			"is_default":  config.IsDefault,
			"enabled":     config.Enabled,
			"created_at":  config.CreatedAt,
			"updated_at":  config.UpdatedAt,
		},
	})
}

// CreateAIProvider 创建AI配置
func (ctrl *AutomationController) CreateAIProvider(c *fiber.Ctx) error {
	var req AIProviderConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 如果设为默认，取消其他默认
	if req.IsDefault {
		model.DB.Model(&model.AIProviderConfig{}).Where("is_default = ?", true).Update("is_default", false)
	}

	config := &model.AIProviderConfig{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Provider:    req.Provider,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		IsDefault:   req.IsDefault,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := model.DB.Create(config).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create AI provider"})
	}

	return c.JSON(fiber.Map{
		"item": fiber.Map{
			"id":   config.ID,
			"name": config.Name,
		},
		"message": "AI provider created",
	})
}

// UpdateAIProvider 更新AI配置
func (ctrl *AutomationController) UpdateAIProvider(c *fiber.Ctx) error {
	id := c.Params("id")
	var config model.AIProviderConfig
	if err := model.DB.First(&config, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "AI provider not found"})
	}

	var req AIProviderConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 如果设为默认，取消其他默认
	if req.IsDefault && !config.IsDefault {
		model.DB.Model(&model.AIProviderConfig{}).Where("is_default = ? AND id != ?", true, id).Update("is_default", false)
	}

	// 保留原有API Key如果没有提供新的
	if req.APIKey == "" || req.APIKey == "********" {
		req.APIKey = config.APIKey
	}

	config.Name = req.Name
	config.Provider = req.Provider
	config.BaseURL = req.BaseURL
	config.APIKey = req.APIKey
	config.Model = req.Model
	config.Temperature = req.Temperature
	config.MaxTokens = req.MaxTokens
	config.IsDefault = req.IsDefault
	config.Enabled = req.Enabled
	config.UpdatedAt = time.Now()

	if err := model.DB.Save(&config).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update AI provider"})
	}

	return c.JSON(fiber.Map{"message": "AI provider updated"})
}

// DeleteAIProvider 删除AI配置
func (ctrl *AutomationController) DeleteAIProvider(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := model.DB.Delete(&model.AIProviderConfig{}, "id = ?", id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete AI provider"})
	}
	return c.JSON(fiber.Map{"message": "AI provider deleted"})
}

// ===== Terminal Automation Config APIs =====

// TerminalAutomationRequest 终端自动化配置请求
type TerminalAutomationRequest struct {
	ApprovalMode      string  `json:"approval_mode"`
	AutoInputType     string  `json:"auto_input_type"`
	WhitelistPatterns string  `json:"whitelist_patterns"`
	BlacklistPatterns string  `json:"blacklist_patterns"`
	AIProviderID      *string `json:"ai_provider_id"`
	AIPrompt          string  `json:"ai_prompt"`
	ContextLines      int     `json:"context_lines"`
	DetectClaudeCode  bool    `json:"detect_claude_code"`
	DetectCodex       bool    `json:"detect_codex"`
	DetectGemini      bool    `json:"detect_gemini"`
	NotifyOnBlock     bool    `json:"notify_on_block"`
	NotifyOnApprove   bool    `json:"notify_on_approve"`
}

// GetTerminalAutomation 获取终端自动化配置
func (ctrl *AutomationController) GetTerminalAutomation(c *fiber.Ctx) error {
	terminalID := c.Params("id")

	config, err := ctrl.approvalEngine.GetAutomationConfig(terminalID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get automation config"})
	}

	return c.JSON(fiber.Map{"item": config})
}

// UpdateTerminalAutomation 更新终端自动化配置
func (ctrl *AutomationController) UpdateTerminalAutomation(c *fiber.Ctx) error {
	terminalID := c.Params("id")

	var req TerminalAutomationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	config := &model.TerminalAutomation{
		TerminalID:        terminalID,
		ApprovalMode:      req.ApprovalMode,
		AutoInputType:     req.AutoInputType,
		WhitelistPatterns: req.WhitelistPatterns,
		BlacklistPatterns: req.BlacklistPatterns,
		AIProviderID:      req.AIProviderID,
		AIPrompt:          req.AIPrompt,
		ContextLines:      req.ContextLines,
		DetectClaudeCode:  req.DetectClaudeCode,
		DetectCodex:       req.DetectCodex,
		DetectGemini:      req.DetectGemini,
		NotifyOnBlock:     req.NotifyOnBlock,
		NotifyOnApprove:   req.NotifyOnApprove,
	}

	if err := ctrl.approvalEngine.SaveAutomationConfig(config); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save automation config"})
	}

	return c.JSON(fiber.Map{"message": "Automation config updated"})
}

// GetDefaultPatterns 获取默认的黑白名单模式
func (ctrl *AutomationController) GetDefaultPatterns(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"whitelist": approval.DefaultWhitelistPatterns,
		"blacklist": approval.DefaultBlacklistPatterns,
	})
}

// ===== Message APIs =====

// ListMessages 获取消息列表
func (ctrl *AutomationController) ListMessages(c *fiber.Ctx) error {
	status := c.Query("status")       // unread, read, handled, dismissed
	msgType := c.Query("type")        // approval_needed, blocked, info, warning, error
	terminalID := c.Query("terminal_id")
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	query := model.DB.Model(&model.Message{}).Order("created_at desc")

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if msgType != "" {
		query = query.Where("type = ?", msgType)
	}
	if terminalID != "" {
		query = query.Where("terminal_id = ?", terminalID)
	}

	var total int64
	query.Count(&total)

	var messages []model.Message
	query.Offset(offset).Limit(limit).Find(&messages)

	return c.JSON(fiber.Map{
		"items": messages,
		"total": total,
	})
}

// GetMessage 获取单条消息
func (ctrl *AutomationController) GetMessage(c *fiber.Ctx) error {
	id := c.Params("id")
	var msg model.Message
	if err := model.DB.First(&msg, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Message not found"})
	}
	return c.JSON(fiber.Map{"item": msg})
}

// MarkMessageRead 标记消息已读
func (ctrl *AutomationController) MarkMessageRead(c *fiber.Ctx) error {
	id := c.Params("id")
	now := time.Now()
	if err := model.DB.Model(&model.Message{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "read",
		"read_at":    now,
		"updated_at": now,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to mark message as read"})
	}
	return c.JSON(fiber.Map{"message": "Message marked as read"})
}

// HandleMessage 处理消息
func (ctrl *AutomationController) HandleMessage(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Action string `json:"action"` // 采取的操作描述
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	now := time.Now()
	if err := model.DB.Model(&model.Message{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       "handled",
		"action_taken": req.Action,
		"handled_at":   now,
		"updated_at":   now,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to handle message"})
	}
	return c.JSON(fiber.Map{"message": "Message handled"})
}

// DismissMessage 忽略消息
func (ctrl *AutomationController) DismissMessage(c *fiber.Ctx) error {
	id := c.Params("id")
	now := time.Now()
	if err := model.DB.Model(&model.Message{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "dismissed",
		"updated_at": now,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to dismiss message"})
	}
	return c.JSON(fiber.Map{"message": "Message dismissed"})
}

// GetUnreadCount 获取未读消息数量
func (ctrl *AutomationController) GetUnreadCount(c *fiber.Ctx) error {
	var count int64
	model.DB.Model(&model.Message{}).Where("status = ?", "unread").Count(&count)
	return c.JSON(fiber.Map{"count": count})
}

// MarkAllRead 标记所有消息已读
func (ctrl *AutomationController) MarkAllRead(c *fiber.Ctx) error {
	now := time.Now()
	if err := model.DB.Model(&model.Message{}).Where("status = ?", "unread").Updates(map[string]interface{}{
		"status":     "read",
		"read_at":    now,
		"updated_at": now,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to mark all messages as read"})
	}
	return c.JSON(fiber.Map{"message": "All messages marked as read"})
}

// ===== Approval Record APIs =====

// ListApprovalRecords 获取审批记录列表
func (ctrl *AutomationController) ListApprovalRecords(c *fiber.Ctx) error {
	terminalID := c.Query("terminal_id")
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	query := model.DB.Model(&model.ApprovalRecord{}).Order("created_at desc")

	if terminalID != "" {
		query = query.Where("terminal_id = ?", terminalID)
	}

	var total int64
	query.Count(&total)

	var records []model.ApprovalRecord
	query.Offset(offset).Limit(limit).Find(&records)

	return c.JSON(fiber.Map{
		"items": records,
		"total": total,
	})
}

// ===== Helper Functions =====

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// RegisterRoutes 注册路由
func (ctrl *AutomationController) RegisterRoutes(app *fiber.App) {
	automation := app.Group("/api/automation")

	// AI Provider配置
	automation.Get("/ai-providers", ctrl.ListAIProviders)
	automation.Post("/ai-providers", ctrl.CreateAIProvider)
	automation.Get("/ai-providers/:id", ctrl.GetAIProvider)
	automation.Put("/ai-providers/:id", ctrl.UpdateAIProvider)
	automation.Delete("/ai-providers/:id", ctrl.DeleteAIProvider)

	// 终端自动化配置
	automation.Get("/terminals/:id/config", ctrl.GetTerminalAutomation)
	automation.Put("/terminals/:id/config", ctrl.UpdateTerminalAutomation)
	automation.Get("/patterns/defaults", ctrl.GetDefaultPatterns)

	// 消息管理
	automation.Get("/messages", ctrl.ListMessages)
	automation.Get("/messages/unread-count", ctrl.GetUnreadCount)
	automation.Post("/messages/mark-all-read", ctrl.MarkAllRead)
	automation.Get("/messages/:id", ctrl.GetMessage)
	automation.Post("/messages/:id/read", ctrl.MarkMessageRead)
	automation.Post("/messages/:id/handle", ctrl.HandleMessage)
	automation.Post("/messages/:id/dismiss", ctrl.DismissMessage)

	// 审批记录
	automation.Get("/approval-records", ctrl.ListApprovalRecords)
}
