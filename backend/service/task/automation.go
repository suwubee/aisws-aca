package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/ai"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var sleep = time.Sleep

// AutomationService 任务自动化服务
type AutomationService struct {
	terminalManager terminalManager
}

type taskTerminal interface {
	ID() string
	Write(data []byte) error
}

type terminalManager interface {
	CreateSession(title string, taskID *string) (taskTerminal, error)
	CreateSSHSession(serverID string) (taskTerminal, error)
	RenameSession(id, title string) error
	LinkTask(id string, taskID *string) error
	GetSession(id string) taskTerminal
}

type terminalManagerAdapter struct {
	manager *terminal.Manager
}

func (a terminalManagerAdapter) CreateSession(title string, taskID *string) (taskTerminal, error) {
	return a.manager.CreateSession(title, taskID)
}

func (a terminalManagerAdapter) CreateSSHSession(serverID string) (taskTerminal, error) {
	return a.manager.CreateSSHSession(serverID)
}

func (a terminalManagerAdapter) RenameSession(id, title string) error {
	return a.manager.RenameSession(id, title)
}

func (a terminalManagerAdapter) LinkTask(id string, taskID *string) error {
	return a.manager.LinkTask(id, taskID)
}

func (a terminalManagerAdapter) GetSession(id string) taskTerminal {
	session := a.manager.GetSession(id)
	if session == nil {
		return nil
	}
	return session
}

// NewAutomationService 创建自动化服务
func NewAutomationService(tm *terminal.Manager) *AutomationService {
	return &AutomationService{
		terminalManager: terminalManagerAdapter{manager: tm},
	}
}

// CLIConfig CLI 配置
type CLIConfig struct {
	Type    string // claude, codex, gemini
	Command string // 实际执行的命令
}

// GetCLIConfig 获取 CLI 配置
func GetCLIConfig(cliType string) *CLIConfig {
	switch cliType {
	case "claude":
		return &CLIConfig{Type: "claude", Command: "claude"}
	case "codex":
		return &CLIConfig{Type: "codex", Command: "codex"}
	case "gemini":
		return &CLIConfig{Type: "gemini", Command: "gemini"}
	default:
		return &CLIConfig{Type: "claude", Command: "claude"}
	}
}

// sendTmuxKeys 使用 tmux send-keys 发送按键到指定会话
func sendTmuxKeys(sessionID string, keys string, literal bool) error {
	return sendTmuxKeysToTarget(sessionID, keys, literal)
}

// sendTmuxKeysToTarget 使用 tmux send-keys 发送按键到指定目标
func sendTmuxKeysToTarget(target string, keys string, literal bool) error {
	args := []string{"send-keys", "-t", target}
	if literal {
		args = append(args, "-l")
	}
	// 使用 -- 分隔符防止以 - 开头的内容被解析为参数
	args = append(args, "--", keys)
	cmd := exec.Command("tmux", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Warn("tmux send-keys failed",
			zap.String("target", target),
			zap.String("keys", keys),
			zap.String("output", string(output)),
			zap.Error(err))
		return err
	}
	utils.Debug("tmux send-keys success",
		zap.String("target", target),
		zap.Bool("literal", literal))
	return nil
}

// TmuxKey 常用 tmux 按键名称
// 使用 tmux send-keys 时，这些名称会被解释为特殊按键
const (
	TmuxKeyEnter     = "Enter"  // 回车键
	TmuxKeyEscape    = "Escape" // ESC 键
	TmuxKeyTab       = "Tab"    // Tab 键
	TmuxKeyBackspace = "BSpace" // 退格键
	TmuxKeyDelete    = "DC"     // Delete 键
	TmuxKeyUp        = "Up"     // 上箭头
	TmuxKeyDown      = "Down"   // 下箭头
	TmuxKeyLeft      = "Left"   // 左箭头
	TmuxKeyRight     = "Right"  // 右箭头
	TmuxKeyHome      = "Home"   // Home 键
	TmuxKeyEnd       = "End"    // End 键
	TmuxKeyPageUp    = "PPage"  // Page Up
	TmuxKeyPageDown  = "NPage"  // Page Down
	TmuxKeySpace     = "Space"  // 空格键
)

// SendCtrlKey 发送 Ctrl+字母 组合键
// 例如: SendCtrlKey(sessionID, 'c') 发送 Ctrl+C
func SendCtrlKey(sessionID string, key rune) error {
	// tmux send-keys 使用 C-x 格式表示 Ctrl+x
	ctrlKey := fmt.Sprintf("C-%c", key)
	return sendTmuxKeys(sessionID, ctrlKey, false)
}

