package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func addWebSocketUpgradeHeaders(req *http.Request) {
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
}

func setupTerminalWebSocketAuthTestApp(t *testing.T) *fiber.App {
	t.Helper()

	t.Setenv("JWT_SECRET", "test-secret")

	app := fiber.New()
	ctrl := NewTerminalController(nil)
	ctrl.RegisterWebSocket(app)

	return app
}

func TestTerminalController_WebSocketAuth_MissingTokenReturns401(t *testing.T) {
	app := setupTerminalWebSocketAuthTestApp(t)

	req := httptest.NewRequest("GET", "/api/terminal/ws", nil)
	addWebSocketUpgradeHeaders(req)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["error"] != "Missing authorization token" {
		t.Fatalf("expected error %q, got %v", "Missing authorization token", body["error"])
	}
}

func TestTerminalController_WebSocketAuth_InvalidTokenReturns401(t *testing.T) {
	app := setupTerminalWebSocketAuthTestApp(t)

	req := httptest.NewRequest("GET", "/api/terminal/ws?token=invalid-token", nil)
	addWebSocketUpgradeHeaders(req)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["error"] != "Invalid or expired token" {
		t.Fatalf("expected error %q, got %v", "Invalid or expired token", body["error"])
	}
}

func TestTerminalController_WebSocketAuth_ValidTokenDoesNotReturn401(t *testing.T) {
	app := setupTerminalWebSocketAuthTestApp(t)

	now := time.Now()
	expiresAt := now.Add(time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      "user-id",
		"username": "alice",
		"role":     "user",
		"exp":      expiresAt.Unix(),
		"iat":      now.Unix(),
	})

	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/terminal/ws?token="+signed, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		t.Fatalf("expected non-401 status, got %d", resp.StatusCode)
	}
}

