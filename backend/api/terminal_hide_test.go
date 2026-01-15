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

func TestTerminalHide_PersistsAndDoesNotDeleteSession(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := terminal.NewManager(&config.TerminalConfig{
		DefaultShell:    "/bin/bash",
		ScrollbackBytes: 8 * 1024,
		IdleTimeout:     time.Minute,
		MaxSessions:     5,
	})

	ctrl := NewTerminalController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	session, err := manager.CreateSession("test", nil)
	if err != nil {
		t.Fatalf("create terminal session failed: %v", err)
	}
	terminalID := session.ID()
	if terminalID == "" {
		t.Fatalf("expected terminal id")
	}
	t.Cleanup(func() { _ = manager.CloseSession(terminalID) })

	// simulate missing db record (historical bug)
	if err := model.DB.Where("id = ?", terminalID).Delete(&model.TerminalSession{}).Error; err != nil {
		t.Fatalf("delete terminal session record failed: %v", err)
	}

	// hide terminal
	hideReq := httptest.NewRequest("POST", "/api/terminals/"+terminalID+"/hide", bytes.NewBufferString(`{"hidden":true}`))
	hideReq.Header.Set("Authorization", "Bearer "+token)
	hideReq.Header.Set("Content-Type", "application/json")
	hideResp, err := app.Test(hideReq)
	if err != nil {
		t.Fatalf("hide terminal request failed: %v", err)
	}
	defer hideResp.Body.Close()
	if hideResp.StatusCode != 200 {
		t.Fatalf("expected hide status 200, got %d", hideResp.StatusCode)
	}

	// list without show_hidden: should NOT include (hidden persists across refresh)
	listReq := httptest.NewRequest("GET", "/api/terminals", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list terminals request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}
	var listBody struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	for _, item := range listBody.Items {
		if item.ID == terminalID {
			t.Fatalf("expected hidden terminal to be excluded from default list")
		}
	}

	// list with show_hidden: should include and report hidden=true
	listAllReq := httptest.NewRequest("GET", "/api/terminals?show_hidden=true", nil)
	listAllReq.Header.Set("Authorization", "Bearer "+token)
	listAllResp, err := app.Test(listAllReq)
	if err != nil {
		t.Fatalf("list all terminals request failed: %v", err)
	}
	defer listAllResp.Body.Close()
	if listAllResp.StatusCode != 200 {
		t.Fatalf("expected list all status 200, got %d", listAllResp.StatusCode)
	}
	var listAllBody struct {
		Items []struct {
			ID     string `json:"id"`
			Hidden bool   `json:"hidden"`
		} `json:"items"`
	}
	if err := json.NewDecoder(listAllResp.Body).Decode(&listAllBody); err != nil {
		t.Fatalf("decode list all response failed: %v", err)
	}
	found := false
	for _, item := range listAllBody.Items {
		if item.ID == terminalID {
			found = true
			if !item.Hidden {
				t.Fatalf("expected hidden=true, got false")
			}
		}
	}
	if !found {
		t.Fatalf("expected hidden terminal to appear when show_hidden=true")
	}
}
