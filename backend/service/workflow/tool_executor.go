package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
)

// ToolExecutor executes workflow tools
type ToolExecutor struct {
	sshManager sshCommandExecutor
	automation automationService
	terminal   terminalManager
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
		return e.createTask(args, sessionCtx)
	case "start_task":
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

	sessionCtx["current_server_id"] = serverID
	sessionCtx["current_server_name"] = server.Name

	execMode := strings.ToLower(strings.TrimSpace(getStringFromMap(sessionCtx, "command_execution_mode")))
	if execMode == "terminal" && e.terminal != nil {
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

	restartLike := isRestartLikeCommand(command)
	taskID := ""
	if sessionCtx != nil {
		taskID = strings.TrimSpace(getStringFromMap(sessionCtx, "task_id"))
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

	execMode := strings.ToLower(strings.TrimSpace(getStringFromMap(sessionCtx, "command_execution_mode")))
	if execMode == "" {
		execMode = "backend"
	}

	// 优先使用“终端执行模式”：让 AI 真正在工作台可见的终端里敲命令，便于实时观察/接管。
	if execMode == "terminal" && e.terminal != nil {
		// 若任务处于“预期断开/等待重连”，非重启类命令应等待重连完成后再执行，避免 AI 指令乱序/串任务。
		if taskID != "" && model.DB != nil && !restartLike {
			if err := waitForTaskReconnect(taskID, 5*time.Minute); err != nil {
				return &ToolResult{Success: false, Error: err.Error()}
			}
		}

		terminalID := e.ensureTerminalForServer(sessionCtx, serverID)
		if strings.TrimSpace(terminalID) != "" {
			displayCmd := strings.TrimSpace(command)
			if workDir != "" {
				displayCmd = fmt.Sprintf("cd %s && %s", strings.TrimSpace(workDir), strings.TrimSpace(command))
			}
			e.emitTerminalAILog(sessionCtx, "action", fmt.Sprintf("[%s] $ %s", strings.TrimSpace(serverID), displayCmd), "command", strings.TrimSpace(displayCmd))

			session, err := e.terminal.GetOrResumeSession(terminalID)
			if err == nil && session != nil {
				// 标记“预期断开”：当 AI 执行重启类命令时，允许后续自动重连逻辑接管。
				if restartLike && taskID != "" && model.DB != nil {
					now := time.Now()
					_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
						"expect_disconnect":  true,
						"ai_status":          "waiting_reconnect",
						"ai_pause_reason":    "",
						"reconnect_attempts": 0,
						"last_reconnect_at":  nil,
						"updated_at":         now,
					}).Error
				}

				output, exitCode, runErr := session.RunCommand(command, workDir, 0)
				if runErr != nil {
					if restartLike && taskID != "" && isExpectedDisconnectError(runErr) {
						e.emitTerminalAILog(sessionCtx, "warning", fmt.Sprintf("[%s] 终端已断开（预期重启），等待自动重连…", strings.TrimSpace(serverID)), "", "")
						return &ToolResult{Success: true, Output: strings.TrimSpace(output)}
					}
					// 重启类命令未导致断开：清理“预期断开”标记，避免后续命令无谓等待。
					if restartLike && taskID != "" && model.DB != nil {
						now := time.Now()
						_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
							"expect_disconnect": false,
							"ai_status":         "running",
							"ai_pause_reason":   "",
							"updated_at":        now,
						}).Error
					}
					msg := fmt.Sprintf("[%s] 命令执行失败: %v", strings.TrimSpace(serverID), runErr)
					if strings.TrimSpace(output) != "" {
						msg = msg + "\n" + strings.TrimRight(output, "\n")
					} else if exitCode >= 0 {
						msg = msg + fmt.Sprintf(" (exit=%d)", exitCode)
					}
					e.emitTerminalAILog(sessionCtx, "error", msg, "", "")
					return &ToolResult{Success: false, Error: runErr.Error(), Output: output}
				}

				// 重启类命令未导致断开：清理“预期断开”标记，避免后续命令无谓等待。
				if restartLike && taskID != "" && model.DB != nil {
					now := time.Now()
					_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
						"expect_disconnect": false,
						"ai_status":         "running",
						"ai_pause_reason":   "",
						"updated_at":        now,
					}).Error
				}

				out := strings.TrimRight(output, "\n")
				if strings.TrimSpace(out) == "" {
					e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] （无输出）", strings.TrimSpace(serverID)), "", "")
				} else {
					e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s]\n%s", strings.TrimSpace(serverID), out), "", "")
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

	// Execute on server or locally
	output, err := e.sshManager.ExecuteCommand(serverID, fullCmd)
	if err != nil {
		msg := fmt.Sprintf("[%s] 命令执行失败: %v", strings.TrimSpace(serverID), err)
		if strings.TrimSpace(output) != "" {
			msg = msg + "\n" + strings.TrimRight(output, "\n")
		}
		e.emitTerminalAILog(sessionCtx, "error", msg, "", "")
		return &ToolResult{Success: false, Error: err.Error(), Output: output}
	}
	out := strings.TrimRight(output, "\n")
	if strings.TrimSpace(out) == "" {
		e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s] （无输出）", strings.TrimSpace(serverID)), "", "")
	} else {
		e.emitTerminalAILog(sessionCtx, "info", fmt.Sprintf("[%s]\n%s", strings.TrimSpace(serverID), out), "", "")
	}

	// 重启类命令未导致断开：清理“预期断开”标记，避免后续命令无谓等待。
	if restartLike && taskID != "" && model.DB != nil {
		now := time.Now()
		_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
			"expect_disconnect": false,
			"ai_status":         "running",
			"ai_pause_reason":   "",
			"updated_at":        now,
		}).Error
	}

	return &ToolResult{Success: true, Output: output}
}

