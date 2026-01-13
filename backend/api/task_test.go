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

func setupTaskTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:task_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		return c.Next()
	})

	ctrl := NewTaskController(nil)
	ctrl.RegisterRoutes(apiGroup)

	return app
}

type taskListItemResponse struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	ServerID *string `json:"server_id"`
	Server   *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"server"`
}

func TestTaskController_CreateTask_WithAndWithoutServer(t *testing.T) {
	app := setupTaskTestApp(t)

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

	createWithServerReq := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(`{"title":"task-1","server_id":" srv-1 "}`))
	createWithServerReq.Header.Set("Content-Type", "application/json")
	createWithServerResp, err := app.Test(createWithServerReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createWithServerResp.Body.Close()
	if createWithServerResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createWithServerResp.StatusCode)
	}

	var createWithServerBody struct {
		Item model.Task `json:"item"`
	}
	if err := json.NewDecoder(createWithServerResp.Body).Decode(&createWithServerBody); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if createWithServerBody.Item.ID == "" {
		t.Fatalf("expected non-empty task id")
	}
	if createWithServerBody.Item.ServerID == nil || *createWithServerBody.Item.ServerID != server.ID {
		t.Fatalf("expected server_id %q, got %v", server.ID, createWithServerBody.Item.ServerID)
	}

	createWithoutServerReq := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(`{"title":"task-2","automation_mode":"none"}`))
	createWithoutServerReq.Header.Set("Content-Type", "application/json")
	createWithoutServerResp, err := app.Test(createWithoutServerReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createWithoutServerResp.Body.Close()
	if createWithoutServerResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createWithoutServerResp.StatusCode)
	}

	var createWithoutServerBody struct {
		Item model.Task `json:"item"`
	}
	if err := json.NewDecoder(createWithoutServerResp.Body).Decode(&createWithoutServerBody); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if createWithoutServerBody.Item.ID == "" {
		t.Fatalf("expected non-empty task id")
	}
	if createWithoutServerBody.Item.ServerID != nil {
		t.Fatalf("expected server_id nil, got %v", createWithoutServerBody.Item.ServerID)
	}

	listReq := httptest.NewRequest("GET", "/api/tasks", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []taskListItemResponse `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(listBody.Items))
	}

	var listItemWithServer *taskListItemResponse
	var listItemWithoutServer *taskListItemResponse
	for i := range listBody.Items {
		item := &listBody.Items[i]
		switch item.Title {
		case "task-1":
			listItemWithServer = item
		case "task-2":
			listItemWithoutServer = item
		}
	}

	if listItemWithServer == nil || listItemWithServer.Server == nil {
		t.Fatalf("expected task-1 to include server info")
	}
	if listItemWithServer.ServerID == nil || *listItemWithServer.ServerID != server.ID {
		t.Fatalf("expected task-1 server_id %q, got %v", server.ID, listItemWithServer.ServerID)
	}
	if listItemWithServer.Server.ID != server.ID || listItemWithServer.Server.Name != server.Name {
		t.Fatalf("expected task-1 server %q/%q, got %q/%q", server.ID, server.Name, listItemWithServer.Server.ID, listItemWithServer.Server.Name)
	}

	if listItemWithoutServer == nil {
		t.Fatalf("expected task-2 to be present")
	}
	if listItemWithoutServer.ServerID != nil {
		t.Fatalf("expected task-2 server_id nil, got %v", listItemWithoutServer.ServerID)
	}
	if listItemWithoutServer.Server != nil {
		t.Fatalf("expected task-2 server nil, got %v", listItemWithoutServer.Server)
	}
}

func TestTaskController_CreateTask_InvalidServerID(t *testing.T) {
	app := setupTaskTestApp(t)

	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(`{"title":"task-1","server_id":"not-exist"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestTaskController_ListTasks_ServerMissingGraceful(t *testing.T) {
	app := setupTaskTestApp(t)

	missingServerID := "srv-missing"
	task := model.Task{
		ID:         "task-1",
		Title:      "task-1",
		Status:     "todo",
		OrderIndex: float64(time.Now().UnixNano()),
		ServerID:   &missingServerID,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	listReq := httptest.NewRequest("GET", "/api/tasks", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []taskListItemResponse `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 task, got %d", len(listBody.Items))
	}
	if listBody.Items[0].ServerID == nil || *listBody.Items[0].ServerID != missingServerID {
		t.Fatalf("expected server_id %q, got %v", missingServerID, listBody.Items[0].ServerID)
	}
	if listBody.Items[0].Server != nil {
		t.Fatalf("expected server to be nil when missing, got %v", listBody.Items[0].Server)
	}
}

