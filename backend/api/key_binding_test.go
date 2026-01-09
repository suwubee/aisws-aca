package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/service/keybinding"
)

func TestKeyBindingEndpoints_AdminCanListUpdateAndReset(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewKeyBindingController()
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	listReq := httptest.NewRequest("GET", "/api/key-bindings/", nil)
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
	if len(listBody.Items) != len(keybinding.SupportedIDs()) {
		t.Fatalf("expected %d key bindings, got %d", len(keybinding.SupportedIDs()), len(listBody.Items))
	}

	updatePayload := bytes.NewBufferString(`{"label":"Enter（确认）","pty_input":"\\r","tmux_keys":"C-m","tmux_literal":false}`)
	updateReq := httptest.NewRequest("PUT", "/api/key-bindings/"+keybinding.IDEnter, updatePayload)
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

	resetReq := httptest.NewRequest("POST", "/api/key-bindings/"+keybinding.IDEnter+"/reset", nil)
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
