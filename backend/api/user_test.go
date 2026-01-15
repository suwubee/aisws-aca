package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
)

func TestUserController_ListAndUpdate_AdminOnly(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	userController := NewUserController()
	userController.RegisterRoutes(apiGroup.Group("", middleware.RequireRole("admin")))

	admin := createTestUser(t, "admin", "admin@example.com", "adminpass", "admin", "active")
	regular := createTestUser(t, "bob", "bob@example.com", "password123", "user", "active")

	adminToken := loginForToken(t, app, admin.Username, "adminpass")
	regularToken := loginForToken(t, app, regular.Username, "password123")

	listReq := httptest.NewRequest("GET", "/api/users", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.User `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}

	found := make(map[string]model.User, len(listBody.Items))
	for _, item := range listBody.Items {
		found[item.ID] = item
	}
	if _, ok := found[admin.ID]; !ok {
		t.Fatalf("expected admin user to be present")
	}
	if _, ok := found[regular.ID]; !ok {
		t.Fatalf("expected regular user to be present")
	}

	updateReq := httptest.NewRequest(
		"PUT",
		fmt.Sprintf("/api/users/%s", regular.ID),
		bytes.NewBufferString(`{"role":"viewer","status":"disabled"}`),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	var updateBody struct {
		Item model.User `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updateBody.Item.ID != regular.ID {
		t.Fatalf("expected updated user id %q, got %q", regular.ID, updateBody.Item.ID)
	}
	if updateBody.Item.Role != "viewer" {
		t.Fatalf("expected role %q, got %q", "viewer", updateBody.Item.Role)
	}
	if updateBody.Item.Status != "disabled" {
		t.Fatalf("expected status %q, got %q", "disabled", updateBody.Item.Status)
	}

	var dbUser model.User
	if err := model.DB.First(&dbUser, "id = ?", regular.ID).Error; err != nil {
		t.Fatalf("query updated user failed: %v", err)
	}
	if dbUser.Role != "viewer" || dbUser.Status != "disabled" {
		t.Fatalf("expected user updated in db, got role=%q status=%q", dbUser.Role, dbUser.Status)
	}

	forbiddenListReq := httptest.NewRequest("GET", "/api/users", nil)
	forbiddenListReq.Header.Set("Authorization", "Bearer "+regularToken)
	forbiddenListResp, err := app.Test(forbiddenListReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer forbiddenListResp.Body.Close()
	if forbiddenListResp.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", forbiddenListResp.StatusCode)
	}

	forbiddenUpdateReq := httptest.NewRequest(
		"PUT",
		fmt.Sprintf("/api/users/%s", admin.ID),
		bytes.NewBufferString(`{"status":"disabled"}`),
	)
	forbiddenUpdateReq.Header.Set("Content-Type", "application/json")
	forbiddenUpdateReq.Header.Set("Authorization", "Bearer "+regularToken)
	forbiddenUpdateResp, err := app.Test(forbiddenUpdateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer forbiddenUpdateResp.Body.Close()
	if forbiddenUpdateResp.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", forbiddenUpdateResp.StatusCode)
	}
}

func TestUserController_Update_ValidationAndNotFound(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	userController := NewUserController()
	userController.RegisterRoutes(apiGroup.Group("", middleware.RequireRole("admin")))

	admin := createTestUser(t, "admin", "admin@example.com", "adminpass", "admin", "active")
	adminToken := loginForToken(t, app, admin.Username, "adminpass")

	notFoundReq := httptest.NewRequest("PUT", "/api/users/not-exist", bytes.NewBufferString(`{"role":"user"}`))
	notFoundReq.Header.Set("Content-Type", "application/json")
	notFoundReq.Header.Set("Authorization", "Bearer "+adminToken)
	notFoundResp, err := app.Test(notFoundReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer notFoundResp.Body.Close()
	if notFoundResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", notFoundResp.StatusCode)
	}

	invalidRoleReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/users/%s", admin.ID), bytes.NewBufferString(`{"role":"root"}`))
	invalidRoleReq.Header.Set("Content-Type", "application/json")
	invalidRoleReq.Header.Set("Authorization", "Bearer "+adminToken)
	invalidRoleResp, err := app.Test(invalidRoleReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer invalidRoleResp.Body.Close()
	if invalidRoleResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidRoleResp.StatusCode)
	}

	invalidStatusReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/users/%s", admin.ID), bytes.NewBufferString(`{"status":"blocked"}`))
	invalidStatusReq.Header.Set("Content-Type", "application/json")
	invalidStatusReq.Header.Set("Authorization", "Bearer "+adminToken)
	invalidStatusResp, err := app.Test(invalidStatusReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer invalidStatusResp.Body.Close()
	if invalidStatusResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidStatusResp.StatusCode)
	}
}
