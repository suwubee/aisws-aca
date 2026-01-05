package terminal

import (
	"sync"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 终端管理器
type Manager struct {
	sessions sync.Map
	config   *config.TerminalConfig
}

// NewManager 创建管理器
func NewManager(cfg *config.TerminalConfig) *Manager {
	m := &Manager{
		config: cfg,
	}

	// 启动空闲会话清理
	go m.reapIdleSessions()

	return m
}

// CreateSession 创建会话
func (m *Manager) CreateSession(title string, taskID *string) (*Session, error) {
	id := uuid.New().String()
	session := NewSession(id, m.config.DefaultShell, m.config.ScrollbackBytes)

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

// GetSession 获取会话
func (m *Manager) GetSession(id string) *Session {
	if v, ok := m.sessions.Load(id); ok {
		return v.(*Session)
	}
	return nil
}

// CloseSession 关闭会话
func (m *Manager) CloseSession(id string) error {
	session := m.GetSession(id)
	if session == nil {
		return nil
	}

	if err := session.Close(); err != nil {
		return err
	}

	m.sessions.Delete(id)

	// 更新数据库
	now := time.Now()
	model.DB.Model(&model.TerminalSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    "exited",
		"closed_at": now,
	})

	utils.Info("Closed terminal session", zap.String("id", id))
	return nil
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
