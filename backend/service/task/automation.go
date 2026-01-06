package task

import (
	"fmt"
	"os"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/utils"
	"go.uber.org/zap"
)

// AutomationService 任务自动化服务
type AutomationService struct {
	terminalManager *terminal.Manager
}

// NewAutomationService 创建自动化服务
func NewAutomationService(tm *terminal.Manager) *AutomationService {
	return &AutomationService{
		terminalManager: tm,
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

// StartTaskResult 启动任务结果
type StartTaskResult struct {
	Task       *model.Task              `json:"task"`
	Terminal   *terminal.Session        `json:"terminal"`
	WorkDir    string                   `json:"work_dir"`
	CLIStarted bool                     `json:"cli_started"`
	Error      string                   `json:"error,omitempty"`
}

// StartTask 启动自动化任务
func (s *AutomationService) StartTask(task *model.Task) (*StartTaskResult, error) {
	result := &StartTaskResult{Task: task}

	// 1. 处理工作目录
	workDir := task.WorkDir
	if workDir == "" {
		workDir = "/tmp/tasks/" + task.ID
	}
	result.WorkDir = workDir

	// 创建目录（如果需要）
	if task.AutoCreateDir {
		if err := os.MkdirAll(workDir, 0755); err != nil {
			result.Error = fmt.Sprintf("Failed to create work directory: %v", err)
			return result, err
		}
		utils.Info("Created work directory", zap.String("path", workDir))
	}

	// 2. 创建终端会话
	terminalTitle := fmt.Sprintf("[%s] %s", task.CLIType, task.Title)
	session, err := s.terminalManager.CreateSession(terminalTitle, &task.ID)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create terminal: %v", err)
		return result, err
	}
	result.Terminal = session

	// 3. 进入工作目录
	cdCmd := fmt.Sprintf("cd %s\r", workDir)
	if err := session.Write([]byte(cdCmd)); err != nil {
		result.Error = fmt.Sprintf("Failed to change directory: %v", err)
		return result, err
	}

	// 等待命令执行
	time.Sleep(300 * time.Millisecond)

	// 4. 启动 CLI
	cliConfig := GetCLIConfig(task.CLIType)
	cliCmd := cliConfig.Command + "\r"
	if err := session.Write([]byte(cliCmd)); err != nil {
		result.Error = fmt.Sprintf("Failed to start CLI: %v", err)
		return result, err
	}
	result.CLIStarted = true

	// 5. 如果有初始提示，等待 CLI 启动后输入
	if task.InitialPrompt != "" {
		// 等待 CLI 启动
		time.Sleep(2 * time.Second)

		// 输入初始提示
		promptCmd := task.InitialPrompt + "\r"
		if err := session.Write([]byte(promptCmd)); err != nil {
			utils.Warn("Failed to send initial prompt", zap.Error(err))
		}
	}

	// 6. 更新任务状态
	model.DB.Model(task).Updates(map[string]interface{}{
		"status":     "in_progress",
		"updated_at": time.Now(),
	})

	utils.Info("Task automation started",
		zap.String("task_id", task.ID),
		zap.String("terminal_id", session.ID()),
		zap.String("cli_type", task.CLIType),
		zap.String("work_dir", workDir))

	return result, nil
}
