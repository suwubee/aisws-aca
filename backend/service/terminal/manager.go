package terminal

import (
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/appsetting"
	sshservice "github.com/ai-coding-assistant/service/ssh"
	"github.com/ai-coding-assistant/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

// Manager 终端管理器
type Manager struct {
	sessions sync.Map
	config   *config.TerminalConfig

	sshMu      sync.Mutex
	sshManager sshSessionProvider
}

type sshSessionProvider interface {
	GetSession(serverID string) (*cryptossh.Session, error)
}

// NewManager 创建管理器
func NewManager(cfg *config.TerminalConfig) *Manager {
	m := &Manager{
		config: cfg,
	}

	// 恢复重启前仍标记为 running 的会话
	if err := m.RecoverSessions(); err != nil {
		utils.Warn("Failed to recover terminal sessions", zap.Error(err))
	}

	// 启动空闲会话清理
	go m.reapIdleSessions()

	return m
}

// RecoverSessions 恢复数据库中仍标记为 running 的会话
func (m *Manager) RecoverSessions() error {
	var dbSessions []model.TerminalSession
	if err := model.DB.Where("status = ?", "running").Find(&dbSessions).Error; err != nil {
		return err
	}

	for _, dbSession := range dbSessions {
		sessionID := dbSession.ID
		if sessionID == "" {
			continue
		}
		if _, ok := m.sessions.Load(sessionID); ok {
			continue
		}

		checkCmd := execCommand("tmux", "has-session", "-t", sessionID)
		if err := checkCmd.Run(); err != nil {
			now := time.Now()
			if err := model.DB.Model(&model.TerminalSession{}).Where("id = ?", sessionID).Updates(map[string]interface{}{
				"status":    "exited",
				"closed_at": now,
			}).Error; err != nil {
				utils.Warn("Failed to update session status to exited", zap.String("id", sessionID), zap.Error(err))
			}
			continue
		}

		session := NewSession(sessionID, m.config.DefaultShell, m.config.ScrollbackBytes)
		session.SetTitle(dbSession.Title)
		if dbSession.TaskID != nil {
			session.SetTaskID(dbSession.TaskID)
		}

		if err := session.RecoverFromTmux(); err != nil {
			utils.Warn("Failed to recover session from tmux", zap.String("id", sessionID), zap.Error(err))
			continue
		}

		m.sessions.Store(sessionID, session)
		utils.Info("Recovered terminal session", zap.String("id", sessionID))
	}

	return nil
}

func (m *Manager) resolveStartDir(taskID *string) string {
	candidates := make([]string, 0, 4)

	if taskID != nil {
		if model.DB != nil {
			taskIDValue := strings.TrimSpace(*taskID)
			if taskIDValue != "" {
				var task model.Task
				if err := model.DB.Select("work_dir").First(&task, "id = ?", taskIDValue).Error; err == nil {
					if strings.TrimSpace(task.WorkDir) != "" {
						candidates = append(candidates, task.WorkDir)
					}
				}
			}
		}
	}

	if dir, err := appsetting.GetTerminalDefaultLoginDir(); err == nil && strings.TrimSpace(dir) != "" {
		candidates = append(candidates, dir)
	}
	if m.config != nil && strings.TrimSpace(m.config.DefaultLoginDir) != "" {
		candidates = append(candidates, m.config.DefaultLoginDir)
	}
	candidates = append(candidates, "~/")

	return resolveExistingWorkDir(candidates...)
}

// CreateSession 创建会话
func (m *Manager) CreateSession(title string, taskID *string) (*Session, error) {
	id := uuid.New().String()
	session := NewSession(id, m.config.DefaultShell, m.config.ScrollbackBytes)
	session.SetStartDir(m.resolveStartDir(taskID))

	if title != "" {
		session.SetTitle(title)
	}
	if taskID != nil {
		session.SetTaskID(taskID)
	}

	if err := session.Start(); err != nil {
		return nil, err
	}

	m.sessions.Store(id, session)

	// 保存到数据库
	dbSession := session.ToDBModel()
	if err := model.DB.Create(dbSession).Error; err != nil {
		utils.Warn("Failed to save session to database", zap.Error(err))
	}

	utils.Info("Created terminal session",
		zap.String("id", id),
		zap.String("shell", m.config.DefaultShell),
	)

	return session, nil
}

func (m *Manager) getSSHManager() (sshSessionProvider, error) {
	m.sshMu.Lock()
	defer m.sshMu.Unlock()

	if m.sshManager != nil {
		return m.sshManager, nil
	}

	cfg := config.Load()
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, errors.New("missing ssh master key")
	}

	m.sshManager = sshservice.NewSSHManager(cfg.Auth.JWTSecret)
	return m.sshManager, nil
}

