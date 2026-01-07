package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/model"
)

func TestAutomationController_AgentConfigs_GetAndUpdate(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	automationController := NewAutomationController(nil)
	automationController.RegisterRoutes(apiGroup)

	createTestUser(t, "alice", "alice@example.com", "password123", "user", "active")
	token := loginForToken(t, app, "alice", "password123")

	getReq := httptest.NewRequest("GET", "/api/automation/agent-configs", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", getResp.StatusCode)
	}

	var getBody struct {
		Items []model.AgentConfig `json:"items"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode GET response failed: %v", err)
	}
	if len(getBody.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(getBody.Items))
	}

	updateReq := httptest.NewRequest(
		"PUT",
		"/api/automation/agent-configs",
		bytes.NewBufferString(`{"items":[{"agent_type":"codex","display_name":"OpenAI Codex","enabled":true,"priority":90,"detect_modes":[" (?i)codex ","","(?i)codex"]}]}`),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	getReq2 := httptest.NewRequest("GET", "/api/automation/agent-configs", nil)
	getReq2.Header.Set("Authorization", "Bearer "+token)
	getResp2, err := app.Test(getReq2)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp2.Body.Close()

	if getResp2.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", getResp2.StatusCode)
	}

	var getBody2 struct {
		Items []model.AgentConfig `json:"items"`
	}
	if err := json.NewDecoder(getResp2.Body).Decode(&getBody2); err != nil {
		t.Fatalf("decode GET response failed: %v", err)
	}
	if len(getBody2.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(getBody2.Items))
	}
	item := getBody2.Items[0]
	if item.AgentType != "codex" {
		t.Fatalf("expected agent_type %q, got %q", "codex", item.AgentType)
	}
	if item.DisplayName != "OpenAI Codex" {
		t.Fatalf("expected display_name %q, got %q", "OpenAI Codex", item.DisplayName)
	}
	if !item.Enabled {
		t.Fatalf("expected enabled true")
	}
	if item.Priority != 90 {
		t.Fatalf("expected priority %d, got %d", 90, item.Priority)
	}
	if len(item.DetectModes) != 1 || item.DetectModes[0] != "(?i)codex" {
		t.Fatalf("expected normalized detect_modes [(?i)codex], got %v", item.DetectModes)
	}

	clearReq := httptest.NewRequest("PUT", "/api/automation/agent-configs", bytes.NewBufferString(`{"items":[]}`))
	clearReq.Header.Set("Content-Type", "application/json")
	clearReq.Header.Set("Authorization", "Bearer "+token)
	clearResp, err := app.Test(clearReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer clearResp.Body.Close()
	if clearResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", clearResp.StatusCode)
	}

	getReq3 := httptest.NewRequest("GET", "/api/automation/agent-configs", nil)
	getReq3.Header.Set("Authorization", "Bearer "+token)
	getResp3, err := app.Test(getReq3)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp3.Body.Close()
	if getResp3.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", getResp3.StatusCode)
	}

	var getBody3 struct {
		Items []model.AgentConfig `json:"items"`
	}
	if err := json.NewDecoder(getResp3.Body).Decode(&getBody3); err != nil {
		t.Fatalf("decode GET response failed: %v", err)
	}
	if len(getBody3.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(getBody3.Items))
	}
}

func TestAutomationController_AgentConfigs_UpdateValidation(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	automationController := NewAutomationController(nil)
	automationController.RegisterRoutes(apiGroup)

	createTestUser(t, "alice", "alice@example.com", "password123", "user", "active")
	token := loginForToken(t, app, "alice", "password123")

	missingTypeReq := httptest.NewRequest(
		"PUT",
		"/api/automation/agent-configs",
		bytes.NewBufferString(`{"items":[{"agent_type":"","display_name":"x","enabled":true,"priority":1,"detect_modes":["x"]}]}`),
	)
	missingTypeReq.Header.Set("Content-Type", "application/json")
	missingTypeReq.Header.Set("Authorization", "Bearer "+token)
	missingTypeResp, err := app.Test(missingTypeReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer missingTypeResp.Body.Close()
	if missingTypeResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", missingTypeResp.StatusCode)
	}

	dupTypeReq := httptest.NewRequest(
		"PUT",
		"/api/automation/agent-configs",
		bytes.NewBufferString(`{"items":[{"agent_type":"codex","display_name":"x","enabled":true,"priority":1,"detect_modes":["x"]},{"agent_type":"codex","display_name":"y","enabled":false,"priority":2,"detect_modes":["y"]}]}`),
	)
	dupTypeReq.Header.Set("Content-Type", "application/json")
	dupTypeReq.Header.Set("Authorization", "Bearer "+token)
	dupTypeResp, err := app.Test(dupTypeReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer dupTypeResp.Body.Close()
	if dupTypeResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", dupTypeResp.StatusCode)
	}
}
