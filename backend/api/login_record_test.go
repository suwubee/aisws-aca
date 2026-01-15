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

func TestLoginRecords_AuditAndAdminOnly(t *testing.T) {
	app, authCfg, apiGroup := setupTestAppWithAuth(t)

	automationController := NewAutomationController(nil)
	automationController.RegisterRoutes(apiGroup)

	// 成功登录（会创建管理员 & 写入登录记录）
	adminToken := loginForToken(t, app, authCfg.Username, authCfg.Password)

	// 失败登录（写入失败记录）
	badReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
	badReq.Header.Set("Content-Type", "application/json")
	badResp, err := app.Test(badReq)
	if err != nil {
		t.Fatalf("bad login request failed: %v", err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != 401 {
		t.Fatalf("expected bad login status 401, got %d", badResp.StatusCode)
	}

	// 管理员可读取登录记录
	listReq := httptest.NewRequest("GET", "/api/automation/login-records?limit=20&offset=0", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list login records request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.LoginRecord `json:"items"`
		Total int64               `json:"total"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if listBody.Total < 2 {
		t.Fatalf("expected total >= 2, got %d", listBody.Total)
	}

	sawSuccess := false
	sawFailure := false
	for _, item := range listBody.Items {
		if item.Success {
			sawSuccess = true
		} else {
			sawFailure = true
		}
	}
	if !sawSuccess || !sawFailure {
		t.Fatalf("expected both success and failure records, got success=%v failure=%v", sawSuccess, sawFailure)
	}

	// 非管理员禁止读取
	hashed, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	normalUser := model.User{
		ID:           uuid.New().String(),
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: string(hashed),
		Role:         "user",
		Status:       "active",
	}
	if err := model.DB.Create(&normalUser).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	userToken := loginForToken(t, app, "bob", "password123")
	userListReq := httptest.NewRequest("GET", "/api/automation/login-records?limit=1&offset=0", nil)
	userListReq.Header.Set("Authorization", "Bearer "+userToken)
	userListResp, err := app.Test(userListReq)
	if err != nil {
		t.Fatalf("user list login records request failed: %v", err)
	}
	userListResp.Body.Close()
	if userListResp.StatusCode != 403 {
		t.Fatalf("expected non-admin list status 403, got %d", userListResp.StatusCode)
	}
}
