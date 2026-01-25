package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/ai"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
	"github.com/ai-coding-assistant/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AIWorkflowEngine executes workflows driven by AI conversation
type AIWorkflowEngine struct {
	aiProvider    *ai.AIProvider
	toolExecutor  *ToolExecutor
	maxIterations int
	inflight      sync.Map // sessionID -> *runtimeSession
}

type runtimeSession struct {
	mu       sync.Mutex
	session  *AIWorkflowSession
	aiConfig *model.AIProviderConfig
	pending  []ai.ChatMessage
	closing  bool
	done     chan struct{}

	// Cached identifiers for safe logging without touching the mutable Context map
	// while tools may be executing and mutating it.
	terminalID string
	taskID     string
}

// NewAIWorkflowEngine creates a new AI workflow engine
func NewAIWorkflowEngine(toolExecutor *ToolExecutor) *AIWorkflowEngine {
	return &AIWorkflowEngine{
		aiProvider:    ai.NewAIProvider(),
		toolExecutor:  toolExecutor,
		maxIterations: 50,
	}
}

// AIWorkflowSession represents an AI-driven workflow session
type AIWorkflowSession struct {
	ID          string           `json:"id"`
	WorkflowID  string           `json:"workflow_id"`
	UserGoal    string           `json:"user_goal"`
	Status      string           `json:"status"` // running, completed, failed, paused
	Messages    []ai.ChatMessage `json:"messages"`
	Steps       []AIWorkflowStep `json:"steps"`
	Context     map[string]any   `json:"context"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at"`
	Summary     string           `json:"summary"`
}

// AIWorkflowStep represents a single step in AI workflow
type AIWorkflowStep struct {
	ID         string    `json:"id"`
	Iteration  int       `json:"iteration"`
	Thought    string    `json:"thought"`
	Action     string    `json:"action"`
	ActionArgs any       `json:"action_args"`
	Result     string    `json:"result"`
	Success    bool      `json:"success"`
	Timestamp  time.Time `json:"timestamp"`
}

type StartWorkflowOptions struct {
	WorkflowID string

	Context map[string]any

	SystemPromptTemplateKey string
	SystemPromptVars        map[string]any

	UserGoalTemplateKey string
	UserGoalVars        map[string]any
}

// StartWorkflow starts an AI-driven workflow with user goal
func (e *AIWorkflowEngine) StartWorkflow(ctx context.Context, userGoal string) (*AIWorkflowSession, error) {
	return e.StartWorkflowWithOptions(ctx, userGoal, StartWorkflowOptions{})
}

func (e *AIWorkflowEngine) StartWorkflowWithOptions(ctx context.Context, userGoal string, opts StartWorkflowOptions) (*AIWorkflowSession, error) {
	goal := strings.TrimSpace(userGoal)
	if goal == "" {
		return nil, errors.New("user goal is required")
	}

	// Get AI config
	aiConfig, err := e.aiProvider.GetDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("no AI provider configured: %w", err)
	}

	workflowID := strings.TrimSpace(opts.WorkflowID)

	// Create session
	session := &AIWorkflowSession{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		UserGoal:   goal,
		Status:     "running",
		Messages:   []ai.ChatMessage{},
		Steps:      []AIWorkflowStep{},
		Context:    make(map[string]any),
		StartedAt:  time.Now(),
	}

	for k, v := range opts.Context {
		if strings.TrimSpace(k) == "" {
			continue
		}
		session.Context[k] = v
	}

	// Build system prompt with tools
	systemKey := strings.TrimSpace(opts.SystemPromptTemplateKey)
	if systemKey == "" {
		systemKey = promptsvc.TemplateKeyAIWorkflowSystemPrompt
	}
	systemPrompt, err := FormatToolsForPromptWithTemplate(systemKey, opts.SystemPromptVars)
	if err != nil {
		return nil, err
	}
	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user goal
	userGoalKey := strings.TrimSpace(opts.UserGoalTemplateKey)
	if userGoalKey == "" {
		userGoalKey = promptsvc.TemplateKeyAIWorkflowUserGoalPrompt
	}
	userVars := map[string]any{"user_goal": goal}
	for k, v := range opts.UserGoalVars {
		if strings.TrimSpace(k) == "" {
			continue
		}
		userVars[k] = v
	}
	userVars["user_goal"] = goal
	userGoalPrompt, err := promptsvc.RenderTemplate(userGoalKey, userVars)
	if err != nil {
		userGoalPrompt = goal
	}
	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "user",
		Content: userGoalPrompt,
	})

	// Save session to DB
	if err := e.saveSession(session); err != nil {
		utils.Warn("Failed to save workflow session", zap.Error(err))
	}

	// Start execution loop in background
	if !e.startExecution(session, aiConfig) {
		return session, errors.New("workflow session already running")
	}

	return session, nil
}

