package approval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/utils"
	"go.uber.org/zap"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	dsn := fmt.Sprintf("file:approval_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if utils.Logger == nil {
		utils.Logger = zap.NewNop()
	}
}

func TestEngine_RecordApproval_PersistsServerIDWhenAvailable(t *testing.T) {
	setupTestDB(t)

	serverID := "srv-1"
	task := model.Task{
		ID:       "task-1",
		Title:    "task-1",
		ServerID: &serverID,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	taskID := task.ID
	terminalID := "terminal-1"
	terminal := model.TerminalSession{
		ID:        terminalID,
		TaskID:    &taskID,
		RuleMode:  "system",
		CreatedAt: time.Now(),
	}
	if err := model.DB.Create(&terminal).Error; err != nil {
		t.Fatalf("create terminal failed: %v", err)
	}

	engine := NewEngine()
	if err := engine.RecordApproval(terminalID, nil, "yes_no", "Allow? (y/n)", "yes", false, "", ""); err != nil {
		t.Fatalf("RecordApproval failed: %v", err)
	}

	var records []model.ApprovalRecord
	if err := model.DB.Find(&records).Error; err != nil {
		t.Fatalf("query approval records failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 approval record, got %d", len(records))
	}
	if records[0].ServerID == nil {
		t.Fatalf("expected server_id to be set")
	}
	if got := *records[0].ServerID; got != serverID {
		t.Fatalf("expected server_id %q, got %q", serverID, got)
	}
}

func TestEngine_RecordApproval_LeavesServerIDNilWhenTerminalMissing(t *testing.T) {
	setupTestDB(t)

	engine := NewEngine()
	if err := engine.RecordApproval("missing-terminal", nil, "yes_no", "Allow? (y/n)", "yes", false, "", ""); err != nil {
		t.Fatalf("RecordApproval failed: %v", err)
	}

	var records []model.ApprovalRecord
	if err := model.DB.Find(&records).Error; err != nil {
		t.Fatalf("query approval records failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 approval record, got %d", len(records))
	}
	if records[0].ServerID != nil {
		t.Fatalf("expected server_id nil, got %v", *records[0].ServerID)
	}
}

func TestEngine_Evaluate_CreatesNotificationWithServerIDWhenAvailable(t *testing.T) {
	setupTestDB(t)

	systemRule := model.RuleSet{
		ID:                "system-rule",
		Name:              "System",
		Type:              "system",
		ApprovalMode:      "manual",
		BlacklistPatterns: `["rm -rf"]`,
		NotifyOnBlock:     true,
		NotifyOnApprove:   false,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := model.DB.Create(&systemRule).Error; err != nil {
		t.Fatalf("create system rule failed: %v", err)
	}

	serverID := "srv-1"
	task := model.Task{
		ID:       "task-1",
		Title:    "task-1",
		ServerID: &serverID,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	taskID := task.ID
	terminalID := "terminal-1"
	terminal := model.TerminalSession{
		ID:        terminalID,
		TaskID:    &taskID,
		RuleMode:  "system",
		CreatedAt: time.Now(),
	}
	if err := model.DB.Create(&terminal).Error; err != nil {
		t.Fatalf("create terminal failed: %v", err)
	}

	engine := NewEngine()
	if _, err := engine.Evaluate(context.Background(), terminalID, "rm -rf / (y/n)"); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	var messages []model.Message
	if err := model.DB.Find(&messages).Error; err != nil {
		t.Fatalf("query messages failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ServerID == nil {
		t.Fatalf("expected message server_id to be set")
	}
	if got := *messages[0].ServerID; got != serverID {
		t.Fatalf("expected message server_id %q, got %q", serverID, got)
	}
	if messages[0].TaskID == nil || *messages[0].TaskID != taskID {
		if messages[0].TaskID == nil {
			t.Fatalf("expected message task_id %q, got nil", taskID)
		}
		t.Fatalf("expected message task_id %q, got %q", taskID, *messages[0].TaskID)
	}
}
