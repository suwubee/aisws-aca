package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/ai-coding-assistant/model"
	clisvc "github.com/ai-coding-assistant/service/cli"
	terminalsvc "github.com/ai-coding-assistant/service/terminal"
	"github.com/google/uuid"
)

// ToolExecutor executes workflow tools
type ToolExecutor struct {
	sshManager sshCommandExecutor
	automation automationService
	terminal   terminalManager
}

func isTerminalExecutionMode(sessionCtx map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(getStringFromMap(sessionCtx, "command_execution_mode")), "terminal")
}

func isOneShotCLICommand(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false
	}
	compact := " " + cmd + " "

	isClaude := strings.HasPrefix(cmd, "claude ") || cmd == "claude"
	isCodex := strings.HasPrefix(cmd, "codex ") || cmd == "codex"
	isGemini := strings.HasPrefix(cmd, "gemini ") || cmd == "gemini"

	if isClaude {
		return strings.Contains(compact, " -p ") || strings.Contains(compact, " --print ") || strings.Contains(compact, " --prompt ")
	}
	if isCodex {
		return strings.Contains(compact, " exec ") || strings.Contains(compact, " --prompt ")
	}
	if isGemini {
		return strings.Contains(compact, " -p ") || strings.Contains(compact, " --prompt ")
	}
	return false
}

func looksLikeShellCommand(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false
	}
	if strings.ContainsAny(cmd, "`<>") || strings.Contains(cmd, "$(") {
		return true
	}
	for _, token := range []string{"&&", "||", ";", " | ", " 2>", " 1>"} {
		if strings.Contains(cmd, token) {
			return true
		}
	}

	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	if strings.HasPrefix(first, "./") || strings.HasPrefix(first, "/") || strings.HasPrefix(first, "~/") {
		return true
	}

	switch first {
	case "cd", "ls", "pwd", "cat", "echo", "grep", "find", "sed", "awk", "head", "tail", "sort", "uniq",
		"mkdir", "rm", "cp", "mv", "touch", "chmod", "chown", "tar", "unzip", "zip",
		"git", "go", "python", "python3", "node", "npm", "pnpm", "yarn", "make", "bash", "sh", "zsh",
		"claude", "codex", "gemini":
		return true
	}
	return false
}

func containsCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func shouldSendAsAIPrompt(command string) bool {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false
	}
	if strings.HasPrefix(cmd, "/") || strings.HasPrefix(cmd, "?") {
		return true
	}
	if containsCJK(cmd) {
		return true
	}
	if strings.ContainsAny(cmd, "。！？，、") {
		return true
	}
	if looksLikeShellCommand(cmd) {
		return false
	}
	fields := strings.Fields(cmd)
	return len(fields) >= 4
}

func isInteractiveAIAssistantSession(session terminalSession) bool {
	if session == nil {
		return false
	}
	metaProvider, ok := any(session).(interface {
		Metadata() *terminalsvc.SessionMetadata
	})
	if !ok {
		return false
	}
	meta := metaProvider.Metadata()
	if meta == nil || meta.AIAssistant == nil || !meta.AIAssistant.Detected {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(meta.AIAssistant.State))
	if state == "" || state == "unknown" {
		return false
	}
	switch state {
	case "waiting_input", "working", "waiting_approval":
		return true
	default:
		return false
	}
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(ssh sshCommandExecutor, auto automationService, term terminalManager) *ToolExecutor {
	return &ToolExecutor{
		sshManager: ssh,
		automation: auto,
		terminal:   term,
	}
}

// Execute executes a tool with given arguments
func (e *ToolExecutor) Execute(ctx context.Context, tool string, args map[string]any, sessionCtx map[string]any) *ToolResult {
	switch tool {
	case "list_servers":
		return e.listServers()
	case "select_server":
		return e.selectServer(args, sessionCtx)
	case "create_task":
		if isTerminalExecutionMode(sessionCtx) {
			return &ToolResult{
				Success: false,
				Error:   "create_task is disabled in terminal mode; reuse current terminal and call execute_command",
			}
		}
		return e.createTask(args, sessionCtx)
	case "start_task":
		if isTerminalExecutionMode(sessionCtx) {
			return &ToolResult{
				Success: false,
				Error:   "start_task is disabled in terminal mode; reuse current terminal and call execute_command",
			}
		}
		return e.startTask(args)
	case "execute_command":
		return e.executeCommand(args, sessionCtx)
	case "batch_execute_command":
		return e.batchExecuteCommand(args, sessionCtx)
	case "git_operation":
		return e.gitOperation(args, sessionCtx)
	case "check_task_status":
		return e.checkTaskStatus(args)
	case "get_terminal_logs":
		return e.getTerminalLogs(args)
	case "wait":
		return e.wait(args)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", tool)}
	}
}

// listServers lists all available servers
func (e *ToolExecutor) listServers() *ToolResult {
	var servers []model.SSHServer
	if err := model.DB.Find(&servers).Error; err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	if len(servers) == 0 {
		return &ToolResult{Success: true, Output: "没有可用的服务器。请先在服务器页面添加服务器（本地也需要添加为一条服务器记录）。"}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个服务器：\n", len(servers)))
	for _, s := range servers {
		sb.WriteString(fmt.Sprintf("- ID: %s, 名称: %s, 主机: %s\n", s.ID, s.Name, s.Host))
	}
	return &ToolResult{Success: true, Output: sb.String()}
}

// selectServer selects a server for subsequent operations
func (e *ToolExecutor) selectServer(args map[string]any, sessionCtx map[string]any) *ToolResult {
	serverID, _ := args["server_id"].(string)
	if serverID == "" {
		return &ToolResult{Success: false, Error: "server_id is required"}
	}

	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", serverID).Error; err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("server not found: %s", serverID)}
	}

	if isTerminalExecutionMode(sessionCtx) {
		lockedServerID := strings.TrimSpace(getStringFromMap(sessionCtx, "current_server_id"))
		if lockedServerID != "" && !strings.EqualFold(lockedServerID, serverID) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("terminal mode locks server to %s, cannot switch to %s", lockedServerID, serverID),
			}
		}
	}

	sessionCtx["current_server_id"] = serverID
	sessionCtx["current_server_name"] = server.Name

	if isTerminalExecutionMode(sessionCtx) && e.terminal != nil {
		_ = e.ensureTerminalForServer(sessionCtx, serverID)
	}
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("已选择服务器: %s (%s)", server.Name, server.Host),
	}
}

