package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	inflight      sync.Map // sessionID -> *aiWorkflowInflight
}

type aiWorkflowInflight struct {
	cancel context.CancelFunc

	pauseMu        sync.Mutex
	pauseRequested bool
	pauseReason    string
}

func (s *aiWorkflowInflight) requestPause(reason string) {
	if s == nil {
		return
	}
	s.pauseMu.Lock()
	s.pauseRequested = true
	s.pauseReason = strings.TrimSpace(reason)
	s.pauseMu.Unlock()
}

func (s *aiWorkflowInflight) paused() (bool, string) {
	if s == nil {
		return false, ""
	}
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	return s.pauseRequested, strings.TrimSpace(s.pauseReason)
}

func normalizePauseReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "用户手动暂停"
	}
	return trimmed
}

func isContextCancelErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}
	return strings.Contains(text, "context canceled") ||
		strings.Contains(text, "context cancelled") ||
		strings.Contains(text, "request canceled") ||
		strings.Contains(text, "request cancelled")
}

func shouldTreatChatErrorAsPause(ctx context.Context, state *aiWorkflowInflight, err error) (bool, string) {
	pauseRequested, pauseReason := state.paused()
	if !pauseRequested {
		return false, ""
	}
	if ctx != nil && ctx.Err() != nil {
		return true, normalizePauseReason(pauseReason)
	}
	if isContextCancelErr(err) {
		return true, normalizePauseReason(pauseReason)
	}
	return false, ""
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

const (
	workflowEventSessionStarted = "session_started"
	workflowEventSessionResumed = "session_resumed"
	workflowEventStepStarted    = "step_started"
	workflowEventToolCall       = "tool_call"
	workflowEventToolResult     = "tool_result"
	workflowEventNeedUserInput  = "need_user_input"
	workflowEventSessionPaused  = "session_paused"
	workflowEventSessionDone    = "session_completed"
	workflowEventSessionFailed  = "session_failed"
	workflowEventSessionError   = "session_error"
	workflowEventSessionCancel  = "session_cancelled"
	workflowEventAIResponse     = "ai_response"
)

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
	if strings.TrimSpace(getStringFromMap(session.Context, "workflow_session_id")) == "" {
		session.Context["workflow_session_id"] = session.ID
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
	e.appendWorkflowEvent(session, -1, "lifecycle", workflowEventSessionStarted, "会话已启动", map[string]any{
		"user_goal":       truncateForWorkflowEvent(goal, 800),
		"workflow_id":     workflowID,
		"context_keys":    mapKeys(session.Context),
		"context_preview": workflowContextPreview(session.Context),
		"message_preview": truncateForWorkflowEvent(userGoalPrompt, 1200),
	})

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

	runCtx, cancel := context.WithCancel(context.Background())
	state := &aiWorkflowInflight{cancel: cancel}
	if _, loaded := e.inflight.LoadOrStore(id, state); loaded {
		cancel()
		return false
	}

	go func() {
		defer e.inflight.Delete(id)
		e.executeLoop(runCtx, session, aiConfig, state)
	}()

	return true
}

// ResumeWorkflow appends a user message and resumes a workflow session.
func (e *AIWorkflowEngine) ResumeWorkflow(ctx context.Context, sessionID string, userMessage string) (*AIWorkflowSession, error) {
	if e == nil {
		return nil, errors.New("engine is nil")
	}

	id := strings.TrimSpace(sessionID)
	if id == "" {
		return nil, errors.New("session id required")
	}

	msg := strings.TrimSpace(userMessage)
	if msg == "" {
		return nil, errors.New("message is required")
	}

	session, err := e.GetSession(id)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(session.Status))
	resumable := status == "paused" || status == "completed" || status == "failed" || status == "cancelled" || status == "timeout"
	if !resumable {
		return nil, errors.New("session is not resumable")
	}
	shouldRestartTaskMonitor := status == "completed" || status == "failed" || status == "cancelled" || status == "timeout"

	aiConfig, err := e.aiProvider.GetDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("no AI provider configured: %w", err)
	}

	if session.Context == nil {
		session.Context = make(map[string]any)
	}
	if strings.TrimSpace(getStringFromMap(session.Context, "workflow_session_id")) == "" {
		session.Context["workflow_session_id"] = session.ID
	}

	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "user",
		Content: msg,
	})
	e.appendNativeConversationLog(session, "ai_input_native", msg)
	session.Status = "running"
	session.Summary = ""
	session.CompletedAt = nil

	if err := e.saveSession(session); err != nil {
		return nil, err
	}
	e.appendWorkflowEvent(session, len(session.Steps), "lifecycle", workflowEventSessionResumed, "会话已恢复", map[string]any{
		"message": truncateForWorkflowEvent(msg, 1200),
	})

	if !e.startExecution(session, aiConfig) {
		return session, errors.New("workflow session already running")
	}

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

