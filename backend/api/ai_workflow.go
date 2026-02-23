package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/workflow"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var aiWorkflowEngine *workflow.AIWorkflowEngine

const (
	defaultWorkflowEventLimit = 200
	maxWorkflowEventLimit     = 1000
	defaultWorkflowLogLimit   = 300
	maxWorkflowLogLimit       = 2000
)

var terminalPromptLineRegex = regexp.MustCompile(`^[^@\s]+@[^:]+:[^\n]*[#$]\s*$`)

type AIWorkflowEventResponse struct {
	ID         uint64      `json:"id"`
	SessionID  string      `json:"session_id"`
	WorkflowID string      `json:"workflow_id"`
	TaskID     *string     `json:"task_id,omitempty"`
	TerminalID *string     `json:"terminal_id,omitempty"`
	Iteration  int         `json:"iteration"`
	Phase      string      `json:"phase"`
	EventType  string      `json:"event_type"`
	Summary    string      `json:"summary"`
	Payload    interface{} `json:"payload"`
	CreatedAt  string      `json:"created_at"`
}

// InitAIWorkflowEngine initializes the AI workflow engine
func InitAIWorkflowEngine(toolExecutor *workflow.ToolExecutor) *workflow.AIWorkflowEngine {
	aiWorkflowEngine = workflow.NewAIWorkflowEngine(toolExecutor)
	return aiWorkflowEngine
}

