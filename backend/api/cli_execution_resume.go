package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	clisvc "github.com/ai-coding-assistant/service/cli"
	"github.com/gofiber/fiber/v2"
)

const (
	resumeStrategyAuto        = "auto"
	resumeStrategyNative      = "native"
	resumeStrategyPrompt      = "prompt_concat"
	resumeStrategyNativeLabel = "nativeResume"
	resumeStrategyPromptLabel = "promptConcat"
)

var (
	cliResumeUUIDRegex      = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	cliResumeCodexFileRegex = regexp.MustCompile(`(?i)(rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl)`)
	cliResumeANSIRegex      = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)`)
)

type ResumeCLIExecutionRequest struct {
	Strategy  string `json:"strategy"` // auto, native, prompt_concat
	SessionID string `json:"session_id"`
	WorkDir   string `json:"work_dir"`
	ServerID  string `json:"server_id"`
	CLIType   string `json:"cli_type"`
	Prompt    string `json:"prompt"`
	Title     string `json:"title"`
}

func (ctrl *CLIExecutionController) ResumeExecution(c *fiber.Ctx) error {
	if ctrl == nil || ctrl.terminalManager == nil {
		return c.Status(500).JSON(fiber.Map{"error": "terminal manager not configured"})
	}

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "execution id is required"})
	}

	var req ResumeCLIExecutionRequest
	if err := c.BodyParser(&req); err != nil {
		req = ResumeCLIExecutionRequest{}
	}

	requestedStrategy, ok := normalizeResumeStrategy(req.Strategy)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "invalid strategy"})
	}

	var parent model.CLIExecution
	if err := model.DB.First(&parent, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "execution not found"})
	}

	parentMeta := parseJSONMap(parent.Metadata)
	taskID := trimPtr(parent.TaskID)
	task, _ := loadTaskByID(taskID)

	cliType := resolveResumeCLIType(
		req.CLIType,
		parent.Tool,
		getStringFromAny(parentMeta, "cli_type"),
		getTaskCLIType(task),
	)
	if cliType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "unable to determine cli_type"})
	}

	workDir := firstNonEmptyTrim(
		req.WorkDir,
		getStringFromAny(parentMeta, "work_dir"),
		getTaskWorkDir(task),
	)
	serverID := firstNonEmptyTrim(
		req.ServerID,
		getStringFromAny(parentMeta, "server_id"),
		getTaskServerID(task),
		resolveTerminalServerID(trimPtr(parent.TerminalID)),
	)
	sessionID := firstNonEmptyTrim(
		req.SessionID,
		getStringFromAny(parentMeta, "session_id"),
		getStringFromAny(parentMeta, "cli_session_id"),
	)

	if sessionID == "" {
		sessionID = lookupLatestAISessionID(taskID, trimPtr(parent.TerminalID), resumeCLITypeToAIType(cliType))
	}
	if sessionID == "" {
		sessionID = detectSessionIDFromTerminalLogs(trimPtr(parent.TerminalID), taskID, cliType)
	}

	selectedStrategy, command, prompt, buildErr := buildResumeCommand(resumeCommandInput{
		RequestedStrategy: requestedStrategy,
		CLIType:           cliType,
		SessionID:         sessionID,
		WorkDir:           workDir,
		Prompt:            req.Prompt,
		ParentPrompt:      parent.PromptPreview,
	})
	if buildErr != nil {
		return c.Status(400).JSON(fiber.Map{"error": buildErr.Error()})
	}

	terminalTitle := strings.TrimSpace(req.Title)
	if terminalTitle == "" {
		terminalTitle = fmt.Sprintf("[%s] Resume %s", cliType, shortID(parent.ID, 8))
	}

	var (
		terminalSessionID string
	)
	if serverID != "" {
		session, err := ctrl.terminalManager.CreateSSHSession(serverID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create resume terminal"})
		}
		terminalSessionID = session.ID()
	} else {
		taskIDPtr := ptrOrNil(taskID)
		session, err := ctrl.terminalManager.CreateSession(terminalTitle, taskIDPtr)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create resume terminal"})
		}
		terminalSessionID = session.ID()
	}

	if taskID != "" {
		taskIDCopy := taskID
		_ = ctrl.terminalManager.LinkTask(terminalSessionID, &taskIDCopy)
	}
	if terminalTitle != "" {
		_ = ctrl.terminalManager.RenameSession(terminalSessionID, terminalTitle)
	}

	session, err := ctrl.terminalManager.GetOrResumeSession(terminalSessionID)
	if err != nil || session == nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to open resume terminal"})
	}

	tracker := clisvc.NewExecutionTracker(model.DB)
	executionID := ""
	appendEvent := func(eventType string, payload map[string]any) {}
	complete := func(status, errMsg string) {}
	if tracker != nil {
		started, startErr := tracker.Start(clisvc.StartExecutionInput{
			TaskID:            ptrOrNil(taskID),
			TerminalID:        ptrOrNil(terminalSessionID),
			WorkflowRunID:     trimStringPtr(parent.WorkflowRunID),
			WorkflowSessionID: trimStringPtr(parent.WorkflowSessionID),
			ParentExecutionID: ptrOrNil(parent.ID),
			Role:              clisvc.ExecutionRoleReplay,
			Tool:              cliType,
			Mode:              "resume",
			Source:            "api-resume",
			Prompt:            command,
			Metadata: map[string]any{
				"parent_execution_id": parent.ID,
				"strategy":            selectedStrategy,
				"server_id":           serverID,
				"work_dir":            workDir,
				"session_id":          sessionID,
				"prompt_concat":       prompt,
			},
		})
		if startErr == nil && started != nil {
			executionID = strings.TrimSpace(started.ID)
			appendEvent = func(eventType string, payload map[string]any) {
				_ = tracker.AppendEvent(executionID, eventType, payload)
			}
			complete = func(status, errMsg string) {
				_ = tracker.Complete(executionID, status, nil, errMsg)
			}
		}
	}

	appendEvent(clisvc.EventTypeProgress, map[string]any{
		"stage":               "resume_requested",
		"parent_execution_id": parent.ID,
		"strategy":            selectedStrategy,
		"server_id":           serverID,
		"work_dir":            workDir,
		"session_id":          sessionID,
		"command":             command,
	})

	if err := session.Write([]byte(command + "\r")); err != nil {
		appendEvent(clisvc.EventTypeError, map[string]any{
			"stage":   "resume_command_failed",
			"command": command,
			"error":   err.Error(),
		})
		complete(clisvc.StatusError, err.Error())
		return c.Status(500).JSON(fiber.Map{"error": "failed to start resume command"})
	}

	appendEvent(clisvc.EventTypeStarted, map[string]any{
		"stage":       "resume_started",
		"terminal_id": terminalSessionID,
		"command":     command,
		"strategy":    selectedStrategy,
	})

	readyErr := error(nil)
	if ctrl.taskLauncher != nil {
		readyErr = ctrl.taskLauncher.WaitForReady(terminalSessionID, 45*time.Second)
	}
	if readyErr != nil {
		appendEvent(clisvc.EventTypeProgress, map[string]any{
			"stage": "resume_wait_ready_timeout",
			"error": readyErr.Error(),
		})
	} else {
		appendEvent(clisvc.EventTypeProgress, map[string]any{
			"stage": "resume_ready",
		})
	}

	if selectedStrategy == resumeStrategyPromptLabel && strings.TrimSpace(prompt) != "" {
		sendErr := error(nil)
		if ctrl.taskLauncher != nil {
			sendErr = ctrl.taskLauncher.SendTask(terminalSessionID, prompt)
		} else {
			sendErr = session.Write([]byte(prompt + "\r"))
		}
		if sendErr != nil {
			appendEvent(clisvc.EventTypeError, map[string]any{
				"stage": "resume_prompt_send_failed",
				"error": sendErr.Error(),
			})
			complete(clisvc.StatusError, sendErr.Error())
			return c.Status(500).JSON(fiber.Map{"error": "failed to send resume prompt"})
		}
		appendEvent(clisvc.EventTypeProgress, map[string]any{
			"stage":      "resume_prompt_sent",
			"prompt_len": len([]rune(prompt)),
		})
	}

	return c.JSON(fiber.Map{
		"message":             "CLI resume started",
		"strategy":            selectedStrategy,
		"execution_id":        executionID,
		"parent_execution_id": parent.ID,
		"terminal_id":         terminalSessionID,
		"command":             command,
		"session_id":          sessionID,
		"server_id":           serverID,
		"work_dir":            workDir,
	})
}

type resumeCommandInput struct {
	RequestedStrategy string
	CLIType           string
	SessionID         string
	WorkDir           string
	Prompt            string
	ParentPrompt      string
}

func buildResumeCommand(input resumeCommandInput) (selectedStrategy string, command string, prompt string, err error) {
	cliType := normalizeResumeCLIType(input.CLIType)
	if cliType == "" {
		return "", "", "", errors.New("unsupported cli_type")
	}

	requested := strings.TrimSpace(input.RequestedStrategy)
	if requested == "" {
		requested = resumeStrategyAuto
	}

	nativeCmd, nativeErr := buildNativeResumeCommand(cliType, strings.TrimSpace(input.SessionID), strings.TrimSpace(input.WorkDir))
	startCmd, startErr := buildCLIStartCommand(cliType, strings.TrimSpace(input.WorkDir))

	finalPrompt := strings.TrimSpace(input.Prompt)
	if finalPrompt == "" {
		parentPrompt := strings.TrimSpace(input.ParentPrompt)
		if parentPrompt != "" {
			finalPrompt = "继续上一轮会话并保持原有上下文。上次指令摘要:\n" + parentPrompt
		} else {
			finalPrompt = "继续之前的任务上下文，并先汇报当前状态和下一步计划。"
		}
	}

	switch requested {
	case resumeStrategyNative:
		if nativeErr != nil {
			return "", "", "", nativeErr
		}
		return resumeStrategyNativeLabel, nativeCmd, "", nil
	case resumeStrategyPrompt:
		if startErr != nil {
			return "", "", "", startErr
		}
		return resumeStrategyPromptLabel, startCmd, finalPrompt, nil
	case resumeStrategyAuto:
		if nativeErr == nil {
			return resumeStrategyNativeLabel, nativeCmd, "", nil
		}
		if startErr == nil {
			return resumeStrategyPromptLabel, startCmd, finalPrompt, nil
		}
		return "", "", "", nativeErr
	default:
		return "", "", "", errors.New("invalid strategy")
	}
}

func normalizeResumeStrategy(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", resumeStrategyAuto:
		return resumeStrategyAuto, true
	case resumeStrategyNative:
		return resumeStrategyNative, true
	case resumeStrategyPrompt, "prompt", "promptconcat":
		return resumeStrategyPrompt, true
	default:
		return "", false
	}
}

func buildNativeResumeCommand(cliType, sessionID, workDir string) (string, error) {
	switch normalizeResumeCLIType(cliType) {
	case "claude":
		return withWorkDir(workDir, "claude --continue"), nil
	case "codex":
		if strings.TrimSpace(sessionID) == "" {
			return "", errors.New("codex native resume requires session_id")
		}
		return withWorkDir(workDir, "codex --resume "+shellQuote(strings.TrimSpace(sessionID))), nil
	default:
		return "", errors.New("native resume is unsupported for this cli_type")
	}
}

func buildCLIStartCommand(cliType, workDir string) (string, error) {
	switch normalizeResumeCLIType(cliType) {
	case "claude":
		return withWorkDir(workDir, "claude"), nil
	case "codex":
		return withWorkDir(workDir, "codex"), nil
	case "gemini":
		return withWorkDir(workDir, "gemini"), nil
	default:
		return "", errors.New("unsupported cli_type")
	}
}

func withWorkDir(workDir, cmd string) string {
	dir := strings.TrimSpace(workDir)
	if dir == "" {
		return strings.TrimSpace(cmd)
	}
	return fmt.Sprintf("cd -- %s && %s", shellQuote(dir), strings.TrimSpace(cmd))
}

func shellQuote(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "''"
	}
	if text == "~" {
		return "\"$HOME\""
	}
	if strings.HasPrefix(text, "~/") {
		return "\"$HOME/" + strings.TrimPrefix(text, "~/") + "\""
	}
	return "'" + strings.ReplaceAll(text, "'", "'\\''") + "'"
}

func normalizeResumeCLIType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "claude", "claude-code", "claude_code":
		return "claude"
	case "codex", "openai-codex":
		return "codex"
	case "gemini":
		return "gemini"
	case "":
		return ""
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func resolveResumeCLIType(candidates ...string) string {
	for _, item := range candidates {
		normalized := normalizeResumeCLIType(item)
		if normalized == "" {
			continue
		}
		switch normalized {
		case "claude", "codex", "gemini":
			return normalized
		}
	}
	return ""
}

func resumeCLITypeToAIType(cliType string) string {
	switch normalizeResumeCLIType(cliType) {
	case "claude":
		return "claude-code"
	case "codex":
		return "codex"
	case "gemini":
		return "gemini"
	default:
		return ""
	}
}

func lookupLatestAISessionID(taskID, terminalID, aiType string) string {
	if model.DB == nil {
		return ""
	}
	query := model.DB.Model(&model.AISession{})
	if strings.TrimSpace(terminalID) != "" {
		query = query.Where("terminal_id = ?", strings.TrimSpace(terminalID))
	}
	if strings.TrimSpace(taskID) != "" {
		query = query.Where("task_id = ? OR task_id IS NULL", strings.TrimSpace(taskID))
	}

	normalizedAIType := strings.TrimSpace(aiType)
	if normalizedAIType != "" {
		candidates := []string{normalizedAIType}
		if normalizedAIType == "claude-code" {
			candidates = append(candidates, "claude")
		}
		query = query.Where("ai_type IN ?", candidates)
	}

	var row model.AISession
	if err := query.Order("updated_at desc").First(&row).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(row.SessionID)
}

func detectSessionIDFromTerminalLogs(terminalID, taskID, cliType string) string {
	tid := strings.TrimSpace(terminalID)
	if tid == "" || model.DB == nil {
		return ""
	}
	query := model.DB.Model(&model.Log{}).
		Where("terminal_id = ?", tid).
		Where("log_type IN ?", []string{"output_raw", "output", "system"}).
		Order("created_at desc").
		Limit(300)
	if strings.TrimSpace(taskID) != "" {
		query = query.Where("task_id = ? OR task_id IS NULL", strings.TrimSpace(taskID))
	}

	var logs []model.Log
	if err := query.Find(&logs).Error; err != nil {
		return ""
	}
	if len(logs) == 0 {
		return ""
	}

	var builder strings.Builder
	for i := len(logs) - 1; i >= 0; i-- {
		builder.WriteString(stripANSIForResume(logs[i].Content))
		builder.WriteString("\n")
	}
	text := builder.String()

	if match := cliResumeCodexFileRegex.FindStringSubmatch(text); len(match) >= 3 {
		return strings.TrimSpace(match[2])
	}
	if normalizeResumeCLIType(cliType) == "claude" || strings.Contains(strings.ToLower(text), "claude") {
		if id := cliResumeUUIDRegex.FindString(text); id != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

func stripANSIForResume(raw string) string {
	return cliResumeANSIRegex.ReplaceAllString(raw, "")
}

func parseJSONMap(raw string) map[string]any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		return map[string]any{}
	}
	return m
}

func getStringFromAny(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func loadTaskByID(taskID string) (*model.Task, bool) {
	id := strings.TrimSpace(taskID)
	if id == "" || model.DB == nil {
		return nil, false
	}
	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return nil, false
	}
	return &task, true
}

func getTaskServerID(task *model.Task) string {
	if task == nil || task.ServerID == nil {
		return ""
	}
	return strings.TrimSpace(*task.ServerID)
}

func getTaskWorkDir(task *model.Task) string {
	if task == nil {
		return ""
	}
	return strings.TrimSpace(task.WorkDir)
}

func getTaskCLIType(task *model.Task) string {
	if task == nil {
		return ""
	}
	return strings.TrimSpace(task.CLIType)
}

func resolveTerminalServerID(terminalID string) string {
	tid := strings.TrimSpace(terminalID)
	if tid == "" || model.DB == nil {
		return ""
	}
	var row model.TerminalSession
	if err := model.DB.Select("server_id").First(&row, "id = ?", tid).Error; err != nil {
		return ""
	}
	if row.ServerID == nil {
		return ""
	}
	return strings.TrimSpace(*row.ServerID)
}

func ptrOrNil(raw string) *string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	value := text
	return &value
}

func trimPtr(raw *string) string {
	if raw == nil {
		return ""
	}
	return strings.TrimSpace(*raw)
}

func trimStringPtr(raw *string) *string {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil
	}
	value := text
	return &value
}

func firstNonEmptyTrim(candidates ...string) string {
	for _, item := range candidates {
		text := strings.TrimSpace(item)
		if text != "" {
			return text
		}
	}
	return ""
}

func shortID(id string, n int) string {
	text := strings.TrimSpace(id)
	if text == "" {
		return ""
	}
	if n <= 0 || len(text) <= n {
		return text
	}
	return text[:n]
}