// executeLoop runs the ReAct loop
func (e *AIWorkflowEngine) executeLoop(ctx context.Context, session *AIWorkflowSession, aiConfig *model.AIProviderConfig, state *aiWorkflowInflight) {
	defer func() {
		if r := recover(); r != nil {
			session.Status = "failed"
			session.Summary = fmt.Sprintf("panic: %v", r)
			e.saveSession(session)
			e.appendWorkflowEvent(session, len(session.Steps), "lifecycle", workflowEventSessionError, "会话异常退出", map[string]any{
				"panic": fmt.Sprintf("%v", r),
			})
		}
	}()

	iterationBase := 0
	if session != nil {
		iterationBase = len(session.Steps)
	}

	for i := 0; i < e.maxIterations; i++ {
		iteration := iterationBase + i
		select {
		case <-ctx.Done():
			pauseRequested, pauseReason := state.paused()
			if pauseRequested {
				if pauseReason == "" {
					pauseReason = "用户手动暂停"
				}
				session.Status = "paused"
				session.Summary = pauseReason
				session.CompletedAt = nil
				e.saveSession(session)
				e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionPaused, "会话已暂停", map[string]any{
					"reason": pauseReason,
					"source": "manual",
				})
				taskID := strings.TrimSpace(session.WorkflowID)
				if taskID == "" {
					taskID = strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
				}
				if taskID != "" {
					updateTaskStatus(taskID, "paused")
				}
				return
			}

			session.Status = "cancelled"
			e.saveSession(session)
			e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionCancel, "会话已取消", nil)
			return
		default:
		}
		phaseForStep := normalizeWorkflowPhase(getStringFromMap(session.Context, "workflow_phase"))
		if phaseForStep == "" {
			phaseForStep = "command"
		}
		e.appendWorkflowEvent(session, iteration, phaseForStep, workflowEventStepStarted, "开始新一轮迭代", map[string]any{
			"iteration": iteration,
		})

		// Call AI
		resp, err := e.aiProvider.Chat(ctx, aiConfig, session.Messages)
		if err != nil {
			utils.Error("AI call failed", zap.Error(err))
			if pausedByUser, pauseReason := shouldTreatChatErrorAsPause(ctx, state, err); pausedByUser {
				session.Status = "paused"
				session.Summary = pauseReason
				session.CompletedAt = nil
				e.saveSession(session)
				e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionPaused, "会话已暂停", map[string]any{
					"reason": pauseReason,
					"source": "manual",
				})
				taskID := strings.TrimSpace(session.WorkflowID)
				if taskID == "" {
					taskID = strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
				}
				if taskID != "" {
					updateTaskStatus(taskID, "paused")
				}
				return
			}
			if ctx.Err() != nil || isContextCancelErr(err) {
				session.Status = "cancelled"
				session.Summary = "会话已取消"
				session.CompletedAt = nil
				e.saveSession(session)
				e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionCancel, "会话已取消", map[string]any{
					"error": truncateForWorkflowEvent(err.Error(), 800),
				})
				return
			}
			session.Status = "failed"
			session.Summary = fmt.Sprintf("AI调用失败: %v", err)
			e.saveSession(session)
			e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionError, "AI 调用失败", map[string]any{
				"error": err.Error(),
			})
			return
		}

		if len(resp.Choices) == 0 {
			continue
		}

		aiResponse := resp.Choices[0].Message.Content
		session.Messages = append(session.Messages, ai.ChatMessage{
			Role:    "assistant",
			Content: aiResponse,
		})
		e.appendNativeConversationLog(session, "ai_output_native", aiResponse)
		e.appendWorkflowEvent(session, iteration, "command", workflowEventAIResponse, "收到 AI 响应", map[string]any{
			"response_preview": truncateForWorkflowEvent(aiResponse, 1200),
		})

		// Parse response
		parsed, err := ParseAIResponse(aiResponse)
		if err != nil {
			utils.Warn("Failed to parse AI response", zap.Error(err))
			e.appendWorkflowEvent(session, iteration, "command", workflowEventSessionError, "AI 响应解析失败", map[string]any{
				"error":            err.Error(),
				"response_preview": truncateForWorkflowEvent(aiResponse, 800),
			})
			session.Messages = append(session.Messages, ai.ChatMessage{
				Role: "user",
				Content: fmt.Sprintf(`<observation>
执行失败: 解析 AI 响应失败，错误：%s
请严格只返回以下结构之一：
1) <thought>...</thought> + <action>{"tool":"...","args":{...}}</action>
2) <thought>...</thought> + <complete>{"status":"success|partial|failed","summary":"..."}</complete>
</observation>`, truncateForWorkflowEvent(err.Error(), 400)),
			})
			continue
		}

		// Check if complete
		if parsed.Complete != nil {
			now := time.Now()
			session.Status = "completed"
			session.Summary = parsed.Complete.Summary
			session.CompletedAt = &now
			e.saveSession(session)
			e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionDone, "会话执行完成", map[string]any{
				"status":  parsed.Complete.Status,
				"summary": truncateForWorkflowEvent(parsed.Complete.Summary, 2000),
			})
			return
		}

		// Execute action
		if parsed.Action != nil {
			actionTool := strings.TrimSpace(parsed.Action.Tool)
			phase := inferWorkflowPhase(actionTool)
			e.appendWorkflowEvent(session, iteration, phase, workflowEventToolCall, "准备调用工具", map[string]any{
				"tool":    actionTool,
				"args":    parsed.Action.Args,
				"thought": truncateForWorkflowEvent(parsed.Thought, 1200),
			})

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
				e.appendWorkflowEvent(session, iteration, "review", workflowEventNeedUserInput, "需要用户输入", map[string]any{
					"question": truncateForWorkflowEvent(question, 2000),
				})

				session.Status = "paused"
				session.Summary = question
				e.saveSession(session)
				e.appendWorkflowEvent(session, iteration, "lifecycle", workflowEventSessionPaused, "会话已暂停", map[string]any{
					"reason": truncateForWorkflowEvent(question, 2000),
				})
				return
			}

			step := e.executeAction(ctx, session, parsed, iteration)
			session.Steps = append(session.Steps, step)
			e.appendWorkflowEvent(session, iteration, phase, workflowEventToolResult, "工具执行完成", map[string]any{
				"tool":           actionTool,
				"success":        step.Success,
				"result_preview": truncateForWorkflowEvent(step.Result, 2000),
			})

			// Add observation to messages
			observation := FormatObservation(&ToolResult{
				Success: step.Success,
				Output:  step.Result,
			})
			session.Messages = append(session.Messages, ai.ChatMessage{
				Role:    "user",
				Content: observation,
			})
		}

		e.saveSession(session)
	}

	session.Status = "failed"
	session.Summary = "达到最大迭代次数"
	e.saveSession(session)
	e.appendWorkflowEvent(session, len(session.Steps), "lifecycle", workflowEventSessionFailed, "会话失败", map[string]any{
		"reason": session.Summary,
	})
}

