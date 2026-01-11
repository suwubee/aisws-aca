package api

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/middleware"
	"github.com/gofiber/fiber/v2"
)

func TestDemoMode_BlocksWrite_AllowsRead_AndAllowsLogout(t *testing.T) {
	app, authCfg, apiGroup := setupTestAppWithAuth(t)

	// Enable demo mode read-only guard for all authenticated routes.
	apiGroup.Use(middleware.DemoModeMiddleware(true))

	// Register a writable endpoint (terminal defaults) to verify it's blocked.
	terminalDefaults := NewTerminalDefaultsController()
	terminalDefaults.RegisterRoutes(apiGroup)

	// Register logout to verify allowlist.
	authCtrl := NewAuthController(authCfg)
	authCtrl.SetDemoMode(true)
	apiGroup.Post("/auth/logout", authCtrl.Logout)

	token := loginForToken(t, app, "admin", "admin123")

	getReq := httptest.NewRequest("GET", "/api/terminal-defaults", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	if getResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected GET status 200, got %d", getResp.StatusCode)
	}

	putReq := httptest.NewRequest("PUT", "/api/terminal-defaults", bytes.NewBufferString(`{"default_login_dir":"/tmp"}`))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := app.Test(putReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	if putResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected PUT status 403 in demo mode, got %d", putResp.StatusCode)
	}

	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutResp, err := app.Test(logoutReq)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	if logoutResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected logout status 200, got %d", logoutResp.StatusCode)
	}
}

