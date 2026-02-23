package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/ai-coding-assistant/model"
)

func TestShouldTreatChatErrorAsPause(t *testing.T) {
	t.Run("returns false when pause not requested", func(t *testing.T) {
		ok, reason := shouldTreatChatErrorAsPause(context.Background(), &aiWorkflowInflight{}, errors.New("context canceled"))
		if ok {
			t.Fatalf("expected false, got true")
		}
		if reason != "" {
			t.Fatalf("expected empty reason, got %q", reason)
		}
	})

	t.Run("returns true when pause requested and context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		state := &aiWorkflowInflight{}
		state.requestPause("")
		ok, reason := shouldTreatChatErrorAsPause(ctx, state, errors.New("send request: context canceled"))
		if !ok {
			t.Fatalf("expected true, got false")
		}
		if reason != "用户手动暂停" {
			t.Fatalf("expected default pause reason, got %q", reason)
		}
	})

	t.Run("returns true when pause requested and cancellation-like error", func(t *testing.T) {
		state := &aiWorkflowInflight{}
		state.requestPause("暂停复核")
		ok, reason := shouldTreatChatErrorAsPause(context.Background(), state, errors.New("request cancelled by client"))
		if !ok {
			t.Fatalf("expected true, got false")
		}
		if reason != "暂停复核" {
			t.Fatalf("expected custom pause reason, got %q", reason)
		}
	})

	t.Run("returns false for non-cancel error even if pause requested", func(t *testing.T) {
		state := &aiWorkflowInflight{}
		state.requestPause("暂停复核")
		ok, reason := shouldTreatChatErrorAsPause(context.Background(), state, errors.New("api error: 500"))
		if ok {
			t.Fatalf("expected false, got true")
		}
		if reason != "" {
			t.Fatalf("expected empty reason, got %q", reason)
		}
	})
}

func TestIsContextCancelErr(t *testing.T) {
	if !isContextCancelErr(context.Canceled) {
		t.Fatalf("expected context.Canceled to be detected")
	}
	if !isContextCancelErr(context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded to be detected")
	}
	if !isContextCancelErr(errors.New("context cancelled by upstream")) {
		t.Fatalf("expected wrapped cancel text to be detected")
	}
	if isContextCancelErr(errors.New("network error")) {
		t.Fatalf("did not expect non-cancel error to be detected")
	}
}

func TestAppendNativeConversationLog(t *testing.T) {
	originalDB := model.DB
	defer func() { model.DB = originalDB }()

	if err := model.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}

	engine := &AIWorkflowEngine{}
	session := &AIWorkflowSession{
		WorkflowID: "task-1",
		Context: map[string]any{
			"terminal_id": "terminal-1",
			"task_id":     "task-1",
		},
	}

	engine.appendNativeConversationLog(session, "ai_input_native", "hello")
	engine.appendNativeConversationLog(session, "ai_input_native", "hello") // duplicate in time window should be ignored
	engine.appendNativeConversationLog(session, "ai_output_native", "world")
	engine.appendNativeConversationLog(session, "system", "ignored")

	var logs []model.Log
	if err := model.DB.Order("created_at asc").Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].LogType != "ai_input_native" {
		t.Fatalf("expected first log type ai_input_native, got %q", logs[0].LogType)
	}
	if logs[1].LogType != "ai_output_native" {
		t.Fatalf("expected second log type ai_output_native, got %q", logs[1].LogType)
	}
}