// createTask creates a new task
func (e *ToolExecutor) createTask(args map[string]any, sessionCtx map[string]any) *ToolResult {
	title, _ := args["title"].(string)
	if title == "" {
		return &ToolResult{Success: false, Error: "title is required"}
	}

	desc, _ := args["description"].(string)
	workDir, _ := args["work_dir"].(string)
	cliType, _ := args["cli_type"].(string)
	prompt, _ := args["initial_prompt"].(string)

	cliType = strings.ToLower(strings.TrimSpace(cliType))
	if cliType == "" {
		cliType = "claude"
	}
	if cliType != "claude" && cliType != "codex" && cliType != "gemini" {
		return &ToolResult{Success: false, Error: fmt.Sprintf("invalid cli_type: %s (allowed: claude, codex, gemini)", cliType)}
	}

	var serverID *string
	if sid, ok := args["server_id"].(string); ok && sid != "" {
		serverID = &sid
	} else if sid, ok := sessionCtx["current_server_id"].(string); ok && sid != "" {
		serverID = &sid
	}

	if serverID == nil || strings.TrimSpace(*serverID) == "" {
		return &ToolResult{Success: false, Error: "server_id is required (local must be configured in Servers)"}
	}

	trimmedServerID := strings.TrimSpace(*serverID)
	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", trimmedServerID).Error; err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("server not found: %s", trimmedServerID)}
	}
	serverID = &trimmedServerID

	task := &model.Task{
		ID:            uuid.New().String(),
		Title:         title,
		Description:   desc,
		Status:        "todo",
		ServerID:      serverID,
		WorkDir:       workDir,
		CLIType:       cliType,
		InitialPrompt: prompt,
		AutoStart:     false,
		AutoCreateDir: true,
	}

	if err := model.DB.Create(task).Error; err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	sessionCtx["last_task_id"] = task.ID
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("任务创建成功\nID: %s\n标题: %s", task.ID, task.Title),
		Data:    map[string]string{"task_id": task.ID},
	}
}

// startTask starts an existing task
func (e *ToolExecutor) startTask(args map[string]any) *ToolResult {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return &ToolResult{Success: false, Error: "task_id is required"}
	}

	var task model.Task
	if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}

	if e.automation == nil {
		return &ToolResult{Success: false, Error: "automation service not configured"}
	}

	result, err := e.automation.StartTask(&task)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	terminalID := ""
	if result != nil && result.Terminal != nil {
		terminalID = result.Terminal.ID()
	}

	if result != nil && result.NeedsUserAction {
		output := fmt.Sprintf("任务已创建并暂停等待用户确认\nID: %s\n终端ID: %s\n提示: %s", task.ID, terminalID, strings.TrimSpace(result.UserActionHint))
		return &ToolResult{
			Success: true,
			Output:  output,
			Data:    map[string]string{"task_id": task.ID, "terminal_id": terminalID},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("任务已启动\nID: %s\n终端ID: %s", task.ID, terminalID),
		Data:    map[string]string{"task_id": task.ID, "terminal_id": terminalID},
	}
}