// StartAIWorkflow starts an AI-driven workflow
// POST /api/ai-workflow/start
func StartAIWorkflow(c *fiber.Ctx) error {
	var req struct {
		Goal                 string         `json:"goal"`
		WorkflowID           string         `json:"workflow_id"`
		TaskID               string         `json:"task_id"`
		TerminalID           string         `json:"terminal_id"`
		ServerID             string         `json:"server_id"`
		CommandExecutionMode string         `json:"command_execution_mode"`
		TargetServerIDs      []string       `json:"target_server_ids"`
		Context              map[string]any `json:"context"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		return c.Status(400).JSON(fiber.Map{"error": "goal is required"})
	}

	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	opts := workflow.StartWorkflowOptions{
		WorkflowID: strings.TrimSpace(req.WorkflowID),
		Context:    map[string]any{},
	}

	for key, value := range req.Context {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		opts.Context[k] = value
	}

	taskID := strings.TrimSpace(req.TaskID)
	terminalID := strings.TrimSpace(req.TerminalID)
	serverID := strings.TrimSpace(req.ServerID)
	serverID = resolveServerIDHint(serverID, terminalID)
	commandExecutionMode := strings.TrimSpace(req.CommandExecutionMode)

	if taskID != "" {
		opts.Context["task_id"] = taskID
	}
	if terminalID != "" {
		opts.Context["terminal_id"] = terminalID
	}
	if serverID != "" {
		opts.Context["current_server_id"] = serverID
	}

	if commandExecutionMode == "" {
		if terminalID != "" {
			commandExecutionMode = "terminal"
		}
	}
	if commandExecutionMode != "" {
		opts.Context["command_execution_mode"] = commandExecutionMode
	}
	enrichWorkflowContextFromTask(taskID, opts.Context)
	enrichWorkflowRuntimeContext(taskID, terminalID, serverID, commandExecutionMode, opts.Context)
	maybeInjectTerminalBootstrapContext(taskID, terminalID, commandExecutionMode, opts.Context)
	seedWorkflowUserGoalVars(&opts, opts.Context)
	goal = composeGoalWithTaskContext(goal, opts.Context)
	if strings.EqualFold(commandExecutionMode, "terminal") {
		if opts.SystemPromptVars == nil {
			opts.SystemPromptVars = map[string]any{}
		}
		opts.SystemPromptVars["tool_blacklist"] = []string{"create_task", "start_task"}
		goal = fmt.Sprintf("【终端接管约束】必须复用当前终端（terminal_id=%s）与当前服务器（server_id=%s）；禁止调用 create_task/start_task，不要创建新终端。除非用户明确要求，不要执行与目标无关的环境探测命令（如 cat /proc/loadavg、uname、whoami）。同一任务不要混用 one-shot CLI（如 claude -p/codex exec）和交互式 CLI，会造成重复执行；优先交互式方式。禁止把任何内部包装命令（ACA_CMD_BEGIN/ACA_CMD_END/ACA_EOF）当作用户输入。\n\n%s",
			terminalID,
			serverID,
			goal,
		)
	}

	targetServerIDs := make([]string, 0, len(req.TargetServerIDs))
	seen := map[string]struct{}{}
	for _, raw := range req.TargetServerIDs {
		sid := strings.TrimSpace(raw)
		if sid == "" {
			continue
		}
		if _, ok := seen[sid]; ok {
			continue
		}
		seen[sid] = struct{}{}
		targetServerIDs = append(targetServerIDs, sid)
	}
	if len(targetServerIDs) == 0 && serverID != "" {
		targetServerIDs = append(targetServerIDs, serverID)
	}
	if len(targetServerIDs) > 0 {
		opts.Context["target_server_ids"] = targetServerIDs
	}

	if terminalID != "" && serverID != "" {
		opts.Context["terminal_ids_by_server"] = map[string]string{
			serverID: terminalID,
		}
	}

	session, err := aiWorkflowEngine.StartWorkflowWithOptions(c.Context(), goal, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	recordAIWorkflowStartupInputLog(session, taskID, goal)

	return c.JSON(fiber.Map{
		"session_id": session.ID,
		"status":     session.Status,
		"task_id":    taskID,
		"terminal_id": func() string {
			if v := getStringFromAnyMap(session.Context, "terminal_id"); v != "" {
				return v
			}
			return terminalID
		}(),
		"message": "工作流已启动",
	})
}

// GetLatestAIWorkflowSessionByTerminal gets latest related workflow session by terminal id.
// GET /api/ai-workflow/session/by-terminal/:terminalId
func GetLatestAIWorkflowSessionByTerminal(c *fiber.Ctx) error {
	terminalID := strings.TrimSpace(c.Params("terminalId"))
	if terminalID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "terminal id required"})
	}

	var events []model.AIWorkflowEvent
	if err := model.DB.
		Select("id", "session_id").
		Where("terminal_id = ?", terminalID).
		Order("id desc").
		Limit(200).
		Find(&events).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load terminal workflow events"})
	}

	sessionIDs := make([]string, 0, len(events))
	seen := map[string]struct{}{}
	for _, row := range events {
		sid := strings.TrimSpace(row.SessionID)
		if sid == "" {
			continue
		}
		if _, ok := seen[sid]; ok {
			continue
		}
		seen[sid] = struct{}{}
		sessionIDs = append(sessionIDs, sid)
	}
	if len(sessionIDs) == 0 {
		return c.JSON(fiber.Map{
			"terminal_id": terminalID,
			"session_id":  "",
			"status":      "",
		})
	}

	var records []model.AIWorkflowSession
	if err := model.DB.
		Select("id", "status", "started_at", "completed_at").
		Where("id IN ?", sessionIDs).
		Find(&records).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load workflow sessions"})
	}

	statusByID := make(map[string]string, len(records))
	for _, row := range records {
		statusByID[strings.TrimSpace(row.ID)] = strings.TrimSpace(row.Status)
	}

	selectedID := ""
	selectedStatus := ""
	for _, sid := range sessionIDs {
		status := strings.ToLower(strings.TrimSpace(statusByID[sid]))
		if status == "running" || status == "paused" {
			selectedID = sid
			selectedStatus = strings.TrimSpace(statusByID[sid])
			break
		}
	}
	if selectedID == "" {
		selectedID = sessionIDs[0]
		selectedStatus = strings.TrimSpace(statusByID[selectedID])
	}

	if aiWorkflowEngine == nil {
		return c.JSON(fiber.Map{
			"terminal_id": terminalID,
			"session_id":  selectedID,
			"status":      selectedStatus,
		})
	}

	session, err := aiWorkflowEngine.GetSession(selectedID)
	if err != nil || session == nil {
		return c.JSON(fiber.Map{
			"terminal_id": terminalID,
			"session_id":  selectedID,
			"status":      selectedStatus,
		})
	}

	return c.JSON(fiber.Map{
		"terminal_id": terminalID,
		"session_id":  selectedID,
		"status":      strings.TrimSpace(session.Status),
		"session":     session,
	})
}

// GetAIWorkflowSession gets workflow session status
// GET /api/ai-workflow/session/:id
func GetAIWorkflowSession(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session id required"})
	}

	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	session, err := aiWorkflowEngine.GetSession(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"session": session})
}

// GetAIWorkflowSessionEvents gets session timeline events.
// GET /api/ai-workflow/session/:id/events
func GetAIWorkflowSessionEvents(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session id required"})
	}

	var exists int64
	if err := model.DB.Model(&model.AIWorkflowSession{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to query session"})
	}
	if exists == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "session not found"})
	}

	limit := parseBoundedInt(c.Query("limit"), defaultWorkflowEventLimit, 1, maxWorkflowEventLimit)
	afterID := parseUint64(c.Query("after_id"))

	query := model.DB.Model(&model.AIWorkflowEvent{}).Where("session_id = ?", id)
	if afterID > 0 {
		query = query.Where("id > ?", afterID)
	}

	var rows []model.AIWorkflowEvent
	if err := query.Order("id asc").Limit(limit).Find(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load workflow events"})
	}

	items := make([]AIWorkflowEventResponse, 0, len(rows))
	lastID := uint64(0)
	for _, row := range rows {
		if row.ID > lastID {
			lastID = row.ID
		}
		items = append(items, AIWorkflowEventResponse{
			ID:         row.ID,
			SessionID:  row.SessionID,
			WorkflowID: row.WorkflowID,
			TaskID:     row.TaskID,
			TerminalID: row.TerminalID,
			Iteration:  row.Iteration,
			Phase:      row.Phase,
			EventType:  row.EventType,
			Summary:    row.Summary,
			Payload:    parseJSONPayload(row.Payload),
			CreatedAt:  row.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		})
	}

	return c.JSON(fiber.Map{
		"items":    items,
		"count":    len(items),
		"after_id": afterID,
		"last_id":  lastID,
		"has_more": len(rows) == limit,
	})
}

// GetAIWorkflowSessionLogs gets terminal logs for a workflow session.
// GET /api/ai-workflow/session/:id/logs
func GetAIWorkflowSessionLogs(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session id required"})
	}
	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	session, err := aiWorkflowEngine.GetSession(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	terminalID := getStringFromAnyMap(session.Context, "terminal_id")
	taskID := getStringFromAnyMap(session.Context, "task_id")
	if terminalID == "" {
		return c.JSON(fiber.Map{
			"items":       []model.Log{},
			"total":       0,
			"terminal_id": "",
			"task_id":     taskID,
		})
	}

	limit := parseBoundedInt(c.Query("limit"), defaultWorkflowLogLimit, 1, maxWorkflowLogLimit)
	offset := parseBoundedInt(c.Query("offset"), 0, 0, 100000)
	order := strings.ToLower(strings.TrimSpace(c.Query("order")))
	if order != "asc" && order != "desc" {
		order = "desc"
	}
	logType := strings.TrimSpace(c.Query("type"))
	includeRaw := parseBoolWithDefault(c.Query("include_raw"), false)
	logSource := strings.ToLower(strings.TrimSpace(c.Query("source"))) // all, native, pty

	query := model.DB.Model(&model.Log{}).Where("terminal_id = ?", terminalID)
	if taskID != "" {
		query = query.Where("task_id = ? OR task_id IS NULL", taskID)
	}

	nativeTypes := []string{"ai_input_native", "ai_output_native"}
	rawTypes := []string{"input_raw", "output_raw"}
	ptyTypes := []string{"input", "output", "system", "input_raw", "output_raw"}

	if logType != "" {
		query = query.Where("log_type = ?", logType)
	} else {
		switch logSource {
		case "native":
			query = query.Where("log_type IN ?", nativeTypes)
		case "pty":
			if includeRaw {
				query = query.Where("log_type IN ?", ptyTypes)
			} else {
				query = query.Where("log_type IN ?", []string{"input", "output", "system"})
			}
		default:
			// 默认原生优先：如果已有 ai_native 记录，则返回 ai_native + system；
			// 否则保持原有行为（过滤 raw）。
			var nativeCount int64
			nativeQuery := model.DB.Model(&model.Log{}).Where("terminal_id = ?", terminalID)
			if taskID != "" {
				nativeQuery = nativeQuery.Where("task_id = ? OR task_id IS NULL", taskID)
			}
			_ = nativeQuery.Where("log_type IN ?", nativeTypes).Count(&nativeCount).Error
			if nativeCount > 0 {
				query = query.Where("log_type IN ?", []string{"ai_input_native", "ai_output_native", "system"})
			} else if !includeRaw {
				query = query.Where("log_type NOT IN ?", rawTypes)
			}
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to count logs"})
	}

	if order == "asc" {
		query = query.Order("created_at asc")
	} else {
		query = query.Order("created_at desc")
	}

	var logs []model.Log
	if err := query.Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load logs"})
	}

	return c.JSON(fiber.Map{
		"items":       logs,
		"total":       total,
		"order":       order,
		"terminal_id": terminalID,
		"task_id":     taskID,
		"type":        logType,
		"source":      logSource,
		"include_raw": includeRaw,
	})
}

// ListAIWorkflowSessions lists all workflow sessions
// GET /api/ai-workflow/sessions
func ListAIWorkflowSessions(c *fiber.Ctx) error {
	var sessions []model.AIWorkflowSession
	if err := model.DB.Order("started_at DESC").Limit(50).Find(&sessions).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": sessions})
}

// PostAIWorkflowMessage appends a user message and resumes a paused session.
// POST /api/ai-workflow/session/:id/message
func PostAIWorkflowMessage(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session id required"})
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	session, err := aiWorkflowEngine.ResumeWorkflow(c.Context(), id, req.Message)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "已提交补充信息，工作流继续执行",
		"session": session,
	})
}

// PostAIWorkflowPause pauses a running workflow session.
// POST /api/ai-workflow/session/:id/pause
func PostAIWorkflowPause(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session id required"})
	}

	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&req)

	session, err := aiWorkflowEngine.PauseWorkflow(c.Context(), id, req.Reason)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "会话已暂停",
		"session": session,
	})
}

func parseBoundedInt(raw string, defaultValue, minValue, maxValue int) int {
	text := strings.TrimSpace(raw)
	if text == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(text)
	if err != nil {
		return defaultValue
	}
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func parseUint64(raw string) uint64 {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0
	}
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseBoolWithDefault(raw string, defaultValue bool) bool {
	text := strings.TrimSpace(strings.ToLower(raw))
	if text == "" {
		return defaultValue
	}
	switch text {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

func getStringFromAnyMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	val, ok := values[key]
	if !ok || val == nil {
		return ""
	}
	switch typed := val.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func parseJSONPayload(raw string) interface{} {
	text := strings.TrimSpace(raw)
	if text == "" {
		return map[string]any{}
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return raw
	}
	return payload
}

func enrichWorkflowContextFromTask(taskID string, ctx map[string]any) {
	tid := strings.TrimSpace(taskID)
	if tid == "" || model.DB == nil || ctx == nil {
		return
	}

	var task model.Task
	if err := model.DB.First(&task, "id = ?", tid).Error; err != nil {
		return
	}

	setContextStringIfEmpty(ctx, "task_title", task.Title)
	setContextStringIfEmpty(ctx, "task_description", task.Description)
	setContextStringIfEmpty(ctx, "task_remark", task.Remark)
	setContextStringIfEmpty(ctx, "task_priority", strconv.Itoa(task.Priority))
	setContextStringIfEmpty(ctx, "task_automation_mode", task.AutomationMode)
	setContextStringIfEmpty(ctx, "task_work_dir", task.WorkDir)
	setContextStringIfEmpty(ctx, "task_initial_prompt", task.InitialPrompt)
	setContextStringIfEmpty(ctx, "task_ai_prompt", task.AIPrompt)
	setContextStringIfEmpty(ctx, "task_ai_end_condition", task.AIEndCondition)
	setContextStringIfEmpty(ctx, "task_ai_error_handling", task.AIErrorHandling)
}

func enrichWorkflowRuntimeContext(taskID, terminalID, serverID, commandExecutionMode string, ctx map[string]any) {
	if ctx == nil {
		return
	}

	mode := strings.TrimSpace(commandExecutionMode)
	if mode == "" {
		mode = strings.TrimSpace(getStringFromAnyMap(ctx, "command_execution_mode"))
	}

	setContextStringIfEmpty(ctx, "aca_runtime_role", "ACA AI托管执行代理")
	setContextStringIfEmpty(ctx, "aca_runtime_capabilities", "可连接服务器终端、执行AI CLI/命令、记录全流程日志并支持复盘审计")
	if strings.EqualFold(mode, "terminal") {
		setContextStringIfEmpty(ctx, "aca_runtime_constraint", "外部终端接管模式：必须复用当前终端，不创建新终端")
	}

	if model.DB == nil {
		return
	}

	tid := firstNonEmptyString(strings.TrimSpace(terminalID), strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_id")))
	var terminalServerID string
	if tid != "" {
		var terminal model.TerminalSession
		if err := model.DB.Select("id", "title", "shell", "status", "server_id", "task_id").First(&terminal, "id = ?", tid).Error; err == nil {
			setContextStringIfEmpty(ctx, "terminal_title", terminal.Title)
			setContextStringIfEmpty(ctx, "terminal_shell", terminal.Shell)
			setContextStringIfEmpty(ctx, "terminal_status", terminal.Status)
			if terminal.ServerID != nil {
				terminalServerID = strings.TrimSpace(*terminal.ServerID)
				setContextStringIfEmpty(ctx, "current_server_id", terminalServerID)
			}
			if strings.TrimSpace(taskID) == "" && terminal.TaskID != nil {
				setContextStringIfEmpty(ctx, "task_id", strings.TrimSpace(*terminal.TaskID))
			}
		}
	}

	sid := firstNonEmptyString(strings.TrimSpace(serverID), terminalServerID, strings.TrimSpace(getStringFromAnyMap(ctx, "current_server_id")))
	if sid != "" {
		var server model.SSHServer
		if err := model.DB.Select("id", "name", "host", "port", "username").First(&server, "id = ?", sid).Error; err == nil {
			setContextStringIfEmpty(ctx, "server_name", server.Name)
			setContextStringIfEmpty(ctx, "server_host", server.Host)
			if server.Port > 0 {
				setContextStringIfEmpty(ctx, "server_port", strconv.Itoa(server.Port))
			}
			setContextStringIfEmpty(ctx, "server_username", server.Username)
			if strings.EqualFold(strings.TrimSpace(server.Username), "root") {
				setContextStringIfEmpty(ctx, "server_permission_hint", "当前登录用户为 root（高权限）")
			}
		}
	}

	taskIDResolved := firstNonEmptyString(strings.TrimSpace(taskID), strings.TrimSpace(getStringFromAnyMap(ctx, "task_id")))
	if taskIDResolved != "" {
		var task model.Task
		if err := model.DB.Select("id", "work_dir").First(&task, "id = ?", taskIDResolved).Error; err == nil {
			setContextStringIfEmpty(ctx, "task_work_dir", task.WorkDir)
		}
	}
}

func setContextStringIfEmpty(ctx map[string]any, key, value string) {
	if ctx == nil {
		return
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return
	}
	if strings.TrimSpace(getStringFromAnyMap(ctx, k)) != "" {
		return
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	ctx[k] = trimmed
}

func seedWorkflowUserGoalVars(opts *workflow.StartWorkflowOptions, ctx map[string]any) {
	if opts == nil || ctx == nil {
		return
	}
	if opts.UserGoalVars == nil {
		opts.UserGoalVars = map[string]any{}
	}
	copyUserGoalVar := func(name string, value any) {
		key := strings.TrimSpace(name)
		if key == "" || value == nil {
			return
		}
		if _, exists := opts.UserGoalVars[key]; exists {
			return
		}
		switch typed := value.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				return
			}
			opts.UserGoalVars[key] = text
		default:
			opts.UserGoalVars[key] = value
		}
	}

	copyUserGoalVar("task_title", getStringFromAnyMap(ctx, "task_title"))
	copyUserGoalVar("task_description", getStringFromAnyMap(ctx, "task_description"))
	copyUserGoalVar("task_initial_prompt", getStringFromAnyMap(ctx, "task_initial_prompt"))
	copyUserGoalVar("task_ai_prompt", getStringFromAnyMap(ctx, "task_ai_prompt"))
	copyUserGoalVar("task_ai_end_condition", getStringFromAnyMap(ctx, "task_ai_end_condition"))
	copyUserGoalVar("task_ai_error_handling", getStringFromAnyMap(ctx, "task_ai_error_handling"))
	copyUserGoalVar("work_dir", firstNonEmptyString(
		getStringFromAnyMap(ctx, "task_work_dir"),
		getStringFromAnyMap(ctx, "work_dir"),
	))
}

func composeGoalWithTaskContext(goal string, ctx map[string]any) string {
	userGoal := strings.TrimSpace(goal)
	if userGoal == "" {
		return ""
	}
	if ctx == nil {
		return userGoal
	}
	if strings.Contains(userGoal, "【任务上下文】") ||
		strings.Contains(userGoal, "【终端关键上下文】") ||
		strings.Contains(userGoal, "【运行环境】") {
		return userGoal
	}

	lines := make([]string, 0, 12)
	appendContextLine := func(label, value string, maxRunes int) {
		text := truncateRunes(strings.TrimSpace(value), maxRunes)
		if text == "" {
			return
		}
		lines = append(lines, fmt.Sprintf("- %s：%s", label, text))
	}

	appendContextLine("任务ID", getStringFromAnyMap(ctx, "task_id"), 80)
	appendContextLine("标题", getStringFromAnyMap(ctx, "task_title"), 200)
	appendContextLine("描述", getStringFromAnyMap(ctx, "task_description"), 500)
	appendContextLine("备注", getStringFromAnyMap(ctx, "task_remark"), 300)
	appendContextLine("优先级", getStringFromAnyMap(ctx, "task_priority"), 20)
	appendContextLine("自动化模式", firstNonEmptyString(
		getStringFromAnyMap(ctx, "task_automation_mode"),
		getStringFromAnyMap(ctx, "automation_mode"),
	), 60)
	appendContextLine("工作目录", firstNonEmptyString(
		getStringFromAnyMap(ctx, "task_work_dir"),
		getStringFromAnyMap(ctx, "work_dir"),
	), 220)
	appendContextLine("任务初始提示", getStringFromAnyMap(ctx, "task_initial_prompt"), 600)
	appendContextLine("任务AI补充要求", getStringFromAnyMap(ctx, "task_ai_prompt"), 600)
	appendContextLine("任务结束条件", getStringFromAnyMap(ctx, "task_ai_end_condition"), 400)
	appendContextLine("任务错误处理", getStringFromAnyMap(ctx, "task_ai_error_handling"), 400)
	appendContextLine("当前运行命令", getStringFromAnyMap(ctx, "running_command"), 220)

	envLines := make([]string, 0, 12)
	appendEnvLine := func(label, value string, maxRunes int) {
		text := truncateRunes(strings.TrimSpace(value), maxRunes)
		if text == "" {
			return
		}
		envLines = append(envLines, fmt.Sprintf("- %s：%s", label, text))
	}

	serverAddr := strings.TrimSpace(getStringFromAnyMap(ctx, "server_host"))
	serverPort := strings.TrimSpace(getStringFromAnyMap(ctx, "server_port"))
	if serverAddr != "" && serverPort != "" {
		serverAddr = serverAddr + ":" + serverPort
	}

	appendEnvLine("执行模式", getStringFromAnyMap(ctx, "command_execution_mode"), 40)
	appendEnvLine("终端ID", getStringFromAnyMap(ctx, "terminal_id"), 100)
	appendEnvLine("终端标题", getStringFromAnyMap(ctx, "terminal_title"), 200)
	appendEnvLine("终端Shell", getStringFromAnyMap(ctx, "terminal_shell"), 60)
	appendEnvLine("终端状态", getStringFromAnyMap(ctx, "terminal_status"), 40)
	appendEnvLine("服务器ID", getStringFromAnyMap(ctx, "current_server_id"), 100)
	appendEnvLine("服务器名称", getStringFromAnyMap(ctx, "server_name"), 160)
	appendEnvLine("服务器地址", serverAddr, 220)
	appendEnvLine("登录用户", getStringFromAnyMap(ctx, "server_username"), 80)
	appendEnvLine("权限提示", getStringFromAnyMap(ctx, "server_permission_hint"), 200)
	appendEnvLine("ACA角色", getStringFromAnyMap(ctx, "aca_runtime_role"), 80)
	appendEnvLine("ACA能力", getStringFromAnyMap(ctx, "aca_runtime_capabilities"), 260)
	appendEnvLine("ACA约束", getStringFromAnyMap(ctx, "aca_runtime_constraint"), 260)

	sections := make([]string, 0, 2)
	if len(lines) > 0 {
		sections = append(sections, fmt.Sprintf("【任务上下文】\n%s", strings.Join(lines, "\n")))
	}

	if len(envLines) > 0 {
		sections = append(sections, fmt.Sprintf("【运行环境】\n%s", strings.Join(envLines, "\n")))
	}

	terminalBootstrap := truncateRunes(getStringFromAnyMap(ctx, "manual_context_block"), 2200)
	if terminalBootstrap == "" {
		terminalBootstrap = truncateRunes(getStringFromAnyMap(ctx, "terminal_bootstrap"), 2200)
	}
	if terminalBootstrap != "" {
		sections = append(sections, fmt.Sprintf("【终端关键上下文】\n%s", terminalBootstrap))
	}

	if len(sections) == 0 {
		return userGoal
	}

	return strings.TrimSpace(fmt.Sprintf("%s\n\n【用户目标】\n%s", strings.Join(sections, "\n\n"), userGoal))
}

func maybeInjectTerminalBootstrapContext(taskID, terminalID, commandExecutionMode string, ctx map[string]any) {
	if ctx == nil || model.DB == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(commandExecutionMode), "terminal") {
		return
	}

	currentTerminalID := strings.TrimSpace(terminalID)
	if currentTerminalID == "" {
		currentTerminalID = strings.TrimSpace(getStringFromAnyMap(ctx, "terminal_id"))
	}
	if currentTerminalID == "" {
		return
	}
	if !isFirstAIWorkflowStartupForTerminal(currentTerminalID) {
		return
	}

	sourceTerminalID := currentTerminalID
	mode := "initial"
	if continuedFrom := resolveContinuedFromTerminalID(currentTerminalID); continuedFrom != "" {
		sourceTerminalID = continuedFrom
		mode = "continued"
	}

	summary := buildTerminalBootstrapSummary(sourceTerminalID, strings.TrimSpace(taskID))
	if summary == "" {
		return
	}

	ctx["terminal_bootstrap"] = summary
	ctx["terminal_bootstrap_mode"] = mode
	ctx["terminal_bootstrap_source_terminal_id"] = sourceTerminalID
}

func isFirstAIWorkflowStartupForTerminal(terminalID string) bool {
	tid := strings.TrimSpace(terminalID)
	if tid == "" || model.DB == nil {
		return false
	}
	var count int64
	if err := model.DB.Model(&model.AIWorkflowEvent{}).
		Where("terminal_id = ? AND event_type = ?", tid, "session_started").
		Count(&count).Error; err != nil {
		return false
	}
	return count == 0
}

func resolveContinuedFromTerminalID(terminalID string) string {
	tid := strings.TrimSpace(terminalID)
	if tid == "" || model.DB == nil {
		return ""
	}
	var record model.TerminalSession
	if err := model.DB.Select("id", "continued_from_id").First(&record, "id = ?", tid).Error; err != nil {
		return ""
	}
	if record.ContinuedFromID == nil {
		return ""
	}
	return strings.TrimSpace(*record.ContinuedFromID)
}

func buildTerminalBootstrapSummary(sourceTerminalID, taskID string) string {
	sourceID := strings.TrimSpace(sourceTerminalID)
	if sourceID == "" || model.DB == nil {
		return ""
	}

	lines := []string{fmt.Sprintf("- 来源终端：%s", sourceID)}
	environment := buildTerminalEnvironmentSummary(sourceID, taskID)
	if environment != "" {
		lines = append(lines, fmt.Sprintf("- 运行环境：%s", environment))
	}

	status, summary := latestWorkflowStatusSummaryByTerminal(sourceID)
	if status != "" {
		lines = append(lines, fmt.Sprintf("- 上一次AI托管状态：%s", status))
	}
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- 上一次AI托管摘要：%s", truncateRunes(summary, 320)))
	}

	userInputs, outputs := extractTerminalKeyPoints(sourceID, taskID)
	if objective := inferTerminalObjective(userInputs); objective != "" {
		lines = append(lines, fmt.Sprintf("- 当前目标线索：%s", objective))
	}
	if len(userInputs) > 0 {
		lines = append(lines, "- 最近用户输入（最多3条）：")
		for _, entry := range userInputs {
			lines = append(lines, "  - "+entry)
		}
	}
	if len(outputs) > 0 {
		lines = append(lines, "- 最近关键结果（最多3条）：")
		for idx, entry := range outputs {
			if idx >= 3 {
				break
			}
			lines = append(lines, "  - "+entry)
		}
	}
	if risks := extractTerminalRiskSignals(outputs); len(risks) > 0 {
		lines = append(lines, "- 最近风险/错误信号（最多3条）：")
		for _, entry := range risks {
			lines = append(lines, "  - "+entry)
		}
	}

	if len(lines) <= 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func buildTerminalEnvironmentSummary(sourceTerminalID, taskID string) string {
	sourceID := strings.TrimSpace(sourceTerminalID)
	if sourceID == "" || model.DB == nil {
		return ""
	}

	parts := make([]string, 0, 8)

	var terminal model.TerminalSession
	if err := model.DB.Select("id", "title", "shell", "status", "server_id", "task_id").First(&terminal, "id = ?", sourceID).Error; err == nil {
		if title := strings.TrimSpace(terminal.Title); title != "" {
			parts = append(parts, "终端="+title)
		}
		if shell := strings.TrimSpace(terminal.Shell); shell != "" {
			parts = append(parts, "shell="+shell)
		}
		if status := strings.TrimSpace(terminal.Status); status != "" {
			parts = append(parts, "状态="+status)
		}

		serverID := ""
		if terminal.ServerID != nil {
			serverID = strings.TrimSpace(*terminal.ServerID)
		}
		if serverID != "" {
			var server model.SSHServer
			if err := model.DB.Select("id", "name", "host", "port", "username").First(&server, "id = ?", serverID).Error; err == nil {
				if name := strings.TrimSpace(server.Name); name != "" {
					parts = append(parts, "服务器="+name)
				}
				host := strings.TrimSpace(server.Host)
				if host != "" {
					if server.Port > 0 {
						host = host + ":" + strconv.Itoa(server.Port)
					}
					parts = append(parts, "地址="+host)
				}
				if user := strings.TrimSpace(server.Username); user != "" {
					parts = append(parts, "用户="+user)
				}
			}
		}

		resolvedTaskID := strings.TrimSpace(taskID)
		if resolvedTaskID == "" && terminal.TaskID != nil {
			resolvedTaskID = strings.TrimSpace(*terminal.TaskID)
		}
		if resolvedTaskID != "" {
			var task model.Task
			if err := model.DB.Select("id", "work_dir").First(&task, "id = ?", resolvedTaskID).Error; err == nil {
				if workDir := strings.TrimSpace(task.WorkDir); workDir != "" {
					parts = append(parts, "目录="+workDir)
				}
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

func inferTerminalObjective(userInputs []string) string {
	for _, item := range userInputs {
		text := strings.TrimSpace(item)
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		if lower == "claude" || lower == "codex" || lower == "gemini" {
			continue
		}
		return truncateRunes(text, 220)
	}
	return ""
}

func extractTerminalRiskSignals(outputs []string) []string {
	if len(outputs) == 0 {
		return nil
	}
	keywords := []string{
		"error", "failed", "failure", "panic", "exception", "denied", "not found", "timeout",
		"错误", "失败", "异常", "拒绝", "未找到", "超时",
	}

	results := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, row := range outputs {
		text := strings.TrimSpace(row)
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		matched := false
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		text = truncateRunes(text, 220)
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		results = append(results, text)
		if len(results) >= 3 {
			break
		}
	}
	return results
}

func latestWorkflowStatusSummaryByTerminal(terminalID string) (status string, summary string) {
	tid := strings.TrimSpace(terminalID)
	if tid == "" || model.DB == nil {
		return "", ""
	}

	var event model.AIWorkflowEvent
	if err := model.DB.Select("session_id").Where("terminal_id = ?", tid).Order("id desc").First(&event).Error; err != nil {
		return "", ""
	}
	sessionID := strings.TrimSpace(event.SessionID)
	if sessionID == "" {
		return "", ""
	}

	var session model.AIWorkflowSession
	if err := model.DB.Select("id", "status", "summary").First(&session, "id = ?", sessionID).Error; err != nil {
		return "", ""
	}
	return strings.TrimSpace(session.Status), strings.TrimSpace(session.Summary)
}

func extractTerminalKeyPoints(terminalID, taskID string) (userInputs []string, outputs []string) {
	tid := strings.TrimSpace(terminalID)
	if tid == "" || model.DB == nil {
		return nil, nil
	}

	query := model.DB.Model(&model.Log{}).
		Where("terminal_id = ?", tid).
		Where("log_type IN ?", []string{"ai_input_native", "ai_output_native", "input", "output", "system"}).
		Order("created_at desc").
		Limit(240)
	if strings.TrimSpace(taskID) != "" {
		query = query.Where("task_id = ? OR task_id IS NULL", strings.TrimSpace(taskID))
	}

	var logs []model.Log
	if err := query.Find(&logs).Error; err != nil || len(logs) == 0 {
		return nil, nil
	}

	inputSeen := map[string]struct{}{}
	outputSeen := map[string]struct{}{}
	for _, row := range logs {
		if len(userInputs) >= 3 && len(outputs) >= 5 {
			break
		}
		text := sanitizeTerminalContextSnippet(row.Content, 220)
		if text == "" {
			continue
		}
		logType := strings.ToLower(strings.TrimSpace(row.LogType))
		switch logType {
		case "ai_input_native", "input":
			if len(userInputs) >= 3 {
				continue
			}
			if _, exists := inputSeen[text]; exists {
				continue
			}
			inputSeen[text] = struct{}{}
			userInputs = append(userInputs, text)
		case "ai_output_native", "output", "system":
			if len(outputs) >= 5 {
				continue
			}
			if _, exists := outputSeen[text]; exists {
				continue
			}
			outputSeen[text] = struct{}{}
			outputs = append(outputs, text)
		}
	}

	return userInputs, outputs
}

func sanitizeTerminalContextSnippet(raw string, maxRunes int) string {
	text := strings.TrimSpace(stripANSIForResume(raw))
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	cleanLines := make([]string, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item == "" {
			continue
		}
		if isTerminalContextNoiseLine(item) {
			continue
		}
		cleanLines = append(cleanLines, item)
	}
	if len(cleanLines) == 0 {
		return ""
	}
	joined := strings.Join(cleanLines, " ")
	joined = strings.Join(strings.Fields(joined), " ")
	if isTerminalContextNoiseLine(joined) {
		return ""
	}
	return truncateRunes(joined, maxRunes)
}

func isTerminalContextNoiseLine(line string) bool {
	text := strings.TrimSpace(line)
	if text == "" {
		return true
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "__aca_cmd_begin_") || strings.Contains(lower, "__aca_cmd_end_") || strings.Contains(lower, "aca_eof") {
		return true
	}
	if strings.HasPrefix(lower, "aca_code=$?;") || strings.HasPrefix(lower, "unset aca_code") {
		return true
	}
	if strings.HasPrefix(lower, "press ctrl-c again to exit") || strings.HasPrefix(lower, "? for shortcuts") {
		return true
	}
	switch text {
	case "✶", "✽", "✻", "✢", "·", "…", "...", "❯":
		return true
	}
	if terminalPromptLineRegex.MatchString(text) {
		return true
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, raw := range values {
		text := strings.TrimSpace(raw)
		if text != "" {
			return text
		}
	}
	return ""
}

func truncateRunes(input string, maxRunes int) string {
	text := strings.TrimSpace(input)
	if text == "" || maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + " ...(truncated)"
}

func recordAIWorkflowStartupInputLog(session *workflow.AIWorkflowSession, fallbackTaskID string, prompt string) {
	if session == nil || model.DB == nil {
		return
	}

	terminalID := strings.TrimSpace(getStringFromAnyMap(session.Context, "terminal_id"))
	if terminalID == "" {
		return
	}

	content := strings.TrimSpace(prompt)
	if content == "" {
		return
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	taskID := strings.TrimSpace(getStringFromAnyMap(session.Context, "task_id"))
	if taskID == "" {
		taskID = strings.TrimSpace(fallbackTaskID)
	}

	duplicateSince := time.Now().Add(-5 * time.Second)
	query := model.DB.Model(&model.Log{}).
		Where("terminal_id = ? AND log_type = ? AND content = ? AND created_at >= ?", terminalID, "ai_input_native", content, duplicateSince)
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	var duplicateCount int64
	if err := query.Count(&duplicateCount).Error; err == nil && duplicateCount > 0 {
		return
	}

	terminalCopy := terminalID
	var taskIDPtr *string
	if taskID != "" {
		taskCopy := taskID
		taskIDPtr = &taskCopy
	}

	_ = model.DB.Create(&model.Log{
		ID:         uuid.NewString(),
		TerminalID: &terminalCopy,
		TaskID:     taskIDPtr,
		LogType:    "ai_input_native",
		Content:    content,
		CreatedAt:  time.Now(),
	}).Error
}

func resolveServerIDHint(serverID, terminalID string) string {
	sid := strings.TrimSpace(serverID)
	tid := strings.TrimSpace(terminalID)

	isValidServerID := func(id string) bool {
		if id == "" || model.DB == nil {
			return false
		}
		var count int64
		if err := model.DB.Model(&model.SSHServer{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return false
		}
		return count > 0
	}

	if isValidServerID(sid) {
		return sid
	}

	if tid != "" && model.DB != nil {
		var terminal model.TerminalSession
		if err := model.DB.Select("id", "server_id").First(&terminal, "id = ?", tid).Error; err == nil {
			if terminal.ServerID != nil {
				candidate := strings.TrimSpace(*terminal.ServerID)
				if isValidServerID(candidate) {
					return candidate
				}
			}
		}
	}

	return ""
}