// PauseWorkflow requests pausing a running workflow session.
func (e *AIWorkflowEngine) PauseWorkflow(ctx context.Context, sessionID string, reason string) (*AIWorkflowSession, error) {
	if e == nil {
		return nil, errors.New("engine is nil")
	}
	_ = ctx

	id := strings.TrimSpace(sessionID)
	if id == "" {
		return nil, errors.New("session id required")
	}

	session, err := e.GetSession(id)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(session.Status))
	if status != "running" {
		return nil, errors.New("session is not running")
	}

	pauseReason := strings.TrimSpace(reason)
	if pauseReason == "" {
		pauseReason = "用户手动暂停"
	}

	if inflightRaw, ok := e.inflight.Load(id); ok {
		if inflight, ok := inflightRaw.(*aiWorkflowInflight); ok {
			inflight.requestPause(pauseReason)
			if inflight.cancel != nil {
				inflight.cancel()
			}
			session.Status = "paused"
			session.Summary = pauseReason
			session.CompletedAt = nil
			return session, nil
		}
	}

	// 会话非活跃时执行兜底暂停。
	session.Status = "paused"
	session.Summary = pauseReason
	session.CompletedAt = nil
	if err := e.saveSession(session); err != nil {
		return nil, err
	}
	e.appendWorkflowEvent(session, len(session.Steps), "lifecycle", workflowEventSessionPaused, "会话已暂停", map[string]any{
		"reason": pauseReason,
		"source": "manual",
	})

	taskID := strings.TrimSpace(session.WorkflowID)
	if taskID == "" {
		taskID = strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
	}
	if taskID != "" {
		updateTaskStatus(taskID, "paused")
	}

	return session, nil
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

	if session != nil {
		if session.Context == nil {
			session.Context = map[string]any{}
		}
		if strings.TrimSpace(getStringFromMap(session.Context, "workflow_session_id")) == "" {
			session.Context["workflow_session_id"] = strings.TrimSpace(session.ID)
		}
		session.Context["workflow_step_id"] = step.ID
		session.Context["workflow_iteration"] = strconv.Itoa(iteration)
		session.Context["workflow_phase"] = inferWorkflowPhase(parsed.Action.Tool)
	}

	result := e.toolExecutor.Execute(ctx, parsed.Action.Tool, parsed.Action.Args, session.Context)
	mergeToolResultContext(session.Context, result.Data)
	if strings.TrimSpace(getStringFromMap(session.Context, "workflow_session_id")) == "" {
		session.Context["workflow_session_id"] = strings.TrimSpace(session.ID)
	}
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
	if session.Context == nil {
		session.Context = map[string]any{}
	}
	if strings.TrimSpace(getStringFromMap(session.Context, "workflow_session_id")) == "" {
		session.Context["workflow_session_id"] = strings.TrimSpace(session.ID)
	}

	return session, nil
}

