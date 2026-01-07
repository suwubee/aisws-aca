package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
)

func setupTestAppWithAuth(t *testing.T) (*fiber.App, *config.AuthConfig, fiber.Router) {
	t.Helper()

	dsn := fmt.Sprintf("file:api_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	authCfg := &config.AuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiration: time.Hour,
		Username:      "admin",
		Password:      "admin123",
	}

	app := fiber.New()
	apiGroup := app.Group("/api")

	ctrl := NewAuthController(authCfg)
	app.Post("/api/auth/login", ctrl.Login)

	apiGroup.Use(middleware.AuthMiddleware(authCfg))

	return app, authCfg, apiGroup
}

func loginForToken(t *testing.T, app *fiber.App, username, password string) string {
	t.Helper()

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected login status 200, got %d", resp.StatusCode)
	}

	var body LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode login response failed: %v", err)
	}
	if body.Token == "" {
		t.Fatalf("expected non-empty token")
	}
	return body.Token
}
