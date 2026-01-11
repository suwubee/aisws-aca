package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestTerminalDefaultsEndpoints_AdminCanUpdate_NonAdminReadonly(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewTerminalDefaultsController()
	ctrl.RegisterRoutes(apiGroup)

	adminToken := loginForToken(t, app, "admin", "admin123")

	createTestUser(t, "user1", "user1@example.com", "user123", "user", "active")
	userToken := loginForToken(t, app, "user1", "user123")

	getReq := httptest.NewRequest("GET", "/api/terminal-defaults", nil)
	getReq.Header.Set("Authorization", "Bearer "+userToken)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("expected GET status 200, got %d", getResp.StatusCode)
	}

	var getBody struct {
		Item struct {
			DefaultLoginDir string `json:"default_login_dir"`
		} `json:"item"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode GET response failed: %v", err)
	}
	if getBody.Item.DefaultLoginDir == "" {
		t.Fatalf("expected non-empty default_login_dir")
	}

	userPutReq := httptest.NewRequest("PUT", "/api/terminal-defaults", bytes.NewBufferString(`{"default_login_dir":"/tmp"}`))
	userPutReq.Header.Set("Authorization", "Bearer "+userToken)
	userPutReq.Header.Set("Content-Type", "application/json")
	userPutResp, err := app.Test(userPutReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer userPutResp.Body.Close()
	if userPutResp.StatusCode != 403 {
		t.Fatalf("expected non-admin PUT status 403, got %d", userPutResp.StatusCode)
	}

	adminPutReq := httptest.NewRequest("PUT", "/api/terminal-defaults", bytes.NewBufferString(`{"default_login_dir":"/tmp"}`))
	adminPutReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminPutReq.Header.Set("Content-Type", "application/json")
	adminPutResp, err := app.Test(adminPutReq)
	if err != nil {
		t.Fatalf("admin PUT request failed: %v", err)
	}
	defer adminPutResp.Body.Close()
	if adminPutResp.StatusCode != 200 {
		t.Fatalf("expected admin PUT status 200, got %d", adminPutResp.StatusCode)
	}

	var putBody struct {
		Item struct {
			DefaultLoginDir string `json:"default_login_dir"`
		} `json:"item"`
	}
	if err := json.NewDecoder(adminPutResp.Body).Decode(&putBody); err != nil {
		t.Fatalf("decode PUT response failed: %v", err)
	}
	if putBody.Item.DefaultLoginDir != "/tmp" {
		t.Fatalf("expected updated default_login_dir %q, got %q", "/tmp", putBody.Item.DefaultLoginDir)
	}
}