func (e *AIWorkflowEngine) appendWorkflowEvent(session *AIWorkflowSession, iteration int, phase, eventType, summary string, payload map[string]any) {
	if e == nil || session == nil || model.DB == nil {
		return
	}

	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return
	}

	workflowID := strings.TrimSpace(session.WorkflowID)
	if workflowID == "" {
		workflowID = strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
	}

	taskID := strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
	terminalID := strings.TrimSpace(getStringFromMap(session.Context, "terminal_id"))
	var taskIDPtr *string
	var terminalIDPtr *string
	if taskID != "" {
		taskIDCopy := taskID
		taskIDPtr = &taskIDCopy
	}
	if terminalID != "" {
		terminalIDCopy := terminalID
		terminalIDPtr = &terminalIDCopy
	}

	normalizedPhase := normalizeWorkflowPhase(phase)
	if normalizedPhase == "" {
		normalizedPhase = "lifecycle"
	}

	body, _ := json.Marshal(payload)
	record := &model.AIWorkflowEvent{
		SessionID:  sessionID,
		WorkflowID: workflowID,
		TaskID:     taskIDPtr,
		TerminalID: terminalIDPtr,
		Iteration:  iteration,
		Phase:      normalizedPhase,
		EventType:  strings.TrimSpace(eventType),
		Summary:    strings.TrimSpace(summary),
		Payload:    string(body),
		CreatedAt:  time.Now(),
	}
	_ = model.DB.Create(record).Error
}

