package api

import (
	"strings"
	"time"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/approval"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AutomationController 自动化控制器
type AutomationController struct {
	approvalEngine *approval.Engine
	terminalMgr    *terminal.Manager
}

func NewAutomationController(terminalMgr *terminal.Manager) *AutomationController {
	return &AutomationController{
		approvalEngine: approval.NewEngine(),
		terminalMgr:    terminalMgr,
	}
}

func (ctrl *AutomationController) refreshAutomationForTerminal(terminalID string) {
	if ctrl.terminalMgr == nil {
		return
	}
	session := ctrl.terminalMgr.GetSession(terminalID)
	if session == nil {
		return
	}
	_ = session.RefreshAutomationConfig()
	session.ReevaluateApprovalIfWaiting()
}

func (ctrl *AutomationController) refreshAutomationForAllSessions() {
	if ctrl.terminalMgr == nil {
		return
	}
	for _, session := range ctrl.terminalMgr.ListAllSessions() {
		_ = session.RefreshAutomationConfig()
		session.ReevaluateApprovalIfWaiting()
	}
}

// ===== RuleSet APIs =====

// RuleSetRequest 规则集请求
type RuleSetRequest struct {
	Name              string   `json:"name"`
	ApprovalMode      string   `json:"approval_mode"`
	AutoInputType     string   `json:"auto_input_type"`
	WhitelistPatterns []string `json:"whitelist_patterns"`
	BlacklistPatterns []string `json:"blacklist_patterns"`
	AIProviderID      *string  `json:"ai_provider_id"`
	AIPrompt          string   `json:"ai_prompt"`
	ContextLines      int      `json:"context_lines"`
	DetectClaudeCode  bool     `json:"detect_claude_code"`
	DetectCodex       bool     `json:"detect_codex"`
	DetectGemini      bool     `json:"detect_gemini"`
	NotifyOnBlock     bool     `json:"notify_on_block"`
	NotifyOnApprove   bool     `json:"notify_on_approve"`
}

// GetSystemRuleSet 获取系统规则集
func (ctrl *AutomationController) GetSystemRuleSet(c *fiber.Ctx) error {
	var ruleSet model.RuleSet
	result := model.DB.First(&ruleSet, "type = ?", "system")

	if result.Error != nil {
		// 自动创建默认系统规则
		ruleSet = model.RuleSet{
			ID:               "system-default",
			Name:             "系统默认规则",
			Type:             "system",
			ApprovalMode:     "manual",
			AutoInputType:    "yes",
			ContextLines:     50,
			DetectClaudeCode: true,
			DetectCodex:      true,
			DetectGemini:     true,
			NotifyOnBlock:    true,
			NotifyOnApprove:  false,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		model.DB.Create(&ruleSet)
	}

	return c.JSON(fiber.Map{"item": ruleSet})
}

// UpdateSystemRuleSet 更新系统规则集
func (ctrl *AutomationController) UpdateSystemRuleSet(c *fiber.Ctx) error {
	var req RuleSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 转换数组为JSON字符串
	whitelistJSON := toJSONArray(req.WhitelistPatterns)
	blacklistJSON := toJSONArray(req.BlacklistPatterns)

	now := time.Now()

	// 查找或创建系统规则
	var ruleSet model.RuleSet
	result := model.DB.First(&ruleSet, "type = ?", "system")

	if result.Error != nil {
		ruleSet = model.RuleSet{
			ID:        "system-default",
			Name:      "系统默认规则",
			Type:      "system",
			CreatedAt: now,
		}
	}

	ruleSet.Name = req.Name
	if ruleSet.Name == "" {
		ruleSet.Name = "系统默认规则"
	}
	ruleSet.ApprovalMode = req.ApprovalMode
	ruleSet.AutoInputType = req.AutoInputType
	ruleSet.WhitelistPatterns = whitelistJSON
	ruleSet.BlacklistPatterns = blacklistJSON
	ruleSet.AIProviderID = req.AIProviderID
	ruleSet.AIPrompt = req.AIPrompt
	ruleSet.ContextLines = req.ContextLines
	ruleSet.DetectClaudeCode = req.DetectClaudeCode
	ruleSet.DetectCodex = req.DetectCodex
	ruleSet.DetectGemini = req.DetectGemini
	ruleSet.NotifyOnBlock = req.NotifyOnBlock
	ruleSet.NotifyOnApprove = req.NotifyOnApprove
	ruleSet.UpdatedAt = now

	if result.Error != nil {
		model.DB.Create(&ruleSet)
	} else {
		model.DB.Save(&ruleSet)
	}

	ctrl.refreshAutomationForAllSessions()

	return c.JSON(fiber.Map{"message": "System rule set updated", "item": ruleSet})
}

// ListRuleSets 获取规则集列表
func (ctrl *AutomationController) ListRuleSets(c *fiber.Ctx) error {
	ruleType := c.Query("type") // system, task, terminal

	query := model.DB.Model(&model.RuleSet{}).Order("created_at desc")
	if ruleType != "" {
		query = query.Where("type = ?", ruleType)
	}

	var ruleSets []model.RuleSet
	query.Find(&ruleSets)

	return c.JSON(fiber.Map{"items": ruleSets})
}

// GetRuleSet 获取单个规则集
func (ctrl *AutomationController) GetRuleSet(c *fiber.Ctx) error {
	id := c.Params("id")
	var ruleSet model.RuleSet
	if err := model.DB.First(&ruleSet, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Rule set not found"})
	}
	return c.JSON(fiber.Map{"item": ruleSet})
}

// CreateRuleSet 创建规则集
func (ctrl *AutomationController) CreateRuleSet(c *fiber.Ctx) error {
	var req RuleSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	ruleType := c.Query("type", "terminal") // default to terminal
	ruleType = strings.TrimSpace(ruleType)
	switch ruleType {
	case "terminal", "task":
		// allowed
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Invalid rule set type"})
	}

	now := time.Now()
	ruleSet := &model.RuleSet{
		ID:                uuid.New().String(),
		Name:              req.Name,
		Type:              ruleType,
		ApprovalMode:      req.ApprovalMode,
		AutoInputType:     req.AutoInputType,
		WhitelistPatterns: toJSONArray(req.WhitelistPatterns),
		BlacklistPatterns: toJSONArray(req.BlacklistPatterns),
		AIProviderID:      req.AIProviderID,
		AIPrompt:          req.AIPrompt,
		ContextLines:      req.ContextLines,
		DetectClaudeCode:  req.DetectClaudeCode,
		DetectCodex:       req.DetectCodex,
		DetectGemini:      req.DetectGemini,
		NotifyOnBlock:     req.NotifyOnBlock,
		NotifyOnApprove:   req.NotifyOnApprove,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := model.DB.Create(ruleSet).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create rule set"})
	}

	return c.JSON(fiber.Map{"item": ruleSet, "message": "Rule set created"})
}

// UpdateRuleSet 更新规则集
func (ctrl *AutomationController) UpdateRuleSet(c *fiber.Ctx) error {
	id := c.Params("id")
	var ruleSet model.RuleSet
	if err := model.DB.First(&ruleSet, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Rule set not found"})
	}
	if ruleSet.Type == "system" {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot update system rule set"})
	}

	var req RuleSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	ruleSet.Name = req.Name
	ruleSet.ApprovalMode = req.ApprovalMode
	ruleSet.AutoInputType = req.AutoInputType
	ruleSet.WhitelistPatterns = toJSONArray(req.WhitelistPatterns)
	ruleSet.BlacklistPatterns = toJSONArray(req.BlacklistPatterns)
	ruleSet.AIProviderID = req.AIProviderID
	ruleSet.AIPrompt = req.AIPrompt
	ruleSet.ContextLines = req.ContextLines
	ruleSet.DetectClaudeCode = req.DetectClaudeCode
	ruleSet.DetectCodex = req.DetectCodex
	ruleSet.DetectGemini = req.DetectGemini
	ruleSet.NotifyOnBlock = req.NotifyOnBlock
	ruleSet.NotifyOnApprove = req.NotifyOnApprove
	ruleSet.UpdatedAt = time.Now()

	if err := model.DB.Save(&ruleSet).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update rule set"})
	}

	ctrl.refreshAutomationForAllSessions()

	return c.JSON(fiber.Map{"message": "Rule set updated"})
}

// DeleteRuleSet 删除规则集
func (ctrl *AutomationController) DeleteRuleSet(c *fiber.Ctx) error {
	id := c.Params("id")

	// 不允许删除系统规则
	var ruleSet model.RuleSet
	if err := model.DB.First(&ruleSet, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Rule set not found"})
	}
	if ruleSet.Type == "system" {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot delete system rule set"})
	}

	if err := model.DB.Delete(&model.RuleSet{}, "id = ?", id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete rule set"})
	}
	return c.JSON(fiber.Map{"message": "Rule set deleted"})
}

// ===== Terminal Rule Mode APIs =====

// TerminalRuleModeRequest 终端规则模式请求
type TerminalRuleModeRequest struct {
	RuleMode  string  `json:"rule_mode"`   // none, system, task, custom
	RuleSetID *string `json:"rule_set_id"` // custom 模式时需要指定规则集ID
}

// GetTerminalRuleMode 获取终端规则模式
func (ctrl *AutomationController) GetTerminalRuleMode(c *fiber.Ctx) error {
	terminalID := c.Params("id")

	var terminal model.TerminalSession
	if err := model.DB.First(&terminal, "id = ?", terminalID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
	}

	// 获取关联的规则集信息
	var ruleSet *model.RuleSet
	var effectiveRuleSet *model.RuleSet

	switch terminal.RuleMode {
	case "system":
		var sysRule model.RuleSet
		if model.DB.First(&sysRule, "type = ?", "system").Error == nil {
			effectiveRuleSet = &sysRule
		}
	case "task":
		if terminal.TaskID != nil {
			var task model.Task
			if model.DB.First(&task, "id = ?", *terminal.TaskID).Error == nil && task.RuleSetID != nil {
				var taskRule model.RuleSet
				if model.DB.First(&taskRule, "id = ?", *task.RuleSetID).Error == nil {
					effectiveRuleSet = &taskRule
				}
			}
		}
	case "custom":
		if terminal.RuleSetID != nil {
			var customRule model.RuleSet
			if model.DB.First(&customRule, "id = ?", *terminal.RuleSetID).Error == nil {
				ruleSet = &customRule
				effectiveRuleSet = &customRule
			}
		}
	}

	return c.JSON(fiber.Map{
		"rule_mode":          terminal.RuleMode,
		"rule_set_id":        terminal.RuleSetID,
		"rule_set":           ruleSet,
		"effective_rule_set": effectiveRuleSet,
		"task_id":            terminal.TaskID,
	})
}

// UpdateTerminalRuleMode 更新终端规则模式
func (ctrl *AutomationController) UpdateTerminalRuleMode(c *fiber.Ctx) error {
	terminalID := c.Params("id")

	var terminal model.TerminalSession
	if err := model.DB.First(&terminal, "id = ?", terminalID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
	}

	var req TerminalRuleModeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 验证规则模式
	validModes := map[string]bool{"none": true, "system": true, "task": true, "custom": true}
	if !validModes[req.RuleMode] {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid rule mode"})
	}

	// custom 模式需要验证规则集存在
	if req.RuleMode == "custom" && req.RuleSetID != nil {
		var ruleSet model.RuleSet
		if err := model.DB.First(&ruleSet, "id = ?", *req.RuleSetID).Error; err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Rule set not found"})
		}
	}

	terminal.RuleMode = req.RuleMode
	terminal.RuleSetID = req.RuleSetID

	if err := model.DB.Save(&terminal).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update terminal"})
	}

	ctrl.refreshAutomationForTerminal(terminalID)

	return c.JSON(fiber.Map{"message": "Terminal rule mode updated"})
}

