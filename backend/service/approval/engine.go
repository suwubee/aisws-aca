package approval

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/ai"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/ai-coding-assistant/service/keybinding"
	"github.com/ai-coding-assistant/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ApprovalAction 审批动作
type ApprovalAction string

const (
	ActionApprove ApprovalAction = "approve" // 自动通过
	ActionReject  ApprovalAction = "reject"  // 自动拒绝
	ActionWait    ApprovalAction = "wait"    // 等待人工处理
	ActionInput   ApprovalAction = "input"   // 自动输入
)

// ApprovalResult 审批结果
type ApprovalResult struct {
	Action      ApprovalAction `json:"action"`
	Input       string         `json:"input"`        // 需要发送到终端的输入
	Confidence  float64        `json:"confidence"`   // 置信度
	Reasoning   string         `json:"reasoning"`    // 决策理由
	RuleMatched string         `json:"rule_matched"` // 匹配的规则
	AIDecision  bool           `json:"ai_decision"`  // 是否由AI决策
}

// Engine 审批引擎
type Engine struct {
	aiProvider *ai.AIProvider
	detector   *detector.Detector
	mu         sync.RWMutex

	// 消息推送回调
	onMessage func(msg *model.Message)
}

var yesNoPromptRegex = regexp.MustCompile(`(?i)(\(y/n\)|\[y/n\]|\(yes/no\)|\[yes/no\]|y/n|yes/no|continue\s*\?|proceed\s*\?|confirm\s*\?|enter\s+to\s+confirm|esc\s+to\s+cancel|\d+\.\s*(yes|no))`)

// isClaudeCodeSelectPrompt 检测是否是 Claude Code 选择提示
func isClaudeCodeSelectPrompt(output string) bool {
	patterns := []string{
		`(?i)enter\s+to\s+(confirm|select)`,
		`(?i)esc\s+to\s+cancel`,
		`❯\s*\d+\.`,
		`(?i)\d+\.\s*yes.*proceed`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, output); matched {
			return true
		}
	}
	return false
}

// NewEngine 创建审批引擎
func NewEngine() *Engine {
	return &Engine{
		aiProvider: ai.NewAIProvider(),
		detector:   detector.NewDetector(),
	}
}

// SetMessageCallback 设置消息回调
func (e *Engine) SetMessageCallback(fn func(msg *model.Message)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onMessage = fn
}

// EffectiveConfig 有效的审批配置（从RuleSet解析）
type EffectiveConfig struct {
	TerminalID        string
	ApprovalMode      string
	AutoInputType     string
	WhitelistPatterns string
	BlacklistPatterns string
	AIProviderID      *string
	AIPrompt          string
	ContextLines      int
	DetectClaudeCode  bool
	DetectCodex       bool
	DetectGemini      bool
	NotifyOnBlock     bool
	NotifyOnApprove   bool
}

