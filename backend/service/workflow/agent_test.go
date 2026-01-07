package workflow

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkflowAgent_DetectTaskCompletion(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	completed, pattern := agent.detectTaskCompletion("Task completed successfully")
	if !completed {
		t.Fatalf("expected completion to be detected")
	}
	if strings.TrimSpace(pattern) == "" {
		t.Fatalf("expected matched pattern to be returned")
	}

	completed, _ = agent.detectTaskCompletion("still working")
	if completed {
		t.Fatalf("expected completion to be false")
	}
}

func TestWorkflowAgent_DetectTaskFailure(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	failed, pattern := agent.detectTaskFailure("Error: something went wrong")
	if !failed {
		t.Fatalf("expected failure to be detected")
	}
	if strings.TrimSpace(pattern) == "" {
		t.Fatalf("expected matched pattern to be returned")
	}

	failed, _ = agent.detectTaskFailure("all good")
	if failed {
		t.Fatalf("expected failure to be false")
	}
}

func TestWorkflowAgent_DetectApprovalNeeded(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	needed, prompt := agent.detectApprovalNeeded("Proceed? (y/n)")
	if !needed {
		t.Fatalf("expected approval to be detected")
	}
	if strings.TrimSpace(prompt) == "" {
		t.Fatalf("expected approval prompt context to be returned")
	}

	needed, _ = agent.detectApprovalNeeded("no prompts here")
	if needed {
		t.Fatalf("expected approval to be false")
	}
}

func TestWorkflowAgent_ShouldStartNextNode_CompletionWinsIfLater(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	decision := agent.shouldStartNextNode("Error: transient\nTask completed", "", "")
	if decision.Action != AgentActionAdvance {
		t.Fatalf("expected action %q, got %q", AgentActionAdvance, decision.Action)
	}
}

func TestWorkflowAgent_ShouldStartNextNode_FailureWinsIfLater(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	decision := agent.shouldStartNextNode("Task completed\nError: hard failure", "", "")
	if decision.Action != AgentActionFail {
		t.Fatalf("expected action %q, got %q", AgentActionFail, decision.Action)
	}
}

func TestWorkflowAgent_MonitorTask_AdvancesOnCompletion(t *testing.T) {
	agent := NewWorkflowAgent(nil)
	agent.sleep = func(time.Duration) {}
	agent.pollInterval = 1 * time.Millisecond
	agent.maxWait = 250 * time.Millisecond
	agent.loadTerminalLogs = func(context.Context, string, int) (string, error) {
		return "Task completed", nil
	}
	agent.loadTaskStatus = func(context.Context, string) (string, error) { return "", nil }
	agent.loadTerminalStatus = func(context.Context, string) (string, error) { return "", nil }

	decision, err := agent.MonitorTask(context.Background(), "run-1", "node-1", "task", "task-1", "term-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if decision == nil || decision.Action != AgentActionAdvance {
		t.Fatalf("expected advance decision, got %#v", decision)
	}
	if agent.Status() != AgentStatusIdle {
		t.Fatalf("expected agent status idle, got %q", agent.Status())
	}
}

func TestWorkflowAgent_MonitorTask_FailsOnFailure(t *testing.T) {
	agent := NewWorkflowAgent(nil)
	agent.sleep = func(time.Duration) {}
	agent.pollInterval = 1 * time.Millisecond
	agent.maxWait = 250 * time.Millisecond
	agent.loadTerminalLogs = func(context.Context, string, int) (string, error) {
		return "Error: boom", nil
	}
	agent.loadTaskStatus = func(context.Context, string) (string, error) { return "", nil }
	agent.loadTerminalStatus = func(context.Context, string) (string, error) { return "", nil }

	decision, err := agent.MonitorTask(context.Background(), "run-1", "node-1", "task", "task-1", "term-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if decision == nil || decision.Action != AgentActionFail {
		t.Fatalf("expected fail decision, got %#v", decision)
	}
}

func TestWorkflowAgent_MonitorTask_TimesOut(t *testing.T) {
	agent := NewWorkflowAgent(nil)
	agent.sleep = func(time.Duration) {}
	agent.pollInterval = 1 * time.Millisecond
	agent.maxWait = 1 * time.Second

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowCalls := 0
	agent.now = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return start
		}
		return start.Add(2 * time.Second)
	}

	agent.loadTerminalLogs = func(context.Context, string, int) (string, error) { return "", nil }
	agent.loadTaskStatus = func(context.Context, string) (string, error) { return "", nil }
	agent.loadTerminalStatus = func(context.Context, string) (string, error) { return "", nil }

	decision, err := agent.MonitorTask(context.Background(), "run-1", "node-1", "task", "task-1", "term-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if decision == nil || decision.Action != AgentActionFail {
		t.Fatalf("expected fail decision, got %#v", decision)
	}
}