func (e *AIWorkflowEngine) startExecution(session *AIWorkflowSession, aiConfig *model.AIProviderConfig) bool {
	if e == nil || session == nil || aiConfig == nil {
		return false
	}

	id := strings.TrimSpace(session.ID)
	if id == "" {
		return false
	}

	rt := &runtimeSession{
		session:  session,
		aiConfig: aiConfig,
		closing:  false,
		done:     make(chan struct{}),
	}
	if session != nil && session.Context != nil {
		rt.terminalID = strings.TrimSpace(getStringFromMap(session.Context, "terminal_id"))
		rt.taskID = strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
	}
	if _, loaded := e.inflight.LoadOrStore(id, rt); loaded {
		return false
	}

	go func() {
		defer func() {
			e.inflight.Delete(id)
			close(rt.done)
		}()

		for {
			e.executeLoop(context.Background(), rt)

			// If a user message slips in right as the loop is terminating (race window),
			// it can be queued in rt.pending but never drained by executeLoop. Recover by
			// appending and restarting the loop.
			rt.mu.Lock()
			if len(rt.pending) == 0 {
				rt.closing = true
				rt.mu.Unlock()
				return
			}

			rt.session.Messages = append(rt.session.Messages, rt.pending...)
			rt.pending = nil
			rt.session.Status = "running"
			rt.session.Summary = ""
			rt.session.CompletedAt = nil
			rt.closing = false
			_ = e.saveSession(rt.session)
			rt.mu.Unlock()
		}
	}()

	return true
}

func (e *AIWorkflowEngine) emitUserMessageLog(sessionCtx map[string]any, userMessage string) {
	if e == nil || e.toolExecutor == nil || sessionCtx == nil {
		return
	}
	if strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id")) == "" {
		return
	}

	text := strings.TrimSpace(userMessage)
	if text == "" {
		return
	}
	const maxRunes = 2000
	runes := []rune(text)
	if len(runes) > maxRunes {
		text = string(runes[:maxRunes]) + "…(truncated)…"
	}
	e.toolExecutor.emitTerminalAILog(sessionCtx, "action", "用户补充信息", "text", text)
}

func (e *AIWorkflowEngine) emitUserMessageLogFromRuntime(rt *runtimeSession, userMessage string) {
	if e == nil || rt == nil {
		return
	}
	tid := strings.TrimSpace(rt.terminalID)
	if tid == "" {
		return
	}
	ctx := map[string]any{
		"terminal_id": tid,
	}
	if strings.TrimSpace(rt.taskID) != "" {
		ctx["task_id"] = rt.taskID
	}
	e.emitUserMessageLog(ctx, userMessage)
}

