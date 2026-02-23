package api

import (
	"strings"
	"testing"
)

func TestResolveResumeCLIType_SkipsEmptyCandidate(t *testing.T) {
	got := resolveResumeCLIType("", "codex", "", "")
	if got != "codex" {
		t.Fatalf("expected codex, got %q", got)
	}
}

func TestResolveResumeCLIType_PrefersFirstRecognized(t *testing.T) {
	got := resolveResumeCLIType("claude", "codex")
	if got != "claude" {
		t.Fatalf("expected claude, got %q", got)
	}
}

func TestBuildResumeCommand_AutoCodexWithSessionUsesNative(t *testing.T) {
	strategy, cmd, prompt, err := buildResumeCommand(resumeCommandInput{
		RequestedStrategy: resumeStrategyAuto,
		CLIType:           "codex",
		SessionID:         "123e4567-e89b-12d3-a456-426614174000",
		WorkDir:           "/tmp/work",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strategy != resumeStrategyNativeLabel {
		t.Fatalf("expected strategy %q, got %q", resumeStrategyNativeLabel, strategy)
	}
	expected := "cd -- '/tmp/work' && codex --resume '123e4567-e89b-12d3-a456-426614174000'"
	if cmd != expected {
		t.Fatalf("expected command %q, got %q", expected, cmd)
	}
	if prompt != "" {
		t.Fatalf("expected empty prompt for native resume, got %q", prompt)
	}
}

func TestBuildResumeCommand_AutoCodexWithoutSessionFallsBackPrompt(t *testing.T) {
	strategy, cmd, prompt, err := buildResumeCommand(resumeCommandInput{
		RequestedStrategy: resumeStrategyAuto,
		CLIType:           "codex",
		WorkDir:           "/tmp/work",
		ParentPrompt:      "继续修复任务",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strategy != resumeStrategyPromptLabel {
		t.Fatalf("expected strategy %q, got %q", resumeStrategyPromptLabel, strategy)
	}
	expected := "cd -- '/tmp/work' && codex"
	if cmd != expected {
		t.Fatalf("expected command %q, got %q", expected, cmd)
	}
	if !strings.Contains(prompt, "继续上一轮会话并保持原有上下文") {
		t.Fatalf("expected fallback prompt to contain resume hint, got %q", prompt)
	}
}

func TestBuildResumeCommand_NativeCodexWithoutSessionReturnsError(t *testing.T) {
	_, _, _, err := buildResumeCommand(resumeCommandInput{
		RequestedStrategy: resumeStrategyNative,
		CLIType:           "codex",
		WorkDir:           "/tmp/work",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires session_id") {
		t.Fatalf("expected session_id error, got %v", err)
	}
}
