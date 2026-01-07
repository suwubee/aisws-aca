package terminal

import (
	"fmt"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

func TestManagerCloseSession_AutoCompletesInProgressTask(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_close_task_complete_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	now := time.Now()
	task := &model.Task{
		ID:        "task-1",
		UserID:    "u1",
		Title:     "Task 1",
		Status:    "in_progress",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	session := NewSession("term-1", "/bin/bash", 1024)
	t.Cleanup(func() { _ = session.Close() })
	taskID := task.ID
	session.SetTaskID(&taskID)

	manager := &Manager{}
	manager.sessions.Store(session.ID(), session)

	if err := manager.CloseSession(session.ID()); err != nil {
		t.Fatalf("CloseSession() error: %v", err)
	}

	var updated model.Task
	if err := model.DB.First(&updated, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if updated.Status != "done" {
		t.Fatalf("expected task status %q, got %q", "done", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}
}

func TestManagerCloseSession_DoesNotCompleteNonInProgressTask(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_close_task_noop_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	now := time.Now()
	task := &model.Task{
		ID:        "task-2",
		UserID:    "u1",
		Title:     "Task 2",
		Status:    "todo",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	session := NewSession("term-2", "/bin/bash", 1024)
	t.Cleanup(func() { _ = session.Close() })
	taskID := task.ID
	session.SetTaskID(&taskID)

	manager := &Manager{}
	manager.sessions.Store(session.ID(), session)

	if err := manager.CloseSession(session.ID()); err != nil {
		t.Fatalf("CloseSession() error: %v", err)
	}

	var updated model.Task
	if err := model.DB.First(&updated, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("query task failed: %v", err)
	}
	if updated.Status != "todo" {
		t.Fatalf("expected task status %q, got %q", "todo", updated.Status)
	}
	if updated.CompletedAt != nil {
		t.Fatalf("expected CompletedAt to be nil")
	}
}

func TestManagerCloseSession_IgnoresMissingTask(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_close_task_missing_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	session := NewSession("term-3", "/bin/bash", 1024)
	t.Cleanup(func() { _ = session.Close() })
	missingTaskID := "missing-task"
	session.SetTaskID(&missingTaskID)

	manager := &Manager{}
	manager.sessions.Store(session.ID(), session)

	if err := manager.CloseSession(session.ID()); err != nil {
		t.Fatalf("CloseSession() error: %v", err)
	}
}