// CreateTerminalCustomRule 为终端创建自定义规则
func (ctrl *AutomationController) CreateTerminalCustomRule(c *fiber.Ctx) error {
	terminalID := c.Params("id")

	var terminal model.TerminalSession
	if err := model.DB.First(&terminal, "id = ?", terminalID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
	}

	var req RuleSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	now := time.Now()
	ruleSet := &model.RuleSet{
		ID:                uuid.New().String(),
		Name:              req.Name,
		Type:              "terminal",
		ApprovalMode:      req.ApprovalMode,
		AutoInputType:     req.AutoInputType,
		WhitelistPatterns: toJSONArray(req.WhitelistPatterns),
		BlacklistPatterns: toJSONArray(req.BlacklistPatterns),
		AIProviderID:      req.AIProviderID,
		AIPrompt:          req.AIPrompt,
		ContextLines:      req.ContextLines,
		DetectClaudeCode:  req.DetectClaudeCode,
		DetectCodex:       req.DetectCodex,
		DetectGemini:      req.DetectGemini,
		NotifyOnBlock:     req.NotifyOnBlock,
		NotifyOnApprove:   req.NotifyOnApprove,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := model.DB.Create(ruleSet).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create rule set"})
	}

	// 更新终端关联
	terminal.RuleMode = "custom"
	terminal.RuleSetID = &ruleSet.ID
	model.DB.Save(&terminal)

	ctrl.refreshAutomationForTerminal(terminalID)

	return c.JSON(fiber.Map{"item": ruleSet, "message": "Custom rule created for terminal"})
}