// buildManagedPrompt 构建 AI 托管模式的提示词
// 将用户目标、托管指令和结束条件组合成结构化提示
func buildManagedPrompt(task *model.Task) string {
	if task == nil {
		return ""
	}

	result, err := promptsvc.RenderTemplate(promptsvc.TemplateKeyTaskManagedPrompt, map[string]any{
		"task_initial_prompt":   strings.TrimSpace(task.InitialPrompt),
		"task_ai_prompt":        strings.TrimSpace(task.AIPrompt),
		"task_ai_end_condition": strings.TrimSpace(task.AIEndCondition),
		"task_done_marker":      "ACA_TASK_DONE",
	})
	if err == nil {
		return strings.TrimSpace(result)
	}

	// 提示词模板不可用时，降级为拼接任务字段（不引入硬编码文案）
	parts := make([]string, 0, 3)
	for _, chunk := range []string{task.InitialPrompt, task.AIPrompt, task.AIEndCondition} {
		text := strings.TrimSpace(chunk)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

// StartTaskResult 启动任务结果
type StartTaskResult struct {
	Task            *model.Task  `json:"task"`
	Terminal        taskTerminal `json:"terminal"`
	TerminalIDs     []string     `json:"terminal_ids,omitempty"`
	WorkDir         string       `json:"work_dir"`
	CLIStarted      bool         `json:"cli_started"`
	NeedsUserAction bool         `json:"needs_user_action,omitempty"`
	UserActionHint  string       `json:"user_action_hint,omitempty"`
	Error           string       `json:"error,omitempty"`
}

// StartTask 启动自动化任务
func (s *AutomationService) StartTask(task *model.Task) (*StartTaskResult, error) {
	result := &StartTaskResult{Task: task}
	if task == nil {
		result.Error = "Task is nil"
		return result, errors.New("task is nil")
	}

	mode := strings.ToLower(strings.TrimSpace(task.AutomationMode))
	if mode == "" {
		mode = "cli"
	}
	switch mode {
	case "none":
		result.Error = "Task automation is disabled"
		return result, errors.New("task automation is disabled")
	case "script":
		return s.startScriptTask(task)
	}

	// 幂等性检查：如果任务已经在进行中，不重复启动
	if task.Status == "in_progress" {
		result.Error = "Task already in progress"
		utils.Info("Task already in progress, skipping start",
			zap.String("task_id", task.ID))
		return result, errors.New("task already in progress")
	}

	var serverID string
	if task.ServerID != nil {
		serverID = strings.TrimSpace(*task.ServerID)
	}

	if serverID == "" {
		result.Error = "Server is required (local must be configured in Servers)"
		return result, errors.New("server is required")
	}

	// 1. 处理工作目录
	workDir := strings.TrimSpace(task.WorkDir)
	if workDir == "" {
		workDir = defaultTaskWorkDir(task.ID, serverID != "")
		task.WorkDir = workDir
		model.DB.Model(task).Updates(map[string]interface{}{
			"work_dir":   workDir,
			"updated_at": time.Now(),
		})
	}
	result.WorkDir = workDir

	var serverLabel string
	if serverID != "" {
		var server model.SSHServer
		if err := model.DB.Select("id", "name", "host").First(&server, "id = ?", serverID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result.Error = "Server not found"
				return result, errors.New("server not found")
			}
			result.Error = "Failed to query server"
			return result, errors.New("failed to query server")
		}
		if strings.TrimSpace(server.Name) != "" {
			serverLabel = server.Name
		} else if strings.TrimSpace(server.Host) != "" {
			serverLabel = server.Host
		} else {
			serverLabel = serverID
		}
	}

	// 创建本地目录（如果需要）
	if task.AutoCreateDir && serverID == "" {
		if err := os.MkdirAll(workDir, 0755); err != nil {
			result.Error = fmt.Sprintf("Failed to create work directory: %v", err)
			return result, err
		}
		utils.Info("Created work directory", zap.String("path", workDir))
	}

	// 2. 创建终端会话
	terminalTitle := fmt.Sprintf("[%s] %s", task.CLIType, task.Title)
	if serverLabel != "" {
		terminalTitle = fmt.Sprintf("%s @ %s", terminalTitle, serverLabel)
	}

	var session taskTerminal
	var err error

	if serverID != "" {
		session, err = s.terminalManager.CreateSSHSession(serverID)
	} else {
		session, err = s.terminalManager.CreateSession(terminalTitle, &task.ID)
	}
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create terminal: %v", err)
		return result, err
	}
	result.Terminal = session

	if serverID != "" {
		_ = s.terminalManager.LinkTask(session.ID(), &task.ID)
		_ = s.terminalManager.RenameSession(session.ID(), terminalTitle)
	}

	// 立即设置任务状态为 in_progress，防止重复启动（幂等性保证）
	now := time.Now()
	model.DB.Model(task).Updates(map[string]interface{}{
		"status":             "in_progress",
		"active_terminal_id": session.ID(),
		"ai_status":          "running",
		"ai_pause_reason":    "",
		"expect_disconnect":  false,
		"reconnect_attempts": 0,
		"last_reconnect_at":  nil,
		"updated_at":         now,
	})

	// 创建远程目录（如果需要）
	if task.AutoCreateDir && serverID != "" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s\r", workDir)
		if err := session.Write([]byte(mkdirCmd)); err != nil {
			result.Error = fmt.Sprintf("Failed to create remote work directory: %v", err)
			return result, err
		}
		sleep(300 * time.Millisecond)
		utils.Info("Created remote work directory", zap.String("path", workDir), zap.String("server_id", serverID))
	}

	// 3. 进入工作目录
	cdCmd := fmt.Sprintf("cd %s\r", workDir)
	if err := session.Write([]byte(cdCmd)); err != nil {
		result.Error = fmt.Sprintf("Failed to change directory: %v", err)
		return result, err
	}

	// 等待命令执行
	sleep(300 * time.Millisecond)

	// 4. 启动 CLI
	cliConfig := GetCLIConfig(task.CLIType)
	cliCmd := cliConfig.Command + "\r"
	if err := session.Write([]byte(cliCmd)); err != nil {
		result.Error = fmt.Sprintf("Failed to start CLI: %v", err)
		return result, err
	}
	result.CLIStarted = true

	// 广播AI日志
	if termSession, ok := session.(*terminal.Session); ok {
		termSession.BroadcastAILog("action", fmt.Sprintf("启动 %s CLI", cliConfig.Type))
	}

	pauseForUserAction := func(hint string) {
		text := strings.TrimSpace(hint)
		if text == "" {
			text = "需要用户确认后继续"
		}

		result.NeedsUserAction = true
		result.UserActionHint = text

		if termSession, ok := session.(*terminal.Session); ok {
			termSession.BroadcastAILog("warning", text)
		}

		s.updateTaskStatus(task.ID, "paused", text)
		s.createTaskMessage(task.ID, session.ID(), "approval_needed", "需要确认：CLI 启动方式", text)
	}

	// 快速判断：命令不存在时不要继续发送提示，避免把 prompt 当作 shell 命令执行
	if termSession, ok := session.(*terminal.Session); ok {
		sleep(800 * time.Millisecond)
		scroll := strings.ToLower(string(termSession.Scrollback()))
		cmdLower := strings.ToLower(strings.TrimSpace(cliConfig.Command))
		if cmdLower != "" && (strings.Contains(scroll, cmdLower+": command not found") || strings.Contains(scroll, cmdLower+": not found") || strings.Contains(scroll, "command not found: "+cmdLower)) {
			hint := "未检测到 CLI 命令可用：可能未安装或不在 PATH。\n"
			switch strings.ToLower(strings.TrimSpace(cliConfig.Type)) {
			case "claude":
				hint += "请在终端中手动尝试执行：claude 或 npx claude（确认可进入 Claude Code 后再继续）。"
			case "codex":
				hint += "请确认 codex 已安装并可直接执行：codex。"
			case "gemini":
				hint += "请确认 gemini CLI 已安装并可直接执行：gemini。"
			default:
				hint += "请确认 CLI 已安装并可直接执行。"
			}
			pauseForUserAction(hint)
			return result, nil
		}
	}

	// 5. 如果有初始提示或AI托管模式，等待 CLI 启动后输入
	promptToSend := task.InitialPrompt
	if task.AIManaged {
		promptToSend = buildManagedPrompt(task)
	}

	if promptToSend != "" {
		termSession, ok := session.(*terminal.Session)
		if !ok {
			// 无法读取元数据/scrollback 时，降级为直接写入提示（保持与旧行为一致，避免卡住）
			promptWithEnter := promptToSend + "\r"
			if err := session.Write([]byte(promptWithEnter)); err != nil {
				utils.Warn("Failed to send prompt via PTY fallback", zap.Error(err))
			}
			utils.Info("Prompt sent via PTY fallback (no terminal metadata)")
			goto PROMPT_SENT
		}

		// 使用detector检测CLI是否准备好
		maxWait := 30 * time.Second
		checkInterval := 500 * time.Millisecond
		startTime := time.Now()
		cliReady := false

		utils.Info("Waiting for CLI ready", zap.String("task_id", task.ID), zap.String("prompt_len", fmt.Sprintf("%d", len(promptToSend))))

		for time.Since(startTime) < maxWait {
			sleep(checkInterval)
			// 检查终端元数据中的AI状态
			if meta := termSession.Metadata(); meta != nil && meta.AIAssistant != nil {
				utils.Debug("CLI state check",
					zap.Bool("detected", meta.AIAssistant.Detected),
					zap.String("state", meta.AIAssistant.State))
				if meta.AIAssistant.Detected {
					state := meta.AIAssistant.State
					if state == "waiting_input" || state == "working" {
						cliReady = true
						utils.Info("CLI ready detected", zap.String("state", state))
						break
					}
				}
			}
		}

		if !cliReady {
			utils.Warn("CLI ready timeout, pausing task instead of sending prompt")
			hint := "CLI 就绪检测超时：未能确认已进入 AI CLI 交互界面，已暂停任务以避免误操作。\n"
			switch strings.ToLower(strings.TrimSpace(cliConfig.Type)) {
			case "claude":
				hint += "请打开终端确认已进入 Claude Code（可尝试执行 claude 或 npx claude），然后再继续。"
			case "codex":
				hint += "请打开终端确认已进入 Codex CLI（可尝试执行 codex），然后再继续。"
			case "gemini":
				hint += "请打开终端确认已进入 Gemini CLI（可尝试执行 gemini），然后再继续。"
			default:
				hint += "请打开终端确认已进入目标 CLI，然后再继续。"
			}
			pauseForUserAction(hint)
			return result, nil
		}

		termSession.BroadcastAILog("info", "CLI已就绪，准备发送提示")

		// 获取 tmux 会话名称
		var tmuxSession string
		if meta := termSession.Metadata(); meta != nil {
			tmuxSession = meta.TmuxSession
		}

		// 优先使用 tmux send-keys（指定完整目标）
		if tmuxSession != "" {
			target := tmuxSession + ":0.0"
			// 发送提示内容（字面量模式）
			if err := sendTmuxKeysToTarget(target, promptToSend, true); err != nil {
				utils.Warn("Failed to send prompt via tmux", zap.Error(err))
				if termSession, ok := session.(*terminal.Session); ok {
					termSession.BroadcastAILog("error", fmt.Sprintf("发送提示失败: %v", err))
				}
			} else {
				utils.Info("Prompt sent via tmux", zap.String("target", target))
				if termSession, ok := session.(*terminal.Session); ok {
					termSession.BroadcastAILogWithInput("action", "发送提示内容", "text", promptToSend)
				}
			}

			sleep(300 * time.Millisecond)

			// 发送回车键 - 使用 C-m (Ctrl+M = CR = \r) 而不是 "Enter" 键名
			if err := sendTmuxKeysToTarget(target, "C-m", false); err != nil {
				utils.Warn("Failed to send Enter via tmux", zap.Error(err))
				// 回退：直接写入 PTY
				if err := session.Write([]byte("\r")); err != nil {
					utils.Warn("Failed to send Enter via PTY fallback", zap.Error(err))
					if termSession, ok := session.(*terminal.Session); ok {
						termSession.BroadcastAILog("error", "发送回车失败")
					}
				} else {
					if termSession, ok := session.(*terminal.Session); ok {
						termSession.BroadcastAILogWithInput("action", "发送回车(PTY)", "key", "Enter")
					}
				}
			} else {
				utils.Info("Enter (C-m) sent via tmux")
				if termSession, ok := session.(*terminal.Session); ok {
					termSession.BroadcastAILogWithInput("action", "发送回车，任务开始执行", "key", "Ctrl+M")
				}
			}
		} else {
			// 回退：直接写入 PTY
			utils.Info("Using PTY fallback")
			promptWithEnter := promptToSend + "\r"
			if err := session.Write([]byte(promptWithEnter)); err != nil {
				utils.Warn("Failed to send via PTY", zap.Error(err))
			}
		}
	}

