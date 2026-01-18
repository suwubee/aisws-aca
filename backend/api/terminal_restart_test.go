package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/google/uuid"
)

func TestTerminalController_RestartTerminal_UpdatesTaskAndChain(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := terminal.NewManager(&config.TerminalConfig{
		DefaultShell:    "/bin/bash",
		ScrollbackBytes: 8 * 1024,
		IdleTimeout:     time.Minute,
		MaxSessions:     5,
	})
	t.Cleanup(func() { _ = manager.CloseAllSessions() })

	ctrl := NewTerminalController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	now := time.Now()
	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:        taskID,
		UserID:    uuid.NewString(),
		Title:     "task",
		Status:    "in_progress",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	session, err := manager.CreateSession("term", &taskID)
	if err != nil {
		t.Fatalf("create terminal session failed: %v", err)
	}
	oldID := session.ID()
	if oldID == "" {
		t.Fatalf("expected terminal id")
	}
	t.Cleanup(func() { _ = manager.CloseSession(oldID) })

	if err := model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"active_terminal_id": oldID,
		"ai_status":          "running",
		"ai_pause_reason":    "",
		"updated_at":         time.Now(),
	}).Error; err != nil {
		t.Fatalf("update task failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/terminals/"+oldID+"/restart", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("restart request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected restart status 200, got %d", resp.StatusCode)
	}

	var body struct {
		NewTerminalID string `json:"new_terminal_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.NewTerminalID == "" || body.NewTerminalID == oldID {
		t.Fatalf("expected new terminal id different from old, got %q", body.NewTerminalID)
	}

	var task model.Task
	if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if task.ActiveTerminalID == nil || *task.ActiveTerminalID != body.NewTerminalID {
		t.Fatalf("expected task active_terminal_id %q, got %v", body.NewTerminalID, task.ActiveTerminalID)
	}

	var oldTerm model.TerminalSession
	if err := model.DB.First(&oldTerm, "id = ?", oldID).Error; err != nil {
		t.Fatalf("query old terminal failed: %v", err)
	}
	if oldTerm.ReplacedByTerminalID == nil || *oldTerm.ReplacedByTerminalID != body.NewTerminalID {
		t.Fatalf("expected replaced_by_terminal_id %q, got %v", body.NewTerminalID, oldTerm.ReplacedByTerminalID)
	}
	if oldTerm.CloseReason != "restart" {
		t.Fatalf("expected close_reason %q, got %q", "restart", oldTerm.CloseReason)
	}

	var newTerm model.TerminalSession
	if err := model.DB.First(&newTerm, "id = ?", body.NewTerminalID).Error; err != nil {
		t.Fatalf("query new terminal failed: %v", err)
	}
	if newTerm.TaskID == nil || *newTerm.TaskID != taskID {
		t.Fatalf("expected new terminal task_id %q, got %v", taskID, newTerm.TaskID)
	}
}
