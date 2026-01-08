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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthTestApp(t *testing.T) (*fiber.App, *config.AuthConfig) {
	t.Helper()

	dsn := fmt.Sprintf("file:auth_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

	// login不需要认证（但AuthMiddleware也会跳过）
	app.Post("/api/auth/login", ctrl.Login)

	// 需要认证的路由
	apiGroup.Use(middleware.AuthMiddleware(authCfg))
	apiGroup.Post("/auth/register", middleware.RequireRole("admin"), ctrl.Register)
	apiGroup.Post("/auth/reset-data", middleware.RequireRole("admin"), ctrl.ResetData)

	return app, authCfg
}

func createTestUser(t *testing.T, username, email, password, role, status string) model.User {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}

	user := model.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         role,
		Status:       status,
	}
	if err := model.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	return user
}

func TestAuthController_Login_SetsRoleClaimAndUpdatesLastLoginAt(t *testing.T) {
	app, authCfg := setupAuthTestApp(t)

	user := createTestUser(t, "alice", "alice@example.com", "password123", "viewer", "active")

	before := time.Now()
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"alice","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Token == "" {
		t.Fatalf("expected non-empty token")
	}
	if body.User.ID != user.ID {
		t.Fatalf("expected user id %q, got %q", user.ID, body.User.ID)
	}
	if body.User.Role != "viewer" {
		t.Fatalf("expected role %q, got %q", "viewer", body.User.Role)
	}

	parsed, err := jwt.Parse(body.Token, func(token *jwt.Token) (interface{}, error) {
		return []byte(authCfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse token failed: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("expected map claims")
	}
	if got, _ := claims["role"].(string); got != "viewer" {
		t.Fatalf("expected claim role %q, got %v", "viewer", claims["role"])
	}

	var updated model.User
	if err := model.DB.First(&updated, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("query user failed: %v", err)
	}
	if updated.LastLoginAt == nil {
		t.Fatalf("expected last_login_at to be set")
	}
	if updated.LastLoginAt.Before(before) {
		t.Fatalf("expected last_login_at >= %v, got %v", before, *updated.LastLoginAt)
	}
}

func TestAuthController_Login_ByEmail(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	createTestUser(t, "bob", "bob@example.com", "password123", "user", "active")

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"bob@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestAuthController_Login_DisabledUserForbidden(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	createTestUser(t, "carol", "carol@example.com", "password123", "user", "disabled")

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"carol","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestAuthController_Register_AdminCanCreateUser(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	createTestUser(t, "admin", "admin@example.com", "adminpass", "admin", "active")

	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"adminpass"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != 200 {
		t.Fatalf("expected login status 200, got %d", loginResp.StatusCode)
	}
	var loginBody LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response failed: %v", err)
	}

	registerReq := httptest.NewRequest(
		"POST",
		"/api/auth/register",
		bytes.NewBufferString(`{"username":"dave","email":"dave@example.com","password":"password123"}`),
	)
	registerReq.Header.Set("Content-Type", "application/json")
	registerReq.Header.Set("Authorization", "Bearer "+loginBody.Token)

	registerResp, err := app.Test(registerReq)
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	defer registerResp.Body.Close()

	if registerResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", registerResp.StatusCode)
	}

	var body struct {
		Item model.User `json:"item"`
	}
	if err := json.NewDecoder(registerResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode register response failed: %v", err)
	}
	if body.Item.ID == "" {
		t.Fatalf("expected non-empty user id")
	}
	if body.Item.Username != "dave" {
		t.Fatalf("expected username %q, got %q", "dave", body.Item.Username)
	}
	if body.Item.Email != "dave@example.com" {
		t.Fatalf("expected email %q, got %q", "dave@example.com", body.Item.Email)
	}
	if body.Item.Role != "user" {
		t.Fatalf("expected default role %q, got %q", "user", body.Item.Role)
	}
	if body.Item.Status != "active" {
		t.Fatalf("expected default status %q, got %q", "active", body.Item.Status)
	}
}

func TestAuthController_Register_ForbiddenForNonAdmin(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	createTestUser(t, "eve", "eve@example.com", "password123", "user", "active")

	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"eve","password":"password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != 200 {
		t.Fatalf("expected login status 200, got %d", loginResp.StatusCode)
	}
	var loginBody LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response failed: %v", err)
	}

	registerReq := httptest.NewRequest(
		"POST",
		"/api/auth/register",
		bytes.NewBufferString(`{"username":"frank","email":"frank@example.com","password":"password123"}`),
	)
	registerReq.Header.Set("Content-Type", "application/json")
	registerReq.Header.Set("Authorization", "Bearer "+loginBody.Token)

	registerResp, err := app.Test(registerReq)
	if err != nil {
		t.Fatalf("register request failed: %v", err)
	}
	defer registerResp.Body.Close()

	if registerResp.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", registerResp.StatusCode)
	}
}