// executeCommand executes a command
func (e *ToolExecutor) executeCommand(args map[string]any, sessionCtx map[string]any) *ToolResult {
	command, _ := args["command"].(string)
	if command == "" {
		return &ToolResult{Success: false, Error: "command is required"}
	}

	workDir, _ := args["work_dir"].(string)
	if strings.TrimSpace(workDir) == "" {
		workDir = strings.TrimSpace(getStringFromMap(sessionCtx, "work_dir"))
	}
	serverID, _ := args["server_id"].(string)
	if serverID == "" {
		if sid, ok := sessionCtx["current_server_id"].(string); ok {
			serverID = sid
		}
	}

	if strings.TrimSpace(serverID) == "" {
		return &ToolResult{Success: false, Error: "server_id is required (local must be configured in Servers)"}
	}

	if isTerminalExecutionMode(sessionCtx) {
		lockedServerID := strings.TrimSpace(getStringFromMap(sessionCtx, "current_server_id"))
		if lockedServerID != "" && !strings.EqualFold(strings.TrimSpace(serverID), lockedServerID) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("terminal mode locks server to %s, got %s", lockedServerID, strings.TrimSpace(serverID)),
			}
		}
	}

	truncateForEvent := func(raw string) string {
		const maxRunes = 4000
		text := strings.TrimSpace(raw)
		if text == "" {
			return ""
		}
		runes := []rune(text)
		if len(runes) <= maxRunes {
			return text
		}
		return string(runes[:maxRunes]) + "\n...(truncated)"
	}

	startTrackedExecution := func(terminalID string) string {
		tracker := clisvc.NewExecutionTracker(model.DB)
		if tracker == nil {
			return ""
		}
		toStringPtr := func(raw string) *string {
			s := strings.TrimSpace(raw)
			if s == "" {
				return nil
			}
			return &s
		}

		var taskIDPtr *string
		taskID := strings.TrimSpace(getStringFromMap(sessionCtx, "task_id"))
		if taskID != "" {
			taskIDCopy := taskID
			taskIDPtr = &taskIDCopy
		}

		var terminalIDPtr *string
		terminalID = strings.TrimSpace(terminalID)
		if terminalID != "" {
			terminalIDCopy := terminalID
			terminalIDPtr = &terminalIDCopy
		}

		tool := strings.TrimSpace(getStringFromMap(sessionCtx, "cli_type"))
		if tool == "" {
			tool = "shell"
		}
		mode := strings.TrimSpace(getStringFromMap(sessionCtx, "workflow_phase"))
		if mode == "" {
			mode = "command"
		}
		source := strings.TrimSpace(getStringFromMap(sessionCtx, "command_execution_mode"))
		if source == "" {
			source = "workflow"
		}
		workflowRunIDPtr := toStringPtr(getStringFromMap(sessionCtx, "workflow_run_id"))
		workflowSessionIDPtr := toStringPtr(getStringFromMap(sessionCtx, "workflow_session_id"))
		workflowStepID := strings.TrimSpace(getStringFromMap(sessionCtx, "workflow_step_id"))
		workflowIteration := strings.TrimSpace(getStringFromMap(sessionCtx, "workflow_iteration"))

		metadata := map[string]any{
			"server_id": serverID,
			"work_dir":  workDir,
			"command":   command,
		}
		if workflowStepID != "" {
			metadata["workflow_step_id"] = workflowStepID
		}
		if workflowIteration != "" {
			metadata["workflow_iteration"] = workflowIteration
		}
		if phase := strings.TrimSpace(mode); phase != "" {
			metadata["workflow_phase"] = phase
		}

		record, err := tracker.Start(clisvc.StartExecutionInput{
			TaskID:            taskIDPtr,
			TerminalID:        terminalIDPtr,
			WorkflowRunID:     workflowRunIDPtr,
			WorkflowSessionID: workflowSessionIDPtr,
			Tool:              tool,
			Mode:              mode,
			Source:            source,
			Prompt:            command,
			Metadata:          metadata,
		})
		if err != nil || record == nil {
			return ""
		}

		_ = tracker.AppendEvent(record.ID, clisvc.EventTypeStarted, map[string]any{
			"server_id":           serverID,
			"work_dir":            workDir,
			"command":             command,
			"workflow_step_id":    workflowStepID,
			"workflow_iteration":  workflowIteration,
			"workflow_session_id": getStringFromMap(sessionCtx, "workflow_session_id"),
			"workflow_run_id":     getStringFromMap(sessionCtx, "workflow_run_id"),
			"workflow_phase":      mode,
		})
		return record.ID
	}

	appendTrackedEvent := func(executionID, eventType string, payload map[string]any) {
		if executionID == "" {
			return
		}
		tracker := clisvc.NewExecutionTracker(model.DB)
		if tracker == nil {
			return
		}
		_ = tracker.AppendEvent(executionID, eventType, payload)
	}

	completeTrackedExecution := func(executionID, status string, exitCode *int, errMsg string) {
		if executionID == "" {
			return
		}
		tracker := clisvc.NewExecutionTracker(model.DB)
		if tracker == nil {
			return
		}
		_ = tracker.Complete(executionID, status, exitCode, errMsg)
	}

	resolveReviewCommand := func() string {
		reviewCmd := strings.TrimSpace(getStringFromMap(args, "review_command"))
		if reviewCmd != "" {
			return reviewCmd
		}
		reviewCmd = strings.TrimSpace(getStringFromMap(sessionCtx, "review_command"))
		if reviewCmd != "" {
			return reviewCmd
		}

		template := strings.TrimSpace(getStringFromMap(args, "review_command_template"))
		if template == "" {
			template = strings.TrimSpace(getStringFromMap(sessionCtx, "review_command_template"))
		}
		if template == "" {
			return ""
		}

		rendered := strings.ReplaceAll(template, "{{command}}", command)
		rendered = strings.ReplaceAll(rendered, "{{server_id}}", strings.TrimSpace(serverID))
		rendered = strings.ReplaceAll(rendered, "{{work_dir}}", strings.TrimSpace(workDir))
		return strings.TrimSpace(rendered)
	}

	reviewCommand := resolveReviewCommand()
	runReview := getBool(args, false, "run_review", "enable_review", "with_review")
	if !runReview {
		runReview = getBool(sessionCtx, false, "run_review", "enable_review", "with_review")
	}
	if strings.TrimSpace(reviewCommand) != "" {
		runReview = true
	}

	reviewWorkDir := strings.TrimSpace(getStringFromMap(args, "review_work_dir"))
	if reviewWorkDir == "" {
		reviewWorkDir = strings.TrimSpace(getStringFromMap(sessionCtx, "review_work_dir"))
	}
	if reviewWorkDir == "" {
		reviewWorkDir = workDir
	}

	reviewTool := strings.TrimSpace(getStringFromMap(args, "review_cli_type"))
	if reviewTool == "" {
		reviewTool = strings.TrimSpace(getStringFromMap(sessionCtx, "review_cli_type"))
	}
	if reviewTool == "" {
		reviewTool = strings.TrimSpace(getStringFromMap(sessionCtx, "cli_type"))
	}
	if reviewTool == "" {
		reviewTool = "shell"
	}

	launchReviewWorker := func(parentExecutionID, primaryStatus, primaryOutput, primaryErr string) string {
		if !runReview || strings.TrimSpace(reviewCommand) == "" {
			return ""
		}

		tracker := clisvc.NewExecutionTracker(model.DB)
		if tracker == nil {
			return ""
		}

		toStringPtr := func(raw string) *string {
			s := strings.TrimSpace(raw)
			if s == "" {
				return nil
			}
			return &s
		}

		taskIDPtr := toStringPtr(getStringFromMap(sessionCtx, "task_id"))
		terminalIDPtr := toStringPtr(getStringFromMap(sessionCtx, "terminal_id"))
		workflowRunIDPtr := toStringPtr(getStringFromMap(sessionCtx, "workflow_run_id"))
		workflowSessionIDPtr := toStringPtr(getStringFromMap(sessionCtx, "workflow_session_id"))
		parentExecutionIDPtr := toStringPtr(parentExecutionID)

		reviewMetadata := map[string]any{
			"server_id":        serverID,
			"work_dir":         reviewWorkDir,
			"review_command":   reviewCommand,
			"parent_execution": strings.TrimSpace(parentExecutionID),
			"primary_status":   strings.TrimSpace(primaryStatus),
			"primary_output":   truncateForEvent(primaryOutput),
			"primary_error":    strings.TrimSpace(primaryErr),
		}

		record, err := tracker.Start(clisvc.StartExecutionInput{
			TaskID:            taskIDPtr,
			TerminalID:        terminalIDPtr,
			WorkflowRunID:     workflowRunIDPtr,
			WorkflowSessionID: workflowSessionIDPtr,
			ParentExecutionID: parentExecutionIDPtr,
			Role:              clisvc.ExecutionRoleReview,
			Tool:              reviewTool,
			Mode:              "review",
			Source:            "workflow-review",
			Prompt:            reviewCommand,
			Metadata:          reviewMetadata,
		})
		if err != nil || record == nil {
			return ""
		}

		reviewExecutionID := record.ID
		_ = tracker.AppendEvent(reviewExecutionID, clisvc.EventTypeReview, map[string]any{
			"stage":            "queued",
			"server_id":        serverID,
			"work_dir":         reviewWorkDir,
			"review_command":   reviewCommand,
			"parent_execution": strings.TrimSpace(parentExecutionID),
			"primary_status":   strings.TrimSpace(primaryStatus),
		})

		terminalID := strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id"))
		taskID := strings.TrimSpace(getStringFromMap(sessionCtx, "task_id"))
		logCtx := map[string]any{}
		if terminalID != "" {
			logCtx["terminal_id"] = terminalID
		}
		if taskID != "" {
			logCtx["task_id"] = taskID
		}

		go func(executionID, reviewCmd, reviewDir, reviewServerID string, localLogCtx map[string]any) {
			reviewTracker := clisvc.NewExecutionTracker(model.DB)
			appendReviewEvent := func(eventType string, payload map[string]any) {
				if reviewTracker == nil {
					return
				}
				_ = reviewTracker.AppendEvent(executionID, eventType, payload)
			}
			completeReview := func(status, errMsg string) {
				if reviewTracker == nil {
					return
				}
				_ = reviewTracker.Complete(executionID, status, nil, errMsg)
			}

			appendReviewEvent(clisvc.EventTypeReview, map[string]any{
				"stage":          "started",
				"server_id":      reviewServerID,
				"work_dir":       reviewDir,
				"review_command": reviewCmd,
			})

			if e.sshManager == nil {
				errMsg := "ssh manager is not configured for review worker"
				appendReviewEvent(clisvc.EventTypeError, map[string]any{"error": errMsg})
				appendReviewEvent(clisvc.EventTypeReview, map[string]any{"stage": "finished", "success": false, "error": errMsg})
				completeReview(clisvc.StatusError, errMsg)
				e.emitTerminalAILog(localLogCtx, "warning", fmt.Sprintf("[%s][review] %s", strings.TrimSpace(reviewServerID), errMsg), "", "")
				return
			}

			fullReviewCmd := strings.TrimSpace(reviewCmd)
			if strings.TrimSpace(reviewDir) != "" {
				fullReviewCmd = fmt.Sprintf("cd %s && %s", strings.TrimSpace(reviewDir), strings.TrimSpace(reviewCmd))
			}

			out, runErr := e.sshManager.ExecuteCommand(reviewServerID, fullReviewCmd)
			if runErr != nil {
				appendReviewEvent(clisvc.EventTypeError, map[string]any{
					"error":  runErr.Error(),
					"output": truncateForEvent(out),
				})
				appendReviewEvent(clisvc.EventTypeReview, map[string]any{
					"stage":   "finished",
					"success": false,
					"error":   runErr.Error(),
				})
				completeReview(clisvc.StatusError, runErr.Error())
				e.emitTerminalAILog(localLogCtx, "warning", fmt.Sprintf("[%s][review] 审核失败: %v", strings.TrimSpace(reviewServerID), runErr), "command", fullReviewCmd)
				return
			}

			trimmedOut := strings.TrimSpace(out)
			if trimmedOut != "" {
				appendReviewEvent(clisvc.EventTypeOutput, map[string]any{
					"output": truncateForEvent(trimmedOut),
				})
			}
			appendReviewEvent(clisvc.EventTypeCompleted, map[string]any{})
			appendReviewEvent(clisvc.EventTypeReview, map[string]any{
				"stage":   "finished",
				"success": true,
			})
			completeReview(clisvc.StatusCompleted, "")

			if trimmedOut == "" {
				e.emitTerminalAILog(localLogCtx, "info", fmt.Sprintf("[%s][review] 审核完成（无输出）", strings.TrimSpace(reviewServerID)), "command", fullReviewCmd)
			} else {
				e.emitTerminalAILog(localLogCtx, "info", fmt.Sprintf("[%s][review]\n%s", strings.TrimSpace(reviewServerID), strings.TrimRight(out, "\n")), "command", fullReviewCmd)
			}
		}(reviewExecutionID, reviewCommand, reviewWorkDir, serverID, logCtx)

		return reviewExecutionID
	}

	execMode := strings.ToLower(strings.TrimSpace(getStringFromMap(sessionCtx, "command_execution_mode")))
	if execMode == "" {
		execMode = "backend"
	}
	if execMode == "terminal" && isOneShotCLICommand(command) {
		return &ToolResult{
			Success: false,
			Error:   "one-shot CLI command is disabled in terminal mode; start/reuse interactive CLI and send prompt text instead",
		}
	}

	// 优先使用“终端执行模式”：让 AI 真正在工作台可见的终端里敲命令，便于实时观察/接管。
	if execMode == "terminal" && e.terminal != nil {
		terminalID := e.ensureTerminalForServer(sessionCtx, serverID)
		if strings.TrimSpace(terminalID) != "" {
			executionID := startTrackedExecution(terminalID)
			if executionID != "" {
				sessionCtx["execution_id"] = executionID
			}

			displayCmd := strings.TrimSpace(command)
			if workDir != "" {
				displayCmd = fmt.Sprintf("cd %s && %s", strings.TrimSpace(workDir), strings.TrimSpace(command))
			}
			e.emitTerminalAILog(sessionCtx, "action", fmt.Sprintf("[%s] $ %s", strings.TrimSpace(serverID), displayCmd), "command", strings.TrimSpace(displayCmd))

			session, err := e.terminal.GetOrResumeSession(terminalID)
			if err == nil && session != nil {
				if isInteractiveAIAssistantSession(session) {
					prompt := strings.TrimSpace(command)
					if shouldSendAsAIPrompt(prompt) {
						if err := session.Write([]byte(strings.TrimRight(prompt, "\r\n") + "\r")); err != nil {
							e.emitTerminalAILog(sessionCtx, "error", fmt.Sprintf("[%s] 发送到CLI失败: %v", strings.TrimSpace(serverID), err), "", "")
							appendTrackedEvent(executionID, clisvc.EventTypeError, map[string]any{
								"error":   err.Error(),
								"command": prompt,
								"mode":    "prompt",
							})
							completeTrackedExecution(executionID, clisvc.StatusError, nil, err.Error())
							return &ToolResult{Success: false, Error: err.Error()}
						}

						appendTrackedEvent(executionID, clisvc.EventTypeProgress, map[string]any{
							"stage":   "prompt_sent",
							"mode":    "prompt",
							"command": truncateForEvent(prompt),
						})
						appendTrackedEvent(executionID, clisvc.EventTypeCompleted, map[string]any{
							"mode": "prompt",
						})
						completeTrackedExecution(executionID, clisvc.StatusCompleted, nil, "")
						e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] 已将请求发送到当前CLI会话", strings.TrimSpace(serverID)), "prompt", prompt)
						return &ToolResult{Success: true, Output: "prompt sent to active CLI session"}
					}

					if e.sshManager == nil {
						appendTrackedEvent(executionID, clisvc.EventTypeError, map[string]any{
							"error": "ssh manager is not configured",
							"mode":  "backend_fallback",
						})
						completeTrackedExecution(executionID, clisvc.StatusError, nil, "ssh manager is not configured")
						return &ToolResult{Success: false, Error: "ssh manager is not configured"}
					}

					fullCmd := command
					if workDir != "" {
						fullCmd = fmt.Sprintf("cd %s && %s", workDir, command)
					}

					e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] 检测到CLI交互会话，命令改为后台执行以避免污染对话", strings.TrimSpace(serverID)), "", "")
					output, runErr := e.sshManager.ExecuteCommand(serverID, fullCmd)
					if runErr != nil {
						msg := fmt.Sprintf("[%s] 命令执行失败: %v", strings.TrimSpace(serverID), runErr)
						if strings.TrimSpace(output) != "" {
							msg = msg + "\n" + strings.TrimRight(output, "\n")
						}
						e.emitTerminalAILog(sessionCtx, "error", msg, "", "")
						appendTrackedEvent(executionID, clisvc.EventTypeError, map[string]any{
							"error":  runErr.Error(),
							"output": truncateForEvent(output),
							"mode":   "backend_fallback",
						})
						completeTrackedExecution(executionID, clisvc.StatusError, nil, runErr.Error())
						reviewExecutionID := launchReviewWorker(executionID, clisvc.StatusError, output, runErr.Error())
						if reviewExecutionID != "" {
							sessionCtx["review_execution_id"] = reviewExecutionID
						}
						return &ToolResult{Success: false, Error: runErr.Error(), Output: output}
					}

					out := strings.TrimRight(output, "\n")
					if strings.TrimSpace(out) != "" {
						appendTrackedEvent(executionID, clisvc.EventTypeOutput, map[string]any{
							"output": truncateForEvent(out),
							"mode":   "backend_fallback",
						})
					}
					if strings.TrimSpace(out) == "" {
						e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] （无输出）", strings.TrimSpace(serverID)), "", "")
					} else {
						e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s]\n%s", strings.TrimSpace(serverID), out), "", "")
					}
					appendTrackedEvent(executionID, clisvc.EventTypeCompleted, map[string]any{
						"mode": "backend_fallback",
					})
					completeTrackedExecution(executionID, clisvc.StatusCompleted, nil, "")
					reviewExecutionID := launchReviewWorker(executionID, clisvc.StatusCompleted, output, "")
					if reviewExecutionID != "" {
						sessionCtx["review_execution_id"] = reviewExecutionID
					}
					return &ToolResult{Success: true, Output: output}
				}

				output, exitCode, runErr := session.RunCommand(command, workDir, 0)
				if runErr != nil {
					msg := fmt.Sprintf("[%s] 命令执行失败: %v", strings.TrimSpace(serverID), runErr)
					if strings.TrimSpace(output) != "" {
						msg = msg + "\n" + strings.TrimRight(output, "\n")
					} else if exitCode >= 0 {
						msg = msg + fmt.Sprintf(" (exit=%d)", exitCode)
					}
					e.emitTerminalAILog(sessionCtx, "error", msg, "", "")
					appendTrackedEvent(executionID, clisvc.EventTypeError, map[string]any{
						"error":    runErr.Error(),
						"exitCode": exitCode,
						"output":   truncateForEvent(output),
					})
					var codePtr *int
					if exitCode >= 0 {
						c := exitCode
						codePtr = &c
					}
					completeTrackedExecution(executionID, clisvc.StatusError, codePtr, runErr.Error())
					reviewExecutionID := launchReviewWorker(executionID, clisvc.StatusError, output, runErr.Error())
					if reviewExecutionID != "" {
						sessionCtx["review_execution_id"] = reviewExecutionID
					}
					return &ToolResult{Success: false, Error: runErr.Error(), Output: output}
				}

				out := strings.TrimRight(output, "\n")
				if strings.TrimSpace(out) != "" {
					appendTrackedEvent(executionID, clisvc.EventTypeOutput, map[string]any{
						"output": truncateForEvent(out),
					})
				}
				if strings.TrimSpace(out) == "" {
					e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] （无输出）", strings.TrimSpace(serverID)), "", "")
				} else {
					e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s]\n%s", strings.TrimSpace(serverID), out), "", "")
				}
				var codePtr *int
				if exitCode >= 0 {
					c := exitCode
					codePtr = &c
				}
				appendTrackedEvent(executionID, clisvc.EventTypeCompleted, map[string]any{"exitCode": exitCode})
				completeTrackedExecution(executionID, clisvc.StatusCompleted, codePtr, "")
				reviewExecutionID := launchReviewWorker(executionID, clisvc.StatusCompleted, output, "")
				if reviewExecutionID != "" {
					sessionCtx["review_execution_id"] = reviewExecutionID
				}

				return &ToolResult{Success: true, Output: output}
			}
		}
		// 终端不可用时降级到后端执行
	}

	if e.sshManager == nil {
		return &ToolResult{Success: false, Error: "ssh manager is not configured"}
	}

	// Build full command with work_dir
	fullCmd := command
	if workDir != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", workDir, command)
	}

	displayCmd := strings.TrimSpace(command)
	if workDir != "" {
		displayCmd = fmt.Sprintf("cd %s && %s", strings.TrimSpace(workDir), strings.TrimSpace(command))
	}
	e.emitTerminalAILog(sessionCtx, "action", fmt.Sprintf("[%s] $ %s", strings.TrimSpace(serverID), displayCmd), "command", fullCmd)

	executionID := startTrackedExecution(strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id")))
	if executionID != "" {
		sessionCtx["execution_id"] = executionID
	}

	// Execute on server or locally
	output, err := e.sshManager.ExecuteCommand(serverID, fullCmd)
	if err != nil {
		msg := fmt.Sprintf("[%s] 命令执行失败: %v", strings.TrimSpace(serverID), err)
		if strings.TrimSpace(output) != "" {
			msg = msg + "\n" + strings.TrimRight(output, "\n")
		}
		e.emitTerminalAILog(sessionCtx, "error", msg, "", "")
		appendTrackedEvent(executionID, clisvc.EventTypeError, map[string]any{
			"error":  err.Error(),
			"output": truncateForEvent(output),
		})
		completeTrackedExecution(executionID, clisvc.StatusError, nil, err.Error())
		reviewExecutionID := launchReviewWorker(executionID, clisvc.StatusError, output, err.Error())
		if reviewExecutionID != "" {
			sessionCtx["review_execution_id"] = reviewExecutionID
		}
		return &ToolResult{Success: false, Error: err.Error(), Output: output}
	}
	out := strings.TrimRight(output, "\n")
	if strings.TrimSpace(out) != "" {
		appendTrackedEvent(executionID, clisvc.EventTypeOutput, map[string]any{
			"output": truncateForEvent(out),
		})
	}
	if strings.TrimSpace(out) == "" {
		e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] （无输出）", strings.TrimSpace(serverID)), "", "")
	} else {
		e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s]\n%s", strings.TrimSpace(serverID), out), "", "")
	}
	appendTrackedEvent(executionID, clisvc.EventTypeCompleted, map[string]any{})
	completeTrackedExecution(executionID, clisvc.StatusCompleted, nil, "")
	reviewExecutionID := launchReviewWorker(executionID, clisvc.StatusCompleted, output, "")
	if reviewExecutionID != "" {
		sessionCtx["review_execution_id"] = reviewExecutionID
	}
	return &ToolResult{Success: true, Output: output}
}

