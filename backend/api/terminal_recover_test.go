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
)

func newTerminalManagerForRecoverTest() *terminal.Manager {
	return terminal.NewManager(&config.TerminalConfig{
		DefaultShell:    "/bin/bash",
		ScrollbackBytes: 8 * 1024,
		IdleTimeout:     time.Minute,
		MaxSessions:     8,
	})
}

func TestTerminalRecover_ResumeRunningSession(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := newTerminalManagerForRecoverTest()
	ctrl := NewTerminalController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	session, err := manager.CreateSession("resume-running", nil)
	if err != nil {
		t.Fatalf("create terminal session failed: %v", err)
	}
	defer func() { _ = manager.CloseSession(session.ID()) }()

	req := httptest.NewRequest(
		"POST",
		"/api/terminals/"+session.ID()+"/recover",
		bytes.NewBufferString(`{"mode":"resume"}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("recover request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Action string `json:"action"`
		Item   struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Action != "resumed" {
		t.Fatalf("expected action resumed, got %q", body.Action)
	}
	if body.Item.ID != session.ID() {
		t.Fatalf("expected resumed id %q, got %q", session.ID(), body.Item.ID)
	}
	if body.Item.Status != "running" {
		t.Fatalf("expected resumed status running, got %q", body.Item.Status)
	}
}

func TestTerminalRecover_ResumeUnavailableReturns409(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := newTerminalManagerForRecoverTest()
	ctrl := NewTerminalController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	missingID := "terminal-missing-resume"
	if err := model.DB.Create(&model.TerminalSession{
		ID:        missingID,
		Title:     "missing",
		Status:    "exited",
		Shell:     "bash",
		CreatedAt: time.Now().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("create terminal db session failed: %v", err)
	}

	req := httptest.NewRequest(
		"POST",
		"/api/terminals/"+missingID+"/recover",
		bytes.NewBufferString(`{"mode":"resume"}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("recover request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 409 {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["action"] != "resume_unavailable" {
		t.Fatalf("expected action resume_unavailable, got %v", body["action"])
	}
}

func TestTerminalRecover_ContinueCreatesNewSession(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := newTerminalManagerForRecoverTest()
	ctrl := NewTerminalController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	taskID := "task-continue"
	task := model.Task{
		ID:             taskID,
		Title:          "Continue Task",
		Status:         "in_progress",
		AutomationMode: "cli",
		WorkDir:        "/tmp",
		OrderIndex:     1,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	origin, err := manager.CreateSession("origin-terminal", &taskID)
	if err != nil {
		t.Fatalf("create origin terminal failed: %v", err)
	}
	originID := origin.ID()
	if err := manager.CloseSession(originID); err != nil {
		t.Fatalf("close origin terminal failed: %v", err)
	}

	req := httptest.NewRequest(
		"POST",
		"/api/terminals/"+originID+"/recover",
		bytes.NewBufferString(`{"mode":"continue","title":"续跑终端"}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("recover request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Action string `json:"action"`
		Item   struct {
			ID     string  `json:"id"`
			Title  string  `json:"title"`
			TaskID *string `json:"task_id"`
			Status string  `json:"status"`
		} `json:"item"`
		SourceTerminalID string `json:"source_terminal_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if body.Action != "continued" {
		t.Fatalf("expected action continued, got %q", body.Action)
	}
	if body.SourceTerminalID != originID {
		t.Fatalf("expected source terminal id %q, got %q", originID, body.SourceTerminalID)
	}
	if body.Item.ID == "" || body.Item.ID == originID {
		t.Fatalf("expected new terminal id, got %q", body.Item.ID)
	}
	if body.Item.Title != "续跑终端" {
		t.Fatalf("expected title 续跑终端, got %q", body.Item.Title)
	}
	if body.Item.TaskID == nil || *body.Item.TaskID != taskID {
		t.Fatalf("expected task id %q, got %v", taskID, body.Item.TaskID)
	}
	if body.Item.Status != "running" {
		t.Fatalf("expected new session running, got %q", body.Item.Status)
	}

	defer func() { _ = manager.CloseSession(body.Item.ID) }()
}
