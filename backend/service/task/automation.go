package task

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
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
	var parts []string

	// 用户目标（必须）
	if task.InitialPrompt != "" {
		parts = append(parts, fmt.Sprintf("## 任务目标\n%s", task.InitialPrompt))
	}

	// AI托管指令（可选）
	if task.AIPrompt != "" {
		parts = append(parts, fmt.Sprintf("## 执行规则\n%s", task.AIPrompt))
	}

	// 结束条件（可选）
	if task.AIEndCondition != "" {
		parts = append(parts, fmt.Sprintf("## 完成条件\n%s\n\n当满足完成条件时，请在输出中包含标记: ACA_TASK_DONE", task.AIEndCondition))
	}

	return strings.Join(parts, "\n\n")
}

// StartTaskResult 启动任务结果
type StartTaskResult struct {
	Task       *model.Task  `json:"task"`
	Terminal   taskTerminal `json:"terminal"`
	WorkDir    string       `json:"work_dir"`
	CLIStarted bool         `json:"cli_started"`
	Error      string       `json:"error,omitempty"`
}

// StartTask 启动自动化任务
func (s *AutomationService) StartTask(task *model.Task) (*StartTaskResult, error) {
	result := &StartTaskResult{Task: task}
	if task == nil {
		result.Error = "Task is nil"
		return result, errors.New("task is nil")
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

	// 1. 处理工作目录
	workDir := task.WorkDir
	if workDir == "" {
		workDir = "/tmp/tasks/" + task.ID
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
	model.DB.Model(task).Updates(map[string]interface{}{
		"status":     "in_progress",
		"updated_at": time.Now(),
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

	// 5. 如果有初始提示或AI托管模式，等待 CLI 启动后输入
	promptToSend := task.InitialPrompt
	if task.AIManaged {
		promptToSend = buildManagedPrompt(task)
	}

	if promptToSend != "" {
		// 使用detector检测CLI是否准备好
		maxWait := 30 * time.Second
		checkInterval := 500 * time.Millisecond
		startTime := time.Now()
		cliReady := false

		utils.Info("Waiting for CLI ready", zap.String("task_id", task.ID), zap.String("prompt_len", fmt.Sprintf("%d", len(promptToSend))))

		for time.Since(startTime) < maxWait {
			sleep(checkInterval)
			// 检查终端元数据中的AI状态
			if termSession, ok := session.(*terminal.Session); ok {
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
			} else {
				utils.Warn("Type assertion failed for terminal.Session")
			}
		}

		if !cliReady {
			utils.Warn("CLI ready timeout, sending prompt anyway")
			if termSession, ok := session.(*terminal.Session); ok {
				termSession.BroadcastAILog("info", "CLI就绪检测超时，继续发送提示")
			}
		} else {
			if termSession, ok := session.(*terminal.Session); ok {
				termSession.BroadcastAILog("info", "CLI已就绪，准备发送提示")
			}
		}

		// 获取 tmux 会话名称
		var tmuxSession string
		if termSession, ok := session.(*terminal.Session); ok {
			if meta := termSession.Metadata(); meta != nil {
				tmuxSession = meta.TmuxSession
			}
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

	utils.Info("Task automation started",
		zap.String("task_id", task.ID),
		zap.String("terminal_id", session.ID()),
		zap.String("cli_type", task.CLIType),
		zap.String("work_dir", workDir))

	// 7. 启动后台任务监控
	go s.monitorTaskCompletion(task.ID, session.ID())

	return result, nil
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
				// AI托管模式下，暂停任务等待用户干预
				s.updateTaskStatus(taskID, "paused", decision.Reason)
				s.createTaskMessage(taskID, terminalID, "approval_needed", "任务需要用户干预", decision.Reason)
				return
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
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == "done" {
		updates["completed_at"] = time.Now()
	}
	model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(updates)
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