PROMPT_SENT:

	utils.Info("Task automation started",
		zap.String("task_id", task.ID),
		zap.String("terminal_id", session.ID()),
		zap.String("cli_type", task.CLIType),
		zap.String("work_dir", workDir))

	// 7. 启动后台任务监控
	go s.monitorTaskCompletion(task.ID, session.ID())

	result.TerminalIDs = []string{session.ID()}

	return result, nil
}

const (
	scriptTaskExitCodeMarker = "ACA_TASK_EXIT_CODE:"
	scriptTaskPauseMarker    = "ACA_TASK_PAUSE"
)

type scriptTarget struct {
	TerminalID string
	Label      string
}

func (s *AutomationService) startScriptTask(task *model.Task) (*StartTaskResult, error) {
	result := &StartTaskResult{Task: task}
	if task == nil {
		result.Error = "Task is nil"
		return result, errors.New("task is nil")
	}

	script := strings.TrimSpace(task.Script)
	if script == "" {
		result.Error = "Script is empty"
		return result, errors.New("script is empty")
	}

	targetServerIDs := make([]string, 0)
	for _, raw := range task.TargetServerIDs {
		sid := strings.TrimSpace(raw)
		if sid == "" {
			continue
		}
		targetServerIDs = append(targetServerIDs, sid)
	}
	if len(targetServerIDs) == 0 && task.ServerID != nil {
		if sid := strings.TrimSpace(*task.ServerID); sid != "" {
			targetServerIDs = append(targetServerIDs, sid)
		}
	}

	if len(targetServerIDs) == 0 {
		result.Error = "Target server is required (local must be configured in Servers)"
		return result, errors.New("target server is required")
	}

	isRemote := len(targetServerIDs) > 0

	workDir := strings.TrimSpace(task.WorkDir)
	if workDir == "" {
		workDir = defaultTaskWorkDir(task.ID, isRemote)
		task.WorkDir = workDir
		model.DB.Model(task).Updates(map[string]interface{}{
			"work_dir":   workDir,
			"updated_at": time.Now(),
		})
	}
	result.WorkDir = workDir

	// 创建终端会话（脚本模式：每台目标服务器一个 SSH 终端）
	targets := make([]scriptTarget, 0, maxInt(1, len(targetServerIDs)))
	sessions := make([]taskTerminal, 0, maxInt(1, len(targetServerIDs)))

	for _, serverID := range targetServerIDs {
		serverLabel := resolveServerLabel(serverID)
		title := fmt.Sprintf("[script] %s", task.Title)
		if serverLabel != "" {
			title = fmt.Sprintf("%s @ %s", title, serverLabel)
		}

		session, err := s.terminalManager.CreateSSHSession(serverID)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to create SSH terminal: %v", err)
			return result, err
		}
		_ = s.terminalManager.LinkTask(session.ID(), &task.ID)
		_ = s.terminalManager.RenameSession(session.ID(), title)

		if result.Terminal == nil {
			result.Terminal = session
		}
		targets = append(targets, scriptTarget{TerminalID: session.ID(), Label: serverLabel})
		sessions = append(sessions, session)
	}

	result.TerminalIDs = make([]string, 0, len(targets))
	for _, t := range targets {
		result.TerminalIDs = append(result.TerminalIDs, t.TerminalID)
	}

	// 设置任务为进行中，防止重复启动
	updates := map[string]interface{}{
		"status":     "in_progress",
		"updated_at": time.Now(),
	}
	if result.Terminal != nil {
		if id := strings.TrimSpace(result.Terminal.ID()); id != "" {
			updates["active_terminal_id"] = id
		}
	}
	model.DB.Model(task).Updates(updates)

	// 写入并执行脚本
	for _, session := range sessions {
		if err := runScriptInTerminal(session, workDir, script, task.AutoCreateDir, isRemote); err != nil {
			result.Error = fmt.Sprintf("Failed to start script: %v", err)
			s.updateTaskStatus(task.ID, "failed", "脚本下发失败")
			return result, err
		}
	}

	// 后台监控脚本执行结果（多终端聚合）
	go s.monitorScriptTask(task.ID, targets)

	return result, nil
}

