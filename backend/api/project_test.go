package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
)

func setupProjectTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:project_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		c.Locals("userID", "u-1")
		c.Locals("role", "admin")
		return c.Next()
	})

	ctrl := NewProjectController()
	ctrl.RegisterRoutes(apiGroup)

	return app
}

func TestProjectController_CRUD(t *testing.T) {
	app := setupProjectTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"name":"proj-1","type":"local","local_path":"/tmp","env_vars":{"FOO":"bar"}}`))
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
		Item model.Project `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty project id")
	}
	if createBody.Item.Name != "proj-1" {
		t.Fatalf("expected name %q, got %q", "proj-1", createBody.Item.Name)
	}
	if createBody.Item.Type != model.ProjectTypeLocal {
		t.Fatalf("expected type %q, got %q", model.ProjectTypeLocal, createBody.Item.Type)
	}
	if createBody.Item.EnvVars["FOO"] != "bar" {
		t.Fatalf("expected env_vars[FOO] %q, got %q", "bar", createBody.Item.EnvVars["FOO"])
	}

	listReq := httptest.NewRequest("GET", "/api/projects", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.Project `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 project, got %d", len(listBody.Items))
	}

	getReq := httptest.NewRequest("GET", "/api/projects/"+createBody.Item.ID, nil)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", getResp.StatusCode)
	}

	var getBody struct {
		Item model.Project `json:"item"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get response failed: %v", err)
	}
	if getBody.Item.ID != createBody.Item.ID {
		t.Fatalf("expected project id %q, got %q", createBody.Item.ID, getBody.Item.ID)
	}
	if getBody.Item.EnvVars["FOO"] != "bar" {
		t.Fatalf("expected env_vars[FOO] %q, got %q", "bar", getBody.Item.EnvVars["FOO"])
	}

	updateReq := httptest.NewRequest("PUT", "/api/projects/"+createBody.Item.ID, bytes.NewBufferString(`{"name":"proj-1-updated","type":"git","git_repo":"https://example.com/repo.git","git_branch":"main","env_vars":{"A":"1"}}`))
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
		Item model.Project `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updateBody.Item.Name != "proj-1-updated" {
		t.Fatalf("expected updated name %q, got %q", "proj-1-updated", updateBody.Item.Name)
	}
	if updateBody.Item.Type != model.ProjectTypeGit {
		t.Fatalf("expected updated type %q, got %q", model.ProjectTypeGit, updateBody.Item.Type)
	}
	if updateBody.Item.GitRepo != "https://example.com/repo.git" {
		t.Fatalf("expected updated git_repo %q, got %q", "https://example.com/repo.git", updateBody.Item.GitRepo)
	}
	if updateBody.Item.GitBranch != "main" {
		t.Fatalf("expected updated git_branch %q, got %q", "main", updateBody.Item.GitBranch)
	}
	if updateBody.Item.EnvVars["A"] != "1" || len(updateBody.Item.EnvVars) != 1 {
		t.Fatalf("expected env_vars %v, got %v", map[string]string{"A": "1"}, updateBody.Item.EnvVars)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/projects/"+createBody.Item.ID, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}

	getAfterDeleteReq := httptest.NewRequest("GET", "/api/projects/"+createBody.Item.ID, nil)
	getAfterDeleteResp, err := app.Test(getAfterDeleteReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getAfterDeleteResp.Body.Close()
	if getAfterDeleteResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", getAfterDeleteResp.StatusCode)
	}
}

func TestProjectController_CreateProject_Validation(t *testing.T) {
	app := setupProjectTestApp(t)

	missingNameReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"type":"local"}`))
	missingNameReq.Header.Set("Content-Type", "application/json")
	missingNameResp, err := app.Test(missingNameReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer missingNameResp.Body.Close()
	if missingNameResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", missingNameResp.StatusCode)
	}

	invalidTypeReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"name":"proj","type":"bad"}`))
	invalidTypeReq.Header.Set("Content-Type", "application/json")
	invalidTypeResp, err := app.Test(invalidTypeReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidTypeResp.Body.Close()
	if invalidTypeResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidTypeResp.StatusCode)
	}
}

func TestProjectController_CreateProject_ServerID(t *testing.T) {
	app := setupProjectTestApp(t)

	server := model.SSHServer{
		ID:       "srv-1",
		Name:     "Prod",
		Host:     "127.0.0.1",
		Port:     22,
		Username: "root",
		AuthType: "password",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	createReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"name":"proj","server_id":" srv-1 "}`))
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
		Item model.Project `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if createBody.Item.ServerID == nil || *createBody.Item.ServerID != server.ID {
		t.Fatalf("expected server_id %q, got %v", server.ID, createBody.Item.ServerID)
	}

	invalidServerReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"name":"proj-2","server_id":"not-exist"}`))
	invalidServerReq.Header.Set("Content-Type", "application/json")
	invalidServerResp, err := app.Test(invalidServerReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidServerResp.Body.Close()
	if invalidServerResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidServerResp.StatusCode)
	}
}

func TestProjectController_CreateProject_GroupID(t *testing.T) {
	app := setupProjectTestApp(t)

	group := model.ProjectGroup{
		ID:   "pg-1",
		Name: "Portfolio",
	}
	if err := model.DB.Create(&group).Error; err != nil {
		t.Fatalf("create project group failed: %v", err)
	}

	createReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"name":"proj","group_id":" pg-1 "}`))
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
		Item model.Project `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if createBody.Item.GroupID == nil || *createBody.Item.GroupID != group.ID {
		t.Fatalf("expected group_id %q, got %v", group.ID, createBody.Item.GroupID)
	}

	invalidGroupReq := httptest.NewRequest("POST", "/api/projects", bytes.NewBufferString(`{"name":"proj-2","group_id":"not-exist"}`))
	invalidGroupReq.Header.Set("Content-Type", "application/json")
	invalidGroupResp, err := app.Test(invalidGroupReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidGroupResp.Body.Close()
	if invalidGroupResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidGroupResp.StatusCode)
	}
}
