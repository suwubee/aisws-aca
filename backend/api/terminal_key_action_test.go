package api

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/service/terminal"
)

func TestTerminalKeyActionEndpoint_Returns500WhenManagerMissing(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTerminalController(nil)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	req := httptest.NewRequest("POST", "/api/terminals/any/key-action", bytes.NewBufferString(`{"action":"enter"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestTerminalKeyActionEndpoint_Returns404WhenTerminalMissing(t *testing.T) {
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

	req := httptest.NewRequest("POST", "/api/terminals/not-exist/key-action", bytes.NewBufferString(`{"action":"enter"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}
