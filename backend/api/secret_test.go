package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	secretservice "github.com/ai-coding-assistant/service/secret"
	"github.com/gofiber/fiber/v2"
)

func setupSecretTestApp(t *testing.T) (*fiber.App, *SecretController) {
	t.Helper()

	dsn := fmt.Sprintf("file:secret_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		c.Locals("role", "admin")
		return c.Next()
	})

	adminGroup := apiGroup.Group("", middleware.RequireRole("admin"))

	ctrl := NewSecretController("test-master-key")
	ctrl.RegisterRoutes(adminGroup)

	return app, ctrl
}

func TestSecretController_CRUD(t *testing.T) {
	app, ctrl := setupSecretTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"name":"my-key","type":"api_key","plaintext":"abc123","meta":"{}"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createResp.StatusCode)
	}

	var createBody struct {
		Item model.Secret `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty secret id")
	}
	if createBody.Item.Name != "my-key" {
		t.Fatalf("expected name %q, got %q", "my-key", createBody.Item.Name)
	}
	if createBody.Item.Type != "api_key" {
		t.Fatalf("expected type %q, got %q", "api_key", createBody.Item.Type)
	}
	if createBody.Item.Meta != "{}" {
		t.Fatalf("expected meta %q, got %q", "{}", createBody.Item.Meta)
	}

	var stored model.Secret
	if err := model.DB.First(&stored, "id = ?", createBody.Item.ID).Error; err != nil {
		t.Fatalf("query stored secret failed: %v", err)
	}
	if stored.Ciphertext == "" {
		t.Fatalf("expected stored ciphertext to be non-empty")
	}
	if stored.Ciphertext == "abc123" {
		t.Fatalf("expected stored ciphertext not equal plaintext")
	}
	plaintext, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, stored.Ciphertext)
	if err != nil {
		t.Fatalf("decrypt stored ciphertext failed: %v", err)
	}
	if plaintext != "abc123" {
		t.Fatalf("expected plaintext %q, got %q", "abc123", plaintext)
	}

	listReq := httptest.NewRequest("GET", "/api/secrets", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.Secret `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(listBody.Items))
	}
	if listBody.Items[0].ID != createBody.Item.ID {
		t.Fatalf("expected secret id %q, got %q", createBody.Item.ID, listBody.Items[0].ID)
	}

	updateReq := httptest.NewRequest("PUT", "/api/secrets/"+createBody.Item.ID, bytes.NewBufferString(`{"name":"my-key-updated","plaintext":"new-value","meta":"{\"purpose\":\"test\"}"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	var updateBody struct {
		Item model.Secret `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updateBody.Item.Name != "my-key-updated" {
		t.Fatalf("expected updated name %q, got %q", "my-key-updated", updateBody.Item.Name)
	}
	if strings.TrimSpace(updateBody.Item.Meta) == "" {
		t.Fatalf("expected meta to be non-empty after update")
	}

	var storedAfterUpdate model.Secret
	if err := model.DB.First(&storedAfterUpdate, "id = ?", createBody.Item.ID).Error; err != nil {
		t.Fatalf("query stored secret after update failed: %v", err)
	}
	updatedPlaintext, err := secretservice.DecryptAESGCMBase64(ctrl.encryptionKey, storedAfterUpdate.Ciphertext)
	if err != nil {
		t.Fatalf("decrypt updated ciphertext failed: %v", err)
	}
	if updatedPlaintext != "new-value" {
		t.Fatalf("expected updated plaintext %q, got %q", "new-value", updatedPlaintext)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/secrets/"+createBody.Item.ID, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}

	deleteMissingReq := httptest.NewRequest("DELETE", "/api/secrets/"+createBody.Item.ID, nil)
	deleteMissingResp, err := app.Test(deleteMissingReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteMissingResp.Body.Close()
	if deleteMissingResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", deleteMissingResp.StatusCode)
	}
}

func TestSecretController_ValidationAndNotFound(t *testing.T) {
	app, _ := setupSecretTestApp(t)

	invalidBodyReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"name":`))
	invalidBodyReq.Header.Set("Content-Type", "application/json")
	invalidBodyResp, err := app.Test(invalidBodyReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidBodyResp.Body.Close()
	if invalidBodyResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidBodyResp.StatusCode)
	}

	missingNameReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"type":"api_key","plaintext":"abc123","meta":"{}"}`))
	missingNameReq.Header.Set("Content-Type", "application/json")
	missingNameResp, err := app.Test(missingNameReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer missingNameResp.Body.Close()
	if missingNameResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", missingNameResp.StatusCode)
	}

	invalidTypeReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"name":"x","type":"bad","plaintext":"abc123","meta":"{}"}`))
	invalidTypeReq.Header.Set("Content-Type", "application/json")
	invalidTypeResp, err := app.Test(invalidTypeReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidTypeResp.Body.Close()
	if invalidTypeResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidTypeResp.StatusCode)
	}

	invalidMetaReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"name":"x","type":"api_key","plaintext":"abc123","meta":"not-json"}`))
	invalidMetaReq.Header.Set("Content-Type", "application/json")
	invalidMetaResp, err := app.Test(invalidMetaReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidMetaResp.Body.Close()
	if invalidMetaResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidMetaResp.StatusCode)
	}

	missingPlaintextReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"name":"x","type":"api_key","meta":"{}"}`))
	missingPlaintextReq.Header.Set("Content-Type", "application/json")
	missingPlaintextResp, err := app.Test(missingPlaintextReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer missingPlaintextResp.Body.Close()
	if missingPlaintextResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", missingPlaintextResp.StatusCode)
	}

	// Create one secret for update-body validation
	createReq := httptest.NewRequest("POST", "/api/secrets", bytes.NewBufferString(`{"name":"valid","type":"api_key","plaintext":"abc123","meta":"{}"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createResp.StatusCode)
	}

	var createBody struct {
		Item model.Secret `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}

	updateInvalidBodyReq := httptest.NewRequest("PUT", "/api/secrets/"+createBody.Item.ID, bytes.NewBufferString(`{"name":`))
	updateInvalidBodyReq.Header.Set("Content-Type", "application/json")
	updateInvalidBodyResp, err := app.Test(updateInvalidBodyReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateInvalidBodyResp.Body.Close()
	if updateInvalidBodyResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", updateInvalidBodyResp.StatusCode)
	}

	updateInvalidMetaReq := httptest.NewRequest("PUT", "/api/secrets/"+createBody.Item.ID, bytes.NewBufferString(`{"meta":"not-json"}`))
	updateInvalidMetaReq.Header.Set("Content-Type", "application/json")
	updateInvalidMetaResp, err := app.Test(updateInvalidMetaReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateInvalidMetaResp.Body.Close()
	if updateInvalidMetaResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", updateInvalidMetaResp.StatusCode)
	}

	updateNotFoundReq := httptest.NewRequest("PUT", "/api/secrets/not-exist", bytes.NewBufferString(`{"name":"x"}`))
	updateNotFoundReq.Header.Set("Content-Type", "application/json")
	updateNotFoundResp, err := app.Test(updateNotFoundReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateNotFoundResp.Body.Close()
	if updateNotFoundResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", updateNotFoundResp.StatusCode)
	}

	deleteNotFoundReq := httptest.NewRequest("DELETE", "/api/secrets/not-exist", nil)
	deleteNotFoundResp, err := app.Test(deleteNotFoundReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteNotFoundResp.Body.Close()
	if deleteNotFoundResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", deleteNotFoundResp.StatusCode)
	}
}