// ResumeWorkflow appends a user message and resumes a workflow session.
func (e *AIWorkflowEngine) ResumeWorkflow(ctx context.Context, sessionID string, userMessage string) (*AIWorkflowSession, error) {
	if e == nil {
		return nil, errors.New("engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	id := strings.TrimSpace(sessionID)
	if id == "" {
		return nil, errors.New("session id required")
	}

	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		return nil, errors.New("message is required")
	}

	// Fast path: if the session is currently running in-memory, allow appending messages without "resumable" gating.
	if rtValue, ok := e.inflight.Load(id); ok {
		if rt, ok := rtValue.(*runtimeSession); ok && rt != nil && rt.session != nil {
			rt.mu.Lock()
			if rt.closing {
				done := rt.done
				rt.mu.Unlock()
				if done != nil {
					select {
					case <-done:
					case <-ctx.Done():
					case <-time.After(2 * time.Second):
					}
				}
				// Fall back to the persisted resume path below.
			} else {
				// Preserve message ordering: queue user messages to be appended at the next loop boundary,
				// so we don't insert a "late" user message before the assistant response of an in-flight AI call.
				rt.pending = append(rt.pending, ai.ChatMessage{
					Role:    "user",
					Content: msg,
				})
				rt.session.Status = "running"
				rt.session.CompletedAt = nil
				session := rt.session
				rt.mu.Unlock()
				e.emitUserMessageLogFromRuntime(rt, msg)
				return session, nil
			}
		}
	}

	session, err := e.GetSession(id)
	if err != nil {
		return nil, err
	}

	rawStatus := strings.ToLower(strings.TrimSpace(session.Status))
	status := rawStatus
	switch status {
	case "":
		// Backward compatibility / DB null -> empty string.
		status = "running"
	case "done":
		status = "completed"
	case "canceled":
		status = "cancelled"
	case "in_progress":
		status = "running"
	}

	resumable := status == "running" || status == "paused" || status == "completed" || status == "failed" || status == "cancelled" || status == "timeout"
	if !resumable {
		// Best-effort recovery for legacy/unknown statuses: allow resuming instead of hard failing.
		utils.Warn("workflow session status not resumable; forcing resume",
			zap.String("session", id),
			zap.String("status", rawStatus))
		status = "running"
	}

	shouldRestartTaskMonitor := status == "completed" || status == "failed" || status == "cancelled" || status == "timeout"

	aiConfig, err := e.aiProvider.GetDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("no AI provider configured: %w", err)
	}

	if session.Context == nil {
		session.Context = make(map[string]any)
	}

	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "user",
		Content: msg,
	})
	session.Status = "running"
	session.Summary = ""
	session.CompletedAt = nil

	if err := e.saveSession(session); err != nil {
		return nil, err
	}

	// Best-effort start: if it's already running (race / restarted loop), treat as success.
	_ = e.startExecution(session, aiConfig)
	e.emitUserMessageLog(session.Context, msg)

	if shouldRestartTaskMonitor {
		taskID := strings.TrimSpace(session.WorkflowID)
		if taskID == "" {
			taskID = strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
		}
		if taskID != "" {
			updateTaskStatus(taskID, "in_progress")
			go e.monitorTaskAgent(taskID, session.ID)
		}
	}

	return session, nil
}

// PauseWorkflow sets a running workflow session to paused.
// Note: execution will stop at the next loop boundary (best-effort).
func (e *AIWorkflowEngine) PauseWorkflow(_ context.Context, sessionID string, reason string) (*AIWorkflowSession, error) {
	if e == nil {
		return nil, errors.New("engine is nil")
	}

	id := strings.TrimSpace(sessionID)
	if id == "" {
		return nil, errors.New("session id required")
	}

	session, err := e.GetSession(id)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(session.Status))
	if status == "paused" {
		return session, nil
	}
	if status != "running" {
		return nil, errors.New("session is not running")
	}

	if session.Context == nil {
		session.Context = make(map[string]any)
	}
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "user_paused"
	}
	session.Context["pause_reason"] = r
	session.Status = "paused"
	if strings.TrimSpace(session.Summary) == "" {
		session.Summary = "用户手动暂停"
	}
	session.CompletedAt = nil

	if err := e.saveSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

func (e *AIWorkflowEngine) applyExternalControl(session *AIWorkflowSession) bool {
	if e == nil || session == nil || model.DB == nil {
		return false
	}
	id := strings.TrimSpace(session.ID)
	if id == "" {
		return false
	}

	var row model.AIWorkflowSession
	if err := model.DB.Select("status", "summary").First(&row, "id = ?", id).Error; err != nil {
		return false
	}

	status := strings.ToLower(strings.TrimSpace(row.Status))
	switch status {
	case "paused", "cancelled":
		session.Status = status
		if s := strings.TrimSpace(row.Summary); s != "" {
			session.Summary = s
		}
		return true
	default:
		return false
	}
}

