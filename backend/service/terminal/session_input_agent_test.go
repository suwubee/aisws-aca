package terminal

import (
	"testing"

	"github.com/ai-coding-assistant/service/detector"
)

func TestSession_DetectAIAgentFromInputCommand(t *testing.T) {
	s := NewSession("detect-input-claude", "bash", 1024)
	s.addInputLog([]byte("claude\r"))

	s.metaMutex.RLock()
	defer s.metaMutex.RUnlock()

	if s.aiAssistant == nil {
		t.Fatalf("expected ai assistant to be detected from input command")
	}
	if s.aiAssistant.Type != string(detector.AIAgentClaudeCode) {
		t.Fatalf("expected ai assistant type %q, got %q", detector.AIAgentClaudeCode, s.aiAssistant.Type)
	}
	if !s.aiAssistant.Detected {
		t.Fatalf("expected ai assistant detected=true")
	}
	if normalizeAISessionType(s.aiSessionType) != string(detector.AIAgentClaudeCode) {
		t.Fatalf("expected ai session type %q, got %q", detector.AIAgentClaudeCode, s.aiSessionType)
	}
}

func TestSession_DetectAIAgentFromBareCodexCommand(t *testing.T) {
	s := NewSession("detect-input-codex", "bash", 1024)
	s.addInputLog([]byte("codex\r"))

	s.metaMutex.RLock()
	defer s.metaMutex.RUnlock()

	if s.aiAssistant == nil {
		t.Fatalf("expected ai assistant to be detected from codex command")
	}
	if s.aiAssistant.Type != string(detector.AIAgentCodex) {
		t.Fatalf("expected ai assistant type %q, got %q", detector.AIAgentCodex, s.aiAssistant.Type)
	}
}
