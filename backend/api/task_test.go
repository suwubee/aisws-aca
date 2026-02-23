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

func TestTaskController_GetTaskDetail_ExcludesRawLogs(t *testing.T) {
	app := setupTaskTestApp(t)

	task := model.Task{
		ID:         "task-detail-1",
		Title:      "detail-task",
		Status:     "todo",
		OrderIndex: float64(time.Now().UnixNano()),
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	terminalID := "term-detail-1"
	terminal := model.TerminalSession{
		ID:     terminalID,
		UserID: "tester",
		Title:  "detail-term",
		TaskID: &task.ID,
		Shell:  "bash",
		Status: "running",
	}
	if err := model.DB.Create(&terminal).Error; err != nil {
		t.Fatalf("create terminal failed: %v", err)
	}

	logs := []model.Log{
		{ID: "log-detail-1", TerminalID: &terminalID, TaskID: &task.ID, LogType: "input_raw", Content: "raw input\n"},
		{ID: "log-detail-2", TerminalID: &terminalID, TaskID: &task.ID, LogType: "output_raw", Content: "raw output\n"},
		{ID: "log-detail-3", TerminalID: &terminalID, TaskID: &task.ID, LogType: "input", Content: "input\n"},
		{ID: "log-detail-4", TerminalID: &terminalID, TaskID: &task.ID, LogType: "output", Content: "output\n"},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/tasks/task-detail-1/detail", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET detail request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Logs []model.Log `json:"logs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode detail response failed: %v", err)
	}

	if len(body.Logs) != 2 {
		t.Fatalf("expected 2 non-raw logs, got %d", len(body.Logs))
	}

	logTypes := map[string]bool{}
	for _, entry := range body.Logs {
		logTypes[entry.LogType] = true
	}
	if logTypes["input_raw"] || logTypes["output_raw"] {
		t.Fatalf("expected raw logs to be excluded, got types: %#v", logTypes)
	}
	if !logTypes["input"] || !logTypes["output"] {
		t.Fatalf("expected input/output logs to remain, got types: %#v", logTypes)
	}
}

func TestTaskController_GetTaskDetail_PrefersNativeLogsWhenAvailable(t *testing.T) {
	app := setupTaskTestApp(t)

	task := model.Task{
		ID:         "task-detail-native-1",
		Title:      "detail-native-task",
		Status:     "todo",
		OrderIndex: float64(time.Now().UnixNano()),
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	terminalID := "term-detail-native-1"
	terminal := model.TerminalSession{
		ID:     terminalID,
		UserID: "tester",
		Title:  "detail-native-term",
		TaskID: &task.ID,
		Shell:  "bash",
		Status: "running",
	}
	if err := model.DB.Create(&terminal).Error; err != nil {
		t.Fatalf("create terminal failed: %v", err)
	}

	logs := []model.Log{
		{ID: "log-native-1", TerminalID: &terminalID, TaskID: &task.ID, LogType: "output", Content: "pty output\n"},
		{ID: "log-native-2", TerminalID: &terminalID, TaskID: &task.ID, LogType: "ai_input_native", Content: "你好\n"},
		{ID: "log-native-3", TerminalID: &terminalID, TaskID: &task.ID, LogType: "ai_output_native", Content: "你好，我是 Claude\n"},
		{ID: "log-native-4", TerminalID: &terminalID, TaskID: nil, LogType: "system", Content: "system\n"},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/tasks/task-detail-native-1/detail", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET detail request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Logs []model.Log `json:"logs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode detail response failed: %v", err)
	}

	if len(body.Logs) != 3 {
		t.Fatalf("expected 3 native-preferred logs, got %d", len(body.Logs))
	}
	for _, entry := range body.Logs {
		switch entry.LogType {
		case "ai_input_native", "ai_output_native", "system":
		default:
			t.Fatalf("expected only native/system logs, got %q", entry.LogType)
		}
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

func TestTaskController_ListTaskHistory_IncludesWorkflowAndLatestExecution(t *testing.T) {
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

	now := time.Now()
	taskID := "task-history-1"
	sessionID := "session-1"
	projectID := project.ID

	failedTask := model.Task{
		ID:             taskID,
		Title:          "history failed task",
		Status:         "failed",
		AutomationMode: "agent",
		ProjectID:      &projectID,
		AgentSessionID: sessionID,
		OrderIndex:     1,
		CreatedAt:      now.Add(-time.Hour),
		UpdatedAt:      now.Add(-5 * time.Minute),
	}
	doneTask := model.Task{
		ID:             "task-history-2",
		Title:          "history done task",
		Status:         "done",
		AutomationMode: "cli",
		ProjectID:      &projectID,
		OrderIndex:     2,
		CreatedAt:      now.Add(-2 * time.Hour),
		UpdatedAt:      now.Add(-10 * time.Minute),
	}

	if err := model.DB.Create(&failedTask).Error; err != nil {
		t.Fatalf("create failed task failed: %v", err)
	}
	if err := model.DB.Create(&doneTask).Error; err != nil {
		t.Fatalf("create done task failed: %v", err)
	}

	session := model.AIWorkflowSession{
		ID:         sessionID,
		WorkflowID: "wf-1",
		Status:     "paused",
		StartedAt:  now.Add(-30 * time.Minute),
	}
	if err := model.DB.Create(&session).Error; err != nil {
		t.Fatalf("create workflow session failed: %v", err)
	}

	taskRef := taskID
	oldExec := model.CLIExecution{
		ID:            "exec-old",
		TaskID:        &taskRef,
		Role:          "primary",
		Tool:          "claude",
		Mode:          "plan",
		Source:        "task",
		Status:        "completed",
		PromptPreview: "old",
		StartedAt:     now.Add(-20 * time.Minute),
		UpdatedAt:     now.Add(-19 * time.Minute),
	}
	latestExec := model.CLIExecution{
		ID:            "exec-latest",
		TaskID:        &taskRef,
		Role:          "review",
		Tool:          "codex",
		Mode:          "review",
		Source:        "workflow",
		Status:        "running",
		PromptPreview: "latest",
		StartedAt:     now.Add(-8 * time.Minute),
		UpdatedAt:     now.Add(-3 * time.Minute),
	}
	if err := model.DB.Create(&oldExec).Error; err != nil {
		t.Fatalf("create old execution failed: %v", err)
	}
	if err := model.DB.Create(&latestExec).Error; err != nil {
		t.Fatalf("create latest execution failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/tasks/history?project_group_id=pg-1&status=failed&automation_mode=agent&limit=20", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items []struct {
			Task struct {
				ID      string `json:"id"`
				Status  string `json:"status"`
				Project *struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Group *struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"group"`
				} `json:"project"`
			} `json:"task"`
			WorkflowSession *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"workflow_session"`
			LatestExecution *struct {
				ID     string `json:"id"`
				Tool   string `json:"tool"`
				Status string `json:"status"`
			} `json:"latest_execution"`
		} `json:"items"`
		Count int `json:"count"`
		Total int `json:"total"`
		Stats struct {
			Overview struct {
				Total    int64            `json:"total"`
				ByStatus map[string]int64 `json:"by_status"`
				ByMode   map[string]int64 `json:"by_mode"`
			} `json:"overview"`
			ByGroup []struct {
				GroupID   string           `json:"group_id"`
				GroupName string           `json:"group_name"`
				Total     int64            `json:"total"`
				ByStatus  map[string]int64 `json:"by_status"`
				ByMode    map[string]int64 `json:"by_mode"`
			} `json:"by_group"`
			ByProject []struct {
				ProjectID   string           `json:"project_id"`
				ProjectName string           `json:"project_name"`
				GroupID     string           `json:"group_id"`
				GroupName   string           `json:"group_name"`
				Total       int64            `json:"total"`
				ByStatus    map[string]int64 `json:"by_status"`
				ByMode      map[string]int64 `json:"by_mode"`
			} `json:"by_project"`
		} `json:"stats"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if body.Total != 1 || body.Count != 1 {
		t.Fatalf("expected total/count to be 1/1, got %d/%d", body.Total, body.Count)
	}

	row := body.Items[0]
	if row.Task.ID != taskID {
		t.Fatalf("expected task id %q, got %q", taskID, row.Task.ID)
	}
	if row.Task.Status != "failed" {
		t.Fatalf("expected task status failed, got %q", row.Task.Status)
	}
	if row.Task.Project == nil || row.Task.Project.ID != project.ID || row.Task.Project.Name != project.Name {
		t.Fatalf("expected project info %q/%q, got %+v", project.ID, project.Name, row.Task.Project)
	}
	if row.Task.Project.Group == nil || row.Task.Project.Group.ID != group.ID {
		t.Fatalf("expected project group id %q, got %+v", group.ID, row.Task.Project.Group)
	}
	if row.WorkflowSession == nil || row.WorkflowSession.ID != sessionID || row.WorkflowSession.Status != "paused" {
		t.Fatalf("expected workflow session %q paused, got %+v", sessionID, row.WorkflowSession)
	}
	if row.LatestExecution == nil || row.LatestExecution.ID != latestExec.ID {
		t.Fatalf("expected latest execution %q, got %+v", latestExec.ID, row.LatestExecution)
	}
	if row.LatestExecution.Tool != "codex" || row.LatestExecution.Status != "running" {
		t.Fatalf("expected latest execution codex/running, got %+v", row.LatestExecution)
	}

	if body.Stats.Overview.Total != 1 {
		t.Fatalf("expected stats overview total 1, got %d", body.Stats.Overview.Total)
	}
	if body.Stats.Overview.ByStatus["failed"] != 1 {
		t.Fatalf("expected stats status failed=1, got %+v", body.Stats.Overview.ByStatus)
	}
	if body.Stats.Overview.ByMode["agent"] != 1 {
		t.Fatalf("expected stats mode agent=1, got %+v", body.Stats.Overview.ByMode)
	}

	if len(body.Stats.ByGroup) != 1 {
		t.Fatalf("expected one group stats row, got %d", len(body.Stats.ByGroup))
	}
	groupStats := body.Stats.ByGroup[0]
	if groupStats.GroupID != group.ID || groupStats.GroupName != group.Name || groupStats.Total != 1 {
		t.Fatalf("unexpected group stats: %+v", groupStats)
	}
	if groupStats.ByStatus["failed"] != 1 || groupStats.ByMode["agent"] != 1 {
		t.Fatalf("unexpected group status/mode stats: %+v", groupStats)
	}

	if len(body.Stats.ByProject) != 1 {
		t.Fatalf("expected one project stats row, got %d", len(body.Stats.ByProject))
	}
	projectStats := body.Stats.ByProject[0]
	if projectStats.ProjectID != project.ID || projectStats.ProjectName != project.Name {
		t.Fatalf("unexpected project stats identity: %+v", projectStats)
	}
	if projectStats.GroupID != group.ID || projectStats.GroupName != group.Name || projectStats.Total != 1 {
		t.Fatalf("unexpected project stats hierarchy: %+v", projectStats)
	}
	if projectStats.ByStatus["failed"] != 1 || projectStats.ByMode["agent"] != 1 {
		t.Fatalf("unexpected project status/mode stats: %+v", projectStats)
	}
}