func (e *ToolExecutor) batchExecuteCommand(args map[string]any, sessionCtx map[string]any) *ToolResult {
	command, _ := args["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return &ToolResult{Success: false, Error: "command is required"}
	}

	workDir, _ := args["work_dir"].(string)
	workDir = strings.TrimSpace(workDir)

	ids := make([]string, 0)
	switch raw := args["server_ids"].(type) {
	case []any:
		for _, item := range raw {
			if s, ok := item.(string); ok {
				id := strings.TrimSpace(s)
				if id != "" {
					ids = append(ids, id)
				}
			}
		}
	case []string:
		for _, s := range raw {
			id := strings.TrimSpace(s)
			if id != "" {
				ids = append(ids, id)
			}
		}
	}

	// 兼容：未显式传 server_ids 时，尝试从上下文读取
	if len(ids) == 0 {
		if ctxIDs, ok := sessionCtx["target_server_ids"].([]string); ok && len(ctxIDs) > 0 {
			for _, sid := range ctxIDs {
				id := strings.TrimSpace(sid)
				if id != "" {
					ids = append(ids, id)
				}
			}
		}
	}

	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, serverID := range ids {
		sid := strings.TrimSpace(serverID)
		if sid == "" {
			continue
		}
		if _, ok := seen[sid]; ok {
			continue
		}
		seen[sid] = struct{}{}
		unique = append(unique, sid)
	}

	if isTerminalExecutionMode(sessionCtx) {
		lockedServerID := strings.TrimSpace(getStringFromMap(sessionCtx, "current_server_id"))
		if len(unique) == 0 && lockedServerID != "" {
			unique = append(unique, lockedServerID)
		}
		if len(unique) > 1 {
			return &ToolResult{
				Success: false,
				Error:   "batch_execute_command is disabled in terminal mode; use execute_command on the current terminal",
			}
		}
		if len(unique) == 1 && lockedServerID != "" && !strings.EqualFold(unique[0], lockedServerID) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("terminal mode locks server to %s, got %s", lockedServerID, unique[0]),
			}
		}
	}

	// Build full command with work_dir
	fullCmd := command
	if workDir != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", workDir, command)
	}

	if len(unique) == 0 {
		return &ToolResult{Success: false, Error: "server_ids is required (local must be configured in Servers)"}
	}
	if e.sshManager == nil {
		return &ToolResult{Success: false, Error: "ssh manager is not configured"}
	}

	e.emitTerminalAILog(sessionCtx, "action", fmt.Sprintf("批量执行 %d 台服务器: %s", len(unique), strings.TrimSpace(command)), "command", fullCmd)

	type resultItem struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}

	results := make(map[string]resultItem, len(unique))
	var sb strings.Builder
	success := true

	type pair struct {
		id  string
		res resultItem
	}

	ch := make(chan pair, len(unique))
	for _, sid := range unique {
		go func(id string) {
			output, err := e.sshManager.ExecuteCommand(id, fullCmd)
			item := resultItem{Output: output}
			if err != nil {
				item.Error = err.Error()
			}
			ch <- pair{id: id, res: item}
		}(sid)
	}

	for i := 0; i < len(unique); i++ {
		p := <-ch
		results[p.id] = p.res
		if p.res.Error != "" {
			success = false
		}

		entry := fmt.Sprintf("=== %s ===", p.id)
		if p.res.Error != "" {
			entry = entry + "\nERROR: " + p.res.Error
		}
		if strings.TrimSpace(p.res.Output) != "" {
			entry = entry + "\n" + strings.TrimRight(p.res.Output, "\n")
		}
		if p.res.Error != "" {
			e.emitTerminalAILog(sessionCtx, "error", entry, "", "")
		} else if strings.TrimSpace(p.res.Output) == "" {
			e.emitTerminalAILog(sessionCtx, "info", entry+"\n（无输出）", "", "")
		} else {
			e.emitTerminalAILog(sessionCtx, "info", entry, "", "")
		}

		sb.WriteString("=== ")
		sb.WriteString(p.id)
		sb.WriteString(" ===\n")
		if p.res.Error != "" {
			sb.WriteString("ERROR: ")
			sb.WriteString(p.res.Error)
			sb.WriteString("\n")
		}
		sb.WriteString(p.res.Output)
		if !strings.HasSuffix(p.res.Output, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return &ToolResult{
		Success: success,
		Output:  strings.TrimSpace(sb.String()),
		Data:    results,
	}
}

func (e *ToolExecutor) emitTerminalAILog(sessionCtx map[string]any, logType, message, inputType, inputData string) {
	if e == nil || sessionCtx == nil {
		return
	}

	terminalID := strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id"))
	if terminalID == "" {
		return
	}

	taskID := strings.TrimSpace(getStringFromMap(sessionCtx, "task_id"))
	var taskIDPtr *string
	if taskID != "" {
		taskIDCopy := taskID
		taskIDPtr = &taskIDCopy
	}

	text := strings.TrimSpace(message)
	if text == "" {
		return
	}
	if strings.TrimSpace(inputData) != "" {
		label := strings.TrimSpace(inputType)
		if label == "" {
			label = "input"
		}
		text = text + "\n" + label + ": " + strings.TrimSpace(inputData)
	}

	// 1) 写入数据库，便于回溯/导出（system 类型不会影响终端输入/输出分组）
	if model.DB != nil {
		now := time.Now()
		content := fmt.Sprintf("[AI][%s] %s", strings.TrimSpace(logType), text)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		terminalCopy := terminalID
		_ = model.DB.Create(&model.Log{
			ID:         uuid.New().String(),
			TerminalID: &terminalCopy,
			TaskID:     taskIDPtr,
			LogType:    "system",
			Content:    content,
			CreatedAt:  now,
		}).Error
	}

	// 2) 实时广播到终端订阅者（工作台/审批面板的 AI 日志）
	if e.terminal == nil {
		return
	}
	session, err := e.terminal.GetOrResumeSession(terminalID)
	if err != nil || session == nil {
		return
	}
	if strings.TrimSpace(inputData) != "" || strings.TrimSpace(inputType) != "" {
		session.BroadcastAILogWithInput(logType, text, inputType, inputData)
	} else {
		session.BroadcastAILog(logType, text)
	}

	execMode := strings.ToLower(strings.TrimSpace(getStringFromMap(sessionCtx, "command_execution_mode")))
	if execMode == "terminal" {
		return
	}

	// 3) 注入到终端输出流，让工作台终端“看起来”与 AI 执行一致（仅用于后端执行模式）
	display := strings.TrimRight(text, "\n")
	display = strings.ReplaceAll(display, "\n", "\r\n")

	if display != "" {
		label := strings.ToLower(strings.TrimSpace(logType))
		prefix := "[AI]"
		color := "\x1b[90m" // gray
		switch label {
		case "action":
			prefix = "[AI][action]"
			color = "\x1b[32m"
		case "error":
			prefix = "[AI][error]"
			color = "\x1b[31m"
		case "warning":
			prefix = "[AI][warning]"
			color = "\x1b[33m"
		case "decision":
			prefix = "[AI][decision]"
			color = "\x1b[35m"
		case "info":
			prefix = "[AI][info]"
			color = "\x1b[36m"
		}

		const maxRunes = 8000
		runes := []rune(display)
		if len(runes) > maxRunes {
			display = string(runes[:maxRunes]) + "\r\n…(truncated)…"
		}

		out := "\r\n" + color + prefix + "\x1b[0m " + display + "\r\n"
		session.InjectOutput([]byte(out))
	}
}

func (e *ToolExecutor) ensureTerminalForServer(sessionCtx map[string]any, serverID string) string {
	if e == nil || e.terminal == nil || sessionCtx == nil {
		return ""
	}

	sid := strings.TrimSpace(serverID)
	if sid == "" {
		return ""
	}

	terminalByServer := getStringMapFromContext(sessionCtx, "terminal_ids_by_server")
	if terminalByServer == nil {
		terminalByServer = map[string]string{}
	}

	// terminal 模式下强制复用当前终端，避免“AI 介入后新开终端”导致上下文断裂。
	if isTerminalExecutionMode(sessionCtx) {
		if current := strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id")); current != "" {
			if sess, err := e.terminal.GetOrResumeSession(current); err == nil && sess != nil {
				terminalByServer[sid] = current
				sessionCtx["terminal_ids_by_server"] = terminalByServer
				sessionCtx["terminal_id"] = current
				return current
			}
			return ""
		}
	}

	if existing := strings.TrimSpace(terminalByServer[sid]); existing != "" {
		if sess, err := e.terminal.GetOrResumeSession(existing); err == nil && sess != nil {
			sessionCtx["terminal_id"] = existing
			return existing
		}
		delete(terminalByServer, sid)
	}

	// 如果上下文已带 terminal_id（例如 StartTaskAgent 创建并由前端打开的默认终端），
	// 且该终端绑定的 server_id 与当前选择的服务器一致，则优先复用，避免创建第二个终端导致“AI 在后台跑，工作台终端静止”。
	if current := strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id")); current != "" && model.DB != nil {
		var t model.TerminalSession
		if err := model.DB.Select("id", "server_id").First(&t, "id = ?", current).Error; err == nil {
			if t.ServerID != nil && strings.TrimSpace(*t.ServerID) == sid {
				if sess, err := e.terminal.GetOrResumeSession(current); err == nil && sess != nil {
					terminalByServer[sid] = current
					sessionCtx["terminal_ids_by_server"] = terminalByServer
					sessionCtx["terminal_id"] = current
					return current
				}
			}
		}
	}

	session, err := e.terminal.CreateSSHSession(sid)
	if err != nil || session == nil {
		return ""
	}

	terminalID := session.ID()
	taskID := strings.TrimSpace(getStringFromMap(sessionCtx, "task_id"))
	if taskID != "" {
		taskIDCopy := taskID
		_ = e.terminal.LinkTask(terminalID, &taskIDCopy)
	}

	title := "AI托管"
	if label := resolveServerLabelForWorkflow(sid); label != "" {
		title = fmt.Sprintf("AI托管: %s", label)
	}
	_ = e.terminal.RenameSession(terminalID, title)

	terminalByServer[sid] = terminalID
	sessionCtx["terminal_ids_by_server"] = terminalByServer
	sessionCtx["terminal_id"] = terminalID
	return terminalID
}

