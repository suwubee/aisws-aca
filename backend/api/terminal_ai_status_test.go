package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
)

func TestTerminalController_UpdateConnectionStatus_TaskAIStatus(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_ai_status_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	taskID := uuid.NewString()
	terminalID := uuid.NewString()
	now := time.Now()

	activeTerminalID := terminalID
	task := &model.Task{
		ID:               taskID,
		UserID:           uuid.NewString(),
		Title:            "task",
		Status:           "in_progress",
		ActiveTerminalID: &activeTerminalID,
		AIStatus:         "paused",
		AIPauseReason:    "user_paused",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if err := model.DB.Create(&model.TerminalSession{
		ID:        terminalID,
		Title:     "term",
		TaskID:    &taskID,
		Status:    "running",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create terminal session failed: %v", err)
	}

	ctrl := &TerminalController{}

	t.Run("connected_does_not_resume_user_paused", func(t *testing.T) {
		ctrl.updateConnectionStatus(terminalID, "connected", "")

		var got model.Task
		if err := model.DB.First(&got, "id = ?", taskID).Error; err != nil {
			t.Fatalf("query task failed: %v", err)
		}
		if got.AIStatus != "paused" {
			t.Fatalf("expected ai_status %q, got %q", "paused", got.AIStatus)
		}
		if got.AIPauseReason != "user_paused" {
			t.Fatalf("expected ai_pause_reason %q, got %q", "user_paused", got.AIPauseReason)
		}
	})

	t.Run("connected_resumes_terminal_disconnected", func(t *testing.T) {
		if err := model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
			"ai_status":       "paused",
			"ai_pause_reason": "terminal_disconnected",
			"updated_at":      time.Now(),
		}).Error; err != nil {
			t.Fatalf("update task failed: %v", err)
		}

		ctrl.updateConnectionStatus(terminalID, "connected", "")

		var got model.Task
		if err := model.DB.First(&got, "id = ?", taskID).Error; err != nil {
			t.Fatalf("query task failed: %v", err)
		}
		if got.AIStatus != "running" {
			t.Fatalf("expected ai_status %q, got %q", "running", got.AIStatus)
		}
		if got.AIPauseReason != "" {
			t.Fatalf("expected ai_pause_reason to be empty, got %q", got.AIPauseReason)
		}
	})

	t.Run("disconnected_pauses_running", func(t *testing.T) {
		if err := model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
			"ai_status":       "running",
			"ai_pause_reason": "",
			"updated_at":      time.Now(),
		}).Error; err != nil {
			t.Fatalf("update task failed: %v", err)
		}

		ctrl.updateConnectionStatus(terminalID, "disconnected", "websocket_closed")

		var got model.Task
		if err := model.DB.First(&got, "id = ?", taskID).Error; err != nil {
			t.Fatalf("query task failed: %v", err)
		}
		if got.AIStatus != "paused" {
			t.Fatalf("expected ai_status %q, got %q", "paused", got.AIStatus)
		}
		if got.AIPauseReason != "terminal_disconnected" {
			t.Fatalf("expected ai_pause_reason %q, got %q", "terminal_disconnected", got.AIPauseReason)
		}
	})
}