func TestAuthController_Login_BootstrapCreatesAdminWhenNoUsers(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"admin123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var user model.User
	if err := model.DB.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("expected admin user to be created: %v", err)
	}
	if user.Role != "admin" {
		t.Fatalf("expected role %q, got %q", "admin", user.Role)
	}
}

func TestAuthController_Login_PromotesSingleDefaultUserToAdmin(t *testing.T) {
	app, authCfg := setupAuthTestApp(t)

	user := createTestUser(t, authCfg.Username, "admin@example.com", "password123", "user", "active")

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.User.Role != "admin" {
		t.Fatalf("expected role %q, got %q", "admin", body.User.Role)
	}

	var updated model.User
	if err := model.DB.First(&updated, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("query user failed: %v", err)
	}
	if updated.Role != "admin" {
		t.Fatalf("expected role %q, got %q", "admin", updated.Role)
	}
}

func TestAuthController_ResetData_ForbiddenForNonAdmin(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	createTestUser(t, "eve", "eve@example.com", "password123", "user", "active")

	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"eve","password":"password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginBody LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response failed: %v", err)
	}

	resetReq := httptest.NewRequest("POST", "/api/auth/reset-data", nil)
	resetReq.Header.Set("Authorization", "Bearer "+loginBody.Token)
	resetResp, err := app.Test(resetReq)
	if err != nil {
		t.Fatalf("reset request failed: %v", err)
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", resetResp.StatusCode)
	}
}

func TestAuthController_ResetData_DeletesTaskAndTerminalData(t *testing.T) {
	app, _ := setupAuthTestApp(t)

	createTestUser(t, "admin", "admin@example.com", "adminpass", "admin", "active")

	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"adminpass"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginBody LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response failed: %v", err)
	}

	taskID := uuid.New().String()
	terminalID := uuid.New().String()
	aiSessionID := uuid.New().String()
	approvalID := uuid.New().String()
	logID := uuid.New().String()
	messageID := uuid.New().String()

	if err := model.DB.Create(&model.Task{
		ID:     taskID,
		UserID: uuid.New().String(),
		Title:  "test task",
		Status: "todo",
	}).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if err := model.DB.Create(&model.TerminalSession{
		ID:     terminalID,
		UserID: uuid.New().String(),
		Title:  "test terminal",
		TaskID: &taskID,
		Status: "running",
	}).Error; err != nil {
		t.Fatalf("create terminal_session failed: %v", err)
	}
	if err := model.DB.Create(&model.AISession{
		ID:         aiSessionID,
		TerminalID: terminalID,
		TaskID:     &taskID,
		AIType:     "codex",
		State:      "waiting_input",
	}).Error; err != nil {
		t.Fatalf("create ai_session failed: %v", err)
	}
	if err := model.DB.Create(&model.ApprovalRecord{
		ID:         approvalID,
		TerminalID: terminalID,
		PromptType: "yes_no",
		Response:   "yes",
	}).Error; err != nil {
		t.Fatalf("create approval_record failed: %v", err)
	}
	if err := model.DB.Create(&model.Log{
		ID:      logID,
		TaskID:  &taskID,
		LogType: "system",
		Content: "hello",
	}).Error; err != nil {
		t.Fatalf("create log failed: %v", err)
	}
	if err := model.DB.Create(&model.Message{
		ID:     messageID,
		TaskID: &taskID,
		Type:   "info",
		Title:  "test message",
	}).Error; err != nil {
		t.Fatalf("create message failed: %v", err)
	}

	assertCount := func(modelValue interface{}, expected int64, name string) {
		t.Helper()
		var count int64
		if err := model.DB.Model(modelValue).Count(&count).Error; err != nil {
			t.Fatalf("count %s failed: %v", name, err)
		}
		if count != expected {
			t.Fatalf("expected %s count %d, got %d", name, expected, count)
		}
	}

	assertCount(&model.Task{}, 1, "tasks")
	assertCount(&model.TerminalSession{}, 1, "terminal_sessions")
	assertCount(&model.AISession{}, 1, "ai_sessions")
	assertCount(&model.ApprovalRecord{}, 1, "approval_records")
	assertCount(&model.Log{}, 1, "logs")
	assertCount(&model.Message{}, 1, "messages")

	resetReq := httptest.NewRequest("POST", "/api/auth/reset-data", nil)
	resetReq.Header.Set("Authorization", "Bearer "+loginBody.Token)
	resetResp, err := app.Test(resetReq)
	if err != nil {
		t.Fatalf("reset request failed: %v", err)
	}
	defer resetResp.Body.Close()

	if resetResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resetResp.StatusCode)
	}

	assertCount(&model.Task{}, 0, "tasks")
	assertCount(&model.TerminalSession{}, 0, "terminal_sessions")
	assertCount(&model.AISession{}, 0, "ai_sessions")
	assertCount(&model.ApprovalRecord{}, 0, "approval_records")
	assertCount(&model.Log{}, 0, "logs")
	assertCount(&model.Message{}, 0, "messages")
}