func getStringMapFromContext(ctx map[string]any, key string) map[string]string {
	if ctx == nil {
		return nil
	}
	raw, ok := ctx[key]
	if !ok || raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case map[string]string:
		out := make(map[string]string, len(v))
		for k, s := range v {
			if strings.TrimSpace(k) == "" || strings.TrimSpace(s) == "" {
				continue
			}
			out[strings.TrimSpace(k)] = strings.TrimSpace(s)
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(v))
		for k, value := range v {
			ks := strings.TrimSpace(k)
			if ks == "" {
				continue
			}
			if s, ok := value.(string); ok {
				vs := strings.TrimSpace(s)
				if vs != "" {
					out[ks] = vs
				}
			}
		}
		return out
	default:
		return nil
	}
}

func resolveServerLabelForWorkflow(serverID string) string {
	id := strings.TrimSpace(serverID)
	if id == "" || model.DB == nil {
		return ""
	}

	var server model.SSHServer
	if err := model.DB.Select("id", "name", "host").First(&server, "id = ?", id).Error; err != nil {
		return id
	}
	if strings.TrimSpace(server.Name) != "" {
		return strings.TrimSpace(server.Name)
	}
	if strings.TrimSpace(server.Host) != "" {
		return strings.TrimSpace(server.Host)
	}
	return id
}