func runScriptInTerminal(session taskTerminal, workDir string, script string, autoCreateDir bool, isRemote bool) error {
	if session == nil {
		return errors.New("terminal session is nil")
	}

	dir := strings.TrimSpace(workDir)
	if dir == "" {
		return errors.New("workDir is empty")
	}

	// 本地创建目录（避免依赖 shell）
	if autoCreateDir && !isRemote {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	if autoCreateDir && isRemote {
		cmd := fmt.Sprintf("mkdir -p -- %s", quoteShellPath(dir))
		if err := writeShellLine(session, cmd); err != nil {
			return err
		}
		sleep(150 * time.Millisecond)
	}

	if err := writeShellLine(session, fmt.Sprintf("cd -- %s", quoteShellPath(dir))); err != nil {
		return err
	}
	sleep(150 * time.Millisecond)

	const scriptFile = ".aca_task.sh"
	if err := writeShellLine(session, fmt.Sprintf("cat > %s <<'ACA_SCRIPT_EOF'", scriptFile)); err != nil {
		return err
	}
	sleep(80 * time.Millisecond)

	normalized := normalizeScript(script)
	for _, line := range normalized {
		if err := writeShellLine(session, line); err != nil {
			return err
		}
	}
	if err := writeShellLine(session, "ACA_SCRIPT_EOF"); err != nil {
		return err
	}

	// 执行并输出退出码标记，供监控聚合判断
	run := fmt.Sprintf("bash %s; ACA_CODE=$?; echo \"%s${ACA_CODE}\"; unset ACA_CODE", scriptFile, scriptTaskExitCodeMarker)
	return writeShellLine(session, run)
}

func normalizeScript(script string) []string {
	text := strings.ReplaceAll(script, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	// 保留空行，避免破坏 here-doc 等结构
	return lines
}

func writeShellLine(session taskTerminal, line string) error {
	if session == nil {
		return errors.New("terminal session is nil")
	}
	return session.Write([]byte(line + "\r"))
}

func quoteShellPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "''"
	}

	if trimmed == "~" {
		return "\"$HOME\""
	}
	if strings.HasPrefix(trimmed, "~/") {
		rest := strings.TrimPrefix(trimmed, "~/")
		return "\"$HOME/" + escapeDoubleQuoted(rest) + "\""
	}

	// 默认使用单引号，避免变量/命令替换
	return "'" + strings.ReplaceAll(trimmed, "'", "'\\''") + "'"
}

