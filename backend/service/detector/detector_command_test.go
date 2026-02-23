package detector

import "testing"

func TestDetectAgentFromCommand_BareCommand(t *testing.T) {
	d := NewDetector()

	claude := d.DetectAgentFromCommand("claude")
	if claude == nil || claude.Type != AIAgentClaudeCode {
		t.Fatalf("expected claude command detected as %q", AIAgentClaudeCode)
	}

	codex := d.DetectAgentFromCommand("codex")
	if codex == nil || codex.Type != AIAgentCodex {
		t.Fatalf("expected codex command detected as %q", AIAgentCodex)
	}

	gemini := d.DetectAgentFromCommand("gemini")
	if gemini == nil || gemini.Type != AIAgentGemini {
		t.Fatalf("expected gemini command detected as %q", AIAgentGemini)
	}
}

func TestDetectAgent_OutputWelcomeToOpus(t *testing.T) {
	d := NewDetector()
	agent := d.DetectAgent("Welcome to Opus 4.6")
	if agent == nil {
		t.Fatalf("expected welcome output to detect claude")
	}
	if agent.Type != AIAgentClaudeCode {
		t.Fatalf("expected agent type %q, got %q", AIAgentClaudeCode, agent.Type)
	}
}
