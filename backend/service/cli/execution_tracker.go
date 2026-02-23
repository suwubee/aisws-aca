package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	EventTypeStarted   = "started"
	EventTypeOutput    = "output"
	EventTypeProgress  = "progress"
	EventTypeCompleted = "completed"
	EventTypeError     = "error"
	EventTypeReview    = "review"
)

const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusError     = "error"
	StatusTimeout   = "timeout"
	StatusCancelled = "cancelled"
)

const (
	ExecutionRolePrimary = "primary"
	ExecutionRoleReview  = "review"
	ExecutionRoleReplay  = "replay"
	ExecutionRoleAudit   = "audit"
)

const (
	maxPromptPreviewRunes = 500
	maxPayloadBytes       = 16 * 1024
)

type StartExecutionInput struct {
	TaskID            *string
	TerminalID        *string
	WorkflowRunID     *string
	WorkflowSessionID *string
	ParentExecutionID *string
	Role              string
	Tool              string
	Mode              string
	Source            string
	Prompt            string
	Metadata          map[string]any
}

type ExecutionTracker struct {
	db *gorm.DB
}

func NewExecutionTracker(db *gorm.DB) *ExecutionTracker {
	if db == nil {
		return nil
	}
	return &ExecutionTracker{db: db}
}

func (t *ExecutionTracker) Start(input StartExecutionInput) (*model.CLIExecution, error) {
	if t == nil || t.db == nil {
		return nil, errors.New("execution tracker not initialized")
	}

	now := time.Now()
	metaJSON, _ := marshalPayload(input.Metadata)
	exec := &model.CLIExecution{
		ID:                uuid.New().String(),
		TaskID:            sanitizeStringPtr(input.TaskID),
		TerminalID:        sanitizeStringPtr(input.TerminalID),
		WorkflowRunID:     sanitizeStringPtr(input.WorkflowRunID),
		WorkflowSessionID: sanitizeStringPtr(input.WorkflowSessionID),
		ParentExecutionID: sanitizeStringPtr(input.ParentExecutionID),
		Role:              ensureExecutionRole(input.Role),
		Tool:              defaultString(strings.TrimSpace(input.Tool), "shell"),
		Mode:              defaultString(strings.TrimSpace(input.Mode), "command"),
		Source:            defaultString(strings.TrimSpace(input.Source), "workflow"),
		PromptPreview:     truncateRunes(strings.TrimSpace(input.Prompt), maxPromptPreviewRunes),
		Status:            StatusRunning,
		Metadata:          string(metaJSON),
		StartedAt:         now,
		UpdatedAt:         now,
	}

	if err := t.db.Create(exec).Error; err != nil {
		return nil, err
	}

	return exec, nil
}

func (t *ExecutionTracker) AppendEvent(executionID, eventType string, payload map[string]any) error {
	if t == nil || t.db == nil {
		return errors.New("execution tracker not initialized")
	}

	executionID = strings.TrimSpace(executionID)
	eventType = strings.TrimSpace(eventType)
	if executionID == "" || eventType == "" {
		return errors.New("executionID and eventType are required")
	}

	body, err := marshalPayload(payload)
	if err != nil {
		return err
	}

	record := &model.CLIExecutionEvent{
		ExecutionID: executionID,
		EventType:   eventType,
		Payload:     string(body),
		CreatedAt:   time.Now(),
	}
	if err := t.db.Create(record).Error; err != nil {
		return err
	}

	return t.db.Model(&model.CLIExecution{}).Where("id = ?", executionID).Update("updated_at", time.Now()).Error
}

func (t *ExecutionTracker) Complete(executionID, status string, exitCode *int, errMsg string) error {
	if t == nil || t.db == nil {
		return errors.New("execution tracker not initialized")
	}

	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return errors.New("executionID is required")
	}

	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case StatusCompleted, StatusError, StatusTimeout, StatusCancelled:
	default:
		status = StatusCompleted
	}

	now := time.Now()
	updates := map[string]any{
		"status":        status,
		"completed_at":  now,
		"updated_at":    now,
		"error_message": strings.TrimSpace(errMsg),
	}
	if exitCode != nil {
		updates["exit_code"] = *exitCode
	}

	return t.db.Model(&model.CLIExecution{}).Where("id = ?", executionID).Updates(updates).Error
}

type ListExecutionsInput struct {
	Status            string
	TaskID            string
	WorkflowSessionID string
	ParentExecutionID string
	Role              string
	Mode              string
	Source            string
	Tool              string
	Limit             int
}

func ListExecutions(status string, limit int) ([]model.CLIExecution, error) {
	return ListExecutionsByFilter(ListExecutionsInput{
		Status: status,
		Limit:  limit,
	})
}

func ListExecutionsByFilter(input ListExecutionsInput) ([]model.CLIExecution, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := model.DB.Model(&model.CLIExecution{})
	if s := strings.TrimSpace(input.Status); s != "" {
		query = query.Where("status = ?", s)
	}
	if taskID := strings.TrimSpace(input.TaskID); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if workflowSessionID := strings.TrimSpace(input.WorkflowSessionID); workflowSessionID != "" {
		query = query.Where("workflow_session_id = ?", workflowSessionID)
	}
	if parentID := strings.TrimSpace(input.ParentExecutionID); parentID != "" {
		if strings.EqualFold(parentID, "root") {
			query = query.Where("parent_execution_id IS NULL")
		} else {
			query = query.Where("parent_execution_id = ?", parentID)
		}
	}
	if strings.TrimSpace(input.Role) != "" {
		role := normalizeExecutionRole(input.Role)
		if role == "" {
			role = strings.ToLower(strings.TrimSpace(input.Role))
		}
		query = query.Where("role = ?", role)
	}
	if mode := strings.TrimSpace(input.Mode); mode != "" {
		query = query.Where("mode = ?", mode)
	}
	if source := strings.TrimSpace(input.Source); source != "" {
		query = query.Where("source = ?", source)
	}
	if tool := strings.TrimSpace(input.Tool); tool != "" {
		query = query.Where("tool = ?", tool)
	}

	var rows []model.CLIExecution
	if err := query.Order("updated_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListExecutionEvents(executionID string, after uint64, limit int) ([]model.CLIExecutionEvent, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, errors.New("executionID is required")
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	query := model.DB.Model(&model.CLIExecutionEvent{}).Where("execution_id = ?", executionID)
	if after > 0 {
		query = query.Where("seq > ?", after)
	}

	var rows []model.CLIExecutionEvent
	if err := query.Order("seq asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func normalizeExecutionRole(raw string) string {
	role := strings.ToLower(strings.TrimSpace(raw))
	switch role {
	case ExecutionRolePrimary, ExecutionRoleReview, ExecutionRoleReplay, ExecutionRoleAudit:
		return role
	default:
		return ""
	}
}

func ensureExecutionRole(raw string) string {
	role := normalizeExecutionRole(raw)
	if role == "" {
		return ExecutionRolePrimary
	}
	return role
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit])
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func sanitizeStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func marshalPayload(payload map[string]any) ([]byte, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(body) <= maxPayloadBytes {
		return body, nil
	}

	truncated := map[string]any{
		"truncated": true,
		"preview":   string(body[:maxPayloadBytes]),
	}
	return json.Marshal(truncated)
}
