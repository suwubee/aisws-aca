package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/ai"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
)

type MonitorAction string

const (
	MonitorActionContinue MonitorAction = "continue"
	MonitorActionRetry    MonitorAction = "retry"
	MonitorActionAlert    MonitorAction = "alert"
	MonitorActionComplete MonitorAction = "complete"
)

// MonitorDecision 任务监控决策结果
type MonitorDecision struct {
	Action     MonitorAction `json:"action"`
	Reason     string        `json:"reason"`
	Suggestion string        `json:"suggestion"`
}

// LogAnalysis 日志分析结果
type LogAnalysis struct {
	HasError   bool   `json:"has_error"`
	Completed  bool   `json:"completed"`
	NeedsUser  bool   `json:"needs_user"`
	Retryable  bool   `json:"retryable"`
	Reason     string `json:"reason,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// TaskMonitor 任务监控服务（监控终端日志并做出AI决策）
type TaskMonitor struct {
	aiProvider *ai.AIProvider
}

func NewTaskMonitor() *TaskMonitor {
	return &TaskMonitor{
		aiProvider: ai.NewAIProvider(),
	}
}

// StartMonitoring 开始监控任务（读取终端日志 -> AI分析 -> 输出决策）
func (m *TaskMonitor) StartMonitoring(taskID, terminalID string) (MonitorDecision, error) {
	if m == nil {
		return MonitorDecision{}, errors.New("task monitor is nil")
	}
	if m.aiProvider == nil {
		m.aiProvider = ai.NewAIProvider()
	}
	tid := strings.TrimSpace(terminalID)
	if tid == "" {
		return MonitorDecision{}, errors.New("terminalID is required")
	}
	if model.DB == nil {
		return MonitorDecision{}, errors.New("database not initialized")
	}

	limit := 200

	logText, err := m.loadTerminalLogs(strings.TrimSpace(taskID), tid, limit)
	if err != nil {
		return MonitorDecision{}, err
	}

	analysis, analyzeErr := m.AnalyzeLogs(logText)
	decision := m.MakeDecision(analysis)
	return decision, analyzeErr
}

func (m *TaskMonitor) loadTerminalLogs(taskID, terminalID string, limit int) (string, error) {
	if model.DB == nil {
		return "", errors.New("database not initialized")
	}
	if strings.TrimSpace(terminalID) == "" {
		return "", errors.New("terminalID is required")
	}
	if limit <= 0 {
		limit = 200
	}

	query := model.DB.Where("terminal_id = ?", terminalID)
	if strings.TrimSpace(taskID) != "" {
		query = query.Where("task_id = ? OR task_id IS NULL", taskID)
	}

	var logs []model.Log
	if err := query.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
		return "", err
	}

	// 反转日志顺序，使最早的日志在前面（按时间正序）
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	var b strings.Builder
	for _, entry := range logs {
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			continue
		}
		b.WriteString(content)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()), nil
}

// AnalyzeLogs 使用AI Provider分析终端日志（失败时自动降级为启发式分析）
func (m *TaskMonitor) AnalyzeLogs(logs string) (*LogAnalysis, error) {
	if m == nil {
		return nil, errors.New("task monitor is nil")
	}

	text := strings.TrimSpace(logs)
	fallback := heuristicAnalyzeLogs(text)
	if text == "" {
		return fallback, nil
	}

	if m.aiProvider == nil {
		m.aiProvider = ai.NewAIProvider()
	}
	aiConfig, err := m.aiProvider.GetDefaultConfig()
	if err != nil {
		// AI 不可用时，使用启发式分析，不返回错误
		return fallback, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	const maxLogChars = 8000
	lineCount := 0
	if text != "" {
		lineCount = strings.Count(text, "\n") + 1
	}
	systemPrompt, err := promptsvc.RenderTemplate(promptsvc.TemplateKeyTaskMonitorSystemPrompt, map[string]any{
		"log_limit":     lineCount,
		"max_log_chars": maxLogChars,
	})
	if err != nil {
		// AI 提示词模板不可用时，使用启发式分析，不返回错误
		return fallback, nil
	}

	userMsg := text
	if r := []rune(userMsg); len(r) > maxLogChars {
		userMsg = string(r[len(r)-maxLogChars:])
	}
	resp, err := m.aiProvider.ChatSimple(ctx, aiConfig, systemPrompt, userMsg)
	if err != nil {
		// AI 调用失败时，使用启发式分析，不返回错误
		return fallback, nil
	}

	parsed, err := parseLogAnalysis(resp)
	if err != nil {
		// JSON 解析失败时，使用启发式分析，不返回错误
		return fallback, nil
	}

	// 合并兜底检测，避免AI漏判明显信号
	parsed.HasError = parsed.HasError || fallback.HasError
	parsed.Completed = parsed.Completed || fallback.Completed
	parsed.NeedsUser = parsed.NeedsUser || fallback.NeedsUser
	parsed.Retryable = parsed.Retryable || fallback.Retryable
	if strings.TrimSpace(parsed.Reason) == "" {
		parsed.Reason = fallback.Reason
	}
	if strings.TrimSpace(parsed.Suggestion) == "" {
		parsed.Suggestion = fallback.Suggestion
	}

	return parsed, nil
}

func parseLogAnalysis(response string) (*LogAnalysis, error) {
	obj := extractJSONObject(response)
	if obj == "" {
		return nil, errors.New("no JSON object found")
	}
	var parsed LogAnalysis
	if err := json.Unmarshal([]byte(obj), &parsed); err == nil {
		return &parsed, nil
	}
	preview := strings.TrimSpace(response)
	if len(preview) > 800 {
		preview = preview[:800] + "…"
	}
	return nil, fmt.Errorf("unable to parse AI response as JSON: %s", preview)
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

func heuristicAnalyzeLogs(logs string) *LogAnalysis {
	text := strings.ToLower(strings.TrimSpace(logs))
	if text == "" {
		return &LogAnalysis{
			Reason:     "no logs yet",
			Suggestion: "continue monitoring",
		}
	}

	needsUser := containsAny(text, "password", "press enter", "confirm", "y/n", "yes/no", "select", "choose", "请输入", "确认", "选择")
	completed := containsAny(text,
		// AI托管完成标记（最高优先级）
		"aca_task_done",
		"all tests passed", "build succeeded", "completed successfully",
		"success", "done", "finished",
		// 退出码相关
		"exit code 0", "exited with code 0", "exit status 0",
		"exited successfully", "process exited",
		"任务完成", "已完成", "执行完成",
		// Claude Code 任务完成标志
		"what would you like", "how can i help", "anything else",
		"is there anything", "let me know if", "feel free to ask",
		"what's next", "what next", "task complete",
		// 更多完成标志
		"all done", "operation completed", "successfully completed",
		"build complete", "test complete", "deployment complete",
	)
	hasError := containsAny(text, "error", "failed", "fatal", "panic", "traceback", "exception", "permission denied", "no such file", "command not found", "构建失败", "测试失败")
	retryable := hasError && containsAny(text, "timeout", "connection reset", "connection refused", "rate limit", "429", "502", "503", "504", "超时", "重试")

	reason := "no completion/error detected"
	suggestion := "continue monitoring"
	switch {
	case completed:
		reason, suggestion = "completion detected in logs", "mark task as complete"
	case needsUser:
		reason, suggestion = "user intervention detected in logs", "notify user to intervene"
	case hasError && retryable:
		reason, suggestion = "retryable error detected in logs", "retry with backoff"
	case hasError:
		reason, suggestion = "error detected in logs", "inspect logs and fix the issue"
	}

	return &LogAnalysis{
		HasError:   hasError,
		Completed:  completed,
		NeedsUser:  needsUser,
		Retryable:  retryable,
		Reason:     reason,
		Suggestion: suggestion,
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// MakeDecision 基于分析做出决策
func (m *TaskMonitor) MakeDecision(analysis *LogAnalysis) MonitorDecision {
	if analysis == nil {
		return MonitorDecision{
			Action:     MonitorActionContinue,
			Reason:     "no analysis available",
			Suggestion: "continue monitoring",
		}
	}

	fallback := func(v, d string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return d
		}
		return v
	}

	switch {
	case analysis.Completed:
		return MonitorDecision{
			Action:     MonitorActionComplete,
			Reason:     fallback(analysis.Reason, "task completed"),
			Suggestion: fallback(analysis.Suggestion, "mark task as complete"),
		}
	case analysis.NeedsUser:
		return MonitorDecision{
			Action:     MonitorActionAlert,
			Reason:     fallback(analysis.Reason, "user intervention required"),
			Suggestion: fallback(analysis.Suggestion, "notify user and wait for input"),
		}
	case analysis.HasError && analysis.Retryable:
		return MonitorDecision{
			Action:     MonitorActionRetry,
			Reason:     fallback(analysis.Reason, "retryable error detected"),
			Suggestion: fallback(analysis.Suggestion, "retry with backoff"),
		}
	case analysis.HasError:
		return MonitorDecision{
			Action:     MonitorActionAlert,
			Reason:     fallback(analysis.Reason, "error detected"),
			Suggestion: fallback(analysis.Suggestion, "inspect logs and fix the issue"),
		}
	default:
		return MonitorDecision{
			Action:     MonitorActionContinue,
			Reason:     fallback(analysis.Reason, "continue"),
			Suggestion: fallback(analysis.Suggestion, "continue monitoring"),
		}
	}
}