// GetDefaultPatterns 获取默认的黑白名单模式
func (ctrl *AutomationController) GetDefaultPatterns(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"whitelist": approval.DefaultWhitelistPatterns,
		"blacklist": approval.DefaultBlacklistPatterns,
	})
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

// ===== Agent Config APIs =====

type UpdateAgentConfigsRequest struct {
	Items []model.AgentConfig `json:"items"`
}

func normalizeStringSlice(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// ListAgentConfigs 获取代理配置列表
func (ctrl *AutomationController) ListAgentConfigs(c *fiber.Ctx) error {
	var configs []model.AgentConfig
	if err := model.DB.Order("priority desc").Find(&configs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list agent configs"})
	}

	return c.JSON(fiber.Map{"items": configs})
}

// UpdateAgentConfigs 更新代理配置
func (ctrl *AutomationController) UpdateAgentConfigs(c *fiber.Ctx) error {
	var req UpdateAgentConfigsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	items := req.Items
	normalized := make([]model.AgentConfig, 0, len(items))
	seenTypes := make(map[string]struct{}, len(items))

	for _, item := range items {
		agentType := strings.TrimSpace(item.AgentType)
		if agentType == "" {
			return c.Status(400).JSON(fiber.Map{"error": "agent_type is required"})
		}
		if _, ok := seenTypes[agentType]; ok {
			return c.Status(400).JSON(fiber.Map{"error": "Duplicate agent_type: " + agentType})
		}
		seenTypes[agentType] = struct{}{}

		displayName := strings.TrimSpace(item.DisplayName)
		detectModes := normalizeStringSlice([]string(item.DetectModes))

		normalized = append(normalized, model.AgentConfig{
			AgentType:   agentType,
			DisplayName: displayName,
			Enabled:     item.Enabled,
			Priority:    item.Priority,
			DetectModes: model.StringArray(detectModes),
		})
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM agent_configs").Error; err != nil {
			return err
		}
		if len(normalized) == 0 {
			return nil
		}
		return tx.Create(&normalized).Error
	}); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update agent configs"})
	}

	return c.JSON(fiber.Map{"message": "Agent configs updated"})
}

