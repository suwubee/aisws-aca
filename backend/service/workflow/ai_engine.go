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
	}
	if _, loaded := e.inflight.LoadOrStore(id, rt); loaded {
		return false
	}

	go func() {
		defer e.inflight.Delete(id)
		e.executeLoop(context.Background(), rt)
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

	// Fast path: if the session is currently running in-memory, allow appending messages without "resumable" gating.
	if rtValue, ok := e.inflight.Load(id); ok {
		if rt, ok := rtValue.(*runtimeSession); ok && rt != nil && rt.session != nil {
			rt.mu.Lock()
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
			return session, nil
		}
	}

	session, err := e.GetSession(id)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(session.Status))
	resumable := status == "running" || status == "paused" || status == "completed" || status == "failed" || status == "cancelled" || status == "timeout"
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

	for i := 0; i < e.maxIterations; i++ {
		iteration := iterationBase + i
		select {
		case <-ctx.Done():
			session.Status = "cancelled"
			e.saveSession(session)
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
			_ = e.saveSession(session)
			rt.mu.Unlock()
			continue
		}

		// Check if complete
		if parsed.Complete != nil {
			now := time.Now()
			session.Status = "completed"
			session.Summary = parsed.Complete.Summary
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
