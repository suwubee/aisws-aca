package workflow

import (
	"context"
	"errors"
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

func TestWorkflowAgent_AnalyzeTaskProgress_Percent(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	percent, confidence, evidence := agent.analyzeTaskProgress("Working... 10%\nMore work 55%")
	if percent != 55 {
		t.Fatalf("expected percent 55, got %d", percent)
	}
	if confidence <= 0 {
		t.Fatalf("expected confidence > 0, got %f", confidence)
	}
	if strings.TrimSpace(evidence) == "" {
		t.Fatalf("expected evidence to be non-empty")
	}
}

func TestWorkflowAgent_AnalyzeTaskProgress_Fraction(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	percent, _, _ := agent.analyzeTaskProgress("Step 2/4: compiling")
	if percent != 50 {
		t.Fatalf("expected percent 50, got %d", percent)
	}
}

func TestWorkflowAgent_DetectErrors_SuggestsFix(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	insight := agent.detectErrors("permission denied: /root/secret\n")
	if insight == nil {
		t.Fatalf("expected insight")
	}
	if strings.TrimSpace(insight.suggestion) == "" {
		t.Fatalf("expected suggestion")
	}
}

func TestWorkflowAgent_ShouldRetry_Transient(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	retry, _, _ := agent.shouldRetry(errors.New("i/o timeout"))
	if !retry {
		t.Fatalf("expected retry for timeout")
	}

	retry, _, _ = agent.shouldRetry(errors.New("exit status 1"))
	if retry {
		t.Fatalf("expected retry to be false for exit status")
	}
}

func TestWorkflowAgent_GenerateNextStepSuggestion_Approval(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	decision := agent.generateNextStepSuggestion(agentSuggestionContext{
		phase:          suggestionPhaseApproval,
		approvalPrompt: "Proceed? (y/n)",
		logs:           "Proceed? (y/n)",
	})
	if decision == nil {
		t.Fatalf("expected decision")
	}
	if decision.Action != AgentActionWait {
		t.Fatalf("expected action %q, got %q", AgentActionWait, decision.Action)
	}
	if decision.SuggestedInput == "" {
		t.Fatalf("expected suggested input to be set")
	}
}

func TestWorkflowAgent_GenerateNextStepSuggestion_ApprovalRisky(t *testing.T) {
	agent := NewWorkflowAgent(nil)

	decision := agent.generateNextStepSuggestion(agentSuggestionContext{
		phase:          suggestionPhaseApproval,
		approvalPrompt: "Delete everything? (y/n)",
		logs:           "Delete everything? (y/n)",
	})
	if decision == nil {
		t.Fatalf("expected decision")
	}
	if decision.SuggestedInput != "" {
		t.Fatalf("expected suggested input to be empty for risky prompts, got %q", decision.SuggestedInput)
	}
}

func TestWorkflowAgent_MonitorTask_AutoInputsApproval(t *testing.T) {
	agent := NewWorkflowAgent(nil)
	agent.sleep = func(time.Duration) {}
	agent.pollInterval = 1 * time.Millisecond
	agent.maxWait = 250 * time.Millisecond

	logCalls := 0
	agent.loadTerminalLogs = func(context.Context, string, int) (string, error) {
		logCalls++
		if logCalls == 1 {
			return "Proceed? (y/n)", nil
		}
		return "Task completed", nil
	}
	agent.loadTaskStatus = func(context.Context, string) (string, error) { return "", nil }
	agent.loadTerminalStatus = func(context.Context, string) (string, error) { return "", nil }

	var sent []string
	agent.sendTerminalInput = func(_ context.Context, _ string, input string) error {
		sent = append(sent, input)
		return nil
	}

	decision, err := agent.MonitorTask(context.Background(), "run-1", "node-1", "task", "task-1", "term-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if decision == nil || decision.Action != AgentActionAdvance {
		t.Fatalf("expected advance decision, got %#v", decision)
	}
	if len(sent) != 1 {
		t.Fatalf("expected one auto input, got %d", len(sent))
	}
	if sent[0] != "y\r" {
		t.Fatalf("expected input %q, got %q", "y\\r", sent[0])
	}
}