// gitOperation executes git operations
func (e *ToolExecutor) gitOperation(args map[string]any, sessionCtx map[string]any) *ToolResult {
	operation, _ := args["operation"].(string)
	if operation == "" {
		return &ToolResult{Success: false, Error: "operation is required"}
	}

	workDir, _ := args["work_dir"].(string)
	repoURL, _ := args["repo_url"].(string)
	branch, _ := args["branch"].(string)
	message, _ := args["message"].(string)
	serverID, _ := args["server_id"].(string)
	if serverID == "" {
		if sid, ok := sessionCtx["current_server_id"].(string); ok {
			serverID = sid
		}
	}

	var cmd string
	switch operation {
	case "clone":
		if repoURL == "" {
			return &ToolResult{Success: false, Error: "repo_url is required for clone"}
		}
		cmd = fmt.Sprintf("git clone %s", repoURL)
		if workDir != "" {
			cmd += " " + workDir
		}
	case "pull":
		cmd = "git pull"
	case "push":
		cmd = "git push"
	case "commit":
		if message == "" {
			return &ToolResult{Success: false, Error: "message is required for commit"}
		}
		cmd = fmt.Sprintf("git commit -m %q", message)
	case "status":
		cmd = "git status"
	case "branch":
		cmd = "git branch -a"
	case "checkout":
		if branch == "" {
			return &ToolResult{Success: false, Error: "branch is required for checkout"}
		}
		cmd = fmt.Sprintf("git checkout %s", branch)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("unknown git operation: %s", operation)}
	}

	return e.executeCommand(map[string]any{"command": cmd, "work_dir": workDir, "server_id": serverID}, sessionCtx)
}