func (e *AIWorkflowEngine) appendNativeConversationLog(session *AIWorkflowSession, logType, content string) {
	if e == nil || session == nil || model.DB == nil {
		return
	}

	normalizedType := strings.ToLower(strings.TrimSpace(logType))
	if normalizedType != "ai_input_native" && normalizedType != "ai_output_native" {
		return
	}

	terminalID := strings.TrimSpace(getStringFromMap(session.Context, "terminal_id"))
	if terminalID == "" {
		return
	}

	text := strings.TrimSpace(content)
	if text == "" {
		return
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	taskID := strings.TrimSpace(getStringFromMap(session.Context, "task_id"))
	if taskID == "" {
		taskID = strings.TrimSpace(session.WorkflowID)
	}

	duplicateSince := time.Now().Add(-5 * time.Second)
	query := model.DB.Model(&model.Log{}).
		Where("terminal_id = ? AND log_type = ? AND content = ? AND created_at >= ?", terminalID, normalizedType, text, duplicateSince)
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	var duplicateCount int64
	if err := query.Count(&duplicateCount).Error; err == nil && duplicateCount > 0 {
		return
	}

	terminalCopy := terminalID
	var taskIDPtr *string
	if taskID != "" {
		taskCopy := taskID
		taskIDPtr = &taskCopy
	}

	_ = model.DB.Create(&model.Log{
		ID:         uuid.NewString(),
		TerminalID: &terminalCopy,
		TaskID:     taskIDPtr,
		LogType:    normalizedType,
		Content:    text,
		CreatedAt:  time.Now(),
	}).Error
}

func normalizeWorkflowPhase(raw string) string {
	phase := strings.ToLower(strings.TrimSpace(raw))
	switch phase {
	case "lifecycle", "plan", "execute", "review", "command":
		return phase
	default:
		return ""
	}
}

func mapKeys(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	return keys
}

func workflowContextPreview(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}

	keys := []string{
		"task_id",
		"task_title",
		"task_description",
		"task_initial_prompt",
		"task_ai_prompt",
		"task_ai_end_condition",
		"task_ai_error_handling",
		"task_work_dir",
		"work_dir",
		"command_execution_mode",
		"terminal_id",
		"terminal_bootstrap_mode",
		"terminal_bootstrap_source_terminal_id",
		"terminal_bootstrap",
		"current_server_id",
		"target_server_ids",
		"running_command",
	}

	out := map[string]any{}
	for _, key := range keys {
		raw, ok := values[key]
		if !ok || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case string:
			text := truncateForWorkflowEvent(typed, 240)
			if text == "" {
				continue
			}
			out[key] = text
		case []string:
			if len(typed) == 0 {
				continue
			}
			copied := make([]string, 0, len(typed))
			for _, item := range typed {
				text := truncateForWorkflowEvent(item, 80)
				if text == "" {
					continue
				}
				copied = append(copied, text)
			}
			if len(copied) > 0 {
				out[key] = copied
			}
		default:
			text := truncateForWorkflowEvent(fmt.Sprintf("%v", typed), 240)
			if text == "" {
				continue
			}
			out[key] = text
		}
	}

	return out
}

func truncateForWorkflowEvent(input string, maxRunes int) string {
	text := strings.TrimSpace(input)
	if text == "" || maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + " ...(truncated)"
}

func inferWorkflowPhase(tool string) string {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "list_servers", "select_server", "create_task", "start_task":
		return "plan"
	case "execute_command", "batch_execute_command", "git_operation":
		return "execute"
	case "check_task_status", "get_terminal_logs":
		return "review"
	default:
		return "command"
	}
}

func mergeToolResultContext(ctx map[string]any, data any) {
	if ctx == nil || data == nil {
		return
	}

	switch typed := data.(type) {
	case map[string]any:
		for key, value := range typed {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			ctx[trimmed] = value
		}
	case map[string]string:
		for key, value := range typed {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			ctx[trimmed] = strings.TrimSpace(value)
		}
	}
}
