package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/ai"
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
	ID           string                `json:"id"`
	WorkflowID   string                `json:"workflow_id"`
	UserGoal     string                `json:"user_goal"`
	Status       string                `json:"status"` // running, completed, failed, paused
	Messages     []ai.ChatMessage      `json:"messages"`
	Steps        []AIWorkflowStep      `json:"steps"`
	Context      map[string]any        `json:"context"`
	StartedAt    time.Time             `json:"started_at"`
	CompletedAt  *time.Time            `json:"completed_at"`
	Summary      string                `json:"summary"`
}

// AIWorkflowStep represents a single step in AI workflow
type AIWorkflowStep struct {
	ID          string     `json:"id"`
	Iteration   int        `json:"iteration"`
	Thought     string     `json:"thought"`
	Action      string     `json:"action"`
	ActionArgs  any        `json:"action_args"`
	Result      string     `json:"result"`
	Success     bool       `json:"success"`
	Timestamp   time.Time  `json:"timestamp"`
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
	systemPrompt := FormatToolsForPrompt()
	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user goal
	session.Messages = append(session.Messages, ai.ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("请帮我完成以下任务：\n\n%s", userGoal),
	})

	// Save session to DB
	if err := e.saveSession(session); err != nil {
		utils.Warn("Failed to save workflow session", zap.Error(err))
	}

	// Start execution loop in background
	go e.executeLoop(context.Background(), session, aiConfig)

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

	for i := 0; i < e.maxIterations; i++ {
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
			step := e.executeAction(ctx, session, parsed, i)
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
