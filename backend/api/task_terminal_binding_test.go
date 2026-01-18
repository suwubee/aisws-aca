package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/google/uuid"
)

func TestTaskController_BindTerminal_AndResumeAI(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := terminal.NewManager(&config.TerminalConfig{
		DefaultShell:    "/bin/bash",
		ScrollbackBytes: 8 * 1024,
		IdleTimeout:     time.Minute,
		MaxSessions:     5,
	})
	t.Cleanup(func() { _ = manager.CloseAllSessions() })

	taskCtrl := NewTaskController(manager)
	taskCtrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	now := time.Now()
	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:        taskID,
		UserID:    uuid.NewString(),
		Title:     "task",
		Status:    "todo",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	session, err := manager.CreateSession("term", nil)
	if err != nil {
		t.Fatalf("create terminal session failed: %v", err)
	}
	terminalID := session.ID()
	if terminalID == "" {
		t.Fatalf("expected terminal id")
	}
	t.Cleanup(func() { _ = manager.CloseSession(terminalID) })

	bindReq := httptest.NewRequest(
		"POST",
		"/api/tasks/"+taskID+"/bind-terminal",
		bytes.NewBufferString(fmt.Sprintf(`{"terminal_id":%q}`, terminalID)),
	)
	bindReq.Header.Set("Authorization", "Bearer "+token)
	bindReq.Header.Set("Content-Type", "application/json")
	bindResp, err := app.Test(bindReq)
	if err != nil {
		t.Fatalf("bind terminal request failed: %v", err)
	}
	defer bindResp.Body.Close()
	if bindResp.StatusCode != 200 {
		t.Fatalf("expected bind status 200, got %d", bindResp.StatusCode)
	}

	var gotTask model.Task
	if err := model.DB.First(&gotTask, "id = ?", taskID).Error; err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if gotTask.ActiveTerminalID == nil || *gotTask.ActiveTerminalID != terminalID {
		t.Fatalf("expected active_terminal_id %q, got %v", terminalID, gotTask.ActiveTerminalID)
	}

	var gotTerminal model.TerminalSession
	if err := model.DB.First(&gotTerminal, "id = ?", terminalID).Error; err != nil {
		t.Fatalf("query terminal failed: %v", err)
	}
	if gotTerminal.TaskID == nil || *gotTerminal.TaskID != taskID {
		t.Fatalf("expected terminal.task_id %q, got %v", taskID, gotTerminal.TaskID)
	}

	// Resume AI: should set ai_status=running and clear ai_pause_reason.
	if err := model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"ai_status":       "paused",
		"ai_pause_reason": "terminal_disconnected",
		"updated_at":      time.Now(),
	}).Error; err != nil {
		t.Fatalf("set ai paused failed: %v", err)
	}

	resumeReq := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/resume", nil)
	resumeReq.Header.Set("Authorization", "Bearer "+token)
	resumeResp, err := app.Test(resumeReq)
	if err != nil {
		t.Fatalf("resume request failed: %v", err)
	}
	defer resumeResp.Body.Close()
	if resumeResp.StatusCode != 200 {
		t.Fatalf("expected resume status 200, got %d", resumeResp.StatusCode)
	}

	if err := model.DB.First(&gotTask, "id = ?", taskID).Error; err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if gotTask.AIStatus != "running" {
		t.Fatalf("expected ai_status %q, got %q", "running", gotTask.AIStatus)
	}
	if gotTask.AIPauseReason != "" {
		t.Fatalf("expected ai_pause_reason to be empty, got %q", gotTask.AIPauseReason)
	}

	// Binding the same terminal to another task should fail.
	otherTaskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:        otherTaskID,
		UserID:    uuid.NewString(),
		Title:     "other",
		Status:    "todo",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create other task failed: %v", err)
	}

	bindOtherReq := httptest.NewRequest(
		"POST",
		"/api/tasks/"+otherTaskID+"/bind-terminal",
		bytes.NewBufferString(fmt.Sprintf(`{"terminal_id":%q}`, terminalID)),
	)
	bindOtherReq.Header.Set("Authorization", "Bearer "+token)
	bindOtherReq.Header.Set("Content-Type", "application/json")
	bindOtherResp, err := app.Test(bindOtherReq)
	if err != nil {
		t.Fatalf("bind other request failed: %v", err)
	}
	defer bindOtherResp.Body.Close()
	if bindOtherResp.StatusCode == 200 {
		t.Fatalf("expected bind other to fail, got status 200")
	}
	var body map[string]any
	_ = json.NewDecoder(bindOtherResp.Body).Decode(&body)
}
