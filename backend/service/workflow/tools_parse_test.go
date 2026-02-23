package workflow

import "testing"

func TestParseAIResponse_ActionWithCodeFence(t *testing.T) {
	resp := `<thought>执行命令</thought>
<action>
` + "```json" + `
{"tool":"execute_command","args":{"command":"pwd"}}
` + "```" + `
</action>`

	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("ParseAIResponse returned error: %v", err)
	}
	if parsed.Action == nil {
		t.Fatalf("expected action, got nil")
	}
	if parsed.Action.Tool != "execute_command" {
		t.Fatalf("expected tool execute_command, got %q", parsed.Action.Tool)
	}
	if got, _ := parsed.Action.Args["command"].(string); got != "pwd" {
		t.Fatalf("expected command pwd, got %q", got)
	}
}

func TestParseAIResponse_ActionWithLooseJSON(t *testing.T) {
	resp := `<thought>执行命令</thought>
<action>{tool:'execute_command',args:{command:'ls -la',},}</action>`

	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("ParseAIResponse returned error: %v", err)
	}
	if parsed.Action == nil {
		t.Fatalf("expected action, got nil")
	}
	if parsed.Action.Tool != "execute_command" {
		t.Fatalf("expected tool execute_command, got %q", parsed.Action.Tool)
	}
	if got, _ := parsed.Action.Args["command"].(string); got != "ls -la" {
		t.Fatalf("expected command ls -la, got %q", got)
	}
}

func TestParseAIResponse_ActionFromKeyValueFallback(t *testing.T) {
	resp := `<thought>执行命令</thought>
<action>
tool: execute_command
args: {"command":"echo hi"}
</action>`

	parsed, err := ParseAIResponse(resp)
	if err != nil {
		t.Fatalf("ParseAIResponse returned error: %v", err)
	}
	if parsed.Action == nil {
		t.Fatalf("expected action, got nil")
	}
	if parsed.Action.Tool != "execute_command" {
		t.Fatalf("expected tool execute_command, got %q", parsed.Action.Tool)
	}
	if got, _ := parsed.Action.Args["command"].(string); got != "echo hi" {
		t.Fatalf("expected command echo hi, got %q", got)
	}
}

func TestFilterToolsForPrompt_Blacklist(t *testing.T) {
	filtered := filterToolsForPrompt(GetAvailableTools(), map[string]any{
		"tool_blacklist": []string{"create_task", "start_task"},
	})

	seen := map[string]struct{}{}
	for _, tool := range filtered {
		seen[tool.Name] = struct{}{}
	}
	if _, ok := seen["create_task"]; ok {
		t.Fatalf("expected create_task to be filtered out")
	}
	if _, ok := seen["start_task"]; ok {
		t.Fatalf("expected start_task to be filtered out")
	}
}
