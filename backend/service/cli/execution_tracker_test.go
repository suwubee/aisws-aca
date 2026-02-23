package cli

import (
	"fmt"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

func initExecutionTrackerDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:cli_execution_tracker_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
}

func TestExecutionTracker_StartAppendComplete(t *testing.T) {
	initExecutionTrackerDB(t)

	tracker := NewExecutionTracker(model.DB)
	if tracker == nil {
		t.Fatalf("expected tracker")
	}

	taskID := "task-1"
	terminalID := "term-1"
	rec, err := tracker.Start(StartExecutionInput{
		TaskID:     &taskID,
		TerminalID: &terminalID,
		Tool:       "codex",
		Mode:       "execute",
		Source:     "workflow",
		Prompt:     "implement feature X",
		Metadata: map[string]any{
			"server_id": "srv-1",
		},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if rec.ID == "" {
		t.Fatalf("expected execution ID")
	}
	if rec.Status != StatusRunning {
		t.Fatalf("expected status %q, got %q", StatusRunning, rec.Status)
	}

	if err := tracker.AppendEvent(rec.ID, EventTypeOutput, map[string]any{"chunk": "hello"}); err != nil {
		t.Fatalf("AppendEvent(output) failed: %v", err)
	}
	if err := tracker.AppendEvent(rec.ID, EventTypeCompleted, map[string]any{"ok": true}); err != nil {
		t.Fatalf("AppendEvent(completed) failed: %v", err)
	}

	exitCode := 0
	if err := tracker.Complete(rec.ID, StatusCompleted, &exitCode, ""); err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	var stored model.CLIExecution
	if err := model.DB.First(&stored, "id = ?", rec.ID).Error; err != nil {
		t.Fatalf("query execution failed: %v", err)
	}
	if stored.Status != StatusCompleted {
		t.Fatalf("expected final status %q, got %q", StatusCompleted, stored.Status)
	}
	if stored.CompletedAt == nil {
		t.Fatalf("expected CompletedAt set")
	}

	events, err := ListExecutionEvents(rec.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListExecutionEvents() failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].EventType != EventTypeOutput {
		t.Fatalf("expected first event %q, got %q", EventTypeOutput, events[0].EventType)
	}
}

func TestListExecutions_FilterByStatus(t *testing.T) {
	initExecutionTrackerDB(t)
	tracker := NewExecutionTracker(model.DB)

	_, _ = tracker.Start(StartExecutionInput{Tool: "shell", Mode: "command", Source: "workflow", Prompt: "echo 1"})
	done, _ := tracker.Start(StartExecutionInput{Tool: "shell", Mode: "command", Source: "workflow", Prompt: "echo 2"})
	exitCode := 0
	_ = tracker.Complete(done.ID, StatusCompleted, &exitCode, "")

	running, err := ListExecutions(StatusRunning, 50)
	if err != nil {
		t.Fatalf("ListExecutions(running) failed: %v", err)
	}
	if len(running) != 1 {
		t.Fatalf("expected 1 running execution, got %d", len(running))
	}

	all, err := ListExecutions("", 50)
	if err != nil {
		t.Fatalf("ListExecutions(all) failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 executions total, got %d", len(all))
	}
}

func TestListExecutionsByFilter_ParentAndRole(t *testing.T) {
	initExecutionTrackerDB(t)
	tracker := NewExecutionTracker(model.DB)
	if tracker == nil {
		t.Fatalf("expected tracker")
	}

	parent, err := tracker.Start(StartExecutionInput{
		Tool:   "codex",
		Mode:   "execute",
		Source: "workflow",
		Prompt: "run primary",
	})
	if err != nil {
		t.Fatalf("Start(parent) failed: %v", err)
	}

	parentID := parent.ID
	child, err := tracker.Start(StartExecutionInput{
		ParentExecutionID: &parentID,
		Role:              ExecutionRoleReview,
		Tool:              "codex",
		Mode:              "review",
		Source:            "workflow-review",
		Prompt:            "run review",
	})
	if err != nil {
		t.Fatalf("Start(child) failed: %v", err)
	}

	rows, err := ListExecutionsByFilter(ListExecutionsInput{
		ParentExecutionID: parent.ID,
		Role:              ExecutionRoleReview,
		Limit:             20,
	})
	if err != nil {
		t.Fatalf("ListExecutionsByFilter() failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 child execution, got %d", len(rows))
	}
	if rows[0].ID != child.ID {
		t.Fatalf("expected child id %q, got %q", child.ID, rows[0].ID)
	}
	if rows[0].ParentExecutionID == nil || *rows[0].ParentExecutionID != parent.ID {
		t.Fatalf("expected parent id %q, got %v", parent.ID, rows[0].ParentExecutionID)
	}
	if rows[0].Role != ExecutionRoleReview {
		t.Fatalf("expected role %q, got %q", ExecutionRoleReview, rows[0].Role)
	}
}

func TestListExecutionsByFilter_ModeSourceTool(t *testing.T) {
	initExecutionTrackerDB(t)
	tracker := NewExecutionTracker(model.DB)
	if tracker == nil {
		t.Fatalf("expected tracker")
	}

	_, err := tracker.Start(StartExecutionInput{
		Tool:   "codex",
		Mode:   "execute",
		Source: "workflow",
		Prompt: "primary command",
	})
	if err != nil {
		t.Fatalf("Start(primary) failed: %v", err)
	}

	target, err := tracker.Start(StartExecutionInput{
		Tool:   "claude",
		Mode:   "review",
		Source: "workflow-review",
		Prompt: "review command",
	})
	if err != nil {
		t.Fatalf("Start(target) failed: %v", err)
	}

	rows, err := ListExecutionsByFilter(ListExecutionsInput{
		Mode:   "review",
		Source: "workflow-review",
		Tool:   "claude",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListExecutionsByFilter(mode/source/tool) failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 filtered execution, got %d", len(rows))
	}
	if rows[0].ID != target.ID {
		t.Fatalf("expected filtered execution %q, got %q", target.ID, rows[0].ID)
	}
}