// ===== Message APIs =====

// ListMessages 获取消息列表
func (ctrl *AutomationController) ListMessages(c *fiber.Ctx) error {
	status := c.Query("status") // unread, read, handled, dismissed
	msgType := c.Query("type")  // approval_needed, blocked, info, warning, error
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

// ===== Login Record APIs =====

// ListLoginRecords 获取登录记录列表（管理员）
func (ctrl *AutomationController) ListLoginRecords(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	keyword := strings.TrimSpace(c.Query("keyword"))
	userID := strings.TrimSpace(c.Query("user_id"))
	successRaw := strings.TrimSpace(c.Query("success"))

	query := model.DB.Model(&model.LoginRecord{}).Order("created_at desc")

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if successRaw != "" {
		v := strings.ToLower(successRaw)
		success := v == "true" || v == "1" || v == "yes" || v == "y"
		query = query.Where("success = ?", success)
	}

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"identifier LIKE ? OR username LIKE ? OR ip LIKE ?",
			like, like, like,
		)
	}

	var total int64
	query.Count(&total)

	var records []model.LoginRecord
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

func toJSONArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	result := "["
	for i, s := range arr {
		if i > 0 {
			result += ","
		}
		// 简单转义双引号
		escaped := ""
		for _, c := range s {
			if c == '"' {
				escaped += "\\\""
			} else if c == '\\' {
				escaped += "\\\\"
			} else {
				escaped += string(c)
			}
		}
		result += "\"" + escaped + "\""
	}
	result += "]"
	return result
}

