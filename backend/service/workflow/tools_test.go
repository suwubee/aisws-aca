package workflow

import "testing"

func TestParseAIResponse_ActionJSONWithoutTags(t *testing.T) {
	resp := `{"tool":"execute_command","args":{"command":"echo hi"}}`
	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if parsed.Action == nil {
		t.Fatalf("expected action, got nil")
	}
	if parsed.Action.Tool != "execute_command" {
		t.Fatalf("expected tool %q, got %q", "execute_command", parsed.Action.Tool)
	}
	if parsed.Action.Args == nil || parsed.Action.Args["command"] != "echo hi" {
		t.Fatalf("expected args.command %q, got %#v", "echo hi", parsed.Action.Args)
	}
}

func TestParseAIResponse_CompleteJSONWithoutTags(t *testing.T) {
	resp := `{"status":"success","summary":"done"}`
	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if parsed.Complete == nil {
		t.Fatalf("expected complete, got nil")
	}
	if parsed.Complete.Status != "success" {
		t.Fatalf("expected status %q, got %q", "success", parsed.Complete.Status)
	}
	if parsed.Complete.Summary != "done" {
		t.Fatalf("expected summary %q, got %q", "done", parsed.Complete.Summary)
	}
}

func TestParseAIResponse_WrapperActionJSON(t *testing.T) {
	resp := `{"action":{"tool":"ask_user","args":{"question":"ok?"}}}`
	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if parsed.Action == nil {
		t.Fatalf("expected action, got nil")
	}
	if parsed.Action.Tool != "ask_user" {
		t.Fatalf("expected tool %q, got %q", "ask_user", parsed.Action.Tool)
	}
	if parsed.Action.Args == nil || parsed.Action.Args["question"] != "ok?" {
		t.Fatalf("expected args.question %q, got %#v", "ok?", parsed.Action.Args)
	}
}

func TestParseAIResponse_WrapperCompleteJSON(t *testing.T) {
	resp := `{"complete":{"status":"partial","summary":"half"}}`
	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if parsed.Complete == nil {
		t.Fatalf("expected complete, got nil")
	}
	if parsed.Complete.Status != "partial" {
		t.Fatalf("expected status %q, got %q", "partial", parsed.Complete.Status)
	}
	if parsed.Complete.Summary != "half" {
		t.Fatalf("expected summary %q, got %q", "half", parsed.Complete.Summary)
	}
}

