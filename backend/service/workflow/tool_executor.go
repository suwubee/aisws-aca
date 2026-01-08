package workflow

import (
	"context"
	"fmt"
	"os/exec"
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
		return &ToolResult{Success: true, Output: "没有可用的服务器。可以在本地执行命令。"}
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

	if cliType == "" {
		cliType = "claude"
	}

	var serverID *string
	if sid, ok := args["server_id"].(string); ok && sid != "" {
		serverID = &sid
	} else if sid, ok := sessionCtx["current_server_id"].(string); ok && sid != "" {
		serverID = &sid
	}

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
	serverID, _ := args["server_id"].(string)
	if serverID == "" {
		if sid, ok := sessionCtx["current_server_id"].(string); ok {
			serverID = sid
		}
	}

	// Build full command with work_dir
	fullCmd := command
	if workDir != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", workDir, command)
	}

	// Execute on server or locally
	if serverID != "" && e.sshManager != nil {
		output, err := e.sshManager.ExecuteCommand(serverID, fullCmd)
		if err != nil {
			return &ToolResult{Success: false, Error: err.Error(), Output: output}
		}
		return &ToolResult{Success: true, Output: output}
	}

	// Local execution
	out, err := exec.Command("sh", "-c", fullCmd).CombinedOutput()
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error(), Output: string(out)}
	}
	return &ToolResult{Success: true, Output: string(out)}
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

	return e.executeCommand(map[string]any{"command": cmd, "work_dir": workDir}, sessionCtx)
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
