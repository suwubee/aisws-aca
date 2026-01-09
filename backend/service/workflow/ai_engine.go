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
	inflight      sync.Map // sessionID -> struct{}
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

// StartWorkflow starts an AI-driven workflow with user goal
func (e *AIWorkflowEngine) StartWorkflow(ctx context.Context, userGoal string) (*AIWorkflowSession, error) {
	if strings.TrimSpace(userGoal) == "" {
		return nil, errors.New("user goal is required")
	}

	// Get AI config
	aiConfig, err := e.aiProvider.GetDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("no AI provider configured: %w", err)
	}

	// Create session
	session := &AIWorkflowSession{
		ID:        uuid.New().String(),
		UserGoal:  userGoal,
		Status:    "running",
		Messages:  []ai.ChatMessage{},
		Steps:     []AIWorkflowStep{},
		Context:   make(map[string]any),
		StartedAt: time.Now(),
	}

	// Build system prompt with tools
	systemPrompt, err := FormatToolsForPrompt()
	if err != nil {
		return nil, err
	}
	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user goal
	userGoalPrompt, err := promptsvc.RenderTemplate(promptsvc.TemplateKeyAIWorkflowUserGoalPrompt, map[string]any{
		"user_goal": userGoal,
	})
	if err != nil {
		userGoalPrompt = userGoal
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

	if _, loaded := e.inflight.LoadOrStore(id, struct{}{}); loaded {
		return false
	}

	go func() {
		defer e.inflight.Delete(id)
		e.executeLoop(context.Background(), session, aiConfig)
	}()

	return true
}

// ResumeWorkflow appends a user message and resumes a paused workflow session.
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
	if status != "paused" {
		return nil, errors.New("session is not paused")
	}

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

	if !e.startExecution(session, aiConfig) {
		return session, errors.New("workflow session already running")
	}

	return session, nil
}

// executeLoop runs the ReAct loop
func (e *AIWorkflowEngine) executeLoop(ctx context.Context, session *AIWorkflowSession, aiConfig *model.AIProviderConfig) {
	defer func() {
		if r := recover(); r != nil {
			session.Status = "failed"
			session.Summary = fmt.Sprintf("panic: %v", r)
			e.saveSession(session)
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
			session.Status = "cancelled"
			e.saveSession(session)
			return
		default:
		}

		// Call AI
		resp, err := e.aiProvider.Chat(ctx, aiConfig, session.Messages)
		if err != nil {
			utils.Error("AI call failed", zap.Error(err))
			session.Status = "failed"
			session.Summary = fmt.Sprintf("AI调用失败: %v", err)
			e.saveSession(session)
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

		// Parse response
		parsed, err := ParseAIResponse(aiResponse)
		if err != nil {
			utils.Warn("Failed to parse AI response", zap.Error(err))
			continue
		}

		// Check if complete
		if parsed.Complete != nil {
			now := time.Now()
			session.Status = "completed"
			session.Summary = parsed.Complete.Summary
			session.CompletedAt = &now
			e.saveSession(session)
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
				e.saveSession(session)
				return
			}

			step := e.executeAction(ctx, session, parsed, iteration)
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

		e.saveSession(session)
	}

	session.Status = "failed"
	session.Summary = "达到最大迭代次数"
	e.saveSession(session)
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