// GetAutomationConfig 获取终端的有效自动化配置
func (e *Engine) GetAutomationConfig(terminalID string) (*EffectiveConfig, error) {
	// 获取终端信息
	var terminal model.TerminalSession
	if err := model.DB.First(&terminal, "id = ?", terminalID).Error; err != nil {
		// 终端不存在，返回默认配置
		utils.Debug("Terminal not found, using default config",
			zap.String("terminal", terminalID))
		return e.getDefaultConfig(terminalID), nil
	}

	utils.Debug("Terminal found",
		zap.String("terminal", terminalID),
		zap.String("rule_mode", terminal.RuleMode))

	// 根据终端的规则模式获取对应的规则集
	var ruleSet *model.RuleSet

	switch terminal.RuleMode {
	case "system":
		// 使用系统规则
		var systemRule model.RuleSet
		if err := model.DB.Where("type = ?", "system").First(&systemRule).Error; err == nil {
			ruleSet = &systemRule
			utils.Debug("Using system rule",
				zap.String("rule_id", systemRule.ID),
				zap.String("approval_mode", systemRule.ApprovalMode),
				zap.String("auto_input_type", systemRule.AutoInputType))
		} else {
			utils.Warn("System rule not found", zap.Error(err))
		}

	case "task":
		// 使用关联任务的规则
		if terminal.TaskID != nil {
			var task model.Task
			if err := model.DB.First(&task, "id = ?", *terminal.TaskID).Error; err == nil && task.RuleSetID != nil {
				var taskRule model.RuleSet
				if err := model.DB.First(&taskRule, "id = ?", *task.RuleSetID).Error; err == nil {
					ruleSet = &taskRule
				}
			}
		}

	case "custom":
		// 使用终端自定义规则
		if terminal.RuleSetID != nil {
			var customRule model.RuleSet
			if err := model.DB.First(&customRule, "id = ?", *terminal.RuleSetID).Error; err == nil {
				ruleSet = &customRule
			}
		}

	default:
		// none 模式，返回默认配置（手动审批）
		return e.getDefaultConfig(terminalID), nil
	}

	// 如果没有找到规则集，返回默认配置
	if ruleSet == nil {
		return e.getDefaultConfig(terminalID), nil
	}

	// 将RuleSet转换为EffectiveConfig
	return &EffectiveConfig{
		TerminalID:        terminalID,
		ApprovalMode:      ruleSet.ApprovalMode,
		AutoInputType:     ruleSet.AutoInputType,
		WhitelistPatterns: ruleSet.WhitelistPatterns,
		BlacklistPatterns: ruleSet.BlacklistPatterns,
		AIProviderID:      ruleSet.AIProviderID,
		AIPrompt:          ruleSet.AIPrompt,
		ContextLines:      ruleSet.ContextLines,
		DetectClaudeCode:  ruleSet.DetectClaudeCode,
		DetectCodex:       ruleSet.DetectCodex,
		DetectGemini:      ruleSet.DetectGemini,
		NotifyOnBlock:     ruleSet.NotifyOnBlock,
		NotifyOnApprove:   ruleSet.NotifyOnApprove,
	}, nil
}

// getDefaultConfig 返回默认配置
func (e *Engine) getDefaultConfig(terminalID string) *EffectiveConfig {
	return &EffectiveConfig{
		TerminalID:       terminalID,
		ApprovalMode:     "manual",
		AutoInputType:    "yes",
		ContextLines:     50,
		DetectClaudeCode: true,
		DetectCodex:      true,
		DetectGemini:     true,
		NotifyOnBlock:    true,
		NotifyOnApprove:  false,
	}
}

// Evaluate 评估是否需要审批以及采取什么动作
func (e *Engine) Evaluate(ctx context.Context, terminalID, output string) (*ApprovalResult, error) {
	return e.EvaluateWithContext(ctx, terminalID, output, output)
}

// EvaluateWithContext 评估审批，使用原始output检测，fullContext用于AI分析
func (e *Engine) EvaluateWithContext(ctx context.Context, terminalID, output, fullContext string) (*ApprovalResult, error) {
	config, err := e.GetAutomationConfig(terminalID)
	if err != nil {
		utils.Error("Failed to get automation config", zap.Error(err), zap.String("terminal", terminalID))
		return nil, err
	}

	utils.Info("Automation config loaded",
		zap.String("terminal", terminalID),
		zap.String("approval_mode", config.ApprovalMode),
		zap.String("auto_input_type", config.AutoInputType))

	// 检测是否是等待审批的提示（使用原始output）
	isApproval := e.detector.IsApprovalPrompt(output)
	utils.Info("IsApprovalPrompt check",
		zap.String("terminal", terminalID),
		zap.Bool("is_approval", isApproval),
		zap.Int("output_len", len(output)))

	if !isApproval {
		return &ApprovalResult{
			Action:    ActionWait,
			Reasoning: "不是审批提示",
		}, nil
	}

	// 根据模式处理（使用fullContext进行分析）
	utils.Info("Processing approval",
		zap.String("terminal", terminalID),
		zap.String("mode", config.ApprovalMode))

	switch config.ApprovalMode {
	case "manual":
		return e.handleManualMode(config, fullContext)
	case "auto_yes":
		return e.handleAutoYesMode(config, fullContext)
	case "smart":
		return e.handleSmartMode(ctx, config, fullContext)
	default:
		utils.Warn("Unknown approval mode, defaulting to manual",
			zap.String("terminal", terminalID),
			zap.String("mode", config.ApprovalMode))
		return e.handleManualMode(config, fullContext)
	}
}

