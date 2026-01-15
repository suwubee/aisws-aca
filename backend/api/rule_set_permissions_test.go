package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestRuleSetEndpoints_NonAdminCanManageNonSystemButNotSystem(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewAutomationController(nil)
	ctrl.RegisterRoutes(apiGroup)

	hashed, err := bcrypt.GenerateFromPassword([]byte("user123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	u := model.User{
		ID:           uuid.NewString(),
		Username:     "user1",
		Email:        "user1@example.com",
		PasswordHash: string(hashed),
		Role:         "user",
		Status:       "active",
	}
	if err := model.DB.Create(&u).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	token := loginForToken(t, app, "user1", "user123")

	createReq := httptest.NewRequest("POST", "/api/automation/rulesets", bytes.NewBufferString(`{"name":"terminal rules","approval_mode":"manual","auto_input_type":"yes","whitelist_patterns":[],"blacklist_patterns":[],"context_lines":50,"detect_claude_code":true,"detect_codex":true,"detect_gemini":true,"notify_on_block":true,"notify_on_approve":false}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 200 {
		t.Fatalf("expected create status 200, got %d", createResp.StatusCode)
	}

	var createBody struct {
		Item struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty ruleset id")
	}
	if createBody.Item.Type != "terminal" {
		t.Fatalf("expected ruleset type terminal, got %q", createBody.Item.Type)
	}

	updateReq := httptest.NewRequest("PUT", "/api/automation/rulesets/"+createBody.Item.ID, bytes.NewBufferString(`{"name":"updated","approval_mode":"manual","auto_input_type":"yes","whitelist_patterns":["(?i)ok"],"blacklist_patterns":[],"context_lines":50,"detect_claude_code":true,"detect_codex":true,"detect_gemini":true,"notify_on_block":true,"notify_on_approve":false}`))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != 200 {
		t.Fatalf("expected update status 200, got %d", updateResp.StatusCode)
	}

	createSystemReq := httptest.NewRequest("POST", "/api/automation/rulesets?type=system", bytes.NewBufferString(`{"name":"bad"}`))
	createSystemReq.Header.Set("Authorization", "Bearer "+token)
	createSystemReq.Header.Set("Content-Type", "application/json")
	createSystemResp, err := app.Test(createSystemReq)
	if err != nil {
		t.Fatalf("create system request failed: %v", err)
	}
	defer createSystemResp.Body.Close()
	if createSystemResp.StatusCode != 400 {
		t.Fatalf("expected create system status 400, got %d", createSystemResp.StatusCode)
	}

	getSysReq := httptest.NewRequest("GET", "/api/automation/system-rule", nil)
	getSysReq.Header.Set("Authorization", "Bearer "+token)
	getSysResp, err := app.Test(getSysReq)
	if err != nil {
		t.Fatalf("get system rule request failed: %v", err)
	}
	defer getSysResp.Body.Close()
	if getSysResp.StatusCode != 200 {
		t.Fatalf("expected get system rule status 200, got %d", getSysResp.StatusCode)
	}

	var getSysBody struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}
	if err := json.NewDecoder(getSysResp.Body).Decode(&getSysBody); err != nil {
		t.Fatalf("decode system rule response failed: %v", err)
	}
	if getSysBody.Item.ID == "" {
		t.Fatalf("expected non-empty system ruleset id")
	}

	updateSysReq := httptest.NewRequest("PUT", "/api/automation/rulesets/"+getSysBody.Item.ID, bytes.NewBufferString(`{"name":"hacked"}`))
	updateSysReq.Header.Set("Authorization", "Bearer "+token)
	updateSysReq.Header.Set("Content-Type", "application/json")
	updateSysResp, err := app.Test(updateSysReq)
	if err != nil {
		t.Fatalf("update system ruleset request failed: %v", err)
	}
	defer updateSysResp.Body.Close()
	if updateSysResp.StatusCode != 400 {
		t.Fatalf("expected update system ruleset status 400, got %d", updateSysResp.StatusCode)
	}

	putSysRuleReq := httptest.NewRequest("PUT", "/api/automation/system-rule", bytes.NewBufferString(`{"name":"hacked"}`))
	putSysRuleReq.Header.Set("Authorization", "Bearer "+token)
	putSysRuleReq.Header.Set("Content-Type", "application/json")
	putSysRuleResp, err := app.Test(putSysRuleReq)
	if err != nil {
		t.Fatalf("put system rule request failed: %v", err)
	}
	defer putSysRuleResp.Body.Close()
	if putSysRuleResp.StatusCode != 403 {
		t.Fatalf("expected update system rule status 403, got %d", putSysRuleResp.StatusCode)
	}
}