// executeLoop runs the ReAct loop
func (e *AIWorkflowEngine) executeLoop(ctx context.Context, rt *runtimeSession) {
	if e == nil || rt == nil || rt.session == nil || rt.aiConfig == nil {
		return
	}
	session := rt.session
	aiConfig := rt.aiConfig
	defer func() {
		if r := recover(); r != nil {
			rt.mu.Lock()
			session.Status = "failed"
			session.Summary = fmt.Sprintf("panic: %v", r)
			_ = e.saveSession(session)
			rt.mu.Unlock()
		}
	}()

	iterationBase := 0
	if session != nil {
		rt.mu.Lock()
		iterationBase = len(session.Steps)
		rt.mu.Unlock()
	}

	// Guardrails: avoid burning tokens when the model repeatedly fails to produce a valid ReAct response
	// or keeps repeating the same failing action.
	const (
		maxConsecutiveInvalidResponses = 3
		maxRepeatedFailureStreak       = 3
	)
	invalidResponseStreak := 0
	repeatedFailureStreak := 0
	lastFailureKey := ""

	for i := 0; i < e.maxIterations; i++ {
		iteration := iterationBase + i
		select {
		case <-ctx.Done():
			rt.mu.Lock()
			if len(rt.pending) > 0 {
				session.Messages = append(session.Messages, rt.pending...)
				rt.pending = nil
			}
			session.Status = "cancelled"
			_ = e.saveSession(session)
			rt.mu.Unlock()
			return
		default:
		}

		// External control: if user paused/cancelled, stop best-effort (avoid overriding status back to running).
		rt.mu.Lock()
		if e.applyExternalControl(session) {
			_ = e.saveSession(session)
			rt.mu.Unlock()
			return
		}
		// Drain any queued user messages from API calls (keeps ordering with in-flight assistant responses).
		if len(rt.pending) > 0 {
			session.Messages = append(session.Messages, rt.pending...)
			rt.pending = nil
		}
		// Take a snapshot of messages for this AI turn; user may append concurrently.
		messages := append([]ai.ChatMessage(nil), session.Messages...)
		rt.mu.Unlock()

		// Call AI
		resp, err := e.aiProvider.Chat(ctx, aiConfig, messages)
		if err != nil {
			utils.Error("AI call failed", zap.Error(err))
			rt.mu.Lock()
			if len(rt.pending) > 0 {
				session.Messages = append(session.Messages, rt.pending...)
				rt.pending = nil
			}
			session.Status = "failed"
			session.Summary = fmt.Sprintf("AI调用失败: %v", err)
			_ = e.saveSession(session)
			rt.mu.Unlock()
			return
		}

		if len(resp.Choices) == 0 {
			continue
		}

		aiResponse := resp.Choices[0].Message.Content
		rt.mu.Lock()
		session.Messages = append(session.Messages, ai.ChatMessage{
			Role:    "assistant",
			Content: aiResponse,
		})

		// Parse response
		parsed, err := ParseAIResponse(aiResponse)
		if err != nil {
			utils.Warn("Failed to parse AI response", zap.Error(err))
			if e.toolExecutor != nil && session.Context != nil && strings.TrimSpace(getStringFromMap(session.Context, "terminal_id")) != "" {
				e.toolExecutor.emitTerminalAILog(session.Context, "error", "解析 AI 响应失败，将继续尝试下一步", "error", err.Error())
			}
			invalidResponseStreak++
			if invalidResponseStreak >= maxConsecutiveInvalidResponses {
				question := fmt.Sprintf("AI 连续 %d 次未能生成可执行步骤（可能是输出未按 ReAct 格式或缺少关键信息）。\n请补充/确认：\n- 目标要达到的具体结果（成功标准）\n- 目标范围/服务器\n- 是否允许执行风险操作（安装/修改配置/重启）\n回复后我将继续执行。", maxConsecutiveInvalidResponses)
				step := AIWorkflowStep{
					ID:         uuid.New().String(),
					Iteration:  iteration,
					Thought:    "guardrail: invalid_ai_response",
					Action:     "ask_user",
					ActionArgs: map[string]any{"question": question},
					Result:     question,
					Success:    true,
					Timestamp:  time.Now(),
				}
				session.Steps = append(session.Steps, step)
				if len(rt.pending) > 0 {
					session.Messages = append(session.Messages, rt.pending...)
					rt.pending = nil
				}
				session.Status = "paused"
				session.Summary = question
				session.CompletedAt = nil
				_ = e.saveSession(session)
				rt.mu.Unlock()
				return
			}
			_ = e.saveSession(session)
			rt.mu.Unlock()
			continue
		}

		// If the model didn't produce either an action or a completion marker, treat it as invalid and retry a bit.
		if parsed.Action == nil && parsed.Complete == nil {
			if e.toolExecutor != nil && session.Context != nil && strings.TrimSpace(getStringFromMap(session.Context, "terminal_id")) != "" {
				e.toolExecutor.emitTerminalAILog(session.Context, "warning", "AI 未输出 action/complete，将重试", "", "")
			}
			invalidResponseStreak++
			if invalidResponseStreak >= maxConsecutiveInvalidResponses {
				question := fmt.Sprintf("AI 连续 %d 次未能生成可执行步骤（未输出 action/complete）。\n请补充/确认：\n- 目标要达到的具体结果（成功标准）\n- 目标范围/服务器\n- 是否允许执行风险操作（安装/修改配置/重启）\n回复后我将继续执行。", maxConsecutiveInvalidResponses)
				step := AIWorkflowStep{
					ID:         uuid.New().String(),
					Iteration:  iteration,
					Thought:    "guardrail: missing_action_or_complete",
					Action:     "ask_user",
					ActionArgs: map[string]any{"question": question},
					Result:     question,
					Success:    true,
					Timestamp:  time.Now(),
				}
				session.Steps = append(session.Steps, step)
				if len(rt.pending) > 0 {
					session.Messages = append(session.Messages, rt.pending...)
					rt.pending = nil
				}
				session.Status = "paused"
				session.Summary = question
				session.CompletedAt = nil
				_ = e.saveSession(session)
				rt.mu.Unlock()
				return
			}
			_ = e.saveSession(session)
			rt.mu.Unlock()
			continue
		}

		invalidResponseStreak = 0

		// Best-effort AI logs for observability (keeps the old "[AI][type]" stream style).
		if e.toolExecutor != nil && session.Context != nil && strings.TrimSpace(getStringFromMap(session.Context, "terminal_id")) != "" {
			if strings.TrimSpace(parsed.Thought) != "" {
				e.toolExecutor.emitTerminalAILog(session.Context, "thinking", strings.TrimSpace(parsed.Thought), "", "")
			}
			if parsed.Action != nil && strings.TrimSpace(parsed.Action.Tool) != "" {
				e.toolExecutor.emitTerminalAILog(session.Context, "decision", fmt.Sprintf("调用工具: %s", strings.TrimSpace(parsed.Action.Tool)), "", "")
			}
		}

		// Check if complete
		if parsed.Complete != nil {
			summary := strings.TrimSpace(parsed.Complete.Summary)

			// If the user already queued follow-up messages while this completion was being generated,
			// do NOT terminate the session; treat this completion as a milestone and continue.
			if len(rt.pending) > 0 {
				step := AIWorkflowStep{
					ID:         uuid.New().String(),
					Iteration:  iteration,
					Thought:    parsed.Thought,
					Action:     "complete",
					ActionArgs: map[string]any{"status": strings.TrimSpace(parsed.Complete.Status)},
					Result:     summary,
					Success:    true,
					Timestamp:  time.Now(),
				}
				session.Steps = append(session.Steps, step)

				if e.toolExecutor != nil && session.Context != nil && strings.TrimSpace(getStringFromMap(session.Context, "terminal_id")) != "" {
					msg := summary
					if msg == "" {
						msg = "AI 托管已完成"
					}
					e.toolExecutor.emitTerminalAILog(session.Context, "info", "AI 托管完成：\n"+msg, "", "")
				}

				session.Messages = append(session.Messages, rt.pending...)
				rt.pending = nil
				session.Status = "running"
				session.Summary = ""
				session.CompletedAt = nil
				_ = e.saveSession(session)
				rt.mu.Unlock()
				continue
			}

			now := time.Now()
			session.Status = "completed"
			session.Summary = summary
			session.CompletedAt = &now
			_ = e.saveSession(session)
			rt.mu.Unlock()
			return
		}

		// Execute action
		if parsed.Action != nil {
			if strings.EqualFold(strings.TrimSpace(parsed.Action.Tool), "ask_user") {
				question, _ := parsed.Action.Args["question"].(string)
				question = strings.TrimSpace(question)
				if question == "" {
					question = "需要用户补充信息/确认后继续"
				}

				step := AIWorkflowStep{
					ID:         uuid.New().String(),
					Iteration:  iteration,
					Thought:    parsed.Thought,
					Action:     parsed.Action.Tool,
					ActionArgs: parsed.Action.Args,
					Result:     question,
					Success:    true,
					Timestamp:  time.Now(),
				}
				session.Steps = append(session.Steps, step)

				// If user messages arrived while the model was deciding to ask, keep running and let the model continue.
				if len(rt.pending) > 0 {
					session.Messages = append(session.Messages, rt.pending...)
					rt.pending = nil
					session.Status = "running"
					session.Summary = ""
					session.CompletedAt = nil
					_ = e.saveSession(session)
					rt.mu.Unlock()
					continue
				}

				session.Status = "paused"
				session.Summary = question
				_ = e.saveSession(session)
				rt.mu.Unlock()
				return
			}

			// Allow AI to complete via action tool as a fallback (some models may prefer tool-style completion).
			if strings.EqualFold(strings.TrimSpace(parsed.Action.Tool), "complete_workflow") {
				summary, _ := parsed.Action.Args["summary"].(string)
				summary = strings.TrimSpace(summary)
				if summary == "" {
					summary = strings.TrimSpace(parsed.Thought)
				}

				// Same as <complete>: if follow-up messages exist, keep running.
				if len(rt.pending) > 0 {
					step := AIWorkflowStep{
						ID:         uuid.New().String(),
						Iteration:  iteration,
						Thought:    parsed.Thought,
						Action:     "complete_workflow",
						ActionArgs: parsed.Action.Args,
						Result:     summary,
						Success:    true,
						Timestamp:  time.Now(),
					}
					session.Steps = append(session.Steps, step)

					if e.toolExecutor != nil && session.Context != nil && strings.TrimSpace(getStringFromMap(session.Context, "terminal_id")) != "" {
						msg := summary
						if msg == "" {
							msg = "AI 托管已完成"
						}
						e.toolExecutor.emitTerminalAILog(session.Context, "info", "AI 托管完成：\n"+msg, "", "")
					}

					session.Messages = append(session.Messages, rt.pending...)
					rt.pending = nil
					session.Status = "running"
					session.Summary = ""
					session.CompletedAt = nil
					_ = e.saveSession(session)
					rt.mu.Unlock()
					continue
				}

				now := time.Now()
				session.Status = "completed"
				session.Summary = summary
				session.CompletedAt = &now
				_ = e.saveSession(session)
				rt.mu.Unlock()
				return
			}

			// Tool execution may take long; avoid blocking user message appends while running the tool.
			rt.mu.Unlock()
			step := e.executeAction(ctx, session, parsed, iteration)
			rt.mu.Lock()
			session.Steps = append(session.Steps, step)

			// Add observation to messages
			observation := FormatObservation(&ToolResult{
				Success: step.Success,
				Output:  step.Result,
			})
			session.Messages = append(session.Messages, ai.ChatMessage{
				Role:    "user",
				Content: observation,
			})

			if step.Success {
				repeatedFailureStreak = 0
				lastFailureKey = ""
			} else {
				failureKey := strings.TrimSpace(step.Action) + "|" + strings.TrimSpace(truncateString(step.Result, 240))
				if failureKey == lastFailureKey && failureKey != "" {
					repeatedFailureStreak++
				} else {
					lastFailureKey = failureKey
					repeatedFailureStreak = 1
				}
				if repeatedFailureStreak >= maxRepeatedFailureStreak {
					question := fmt.Sprintf("AI 连续 %d 次执行失败，已暂停以避免无效重试。\n最近失败信息：%s\n\n请确认/补充：\n- 是否允许继续重试或更换方案？\n- 是否需要执行 sudo/安装依赖/修改配置？\n- 是否有你期望的具体命令或检查方向？", maxRepeatedFailureStreak, strings.TrimSpace(truncateString(step.Result, 800)))
					guardStep := AIWorkflowStep{
						ID:         uuid.New().String(),
						Iteration:  iteration,
						Thought:    "guardrail: repeated_action_failure",
						Action:     "ask_user",
						ActionArgs: map[string]any{"question": question},
						Result:     question,
						Success:    true,
						Timestamp:  time.Now(),
					}
					session.Steps = append(session.Steps, guardStep)
					if len(rt.pending) > 0 {
						session.Messages = append(session.Messages, rt.pending...)
						rt.pending = nil
					}
					session.Status = "paused"
					session.Summary = question
					session.CompletedAt = nil
					_ = e.saveSession(session)
					rt.mu.Unlock()
					return
				}
			}
		}

		// Respect external pause/cancel right before saving this iteration.
		if e.applyExternalControl(session) {
			_ = e.saveSession(session)
			rt.mu.Unlock()
			return
		}
		_ = e.saveSession(session)
		rt.mu.Unlock()
	}

	rt.mu.Lock()
	session.Status = "failed"
	if strings.TrimSpace(session.Summary) == "" {
		session.Summary = "达到最大迭代次数"
	}
	_ = e.saveSession(session)
	rt.mu.Unlock()
}

