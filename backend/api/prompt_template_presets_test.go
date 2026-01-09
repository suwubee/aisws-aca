package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/middleware"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
)

func TestPromptTemplatePresetEndpoints_AdminCanCreateApplyAndDelete(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewPromptTemplateController()
	ctrl.RegisterRoutes(apiGroup.Group("", middleware.RequireRole("admin")))

	token := loginForToken(t, app, "admin", "admin123")
	key := promptsvc.TemplateKeyApprovalSystemPrompt

	listReq := httptest.NewRequest("GET", "/api/prompt-templates/"+key+"/presets", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list presets request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected list presets status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list presets response failed: %v", err)
	}
	if len(listBody.Items) == 0 {
		t.Fatalf("expected non-empty presets list")
	}

	createPayload := bytes.NewBufferString(`{"name":"自定义-1","template":"hello {{.extra_rules}}"}`)
	createReq := httptest.NewRequest("POST", "/api/prompt-templates/"+key+"/presets", createPayload)
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create preset request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 200 {
		t.Fatalf("expected create preset status 200, got %d", createResp.StatusCode)
	}

	var createBody struct {
		Item map[string]any `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create preset response failed: %v", err)
	}
	presetID, _ := createBody.Item["id"].(string)
	if presetID == "" {
		t.Fatalf("expected created preset id")
	}

	applyReq := httptest.NewRequest("POST", "/api/prompt-templates/"+key+"/presets/"+presetID+"/apply", nil)
	applyReq.Header.Set("Authorization", "Bearer "+token)
	applyResp, err := app.Test(applyReq)
	if err != nil {
		t.Fatalf("apply preset request failed: %v", err)
	}
	defer applyResp.Body.Close()
	if applyResp.StatusCode != 200 {
		t.Fatalf("expected apply preset status 200, got %d", applyResp.StatusCode)
	}

	var applyBody struct {
		Item map[string]any `json:"item"`
	}
	if err := json.NewDecoder(applyResp.Body).Decode(&applyBody); err != nil {
		t.Fatalf("decode apply preset response failed: %v", err)
	}
	if applyBody.Item["active_preset_id"] != presetID {
		t.Fatalf("expected active_preset_id to be %q, got %v", presetID, applyBody.Item["active_preset_id"])
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/prompt-templates/"+key+"/presets/"+presetID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete preset request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected delete preset status 200, got %d", deleteResp.StatusCode)
	}

	getReq := httptest.NewRequest("GET", "/api/prompt-templates/"+key, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("get template request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("expected get template status 200, got %d", getResp.StatusCode)
	}

	var getBody struct {
		Item map[string]any `json:"item"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get template response failed: %v", err)
	}
	if getBody.Item["active_preset_id"] != "" {
		t.Fatalf("expected active_preset_id to be cleared after delete")
	}
}