// handleManualMode 手动模式 - 检查黑名单并通知
func (e *Engine) handleManualMode(config *EffectiveConfig, output string) (*ApprovalResult, error) {
	// 检查黑名单
	if config.BlacklistPatterns != "" {
		patterns := parsePatterns(config.BlacklistPatterns)
		for _, pattern := range patterns {
			if matchPattern(output, pattern) {
				result := &ApprovalResult{
					Action:      ActionWait,
					Reasoning:   "匹配黑名单模式，需要人工审批",
					RuleMatched: pattern,
				}
				// 发送通知
				if config.NotifyOnBlock {
					e.sendNotification(config.TerminalID, "blocked", "检测到危险操作", "匹配黑名单: "+pattern, output)
				}
				return result, nil
			}
		}
	}

	return &ApprovalResult{
		Action:    ActionWait,
		Reasoning: "手动模式，等待人工审批",
	}, nil
}

// handleAutoYesMode 自动通过模式 - 检查黑名单，否则自动输入
func (e *Engine) handleAutoYesMode(config *EffectiveConfig, output string) (*ApprovalResult, error) {
	// 先检查黑名单
	if config.BlacklistPatterns != "" {
		patterns := parsePatterns(config.BlacklistPatterns)
		for _, pattern := range patterns {
			if matchPattern(output, pattern) {
				result := &ApprovalResult{
					Action:      ActionWait,
					Reasoning:   "匹配黑名单，即使在自动模式下也需要人工审批",
					RuleMatched: pattern,
				}
				if config.NotifyOnBlock {
					e.sendNotification(config.TerminalID, "blocked", "自动模式被阻止", "匹配黑名单: "+pattern, output)
				}
				return result, nil
			}
		}
	}

	// 确定输入内容
	input := getAutoInput(config.AutoInputType)

	result := &ApprovalResult{
		Action:     ActionApprove,
		Input:      input,
		Confidence: 1.0,
		Reasoning:  "自动模式，自动通过",
	}

	if config.NotifyOnApprove {
		e.sendNotification(config.TerminalID, "info", "自动通过", "已自动输入: "+input, output)
	}

	return result, nil
}

// handleSmartMode AI辅助模式
func (e *Engine) handleSmartMode(ctx context.Context, config *EffectiveConfig, output string) (*ApprovalResult, error) {
	// 先检查白名单
	if config.WhitelistPatterns != "" {
		patterns := parsePatterns(config.WhitelistPatterns)
		for _, pattern := range patterns {
			if matchPattern(output, pattern) {
				input := getAutoInput(config.AutoInputType)
				return &ApprovalResult{
					Action:      ActionApprove,
					Input:       input,
					Confidence:  1.0,
					Reasoning:   "匹配白名单，自动通过",
					RuleMatched: pattern,
				}, nil
			}
		}
	}

	// 检查黑名单
	if config.BlacklistPatterns != "" {
		patterns := parsePatterns(config.BlacklistPatterns)
		for _, pattern := range patterns {
			if matchPattern(output, pattern) {
				result := &ApprovalResult{
					Action:      ActionWait,
					Reasoning:   "匹配黑名单，需要人工审批",
					RuleMatched: pattern,
				}
				if config.NotifyOnBlock {
					e.sendNotification(config.TerminalID, "blocked", "检测到危险操作", "匹配黑名单: "+pattern, output)
				}
				return result, nil
			}
		}
	}

	// 使用AI判断
	if config.AIProviderID != nil {
		aiConfig, err := e.aiProvider.GetConfigByID(*config.AIProviderID)
		if err == nil && aiConfig != nil {
			decision, err := e.aiProvider.AnalyzeForApproval(ctx, aiConfig, config.AIPrompt, output)
			if err != nil {
				utils.Warn("AI analysis failed", zap.Error(err))
			} else {
				action := strings.TrimSpace(strings.ToLower(decision.Action))
				if action == "" {
					action = "wait"
				}
				// AI 返回 input 但没有输入内容，降级为等待人工
				if action == "input" && strings.TrimSpace(decision.Input) == "" {
					action = "wait"
					if strings.TrimSpace(decision.Reasoning) == "" {
						decision.Reasoning = "AI返回input但未提供input内容"
					}
				}
				// AI 批准但未给出输入时，若为常见 y/n 确认提示则补全默认输入
				if action == "approve" && strings.TrimSpace(decision.Input) == "" && yesNoPromptRegex.MatchString(output) {
					decision.Input = getAutoInput(config.AutoInputType)
				}

				result := &ApprovalResult{
					Action:     ApprovalAction(action),
					Input:      decision.Input,
					Confidence: decision.Confidence,
					Reasoning:  decision.Reasoning,
					AIDecision: true,
				}

				// 强制转换：Claude Code 选择提示需要回车
				if action == "approve" && isClaudeCodeSelectPrompt(output) {
					result.Input = "\r" // 使用 \r 而不是 \n
				}

				// 根据AI决策发送通知
				if action == "reject" && config.NotifyOnBlock {
					e.sendNotification(config.TerminalID, "blocked", "AI建议拒绝", decision.Reasoning, output)
				} else if action == "approve" && config.NotifyOnApprove {
					e.sendNotification(config.TerminalID, "info", "AI自动通过", decision.Reasoning, output)
				}

				return result, nil
			}
		}
	}

	// 默认等待
	return &ApprovalResult{
		Action:    ActionWait,
		Reasoning: "无法确定，等待人工审批",
	}, nil
}

