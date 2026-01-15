package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/keybinding"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestKeyBindingEndpoints_NonAdminCanListButCannotUpdate(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	ctrl := NewKeyBindingController()
	ctrl.RegisterRoutes(apiGroup)

	_ = loginForToken(t, app, "admin", "admin123")

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

	listReq := httptest.NewRequest("GET", "/api/key-bindings", nil)
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
		t.Fatalf("expected non-empty key bindings list")
	}

	updatePayload := bytes.NewBufferString(`{"label":"x"}`)
	updateReq := httptest.NewRequest("PUT", "/api/key-bindings/"+keybinding.IDEnter, updatePayload)
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != 403 {
		t.Fatalf("expected update status 403 for non-admin, got %d", updateResp.StatusCode)
	}
}
