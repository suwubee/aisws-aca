package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/google/uuid"
)

func TestTaskController_ListTaskAISessions(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTaskController(nil)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	now := time.Now()
	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "todo",
		AutomationMode: "cli",
		CLIType:        "claude",
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	s1 := model.AISession{
		ID:         uuid.NewString(),
		TerminalID: "t-1",
		TaskID:     taskID,
		AIType:     "claude-code",
		State:      "ready",
		SessionID:  "11111111-1111-1111-1111-111111111111",
		CreatedAt:  now,
		UpdatedAt:  now.Add(-time.Minute),
	}
	s2 := model.AISession{
		ID:         uuid.NewString(),
		TerminalID: "t-2",
		TaskID:     taskID,
		AIType:     "codex",
		State:      "working",
		SessionID:  "22222222-2222-2222-2222-222222222222",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(&s1).Error; err != nil {
		t.Fatalf("create ai session 1 failed: %v", err)
	}
	if err := model.DB.Create(&s2).Error; err != nil {
		t.Fatalf("create ai session 2 failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/ai-sessions?limit=50", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items []model.AISession `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(body.Items))
	}
	// Ordered by updated_at desc
	if body.Items[0].ID != s2.ID {
		t.Fatalf("expected first item %q, got %q", s2.ID, body.Items[0].ID)
	}
	if body.Items[1].ID != s1.ID {
		t.Fatalf("expected second item %q, got %q", s1.ID, body.Items[1].ID)
	}
}

func TestTaskController_DiscoverTaskAISessions_LocalClaude_ScopeTask(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTaskController(nil)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := "/tmp/myproj"
	projectKey := "-tmp-myproj"
	projectDir := filepath.Join(home, ".claude", "projects", projectKey)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := uuid.NewString()
	sessionFile := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionFile, []byte("{\"type\":\"test\"}\n"), 0o600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	now := time.Now()
	_ = os.Chtimes(sessionFile, now.Add(-time.Minute), now.Add(-time.Minute))

	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "todo",
		AutomationMode: "cli",
		CLIType:        "claude",
		WorkDir:        workDir,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/ai-sessions/discover?tool=claude&scope=task&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items []DiscoveredTaskAISession `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
	if body.Items[0].Tool != "claude" {
		t.Fatalf("expected tool claude, got %q", body.Items[0].Tool)
	}
	if body.Items[0].SessionID != sessionID {
		t.Fatalf("expected session_id %q, got %q", sessionID, body.Items[0].SessionID)
	}
	if body.Items[0].Imported {
		t.Fatalf("expected imported=false")
	}
}

func TestTaskController_DiscoverTaskAISessions_LocalCodex_FiltersByWorkDir(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTaskController(nil)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := "/tmp/myproj"
	sessionID := "11111111-1111-1111-1111-111111111111"
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "02", "04")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	sessionFile := filepath.Join(sessionDir, "rollout-2026-02-04T12-00-00-"+sessionID+".jsonl")
	firstLine := `{"type":"session_meta","payload":{"id":"` + sessionID + `","cwd":"` + workDir + `"}}` + "\n"
	if err := os.WriteFile(sessionFile, []byte(firstLine), 0o600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	now := time.Now()
	_ = os.Chtimes(sessionFile, now.Add(-time.Minute), now.Add(-time.Minute))

	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "todo",
		AutomationMode: "cli",
		CLIType:        "codex",
		WorkDir:        workDir,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/ai-sessions/discover?tool=codex&scope=task&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items []DiscoveredTaskAISession `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
	if body.Items[0].Tool != "codex" {
		t.Fatalf("expected tool codex, got %q", body.Items[0].Tool)
	}
	if body.Items[0].SessionID != sessionID {
		t.Fatalf("expected session_id %q, got %q", sessionID, body.Items[0].SessionID)
	}
	if strings.TrimSpace(body.Items[0].CWD) != workDir {
		t.Fatalf("expected cwd %q, got %q", workDir, body.Items[0].CWD)
	}
}

func TestTaskController_CollectTaskAISessions_ImportsDiscovered(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTaskController(nil)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := "/tmp/myproj"
	projectKey := "-tmp-myproj"
	projectDir := filepath.Join(home, ".claude", "projects", projectKey)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sessionID := uuid.NewString()
	sessionFile := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionFile, []byte("{\"type\":\"test\"}\n"), 0o600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	now := time.Now()
	_ = os.Chtimes(sessionFile, now.Add(-time.Hour), now.Add(-time.Hour))

	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "todo",
		AutomationMode: "cli",
		CLIType:        "claude",
		WorkDir:        workDir,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/ai-sessions/collect?tool=claude&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		ImportedCount int `json:"imported_count"`
		ExistingCount int `json:"existing_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.ImportedCount != 1 {
		t.Fatalf("expected imported_count=1, got %d", body.ImportedCount)
	}
	if body.ExistingCount != 0 {
		t.Fatalf("expected existing_count=0, got %d", body.ExistingCount)
	}

	var got model.AISession
	if err := model.DB.First(&got, "task_id = ? AND ai_type = ? AND session_id = ?", taskID, "claude-code", sessionID).Error; err != nil {
		t.Fatalf("expected imported ai session record, got error: %v", err)
	}

	// idempotent: collect again should not create duplicates
	req2 := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/ai-sessions/collect?tool=claude&limit=10", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request2 failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp2.StatusCode)
	}
	var body2 struct {
		ImportedCount int `json:"imported_count"`
		ExistingCount int `json:"existing_count"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&body2); err != nil {
		t.Fatalf("decode response2 failed: %v", err)
	}
	if body2.ImportedCount != 0 {
		t.Fatalf("expected imported_count=0, got %d", body2.ImportedCount)
	}
	if body2.ExistingCount == 0 {
		t.Fatalf("expected existing_count>0")
	}
}

func TestTaskController_ImportTaskAISession_CreatesAISession(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTaskController(nil)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	taskID := uuid.NewString()
	now := time.Now()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "todo",
		AutomationMode: "cli",
		CLIType:        "claude",
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	sessionID := "11111111-1111-1111-1111-111111111111"
	bodyJSON := `{"tool":"claude","session_id":"` + sessionID + `","session_file":"/tmp/x.jsonl"}`
	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/ai-sessions/import", strings.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var got model.AISession
	if err := model.DB.First(&got, "task_id = ? AND ai_type = ? AND session_id = ?", taskID, "claude-code", sessionID).Error; err != nil {
		t.Fatalf("expected imported ai session, got error: %v", err)
	}
}

func TestTaskController_ResumeTaskAISession_CreatesTerminalAndUpdatesTask(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := terminal.NewManager(&config.TerminalConfig{
		DefaultShell:    "/bin/bash",
		ScrollbackBytes: 8 * 1024,
		IdleTimeout:     time.Minute,
		MaxSessions:     5,
	})
	t.Cleanup(func() { _ = manager.CloseAllSessions() })

	ctrl := NewTaskController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	now := time.Now()
	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "paused",
		AutomationMode: "cli",
		CLIType:        "claude",
		WorkDir:        "",
		AutoCreateDir:  true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	aiSessionID := uuid.NewString()
	if err := model.DB.Create(&model.AISession{
		ID:         aiSessionID,
		TerminalID: "old-terminal",
		TaskID:     taskID,
		AIType:     "claude-code",
		State:      "ready",
		SessionID:  "123e4567-e89b-12d3-a456-426614174000",
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("create ai session failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/ai-sessions/"+aiSessionID+"/resume", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		TerminalID    string `json:"terminal_id"`
		WorkDir       string `json:"work_dir"`
		ResumeCommand string `json:"resume_command"`
		CLIStarted    bool   `json:"cli_started"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.TerminalID == "" {
		t.Fatalf("expected terminal_id")
	}
	if body.ResumeCommand != "claude --resume 123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("unexpected resume_command: %q", body.ResumeCommand)
	}
	if !body.CLIStarted {
		t.Fatalf("expected cli_started=true")
	}
	if body.WorkDir == "" {
		t.Fatalf("expected work_dir to be set")
	}

	var gotTask model.Task
	if err := model.DB.First(&gotTask, "id = ?", taskID).Error; err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if gotTask.Status != "in_progress" {
		t.Fatalf("expected task status in_progress, got %q", gotTask.Status)
	}
	if gotTask.ActiveTerminalID == nil || *gotTask.ActiveTerminalID != body.TerminalID {
		t.Fatalf("expected active_terminal_id %q, got %v", body.TerminalID, gotTask.ActiveTerminalID)
	}
	if gotTask.AIStatus != "running" {
		t.Fatalf("expected ai_status running, got %q", gotTask.AIStatus)
	}

	var gotTerminal model.TerminalSession
	if err := model.DB.First(&gotTerminal, "id = ?", body.TerminalID).Error; err != nil {
		t.Fatalf("query terminal failed: %v", err)
	}
	if gotTerminal.TaskID == nil || *gotTerminal.TaskID != taskID {
		t.Fatalf("expected terminal.task_id %q, got %v", taskID, gotTerminal.TaskID)
	}
}

func TestTaskController_ResumeTaskAISession_UnsupportedTool(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := terminal.NewManager(&config.TerminalConfig{
		DefaultShell:    "/bin/bash",
		ScrollbackBytes: 8 * 1024,
		IdleTimeout:     time.Minute,
		MaxSessions:     5,
	})
	t.Cleanup(func() { _ = manager.CloseAllSessions() })

	ctrl := NewTaskController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	now := time.Now()
	taskID := uuid.NewString()
	if err := model.DB.Create(&model.Task{
		ID:             taskID,
		Title:          "task",
		Status:         "todo",
		AutomationMode: "cli",
		CLIType:        "gemini",
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	aiSessionID := uuid.NewString()
	if err := model.DB.Create(&model.AISession{
		ID:         aiSessionID,
		TerminalID: "old-terminal",
		TaskID:     taskID,
		AIType:     "gemini",
		State:      "ready",
		SessionID:  "x",
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("create ai session failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/ai-sessions/"+aiSessionID+"/resume", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}