func escapeDoubleQuoted(text string) string {
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "$", "\\$")
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	return escaped
}

func resolveServerLabel(serverID string) string {
	id := strings.TrimSpace(serverID)
	if id == "" {
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

func defaultTaskWorkDir(taskID string, remote bool) string {
	id := strings.TrimSpace(taskID)
	if id == "" {
		id = uuid.New().String()
	}
	if remote {
		return "~/.aca/tasks/" + id
	}
	runtimeDir := resolveRuntimeDir()
	return filepath.Join(runtimeDir, "tasks", id)
}

func resolveRuntimeDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ".aca"
	}

	dir := cwd
	for {
		if fileExists(filepath.Join(dir, "backend", "go.mod")) {
			return filepath.Join(dir, ".aca")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Join(cwd, ".aca")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *AutomationService) monitorScriptTask(taskID string, targets []scriptTarget) {
	checkInterval := 5 * time.Second
	maxDuration := 2 * time.Hour
	startTime := time.Now()

	utils.Info("Script task monitoring started",
		zap.String("task_id", taskID),
		zap.Int("targets", len(targets)))

	for {
		if time.Since(startTime) > maxDuration {
			s.updateTaskStatus(taskID, "timeout", "脚本任务监控超时")
			return
		}

		sleep(checkInterval)

		var task model.Task
		if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
			return
		}
		if task.Status != "in_progress" {
			return
		}

		allDone := true
		var firstTerminal string
		var pauseReasons []string
		var failReasons []string

		for _, target := range targets {
			terminalID := strings.TrimSpace(target.TerminalID)
			if terminalID == "" {
				continue
			}
			if firstTerminal == "" {
				firstTerminal = terminalID
			}

			logText := loadTerminalLogsText(taskID, terminalID, 400)
			exitFound, exitCode := parseExitCodeMarker(logText)
			needsUser := containsScriptNeedsUserSignal(logText)

			if !exitFound {
				allDone = false
				continue
			}

			if exitCode != 0 {
				failReasons = append(failReasons, fmt.Sprintf("%s: exit code %d", target.Label, exitCode))
				continue
			}

			if needsUser {
				pauseReasons = append(pauseReasons, fmt.Sprintf("%s: user action required", target.Label))
				continue
			}
		}

		if len(failReasons) > 0 {
			reason := strings.Join(failReasons, "; ")
			s.updateTaskStatus(taskID, "failed", reason)
			if firstTerminal != "" {
				s.createTaskMessage(taskID, firstTerminal, "error", "脚本执行失败", reason)
			}
			return
		}

		if len(pauseReasons) > 0 {
			reason := strings.Join(pauseReasons, "; ")
			s.updateTaskStatus(taskID, "paused", reason)
			if firstTerminal != "" {
				s.createTaskMessage(taskID, firstTerminal, "approval_needed", "脚本需要人工确认", reason)
			}
			return
		}

		if allDone {
			s.updateTaskStatus(taskID, "done", "脚本执行完成")
			return
		}
	}
}

func loadTerminalLogsText(taskID, terminalID string, limit int) string {
	if model.DB == nil {
		return ""
	}
	tid := strings.TrimSpace(terminalID)
	if tid == "" {
		return ""
	}
	if limit <= 0 {
		limit = 200
	}

	query := model.DB.Where("terminal_id = ?", tid)
	if strings.TrimSpace(taskID) != "" {
		query = query.Where("task_id = ? OR task_id IS NULL", strings.TrimSpace(taskID))
	}

	var logs []model.Log
	if err := query.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
		return ""
	}

	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	var b strings.Builder
	for _, entry := range logs {
		content := strings.TrimRight(entry.Content, "\n")
		if strings.TrimSpace(content) == "" {
			continue
		}
		b.WriteString(content)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func parseExitCodeMarker(logText string) (bool, int) {
	text := strings.ToUpper(logText)
	idx := strings.LastIndex(text, strings.ToUpper(scriptTaskExitCodeMarker))
	if idx < 0 {
		return false, 0
	}

	after := strings.TrimSpace(text[idx+len(scriptTaskExitCodeMarker):])
	if after == "" {
		return true, 0
	}

	end := 0
	for end < len(after) {
		ch := after[end]
		if ch < '0' || ch > '9' {
			break
		}
		end++
	}
	if end == 0 {
		return true, 0
	}

	code, err := strconv.Atoi(after[:end])
	if err != nil {
		return true, 0
	}
	return true, code
}

func containsScriptNeedsUserSignal(logText string) bool {
	text := strings.ToLower(logText)
	return strings.Contains(text, strings.ToLower(scriptTaskPauseMarker)) ||
		strings.Contains(text, "reboot") ||
		strings.Contains(text, "重启") ||
		strings.Contains(text, "restart")
}

// StartMonitoring 启动任务完成状态监控（用于中途启用AI托管的情况）
func (s *AutomationService) StartMonitoring(taskID, terminalID string) {
	go s.monitorTaskCompletion(taskID, terminalID)
}

// monitorTaskCompletion 后台监控任务完成状态
func (s *AutomationService) monitorTaskCompletion(taskID, terminalID string) {
	monitor := NewTaskMonitor()
	checkInterval := 10 * time.Second
	maxDuration := 2 * time.Hour
	startTime := time.Now()
	retryCount := 0
	maxRetries := 3

	utils.Info("Task monitoring started",
		zap.String("task_id", taskID),
		zap.String("terminal_id", terminalID))

	for {
		// 超时检查
		if time.Since(startTime) > maxDuration {
			utils.Warn("Task monitoring timeout",
				zap.String("task_id", taskID))
			s.updateTaskStatus(taskID, "timeout", "任务监控超时")
			return
		}

		sleep(checkInterval)

		// 检查终端会话是否还存在
		if s.terminalManager.GetSession(terminalID) == nil {
			utils.Warn("Terminal session not found, doing final log analysis",
				zap.String("task_id", taskID),
				zap.String("terminal_id", terminalID))

			// 如果任务状态已经变化（例如终端关闭触发的自动完成），不要覆盖现有结果
			var current model.Task
			if err := model.DB.Select("id", "status").First(&current, "id = ?", taskID).Error; err != nil {
				return
			}
			if strings.TrimSpace(current.Status) != "in_progress" {
				return
			}

			// 终端关闭前做最后一次日志分析，判断任务是否已完成
			finalDecision, _ := monitor.StartMonitoring(taskID, terminalID)
			if finalDecision.Action == MonitorActionComplete {
				s.updateTaskStatus(taskID, "done", finalDecision.Reason)
				utils.Info("Task completed (final analysis)",
					zap.String("task_id", taskID),
					zap.String("reason", finalDecision.Reason))
			} else {
				s.updateTaskStatus(taskID, "failed", "终端会话已关闭")
			}
			return
		}

		// 检查任务是否还存在
		var task model.Task
		if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
			utils.Warn("Task not found, stopping monitor",
				zap.String("task_id", taskID))
			return
		}

		// 如果任务已经不是进行中状态，停止监控
		if task.Status != "in_progress" {
			utils.Info("Task status changed, stopping monitor",
				zap.String("task_id", taskID),
				zap.String("status", task.Status))
			return
		}

		// 执行监控检查
		decision, err := monitor.StartMonitoring(taskID, terminalID)
		if err != nil {
			utils.Debug("Monitor check error",
				zap.String("task_id", taskID),
				zap.Error(err))
			continue
		}

		utils.Debug("Monitor decision",
			zap.String("task_id", taskID),
			zap.String("action", string(decision.Action)),
			zap.String("reason", decision.Reason))

		// 根据决策采取行动
		switch decision.Action {
		case MonitorActionComplete:
			s.updateTaskStatus(taskID, "done", decision.Reason)
			utils.Info("Task completed",
				zap.String("task_id", taskID),
				zap.String("reason", decision.Reason))
			return

		case MonitorActionAlert:
			// 需要用户干预，根据AI托管配置处理
			utils.Warn("Task needs attention",
				zap.String("task_id", taskID),
				zap.String("reason", decision.Reason))
			if task.AIManaged {
				// AI托管模式下，先用AI判断是否真的需要人工审批
				aiDecision, err := s.judgeAIApproval(taskID, terminalID, decision.Reason)
				if err != nil {
					utils.Error("AI approval judgment failed",
						zap.String("task_id", taskID),
						zap.Error(err))
					// AI判断失败，降级为暂停等待人工处理
					s.updateTaskStatus(taskID, "paused", decision.Reason)
					s.createTaskMessage(taskID, terminalID, "approval_needed", "任务需要用户干预", decision.Reason)
					return
				}

				// 根据AI判断结果处理
				if aiDecision.IsCompleted {
					// AI判断任务已完成
					s.updateTaskStatus(taskID, "done", aiDecision.Reasoning)
					utils.Info("Task completed by AI judgment",
						zap.String("task_id", taskID),
						zap.String("reasoning", aiDecision.Reasoning))
					return
				} else if aiDecision.NeedsApproval {
					// AI判断确实需要人工审批
					s.updateTaskStatus(taskID, "paused", aiDecision.Reasoning)
					s.createTaskMessage(taskID, terminalID, "approval_needed", "AI判断需要人工审批", aiDecision.Reasoning)
					return
				} else {
					// AI判断不需要审批，继续监控
					utils.Info("AI judgment: continue monitoring",
						zap.String("task_id", taskID),
						zap.String("reasoning", aiDecision.Reasoning))
					// 继续下一轮监控
				}
			}

		case MonitorActionRetry:
			// 根据AI托管配置处理错误
			if task.AIManaged && task.AIErrorHandling != "" {
				switch task.AIErrorHandling {
				case "retry":
					retryCount++
					if retryCount <= maxRetries {
						utils.Info("AI managed retry",
							zap.String("task_id", taskID),
							zap.Int("retry_count", retryCount))
						// 继续监控，等待重试结果
					} else {
						// 超过重试次数，降级为暂停
						s.updateTaskStatus(taskID, "paused", "重试次数已达上限")
						s.createTaskMessage(taskID, terminalID, "warning", "重试失败", decision.Reason)
						return
					}
				case "fail":
					s.updateTaskStatus(taskID, "failed", decision.Reason)
					s.createTaskMessage(taskID, terminalID, "error", "任务失败", decision.Reason)
					return
				case "pause":
					s.updateTaskStatus(taskID, "paused", decision.Reason)
					s.createTaskMessage(taskID, terminalID, "warning", "任务已暂停", decision.Reason)
					return
				}
			} else {
				utils.Info("Task retry suggested",
					zap.String("task_id", taskID),
					zap.String("reason", decision.Reason))
			}

		case MonitorActionContinue:
			// 继续监控
		}
	}
}

// updateTaskStatus 更新任务状态
func (s *AutomationService) updateTaskStatus(taskID, status, reason string) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}
	if status == "done" {
		updates["completed_at"] = now
	}
	model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(updates)

	// 备注字段：如果用户未填写，则用系统决策/总结补全（不覆盖用户内容）
	trimmed := strings.TrimSpace(reason)
	if trimmed != "" {
		model.DB.Model(&model.Task{}).
			Where("id = ? AND (remark IS NULL OR remark = '')", taskID).
			Update("remark", trimmed)
	}
}