// checkTaskStatus checks task status
func (e *ToolExecutor) checkTaskStatus(args map[string]any) *ToolResult {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return &ToolResult{Success: false, Error: "task_id is required"}
	}

	var task model.Task
	if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
		return &ToolResult{Success: false, Error: "task not found"}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("任务状态: %s\n标题: %s", task.Status, task.Title),
		Data:    map[string]string{"status": task.Status},
	}
}

// getTerminalLogs gets terminal logs
func (e *ToolExecutor) getTerminalLogs(args map[string]any) *ToolResult {
	terminalID, _ := args["terminal_id"].(string)
	if terminalID == "" {
		return &ToolResult{Success: false, Error: "terminal_id is required"}
	}

	lines := 100
	if l, ok := args["lines"].(float64); ok {
		lines = int(l)
	}

	var logs []model.Log
	if err := model.DB.Where("terminal_id = ?", terminalID).
		Order("created_at DESC").Limit(lines).Find(&logs).Error; err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

	var sb strings.Builder
	for i := len(logs) - 1; i >= 0; i-- {
		sb.WriteString(logs[i].Content)
	}

	return &ToolResult{Success: true, Output: sb.String()}
}

// wait waits for specified seconds
func (e *ToolExecutor) wait(args map[string]any) *ToolResult {
	seconds := 5
	if s, ok := args["seconds"].(float64); ok {
		seconds = int(s)
	}

	reason, _ := args["reason"].(string)
	time.Sleep(time.Duration(seconds) * time.Second)

	output := fmt.Sprintf("等待了 %d 秒", seconds)
	if reason != "" {
		output += fmt.Sprintf(" (%s)", reason)
	}
	return &ToolResult{Success: true, Output: output}
}