// sendNotification 发送通知消息
func (e *Engine) sendNotification(terminalID, msgType, title, content, context string) {
	taskID, serverID := resolveServerAssociationFromTerminal(terminalID)

	msg := &model.Message{
		ID:         uuid.New().String(),
		TerminalID: &terminalID,
		TaskID:     taskID,
		ServerID:   serverID,
		Type:       msgType,
		Title:      title,
		Content:    content,
		Context:    context,
		Status:     "unread",
		Priority:   getPriorityByType(msgType),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 保存到数据库
	if err := model.DB.Create(msg).Error; err != nil {
		utils.Error("Failed to save notification", zap.Error(err))
	}

	// 调用回调
	e.mu.RLock()
	callback := e.onMessage
	e.mu.RUnlock()

	if callback != nil {
		callback(msg)
	}
}

// RecordApproval 记录审批操作
func (e *Engine) RecordApproval(terminalID string, aiSessionID *string, promptType, promptContent, response string, autoApproved bool, ruleMatched, aiDecision string) error {
	promptContent = sanitizeApprovalText(promptContent)
	response = strings.TrimSpace(response)
	ruleMatched = strings.TrimSpace(ruleMatched)
	aiDecision = strings.TrimSpace(aiDecision)

	_, serverID := resolveServerAssociationFromTerminal(terminalID)
	record := &model.ApprovalRecord{
		ID:            uuid.New().String(),
		TerminalID:    terminalID,
		ServerID:      serverID,
		AISessionID:   aiSessionID,
		PromptType:    promptType,
		PromptContent: promptContent,
		Response:      response,
		AutoApproved:  autoApproved,
		RuleMatched:   ruleMatched,
		AIDecision:    aiDecision,
		CreatedAt:     time.Now(),
	}
	return model.DB.Create(record).Error
}

// 辅助函数

func resolveServerAssociationFromTerminal(terminalID string) (*string, *string) {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return nil, nil
	}

	var terminal model.TerminalSession
	if err := model.DB.Select("task_id").First(&terminal, "id = ?", terminalID).Error; err != nil {
		return nil, nil
	}

	if terminal.TaskID == nil {
		return nil, nil
	}
	trimmedTaskID := strings.TrimSpace(*terminal.TaskID)
	if trimmedTaskID == "" {
		return nil, nil
	}
	taskID := &trimmedTaskID

	var task model.Task
	if err := model.DB.Select("server_id").First(&task, "id = ?", trimmedTaskID).Error; err != nil {
		return taskID, nil
	}
	if task.ServerID == nil {
		return taskID, nil
	}
	serverID := strings.TrimSpace(*task.ServerID)
	if serverID == "" {
		return taskID, nil
	}

	return taskID, &serverID
}

var approvalANSIRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b[PX^_].*?\x1b\\`)
var approvalANSIRemnantRegex = regexp.MustCompile(`\[[0-9;]{1,20}[a-zA-Z]`)
var approvalANSIRemnantNoBracketRegex = regexp.MustCompile(`(^|[ \t])(?:[0-9]{1,3};){1,8}[0-9]{1,3}m([+\\-\\[]|[ \t])`)

func sanitizeApprovalText(s string) string {
	if s == "" {
		return ""
	}

	// 统一换行
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// 去除 ANSI 控制序列
	s = approvalANSIRegex.ReplaceAllString(s, "")
	// 二次兜底：去除可能残留的类似 "[39m" / "[K" 等片段（复制/编码导致 ESC 丢失时）
	s = approvalANSIRemnantRegex.ReplaceAllString(s, "")
	s = approvalANSIRemnantNoBracketRegex.ReplaceAllString(s, "$1$2")

	// 去除其他控制字符（保留换行和制表符）
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)

	// 压缩多余空行
	s = strings.TrimSpace(s)
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}

	// 防止极端情况下存储过大（UI 也更友好）
	const maxLen = 8000
	if len(s) > maxLen {
		s = s[:maxLen] + "\n…(truncated)"
	}

	return s
}

func isApprovalPrompt(output string) bool {
	// 常见的审批提示模式
	patterns := []string{
		`(?i)\(y/n\)`,
		`(?i)\[y/n\]`,
		`(?i)\(yes/no\)`,
		`(?i)\[yes/no\]`,
		`(?i)continue\?`,
		`(?i)proceed\?`,
		`(?i)confirm`,
		`(?i)allow.*\?`,
		`(?i)approve.*\?`,
		`(?i)permission`,
		`(?i)do you want to`,
		`(?i)would you like to`,
		// Claude Code 特有提示
		`(?i)allow tool`,
		`(?i)allow read`,
		`(?i)allow write`,
		`(?i)allow execute`,
		`(?i)allow bash`,
		// Codex/Copilot 提示
		`(?i)accept suggestion`,
		// 选择提示
		`\[1\].*\[2\]`,
		`1\).*2\)`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, output); matched {
			return true
		}
	}
	return false
}

func parsePatterns(jsonStr string) []string {
	var patterns []string
	if err := json.Unmarshal([]byte(jsonStr), &patterns); err != nil {
		// 如果不是JSON数组，按逗号分割
		patterns = strings.Split(jsonStr, ",")
		for i := range patterns {
			patterns[i] = strings.TrimSpace(patterns[i])
		}
	}
	return patterns
}

func matchPattern(text, pattern string) bool {
	// 先尝试正则匹配
	if matched, _ := regexp.MatchString(pattern, text); matched {
		return true
	}
	// 再尝试简单字符串匹配（忽略大小写）
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

func getAutoInput(inputType string) string {
	id := keybinding.Alias(inputType)
	if id == "" {
		id = keybinding.IDYes
	}
	out, err := keybinding.ResolvePty(id)
	if err != nil || out == "" {
		// Safe fallback for automation.
		return "yes\r"
	}
	return out
}

func getPriorityByType(msgType string) int {
	switch msgType {
	case "blocked":
		return 2 // urgent
	case "approval_needed":
		return 1 // high
	case "warning":
		return 1 // high
	default:
		return 0 // normal
	}
}

// 预设的危险模式
var DefaultBlacklistPatterns = []string{
	"rm -rf",
	"rm -r /",
	"rm -rf /",
	":(){ :|:& };:", // fork bomb
	"dd if=/dev/zero",
	"mkfs",
	"fdisk",
	"chmod -R 777",
	"chmod 777 /",
	"> /dev/sda",
	"mv /* ",
	"wget.*|.*sh",
	"curl.*|.*sh",
}

// 预设的安全模式（Claude Code常见操作）
var DefaultWhitelistPatterns = []string{
	"allow read",
	"allow write.*\\.go",
	"allow write.*\\.ts",
	"allow write.*\\.vue",
	"allow write.*\\.json",
	"create file",
	"create directory",
	"update file",
}