// executeAction executes a tool action
func (e *AIWorkflowEngine) executeAction(ctx context.Context, session *AIWorkflowSession, parsed *ParsedResponse, iteration int) AIWorkflowStep {
	step := AIWorkflowStep{
		ID:         uuid.New().String(),
		Iteration:  iteration,
		Thought:    parsed.Thought,
		Action:     parsed.Action.Tool,
		ActionArgs: parsed.Action.Args,
		Timestamp:  time.Now(),
	}

	if e.toolExecutor == nil {
		step.Success = false
		step.Result = "工具执行器未配置"
		return step
	}

	result := e.toolExecutor.Execute(ctx, parsed.Action.Tool, parsed.Action.Args, session.Context)
	step.Success = result.Success
	step.Result = result.Output
	if !result.Success && result.Error != "" {
		step.Result = fmt.Sprintf("%s: %s", result.Error, result.Output)
	}

	return step
}

// saveSession saves session to database
func (e *AIWorkflowEngine) saveSession(session *AIWorkflowSession) error {
	if model.DB == nil {
		return errors.New("database not initialized")
	}

	stepsJSON, _ := json.Marshal(session.Steps)
	messagesJSON, _ := json.Marshal(session.Messages)
	contextJSON, _ := json.Marshal(session.Context)

	record := &model.AIWorkflowSession{
		ID:          session.ID,
		WorkflowID:  session.WorkflowID,
		UserGoal:    session.UserGoal,
		Status:      session.Status,
		Messages:    string(messagesJSON),
		Steps:       string(stepsJSON),
		Context:     string(contextJSON),
		Summary:     session.Summary,
		StartedAt:   session.StartedAt,
		CompletedAt: session.CompletedAt,
	}

	return model.DB.Save(record).Error
}

// GetSession retrieves a session by ID
func (e *AIWorkflowEngine) GetSession(id string) (*AIWorkflowSession, error) {
	var record model.AIWorkflowSession
	if err := model.DB.First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	session := &AIWorkflowSession{
		ID:          record.ID,
		WorkflowID:  record.WorkflowID,
		UserGoal:    record.UserGoal,
		Status:      record.Status,
		Summary:     record.Summary,
		StartedAt:   record.StartedAt,
		CompletedAt: record.CompletedAt,
	}

	json.Unmarshal([]byte(record.Messages), &session.Messages)
	json.Unmarshal([]byte(record.Steps), &session.Steps)
	json.Unmarshal([]byte(record.Context), &session.Context)

	return session, nil
}
