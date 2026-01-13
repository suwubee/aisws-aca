package task

import (
	"strings"
	"testing"
)

func TestQuoteShellPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: "''"},
		{input: "   ", want: "''"},
		{input: "~", want: "\"$HOME\""},
		{input: "~/work", want: "\"$HOME/work\""},
		{input: "/var/lib/app", want: "'/var/lib/app'"},
	}

	for _, tt := range tests {
		if got := quoteShellPath(tt.input); got != tt.want {
			t.Fatalf("quoteShellPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}

	// Ensure we do not allow unescaped expansions in the suffix part of ~/...
	got := quoteShellPath("~/a$(`b\")")
	if !strings.HasPrefix(got, "\"$HOME/") {
		t.Fatalf("expected $HOME prefix, got %q", got)
	}
	if !strings.Contains(got, "\\$") {
		t.Fatalf("expected $ escaped, got %q", got)
	}
	if !strings.Contains(got, "\\`") {
		t.Fatalf("expected backtick escaped, got %q", got)
	}
	if !strings.Contains(got, "\\\"") {
		t.Fatalf("expected quote escaped, got %q", got)
	}
}

func TestNormalizeScript(t *testing.T) {
	lines := normalizeScript("a\r\nb\rc\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %#v", len(lines), lines)
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" || lines[3] != "" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestParseExitCodeMarker(t *testing.T) {
	found, code := parseExitCodeMarker("ok\nACA_TASK_EXIT_CODE:0\n")
	if !found || code != 0 {
		t.Fatalf("expected found=true code=0, got found=%v code=%d", found, code)
	}

	found, code = parseExitCodeMarker("...\naca_task_exit_code:12\n")
	if !found || code != 12 {
		t.Fatalf("expected found=true code=12, got found=%v code=%d", found, code)
	}

	found, code = parseExitCodeMarker("no marker here")
	if found || code != 0 {
		t.Fatalf("expected found=false code=0, got found=%v code=%d", found, code)
	}
}

func TestContainsScriptNeedsUserSignal(t *testing.T) {
	if !containsScriptNeedsUserSignal("ACA_TASK_PAUSE: please confirm") {
		t.Fatalf("expected pause marker detected")
	}
	if !containsScriptNeedsUserSignal("please reboot the server") {
		t.Fatalf("expected reboot detected")
	}
	if containsScriptNeedsUserSignal("all good") {
		t.Fatalf("did not expect needs-user signal")
	}
}
