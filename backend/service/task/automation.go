package task

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/utils"
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

	// 5. 如果有初始提示，等待 CLI 启动后输入
	if task.InitialPrompt != "" {
		// 等待 CLI 启动
		sleep(2 * time.Second)

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