// createTaskMessage 创建任务消息通知
func (s *AutomationService) createTaskMessage(taskID, terminalID, msgType, title, content string) {
	msg := model.Message{
		ID:         uuid.New().String(),
		TaskID:     &taskID,
		TerminalID: &terminalID,
		Type:       msgType,
		Title:      title,
		Content:    content,
		Status:     "unread",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	model.DB.Create(&msg)
}

// aiApprovalDecision AI审批决策结果
type aiApprovalDecision struct {
	NeedsApproval bool   `json:"needs_approval"` // 是否需要人工审批
	IsCompleted   bool   `json:"is_completed"`   // 任务是否已完成
	Reasoning     string `json:"reasoning"`      // 决策理由
	Confidence    float64 `json:"confidence"`    // 置信度 0-1
}

// judgeAIApproval 使用AI判断是否需要人工审批
// 在AI托管模式下，当监控判断需要用户干预时，先用AI分析是否真的需要审批
func (s *AutomationService) judgeAIApproval(taskID, terminalID string, alertReason string) (*aiApprovalDecision, error) {
	// 获取任务信息
	var task model.Task
	if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// 获取最近的终端输出（最后500行）
	var logs []model.Log
	if err := model.DB.Where("terminal_id = ?", terminalID).
		Order("created_at desc").
		Limit(500).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// 反转日志顺序（从旧到新）
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	// 构建终端输出文本
	var outputBuilder strings.Builder
	for _, log := range logs {
		if log.LogType == "output" {
			outputBuilder.WriteString(log.Content)
		}
	}
	terminalOutput := outputBuilder.String()

	// 构建AI分析提示词
	prompt := fmt.Sprintf(`你是一个AI任务管理助手。当前任务的监控系统检测到可能需要用户干预，但在AI托管模式下，你需要判断是否真的需要人工审批。

任务信息：
- 任务标题：%s
- 任务描述：%s
- 监控告警原因：%s

最近的终端输出（最后500行）：
%s

请分析以下情况并给出判断：
1. 是否存在明确的审批提示（如 yes/no, y/n, confirm 等）？
2. 任务是否已经完成（看到成功标志、完成消息等）？
3. 是否只是正常的进度输出或信息提示，不需要人工干预？

请以JSON格式返回你的判断：
{
  "needs_approval": true/false,  // 是否需要人工审批
  "is_completed": true/false,    // 任务是否已完成
  "reasoning": "你的分析理由",
  "confidence": 0.0-1.0          // 判断的置信度
}`, task.Title, task.Description, alertReason, terminalOutput)

	// 调用AI进行分析
	aiProvider := ai.NewAIProvider()
	config, err := aiProvider.GetDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get AI config: %w", err)
	}

	ctx := context.Background()
	aiResponse, err := aiProvider.ChatSimple(ctx, config, "你是一个AI任务管理助手", prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// 解析AI响应
	var decision aiApprovalDecision
	if err := parseJSONFromText(aiResponse, &decision); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	utils.Info("AI approval judgment",
		zap.String("task_id", taskID),
		zap.Bool("needs_approval", decision.NeedsApproval),
		zap.Bool("is_completed", decision.IsCompleted),
		zap.Float64("confidence", decision.Confidence),
		zap.String("reasoning", decision.Reasoning))

	return &decision, nil
}

// parseJSONFromText 从文本中提取并解析JSON
func parseJSONFromText(text string, v any) error {
	// 尝试直接解析
	if err := json.Unmarshal([]byte(text), v); err == nil {
		return nil
	}

	// 尝试提取JSON代码块
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || start >= end {
		return fmt.Errorf("no JSON found in text")
	}

	jsonStr := text[start : end+1]
	return json.Unmarshal([]byte(jsonStr), v)
}
