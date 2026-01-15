package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	promptsvc "github.com/ai-coding-assistant/service/prompt"
)

func TestPromptTemplateEndpoints_AdminCanListUpdateAndReset(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewPromptTemplateController()
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	listReq := httptest.NewRequest("GET", "/api/prompt-templates/", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != 200 {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) == 0 {
		t.Fatalf("expected non-empty prompt templates list")
	}

	updatePayload := bytes.NewBufferString(`{"template":"hello {{.extra_rules}}"}`)
	updateReq := httptest.NewRequest("PUT", "/api/prompt-templates/"+promptsvc.TemplateKeyApprovalSystemPrompt, updatePayload)
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

	var updateBody struct {
		Item map[string]any `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updateBody.Item["key"] != promptsvc.TemplateKeyApprovalSystemPrompt {
		t.Fatalf("unexpected updated key: %v", updateBody.Item["key"])
	}

	resetReq := httptest.NewRequest("POST", "/api/prompt-templates/"+promptsvc.TemplateKeyApprovalSystemPrompt+"/reset", nil)
	resetReq.Header.Set("Authorization", "Bearer "+token)
	resetResp, err := app.Test(resetReq)
	if err != nil {
		t.Fatalf("reset request failed: %v", err)
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != 200 {
		t.Fatalf("expected reset status 200, got %d", resetResp.StatusCode)
	}
}