// CreateSSHSession 创建SSH终端会话，并注册到 sessions map。
func (m *Manager) CreateSSHSession(serverID string) (*Session, error) {
	id := uuid.New().String()
	session := NewSession(id, "ssh", m.config.ScrollbackBytes)

	serverName := strings.TrimSpace(serverID)
	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", serverID).Error; err == nil {
		if strings.TrimSpace(server.Name) != "" {
			serverName = server.Name
		}
	}
	if serverName != "" {
		session.SetTitle("SSH: " + serverName)
	} else {
		session.SetTitle("SSH")
	}

	sshManager, err := m.getSSHManager()
	if err != nil {
		return nil, err
	}

	sshSession, err := sshManager.GetSession(serverID)
	if err != nil {
		return nil, err
	}
	if sshSession == nil {
		return nil, errors.New("failed to create ssh session")
	}

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		_ = sshSession.Close()
		return nil, err
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = sshSession.Close()
		return nil, err
	}

	sshSession.Stdin = stdinReader
	sshSession.Stdout = stdoutWriter
	sshSession.Stderr = stdoutWriter

	adapter := sshservice.NewSSHTerminalSession(sshSession, stdinReader, stdinWriter, stdoutReader, stdoutWriter)

	if err := sshSession.RequestPty("xterm-256color", 30, 120, cryptossh.TerminalModes{
		cryptossh.ECHO:          1,
		cryptossh.TTY_OP_ISPEED: 14400,
		cryptossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = adapter.Close()
		return nil, err
	}

	if err := sshSession.Shell(); err != nil {
		_ = adapter.Close()
		return nil, err
	}

	session.backend = adapter
	session.pty = adapter.StdoutPipe()
	session.metadata.PID = 0

	session.loadAutomationConfig()
	go session.readPTY()
	go session.flushLogs()
	go m.waitSSHSession(session, adapter)

	m.sessions.Store(id, session)

	// 保存到数据库（用于日志关联等），但不支持重启后恢复 SSH 会话。
	dbSession := session.ToDBModel()
	if err := model.DB.Create(dbSession).Error; err != nil {
		utils.Warn("Failed to save ssh session to database", zap.Error(err))
	}

	utils.Info("Created ssh terminal session",
		zap.String("id", id),
		zap.String("server_id", serverID),
	)

	return session, nil
}

func (m *Manager) waitSSHSession(session *Session, adapter *sshservice.SSHTerminalSession) {
	err := adapter.Session.Wait()

	exitCode := 0
	message := "SSH session exited"

	if err != nil {
		var exitErr *cryptossh.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitStatus()
		} else {
			var missing *cryptossh.ExitMissingError
			if errors.As(err, &missing) {
				exitCode = 0
			} else {
				exitCode = -1
				message = err.Error()
			}
		}
	}

	// 确保关闭管道，解除 readPTY 阻塞
	_ = adapter.Close()

	session.metaMutex.Lock()
	session.status = "exited"
	session.metadata.Status = "exited"
	now := time.Now()
	session.closedAt = &now
	session.metaMutex.Unlock()

	session.broadcast(StreamEvent{
		Type:     StreamEventExit,
		ExitCode: exitCode,
		Message:  message,
	})

	session.doneOnce.Do(func() { close(session.done) })
}

// GetSession 获取会话
func (m *Manager) GetSession(id string) *Session {
	if v, ok := m.sessions.Load(id); ok {
		return v.(*Session)
	}
	return nil
}

// GetOrResumeSession 获取或恢复会话
func (m *Manager) GetOrResumeSession(id string) (*Session, error) {
	// 先检查内存中是否有
	if session := m.GetSession(id); session != nil {
		return session, nil
	}

	// 检查数据库中是否有这个会话
	var dbSession model.TerminalSession
	if err := model.DB.First(&dbSession, "id = ?", id).Error; err != nil {
		return nil, err
	}

	// 检查 tmux 会话是否存在
	checkCmd := execCommand("tmux", "has-session", "-t", id)
	if checkCmd.Run() != nil {
		// tmux 会话不存在
		return nil, nil
	}

	// 恢复会话
	session := NewSession(id, m.config.DefaultShell, m.config.ScrollbackBytes)
	session.SetTitle(dbSession.Title)
	if dbSession.TaskID != nil {
		session.SetTaskID(dbSession.TaskID)
	}

	// 使用 attach 模式启动
	if err := session.RecoverFromTmux(); err != nil {
		return nil, err
	}

	m.sessions.Store(id, session)
	utils.Info("Resumed terminal session", zap.String("id", id))

	return session, nil
}

