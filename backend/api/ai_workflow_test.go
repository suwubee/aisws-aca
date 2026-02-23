package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/workflow"
	"github.com/gofiber/fiber/v2"
)

func setupAIWorkflowTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:ai_workflow_api_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	InitAIWorkflowEngine(nil)

	app := fiber.New()
	group := app.Group("/api/ai-workflow")
	group.Get("/session/:id/events", GetAIWorkflowSessionEvents)
	group.Get("/session/:id/logs", GetAIWorkflowSessionLogs)
	return app
}

func createAIWorkflowSessionFixture(t *testing.T, sessionID string) {
	t.Helper()

	contextJSON := `{"terminal_id":"term-1","task_id":"task-1"}`
	record := model.AIWorkflowSession{
		ID:         sessionID,
		WorkflowID: "wf-1",
		UserGoal:   "test goal",
		Status:     "running",
		Messages:   "[]",
		Steps:      "[]",
		Context:    contextJSON,
		Summary:    "",
		StartedAt:  time.Now(),
	}
	if err := model.DB.Create(&record).Error; err != nil {
		t.Fatalf("create AIWorkflowSession fixture failed: %v", err)
	}
}

func TestAIWorkflowAPI_GetSessionEvents(t *testing.T) {
	app := setupAIWorkflowTestApp(t)
	createAIWorkflowSessionFixture(t, "session-events-1")

	taskID := "task-1"
	terminalID := "term-1"
	now := time.Now()
	events := []model.AIWorkflowEvent{
		{
			SessionID:  "session-events-1",
			WorkflowID: "wf-1",
			TaskID:     &taskID,
			TerminalID: &terminalID,
			Iteration:  0,
			Phase:      "plan",
			EventType:  "step_started",
			Summary:    "start",
			Payload:    `{"k":"v"}`,
			CreatedAt:  now,
		},
		{
			SessionID:  "session-events-1",
			WorkflowID: "wf-1",
			TaskID:     &taskID,
			TerminalID: &terminalID,
			Iteration:  0,
			Phase:      "execute",
			EventType:  "tool_result",
			Summary:    "done",
			Payload:    `plain-text-payload`,
			CreatedAt:  now.Add(time.Second),
		},
	}
	if err := model.DB.Create(&events).Error; err != nil {
		t.Fatalf("create AIWorkflowEvent fixtures failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/ai-workflow/session/session-events-1/events?limit=20", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("events request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items   []AIWorkflowEventResponse `json:"items"`
		Count   int                       `json:"count"`
		LastID  uint64                    `json:"last_id"`
		HasMore bool                      `json:"has_more"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body.Count != 2 || len(body.Items) != 2 {
		t.Fatalf("expected 2 events, got count=%d len=%d", body.Count, len(body.Items))
	}
	if body.LastID == 0 {
		t.Fatalf("expected non-zero last_id")
	}
	if body.HasMore {
		t.Fatalf("expected has_more false for full response")
	}
	if body.Items[0].EventType != "step_started" {
		t.Fatalf("expected first event_type step_started, got %q", body.Items[0].EventType)
	}
	payloadMap, ok := body.Items[0].Payload.(map[string]any)
	if !ok || strings.TrimSpace(fmt.Sprintf("%v", payloadMap["k"])) != "v" {
		t.Fatalf("expected parsed JSON payload map, got %#v", body.Items[0].Payload)
	}
	if raw, ok := body.Items[1].Payload.(string); !ok || raw != "plain-text-payload" {
		t.Fatalf("expected raw payload fallback, got %#v", body.Items[1].Payload)
	}

	afterReq := httptest.NewRequest("GET", fmt.Sprintf("/api/ai-workflow/session/session-events-1/events?after_id=%d", body.Items[0].ID), nil)
	afterResp, err := app.Test(afterReq)
	if err != nil {
		t.Fatalf("events-after request failed: %v", err)
	}
	defer afterResp.Body.Close()
	if afterResp.StatusCode != 200 {
		t.Fatalf("expected after response status 200, got %d", afterResp.StatusCode)
	}

	var afterBody struct {
		Items []AIWorkflowEventResponse `json:"items"`
		Count int                       `json:"count"`
	}
	if err := json.NewDecoder(afterResp.Body).Decode(&afterBody); err != nil {
		t.Fatalf("decode events-after response failed: %v", err)
	}
	if afterBody.Count != 1 || len(afterBody.Items) != 1 {
		t.Fatalf("expected 1 event after cursor, got count=%d len=%d", afterBody.Count, len(afterBody.Items))
	}
	if afterBody.Items[0].EventType != "tool_result" {
		t.Fatalf("expected remaining event tool_result, got %q", afterBody.Items[0].EventType)
	}
}

func TestAIWorkflowAPI_GetSessionLogs(t *testing.T) {
	app := setupAIWorkflowTestApp(t)
	createAIWorkflowSessionFixture(t, "session-logs-1")

	terminalID := "term-1"
	taskID := "task-1"
	otherTaskID := "task-other"
	now := time.Now()

	logs := []model.Log{
		{ID: "log-1", TerminalID: &terminalID, TaskID: &taskID, LogType: "input_raw", Content: "raw-in", CreatedAt: now},
		{ID: "log-2", TerminalID: &terminalID, TaskID: &taskID, LogType: "output_raw", Content: "raw-out", CreatedAt: now.Add(time.Second)},
		{ID: "log-3", TerminalID: &terminalID, TaskID: &taskID, LogType: "output", Content: "out", CreatedAt: now.Add(2 * time.Second)},
		{ID: "log-4", TerminalID: &terminalID, TaskID: nil, LogType: "system", Content: "sys", CreatedAt: now.Add(3 * time.Second)},
		{ID: "log-5", TerminalID: &terminalID, TaskID: &otherTaskID, LogType: "output_raw", Content: "should-be-filtered", CreatedAt: now.Add(4 * time.Second)},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create log fixtures failed: %v", err)
	}

	reqAll := httptest.NewRequest("GET", "/api/ai-workflow/session/session-logs-1/logs?order=asc", nil)
	respAll, err := app.Test(reqAll)
	if err != nil {
		t.Fatalf("logs request failed: %v", err)
	}
	defer respAll.Body.Close()
	if respAll.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", respAll.StatusCode)
	}

	var allBody struct {
		Items      []model.Log `json:"items"`
		Total      int64       `json:"total"`
		IncludeRaw bool        `json:"include_raw"`
	}
	if err := json.NewDecoder(respAll.Body).Decode(&allBody); err != nil {
		t.Fatalf("decode logs response failed: %v", err)
	}
	if allBody.Total != 2 {
		t.Fatalf("expected total 2 after default raw filter, got %d", allBody.Total)
	}
	if len(allBody.Items) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(allBody.Items))
	}
	if allBody.IncludeRaw {
		t.Fatalf("expected include_raw default false")
	}

	reqNoRaw := httptest.NewRequest("GET", "/api/ai-workflow/session/session-logs-1/logs?include_raw=false", nil)
	respNoRaw, err := app.Test(reqNoRaw)
	if err != nil {
		t.Fatalf("logs include_raw=false request failed: %v", err)
	}
	defer respNoRaw.Body.Close()
	if respNoRaw.StatusCode != 200 {
		t.Fatalf("expected include_raw=false status 200, got %d", respNoRaw.StatusCode)
	}

	var noRawBody struct {
		Items []model.Log `json:"items"`
		Total int64       `json:"total"`
	}
	if err := json.NewDecoder(respNoRaw.Body).Decode(&noRawBody); err != nil {
		t.Fatalf("decode include_raw=false response failed: %v", err)
	}
	if noRawBody.Total != 2 {
		t.Fatalf("expected total 2 after raw filter, got %d", noRawBody.Total)
	}
	for _, entry := range noRawBody.Items {
		if entry.LogType == "input_raw" || entry.LogType == "output_raw" {
			t.Fatalf("expected raw logs filtered, got %q", entry.LogType)
		}
	}

	reqType := httptest.NewRequest("GET", "/api/ai-workflow/session/session-logs-1/logs?type=output_raw", nil)
	respType, err := app.Test(reqType)
	if err != nil {
		t.Fatalf("logs type request failed: %v", err)
	}
	defer respType.Body.Close()
	if respType.StatusCode != 200 {
		t.Fatalf("expected type filter status 200, got %d", respType.StatusCode)
	}

	var typeBody struct {
		Items []model.Log `json:"items"`
		Total int64       `json:"total"`
	}
	if err := json.NewDecoder(respType.Body).Decode(&typeBody); err != nil {
		t.Fatalf("decode type filter response failed: %v", err)
	}
	if typeBody.Total != 1 || len(typeBody.Items) != 1 {
		t.Fatalf("expected one output_raw item, got total=%d len=%d", typeBody.Total, len(typeBody.Items))
	}
	if typeBody.Items[0].LogType != "output_raw" {
		t.Fatalf("expected output_raw, got %q", typeBody.Items[0].LogType)
	}

	nativeLogs := []model.Log{
		{ID: "log-6", TerminalID: &terminalID, TaskID: &taskID, LogType: "ai_input_native", Content: "你好", CreatedAt: now.Add(5 * time.Second)},
		{ID: "log-7", TerminalID: &terminalID, TaskID: &taskID, LogType: "ai_output_native", Content: "你好，我是 Claude", CreatedAt: now.Add(6 * time.Second)},
	}
	if err := model.DB.Create(&nativeLogs).Error; err != nil {
		t.Fatalf("create native log fixtures failed: %v", err)
	}

	reqNativePreferred := httptest.NewRequest("GET", "/api/ai-workflow/session/session-logs-1/logs", nil)
	respNativePreferred, err := app.Test(reqNativePreferred)
	if err != nil {
		t.Fatalf("native preferred request failed: %v", err)
	}
	defer respNativePreferred.Body.Close()
	if respNativePreferred.StatusCode != 200 {
		t.Fatalf("expected native preferred status 200, got %d", respNativePreferred.StatusCode)
	}

	var nativePreferredBody struct {
		Items []model.Log `json:"items"`
		Total int64       `json:"total"`
	}
	if err := json.NewDecoder(respNativePreferred.Body).Decode(&nativePreferredBody); err != nil {
		t.Fatalf("decode native preferred response failed: %v", err)
	}
	if nativePreferredBody.Total != 3 {
		t.Fatalf("expected native preferred total 3 (ai_input/ai_output/system), got %d", nativePreferredBody.Total)
	}
	for _, entry := range nativePreferredBody.Items {
		switch entry.LogType {
		case "ai_input_native", "ai_output_native", "system":
		default:
			t.Fatalf("expected native/system logs only, got %q", entry.LogType)
		}
	}

	reqSourceNative := httptest.NewRequest("GET", "/api/ai-workflow/session/session-logs-1/logs?source=native", nil)
	respSourceNative, err := app.Test(reqSourceNative)
	if err != nil {
		t.Fatalf("source=native request failed: %v", err)
	}
	defer respSourceNative.Body.Close()
	var sourceNativeBody struct {
		Items []model.Log `json:"items"`
		Total int64       `json:"total"`
	}
	if err := json.NewDecoder(respSourceNative.Body).Decode(&sourceNativeBody); err != nil {
		t.Fatalf("decode source=native response failed: %v", err)
	}
	if sourceNativeBody.Total != 2 {
		t.Fatalf("expected source=native total 2, got %d", sourceNativeBody.Total)
	}
	for _, entry := range sourceNativeBody.Items {
		if entry.LogType != "ai_input_native" && entry.LogType != "ai_output_native" {
			t.Fatalf("expected only native logs for source=native, got %q", entry.LogType)
		}
	}

	reqSourcePTY := httptest.NewRequest("GET", "/api/ai-workflow/session/session-logs-1/logs?source=pty", nil)
	respSourcePTY, err := app.Test(reqSourcePTY)
	if err != nil {
		t.Fatalf("source=pty request failed: %v", err)
	}
	defer respSourcePTY.Body.Close()
	var sourcePTYBody struct {
		Items []model.Log `json:"items"`
		Total int64       `json:"total"`
	}
	if err := json.NewDecoder(respSourcePTY.Body).Decode(&sourcePTYBody); err != nil {
		t.Fatalf("decode source=pty response failed: %v", err)
	}
	if sourcePTYBody.Total != 2 {
		t.Fatalf("expected source=pty total 2 (output/system), got %d", sourcePTYBody.Total)
	}
	for _, entry := range sourcePTYBody.Items {
		if entry.LogType == "ai_input_native" || entry.LogType == "ai_output_native" {
			t.Fatalf("expected source=pty to exclude native logs, got %q", entry.LogType)
		}
	}
}

func TestComposeGoalWithTaskContext(t *testing.T) {
	goal := "请继续完成部署"
	ctx := map[string]any{
		"task_id":                "task-ctx-1",
		"task_title":             "优化终端体验",
		"task_description":       "修复日志展示与流程可观测性",
		"task_initial_prompt":    "补齐 AI 托管全流程",
		"task_ai_end_condition":  "用户确认可完整追踪",
		"task_ai_error_handling": "失败后自动重试一次",
		"task_work_dir":          "/root/dev/aca",
		"running_command":        "claude",
		"task_automation_mode":   "terminal",
		"task_priority":          "2",
		"task_ai_prompt":         "禁止创建新终端",
		"task_remark":            "优先修复日志",
		"command_execution_mode": "terminal",
		"workflow_session_id":    "wf-session-x",
		"terminal_ids_by_server": map[string]string{"s1": "t1"},
		"target_server_ids":      []string{"s1"},
		"current_server_id":      "s1",
		"terminal_id":            "t1",
	}

	composed := composeGoalWithTaskContext(goal, ctx)
	if !strings.Contains(composed, "【任务上下文】") {
		t.Fatalf("expected composed goal to contain task context block, got: %s", composed)
	}
	if !strings.Contains(composed, "【用户目标】") {
		t.Fatalf("expected composed goal to contain user goal label, got: %s", composed)
	}
	if !strings.Contains(composed, "优化终端体验") {
		t.Fatalf("expected composed goal to contain task title, got: %s", composed)
	}
	if !strings.Contains(composed, goal) {
		t.Fatalf("expected composed goal to contain original goal, got: %s", composed)
	}
}

func TestComposeGoalWithTaskContext_ManualContextPreferred(t *testing.T) {
	goal := "继续修复"
	ctx := map[string]any{
		"task_id":              "task-1",
		"task_title":           "任务A",
		"manual_context_block": "这是人工编辑上下文",
		"terminal_bootstrap":   "这是自动摘要上下文",
	}

	composed := composeGoalWithTaskContext(goal, ctx)
	if !strings.Contains(composed, "这是人工编辑上下文") {
		t.Fatalf("expected manual context included, got: %s", composed)
	}
	if strings.Contains(composed, "这是自动摘要上下文") {
		t.Fatalf("expected auto bootstrap suppressed when manual context exists, got: %s", composed)
	}
}

func TestRecordAIWorkflowStartupInputLog(t *testing.T) {
	_ = setupAIWorkflowTestApp(t)

	session := &workflow.AIWorkflowSession{
		ID: "wf-log-1",
		Context: map[string]any{
			"terminal_id": "term-log-1",
			"task_id":     "task-log-1",
		},
	}

	recordAIWorkflowStartupInputLog(session, "", "第一次启动提示")
	recordAIWorkflowStartupInputLog(session, "", "第一次启动提示") // duplicate within window should be ignored
	recordAIWorkflowStartupInputLog(session, "", "第二次不同提示")

	var logs []model.Log
	if err := model.DB.Order("created_at asc").Find(&logs).Error; err != nil {
		t.Fatalf("query startup logs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs (deduped), got %d", len(logs))
	}
	if logs[0].LogType != "ai_input_native" || logs[1].LogType != "ai_input_native" {
		t.Fatalf("expected ai_input_native logs, got %q and %q", logs[0].LogType, logs[1].LogType)
	}
	if !strings.Contains(logs[0].Content, "第一次启动提示") {
		t.Fatalf("unexpected first log content: %q", logs[0].Content)
	}
	if !strings.Contains(logs[1].Content, "第二次不同提示") {
		t.Fatalf("unexpected second log content: %q", logs[1].Content)
	}
}

func TestEnrichWorkflowContextFromTaskAndSeedVars(t *testing.T) {
	_ = setupAIWorkflowTestApp(t)

	task := model.Task{
		ID:              "task-fill-1",
		Title:           "任务标题",
		Description:     "任务描述",
		Remark:          "任务备注",
		Priority:        3,
		AutomationMode:  "terminal",
		WorkDir:         "/root/dev/aca",
		InitialPrompt:   "先完成日志与流程",
		AIPrompt:        "避免创建新终端",
		AIEndCondition:  "输出完整复盘",
		AIErrorHandling: "失败后暂停等待人工",
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task fixture failed: %v", err)
	}

	ctx := map[string]any{
		"task_id":    task.ID,
		"task_title": "前端传入标题", // should not be overwritten
	}
	enrichWorkflowContextFromTask(task.ID, ctx)

	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "task_title")); got != "前端传入标题" {
		t.Fatalf("expected existing task_title preserved, got %q", got)
	}
	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "task_description")); got != "任务描述" {
		t.Fatalf("expected task_description filled from DB, got %q", got)
	}
	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "task_work_dir")); got != "/root/dev/aca" {
		t.Fatalf("expected task_work_dir filled from DB, got %q", got)
	}

	opts := workflow.StartWorkflowOptions{}
	seedWorkflowUserGoalVars(&opts, ctx)
	if opts.UserGoalVars == nil {
		t.Fatalf("expected UserGoalVars initialized")
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", opts.UserGoalVars["task_title"])); got != "前端传入标题" {
		t.Fatalf("expected task_title copied into UserGoalVars, got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprintf("%v", opts.UserGoalVars["task_ai_prompt"])); got != "避免创建新终端" {
		t.Fatalf("expected task_ai_prompt copied into UserGoalVars, got %q", got)
	}
}

func TestEnrichWorkflowRuntimeContext(t *testing.T) {
	_ = setupAIWorkflowTestApp(t)

	task := model.Task{
		ID:      "task-runtime-1",
		Title:   "runtime task",
		WorkDir: "/root/dev/aca",
	}
	if err := model.DB.Create(&task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	server := model.SSHServer{
		ID:       "srv-runtime-1",
		Name:     "nat",
		Host:     "114.66.41.143",
		Port:     22,
		Username: "root",
	}
	if err := model.DB.Create(&server).Error; err != nil {
		t.Fatalf("create server failed: %v", err)
	}

	if err := model.DB.Create(&model.TerminalSession{
		ID:        "term-runtime-1",
		Title:     "terminal runtime",
		TaskID:    &task.ID,
		ServerID:  &server.ID,
		Shell:     "bash",
		Status:    "running",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create terminal failed: %v", err)
	}

	ctx := map[string]any{
		"terminal_id": "term-runtime-1",
	}
	enrichWorkflowRuntimeContext(task.ID, "term-runtime-1", server.ID, "terminal", ctx)

	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "aca_runtime_role")); got == "" {
		t.Fatalf("expected aca_runtime_role set")
	}
	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "server_name")); got != "nat" {
		t.Fatalf("expected server_name nat, got %q", got)
	}
	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "server_username")); got != "root" {
		t.Fatalf("expected server_username root, got %q", got)
	}
	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "server_permission_hint")); !strings.Contains(got, "root") {
		t.Fatalf("expected root permission hint, got %q", got)
	}
	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_shell")); got != "bash" {
		t.Fatalf("expected terminal_shell bash, got %q", got)
	}
}

func TestMaybeInjectTerminalBootstrapContext_FirstStartupIncludesBootstrap(t *testing.T) {
	_ = setupAIWorkflowTestApp(t)

	terminalID := "term-bootstrap-first"
	if err := model.DB.Create(&model.TerminalSession{
		ID:        terminalID,
		Title:     "bootstrap first",
		Status:    "running",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create terminal fixture failed: %v", err)
	}

	if err := model.DB.Create([]model.Log{
		{
			ID:         "log-bootstrap-in-1",
			TerminalID: &terminalID,
			LogType:    "ai_input_native",
			Content:    "请继续完成任务A\n",
			CreatedAt:  time.Now().Add(-2 * time.Second),
		},
		{
			ID:         "log-bootstrap-out-1",
			TerminalID: &terminalID,
			LogType:    "ai_output_native",
			Content:    "已完成第一步，准备第二步\n",
			CreatedAt:  time.Now().Add(-1 * time.Second),
		},
	}).Error; err != nil {
		t.Fatalf("create bootstrap logs failed: %v", err)
	}

	ctx := map[string]any{"terminal_id": terminalID}
	maybeInjectTerminalBootstrapContext("", terminalID, "terminal", ctx)

	bootstrap := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_bootstrap"))
	if bootstrap == "" {
		t.Fatalf("expected terminal bootstrap context injected")
	}
	if !strings.Contains(bootstrap, "请继续完成任务A") {
		t.Fatalf("expected bootstrap contains recent input, got: %s", bootstrap)
	}
	if !strings.Contains(bootstrap, "已完成第一步") {
		t.Fatalf("expected bootstrap contains recent output, got: %s", bootstrap)
	}
	if mode := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_bootstrap_mode")); mode != "initial" {
		t.Fatalf("expected bootstrap mode initial, got %q", mode)
	}
}

func TestMaybeInjectTerminalBootstrapContext_NonFirstStartupSkipsBootstrap(t *testing.T) {
	_ = setupAIWorkflowTestApp(t)

	terminalID := "term-bootstrap-repeat"
	if err := model.DB.Create(&model.TerminalSession{
		ID:        terminalID,
		Title:     "bootstrap repeat",
		Status:    "running",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create terminal fixture failed: %v", err)
	}

	if err := model.DB.Create(&model.AIWorkflowEvent{
		SessionID:  "session-repeat-1",
		WorkflowID: "wf-repeat",
		TerminalID: &terminalID,
		EventType:  "session_started",
		Phase:      "lifecycle",
		Summary:    "already started",
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("create existing workflow event failed: %v", err)
	}

	ctx := map[string]any{"terminal_id": terminalID}
	maybeInjectTerminalBootstrapContext("", terminalID, "terminal", ctx)

	if got := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_bootstrap")); got != "" {
		t.Fatalf("expected no bootstrap injected on non-first startup, got: %s", got)
	}
}

func TestMaybeInjectTerminalBootstrapContext_ContinuedTerminalUsesSource(t *testing.T) {
	_ = setupAIWorkflowTestApp(t)

	sourceTerminalID := "term-bootstrap-source"
	newTerminalID := "term-bootstrap-new"

	if err := model.DB.Create(&model.TerminalSession{
		ID:        sourceTerminalID,
		Title:     "source terminal",
		Status:    "exited",
		CreatedAt: time.Now().Add(-10 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create source terminal failed: %v", err)
	}

	if err := model.DB.Create(&model.TerminalSession{
		ID:              newTerminalID,
		Title:           "continued terminal",
		Status:          "running",
		ContinuedFromID: &sourceTerminalID,
		CreatedAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("create new terminal failed: %v", err)
	}

	if err := model.DB.Create([]model.Log{
		{
			ID:         "log-bootstrap-source-in-1",
			TerminalID: &sourceTerminalID,
			LogType:    "input",
			Content:    "继续修复日志展示\n",
			CreatedAt:  time.Now().Add(-3 * time.Minute),
		},
		{
			ID:         "log-bootstrap-source-out-1",
			TerminalID: &sourceTerminalID,
			LogType:    "output",
			Content:    "日志面板已支持分页加载\n",
			CreatedAt:  time.Now().Add(-2 * time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("create source logs failed: %v", err)
	}

	if err := model.DB.Create(&model.AIWorkflowSession{
		ID:         "session-source-1",
		WorkflowID: "wf-source",
		UserGoal:   "test",
		Status:     "paused",
		Messages:   "[]",
		Steps:      "[]",
		Context:    "{}",
		Summary:    "等待用户确认后继续",
		StartedAt:  time.Now().Add(-4 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create source workflow session failed: %v", err)
	}

	if err := model.DB.Create(&model.AIWorkflowEvent{
		SessionID:  "session-source-1",
		WorkflowID: "wf-source",
		TerminalID: &sourceTerminalID,
		EventType:  "session_paused",
		Phase:      "lifecycle",
		Summary:    "paused",
		CreatedAt:  time.Now().Add(-90 * time.Second),
	}).Error; err != nil {
		t.Fatalf("create source workflow event failed: %v", err)
	}

	ctx := map[string]any{"terminal_id": newTerminalID}
	maybeInjectTerminalBootstrapContext("", newTerminalID, "terminal", ctx)

	if mode := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_bootstrap_mode")); mode != "continued" {
		t.Fatalf("expected continued mode, got %q", mode)
	}
	if source := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_bootstrap_source_terminal_id")); source != sourceTerminalID {
		t.Fatalf("expected bootstrap source terminal %q, got %q", sourceTerminalID, source)
	}
	bootstrap := strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_bootstrap"))
	if !strings.Contains(bootstrap, "等待用户确认后继续") {
		t.Fatalf("expected bootstrap includes previous workflow summary, got: %s", bootstrap)
	}
	if !strings.Contains(bootstrap, "继续修复日志展示") {
		t.Fatalf("expected bootstrap includes source input keypoint, got: %s", bootstrap)
	}
	if !strings.Contains(bootstrap, "日志面板已支持分页加载") {
		t.Fatalf("expected bootstrap includes source output keypoint, got: %s", bootstrap)
	}
}
