package terminal

import (
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/utils"
	"go.uber.org/zap"
)

const (
	taskAutoReconnectInterval    = 10 * time.Second
	taskAutoReconnectMaxWait     = 5 * time.Minute
	taskAutoReconnectMaxAttempts = 30 // 5min / 10s
)

var taskAutoReconnectInFlight sync.Map // taskID -> struct{}

func (m *Manager) maybeAutoReconnectTaskTerminal(taskID *string, terminalID string) {
	if m == nil || model.DB == nil {
		return
	}

	if taskID == nil {
		return
	}
	tid := strings.TrimSpace(*taskID)
	if tid == "" {
		return
	}

	oldTerminalID := strings.TrimSpace(terminalID)
	if oldTerminalID == "" {
		return
	}

	// Only reconnect when task explicitly expects disconnect and the exiting terminal is the active one.
	var task model.Task
	if err := model.DB.Select("id", "expect_disconnect", "active_terminal_id").
		First(&task, "id = ?", tid).Error; err != nil {
		return
	}
	if !task.ExpectDisconnect {
		return
	}
	if task.ActiveTerminalID == nil || strings.TrimSpace(*task.ActiveTerminalID) != oldTerminalID {
		return
	}

	if _, loaded := taskAutoReconnectInFlight.LoadOrStore(tid, struct{}{}); loaded {
		return
	}

	go func() {
		defer taskAutoReconnectInFlight.Delete(tid)
		m.autoReconnectTaskTerminal(tid, oldTerminalID)
	}()
}

func (m *Manager) autoReconnectTaskTerminal(taskID, oldTerminalID string) {
	if m == nil || model.DB == nil {
		return
	}

	startedAt := time.Now()
	attempt := 0

	for {
		if time.Since(startedAt) > taskAutoReconnectMaxWait || attempt >= taskAutoReconnectMaxAttempts {
			now := time.Now()
			_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
				"expect_disconnect": false,
				"ai_status":         "paused",
				"ai_pause_reason":   "reconnect_timeout",
				"updated_at":        now,
			}).Error
			utils.Warn("Auto reconnect timed out",
				zap.String("task_id", taskID),
				zap.String("terminal_id", oldTerminalID))
			return
		}

		// Re-check state: user might have restarted/bound a different terminal.
		var task model.Task
		if err := model.DB.Select("expect_disconnect", "active_terminal_id").
			First(&task, "id = ?", taskID).Error; err != nil {
			return
		}
		if !task.ExpectDisconnect {
			return
		}
		if task.ActiveTerminalID == nil || strings.TrimSpace(*task.ActiveTerminalID) != oldTerminalID {
			return
		}

		attempt++
		now := time.Now()
		_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
			"reconnect_attempts": attempt,
			"last_reconnect_at":  now,
			"updated_at":         now,
		}).Error

		newSession, err := m.RestartSession(oldTerminalID)
		if err == nil && newSession != nil && strings.TrimSpace(newSession.ID()) != "" {
			now := time.Now()
			_ = model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
				"expect_disconnect": false,
				"ai_status":         "running",
				"ai_pause_reason":   "",
				"last_reconnect_at": now,
				"updated_at":        now,
			}).Error

			utils.Info("Auto reconnected task terminal",
				zap.String("task_id", taskID),
				zap.String("old_terminal_id", oldTerminalID),
				zap.String("new_terminal_id", newSession.ID()),
				zap.Int("attempt", attempt))
			return
		}

		utils.Info("Auto reconnect attempt failed",
			zap.String("task_id", taskID),
			zap.String("terminal_id", oldTerminalID),
			zap.Int("attempt", attempt),
			zap.Error(err))

		time.Sleep(taskAutoReconnectInterval)
	}
}