// CloseSession 关闭会话
func (m *Manager) CloseSession(id string) error {
	session := m.GetSession(id)
	if session == nil {
		return nil
	}

	taskID := session.Metadata().TaskID

	if err := session.Close(); err != nil {
		return err
	}

	m.sessions.Delete(id)

	// 更新数据库
	now := time.Now()
	if model.DB != nil {
		if err := model.DB.Model(&model.TerminalSession{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":    "exited",
			"closed_at": now,
		}).Error; err != nil {
			utils.Warn("Failed to update session status to exited", zap.String("id", id), zap.Error(err))
		}
	}

	m.completeTaskIfNeeded(taskID, id)

	utils.Info("Closed terminal session", zap.String("id", id))
	return nil
}

// CloseAllSessions 关闭所有会话（用于重置数据等管理操作）
func (m *Manager) CloseAllSessions() error {
	var ids []string
	m.sessions.Range(func(key, _ interface{}) bool {
		if id, ok := key.(string); ok && strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
		return true
	})

	var firstErr error
	for _, id := range ids {
		if err := m.CloseSession(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (m *Manager) completeTaskIfNeeded(taskID *string, terminalID string) {
	if taskID == nil {
		return
	}

	id := strings.TrimSpace(*taskID)
	if id == "" {
		return
	}

	if model.DB == nil {
		utils.Warn("Skip task auto-complete: database not initialized", zap.String("task_id", id), zap.String("terminal_id", terminalID))
		return
	}

	now := time.Now()
	result := model.DB.Model(&model.Task{}).Where("id = ? AND status = ?", id, "in_progress").Updates(map[string]interface{}{
		"status":       "done",
		"completed_at": now,
		"updated_at":   now,
	})
	if result.Error != nil {
		utils.Warn("Failed to auto-complete task on terminal close", zap.String("task_id", id), zap.String("terminal_id", terminalID), zap.Error(result.Error))
		return
	}

	if result.RowsAffected == 0 {
		return
	}

	utils.Info("Task status updated on terminal close",
		zap.String("task_id", id),
		zap.String("terminal_id", terminalID),
		zap.String("from", "in_progress"),
		zap.String("to", "done"),
	)
}

// RenameSession 重命名会话
func (m *Manager) RenameSession(id, title string) error {
	session := m.GetSession(id)
	if session == nil {
		return nil
	}

	session.SetTitle(title)
	model.DB.Model(&model.TerminalSession{}).Where("id = ?", id).Update("title", title)
	return nil
}

// LinkTask 关联任务
func (m *Manager) LinkTask(id string, taskID *string) error {
	session := m.GetSession(id)
	if session == nil {
		return nil
	}

	session.SetTaskID(taskID)
	model.DB.Model(&model.TerminalSession{}).Where("id = ?", id).Update("task_id", taskID)
	_ = session.RefreshAutomationConfig()
	session.ReevaluateApprovalIfWaiting()
	return nil
}

// ListSessions 列出会话
func (m *Manager) ListSessions(taskID *string) []*Session {
	var sessions []*Session

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		if taskID == nil || (session.TaskID() != nil && *session.TaskID() == *taskID) {
			sessions = append(sessions, session)
		}
		return true
	})

	return sessions
}

// ListAllSessions 列出所有会话
func (m *Manager) ListAllSessions() []*Session {
	var sessions []*Session

	m.sessions.Range(func(key, value interface{}) bool {
		sessions = append(sessions, value.(*Session))
		return true
	})

	return sessions
}

// reapIdleSessions 清理空闲会话
func (m *Manager) reapIdleSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.sessions.Range(func(key, value interface{}) bool {
			session := value.(*Session)
			if session.Status() == "exited" {
				m.sessions.Delete(key)
				utils.Debug("Reaped exited session", zap.String("id", session.ID()))
			}
			return true
		})
	}
}

// GetSessionCount 获取会话数量
func (m *Manager) GetSessionCount() int {
	count := 0
	m.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