func TestTaskController_GetTasksByStatus_IncludesServerInfo(t *testing.T) {
	app := setupTaskTestApp(t)

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

	createReq := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(`{"title":"task-1","status":"in_progress","server_id":"srv-1"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createResp.StatusCode)
	}

	statusReq := httptest.NewRequest("GET", "/api/tasks/by-status", nil)
	statusResp, err := app.Test(statusReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", statusResp.StatusCode)
	}

	var statusBody struct {
		Items map[string][]taskListItemResponse `json:"items"`
	}
	if err := json.NewDecoder(statusResp.Body).Decode(&statusBody); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	items := statusBody.Items["in_progress"]
	if len(items) != 1 {
		t.Fatalf("expected 1 in_progress task, got %d", len(items))
	}
	if items[0].Server == nil || items[0].Server.ID != server.ID || items[0].Server.Name != server.Name {
		t.Fatalf("expected server info %q/%q, got %v", server.ID, server.Name, items[0].Server)
	}
}

func TestTaskController_StartTask_InProgressPrefersServerMatchedTerminal(t *testing.T) {
	app := setupTaskTestApp(t)

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

	now := time.Now()
	serverID := server.ID
	task := model.Task{
		ID:             "task-1",
		UserID:         "u1",
		Title:          "task-1",
		Status:         "in_progress",
		AutomationMode: "cli",
		ServerID:       &serverID,
		OrderIndex:     float64(now.UnixNano()),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	sshTerminal := model.TerminalSession{
		ID:        "term-ssh",
		UserID:    "u1",
		Title:     "SSH terminal",
		TaskID:    &task.ID,
		ServerID:  &serverID,
		Shell:     "ssh",
		Status:    "running",
		CreatedAt: now.Add(-time.Minute),
	}
	localTerminal := model.TerminalSession{
		ID:        "term-local",
		UserID:    "u1",
		Title:     "Local terminal",
		TaskID:    &task.ID,
		Shell:     "bash",
		Status:    "running",
		CreatedAt: now.Add(time.Minute),
	}
	if err := model.DB.Create(&sshTerminal).Error; err != nil {
		t.Fatalf("create ssh terminal failed: %v", err)
	}
	if err := model.DB.Create(&localTerminal).Error; err != nil {
		t.Fatalf("create local terminal failed: %v", err)
	}

	startReq := httptest.NewRequest("POST", "/api/tasks/task-1/start", bytes.NewBufferString(`{}`))
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := app.Test(startReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer startResp.Body.Close()

	if startResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", startResp.StatusCode)
	}

	var body struct {
		TerminalID  string   `json:"terminal_id"`
		TerminalIDs []string `json:"terminal_ids"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if body.TerminalID != "term-ssh" {
		t.Fatalf("expected terminal_id %q, got %q", "term-ssh", body.TerminalID)
	}
	if len(body.TerminalIDs) == 0 || body.TerminalIDs[0] != "term-ssh" {
		t.Fatalf("expected terminal_ids to start with %q, got %v", "term-ssh", body.TerminalIDs)
	}
}

func TestTaskController_CreateTask_WithProjectInfo(t *testing.T) {
	app := setupTaskTestApp(t)

	group := model.ProjectGroup{ID: "pg-1", Name: "Portfolio"}
	if err := model.DB.Create(&group).Error; err != nil {
		t.Fatalf("create project group failed: %v", err)
	}

	project := model.Project{
		ID:      "p-1",
		Name:    "ProjectA",
		Type:    model.ProjectTypeLocal,
		GroupID: &group.ID,
	}
	if err := model.DB.Create(&project).Error; err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(`{"title":"task-1","project_id":"p-1","automation_mode":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	listReq := httptest.NewRequest("GET", "/api/tasks", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []struct {
			ID        string  `json:"id"`
			Title     string  `json:"title"`
			ProjectID *string `json:"project_id"`
			Project   *struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Group *struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"group"`
			} `json:"project"`
		} `json:"items"`
	}

	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 task, got %d", len(listBody.Items))
	}

	item := listBody.Items[0]
	if item.ProjectID == nil || *item.ProjectID != project.ID {
		t.Fatalf("expected project_id %q, got %v", project.ID, item.ProjectID)
	}
	if item.Project == nil || item.Project.ID != project.ID || item.Project.Name != project.Name {
		t.Fatalf("expected project info %q/%q, got %+v", project.ID, project.Name, item.Project)
	}
	if item.Project.Group == nil || item.Project.Group.ID != group.ID || item.Project.Group.Name != group.Name {
		t.Fatalf("expected project group info %q/%q, got %+v", group.ID, group.Name, item.Project.Group)
	}
}