func isRestartLikeCommand(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false
	}

	// common patterns; keep broad (false positives are acceptable, false negatives are costly)
	if strings.Contains(cmd, " reboot") || cmd == "reboot" || strings.HasPrefix(cmd, "reboot ") {
		return true
	}
	if strings.Contains(cmd, "shutdown") {
		// shutdown -r / --reboot / now
		if strings.Contains(cmd, "-r") || strings.Contains(cmd, "--reboot") {
			return true
		}
	}
	if strings.Contains(cmd, "init 6") {
		return true
	}
	if strings.Contains(cmd, "systemctl reboot") || strings.Contains(cmd, "systemctl poweroff") {
		return true
	}
	if strings.Contains(cmd, "systemctl restart") {
		return true
	}
	return false
}

func waitForTaskReconnect(taskID string, timeout time.Duration) error {
	if model.DB == nil {
		return nil
	}
	id := strings.TrimSpace(taskID)
	if id == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	deadline := time.Now().Add(timeout)
	for {
		var task model.Task
		if err := model.DB.Select("expect_disconnect", "ai_status", "ai_pause_reason").First(&task, "id = ?", id).Error; err != nil {
			return err
		}
		if !task.ExpectDisconnect {
			if strings.EqualFold(strings.TrimSpace(task.AIStatus), "paused") && strings.EqualFold(strings.TrimSpace(task.AIPauseReason), "reconnect_timeout") {
				return errors.New("task reconnect timed out")
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for task reconnect (%s)", timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

func isExpectedDisconnectError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "terminal session closed") || strings.Contains(msg, "terminal subscription closed")
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

	taskID := strings.TrimSpace(getStringFromMap(sessionCtx, "task_id"))

	terminalByServer := getStringMapFromContext(sessionCtx, "terminal_ids_by_server")
	if terminalByServer == nil {
		terminalByServer = map[string]string{}
	}

	// 优先使用任务的 ActiveTerminalID（避免重启后上下文仍指向旧终端导致重复创建）
	if taskID != "" && model.DB != nil {
		var task model.Task
		if err := model.DB.Select("active_terminal_id").First(&task, "id = ?", taskID).Error; err == nil {
			if task.ActiveTerminalID != nil {
				active := strings.TrimSpace(*task.ActiveTerminalID)
				if active != "" {
					var t model.TerminalSession
					if err := model.DB.Select("id", "server_id", "task_id").First(&t, "id = ?", active).Error; err == nil {
						if t.ServerID != nil && strings.TrimSpace(*t.ServerID) == sid {
							if t.TaskID == nil || strings.TrimSpace(*t.TaskID) == "" || strings.TrimSpace(*t.TaskID) == taskID {
								if sess, err := e.terminal.GetOrResumeSession(active); err == nil && sess != nil {
									terminalByServer[sid] = active
									sessionCtx["terminal_ids_by_server"] = terminalByServer
									sessionCtx["terminal_id"] = active
									return active
								}
							}
						}
					}
				}
			}
		}
	}

	if existing := strings.TrimSpace(terminalByServer[sid]); existing != "" {
		// 任务隔离：禁止复用其他任务的终端
		if taskID != "" && model.DB != nil {
			var t model.TerminalSession
			if err := model.DB.Select("task_id").First(&t, "id = ?", existing).Error; err == nil {
				if t.TaskID != nil {
					bound := strings.TrimSpace(*t.TaskID)
					if bound != "" && bound != taskID {
						delete(terminalByServer, sid)
						sessionCtx["terminal_ids_by_server"] = terminalByServer
						existing = ""
					}
				}
			}
		}
		if existing != "" {
			if sess, err := e.terminal.GetOrResumeSession(existing); err == nil && sess != nil {
				sessionCtx["terminal_id"] = existing
				if taskID != "" && model.DB != nil {
					_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
						"active_terminal_id": existing,
						"updated_at":         time.Now(),
					}).Error
				}
				return existing
			}
		}
		delete(terminalByServer, sid)
	}

	// 如果上下文已带 terminal_id（例如 StartTaskAgent 创建并由前端打开的默认终端），
	// 且该终端绑定的 server_id 与当前选择的服务器一致，则优先复用，避免创建第二个终端导致“AI 在后台跑，工作台终端静止”。
	current := strings.TrimSpace(getStringFromMap(sessionCtx, "terminal_id"))
	if current != "" && model.DB != nil {
		var t model.TerminalSession
		if err := model.DB.Select("id", "server_id", "task_id").First(&t, "id = ?", current).Error; err == nil {
			if t.ServerID != nil && strings.TrimSpace(*t.ServerID) == sid {
				if taskID != "" && t.TaskID != nil {
					bound := strings.TrimSpace(*t.TaskID)
					if bound != "" && bound != taskID {
						// 不同任务必须新建终端：跳过复用
						current = ""
					}
				}
				if current != "" {
					if sess, err := e.terminal.GetOrResumeSession(current); err == nil && sess != nil {
						terminalByServer[sid] = current
						sessionCtx["terminal_ids_by_server"] = terminalByServer
						sessionCtx["terminal_id"] = current
						if taskID != "" {
							_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
								"active_terminal_id": current,
								"updated_at":         time.Now(),
							}).Error
						}
						return current
					}
				}
			}
		}
	}

	session, err := e.terminal.CreateSSHSession(sid)
	if err != nil || session == nil {
		return ""
	}

	terminalID := session.ID()
	if taskID != "" {
		taskIDCopy := taskID
		_ = e.terminal.LinkTask(terminalID, &taskIDCopy)
		if model.DB != nil {
			_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
				"active_terminal_id": terminalID,
				"updated_at":         time.Now(),
			}).Error
		}
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