// RegisterRoutes 注册路由
func (ctrl *AutomationController) RegisterRoutes(app fiber.Router) {
	// 规则集导入/导出（系统级配置：仅管理员）
	app.Get("/rule-sets/export", middleware.RequireRole("admin"), ctrl.ExportRuleSets)
	app.Post("/rule-sets/import", middleware.RequireRole("admin"), ctrl.ImportRuleSets)

	automation := app.Group("/automation")

	// 系统规则
	automation.Get("/system-rule", ctrl.GetSystemRuleSet)
	automation.Put("/system-rule", middleware.RequireRole("admin"), ctrl.UpdateSystemRuleSet)

	// 规则集 CRUD
	automation.Get("/rulesets", ctrl.ListRuleSets)
	automation.Post("/rulesets", ctrl.CreateRuleSet)
	automation.Get("/rulesets/:id", ctrl.GetRuleSet)
	automation.Put("/rulesets/:id", ctrl.UpdateRuleSet)
	automation.Delete("/rulesets/:id", ctrl.DeleteRuleSet)

	// 终端规则模式
	automation.Get("/terminals/:id/rule-mode", ctrl.GetTerminalRuleMode)
	automation.Put("/terminals/:id/rule-mode", ctrl.UpdateTerminalRuleMode)
	automation.Post("/terminals/:id/custom-rule", ctrl.CreateTerminalCustomRule)

	// 默认规则模板
	automation.Get("/patterns/defaults", ctrl.GetDefaultPatterns)

	// AI代理配置
	automation.Get("/agent-configs", ctrl.ListAgentConfigs)
	automation.Put("/agent-configs", middleware.RequireRole("admin"), ctrl.UpdateAgentConfigs)

	// AI Provider配置
	automation.Get("/ai-providers", ctrl.ListAIProviders)
	automation.Post("/ai-providers", middleware.RequireRole("admin"), ctrl.CreateAIProvider)
	automation.Get("/ai-providers/:id", ctrl.GetAIProvider)
	automation.Put("/ai-providers/:id", middleware.RequireRole("admin"), ctrl.UpdateAIProvider)
	automation.Delete("/ai-providers/:id", middleware.RequireRole("admin"), ctrl.DeleteAIProvider)

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

	// 登录记录（管理员）
	automation.Get("/login-records", middleware.RequireRole("admin"), ctrl.ListLoginRecords)
}
