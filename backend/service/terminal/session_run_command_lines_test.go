package terminal

import "testing"

func TestBuildRunCommandLines_SingleLine_NoWorkDir_Direct(t *testing.T) {
	lines := buildRunCommandLines("__BEGIN__", "__END__:", "ACA_EOF_X", "ps aux | grep screen | grep -v grep", "")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %#v", len(lines), lines)
	}
	if lines[0] != "echo '__BEGIN__'" {
		t.Fatalf("unexpected line[0]: %q", lines[0])
	}
	if lines[1] != "ps aux | grep screen | grep -v grep" {
		t.Fatalf("unexpected line[1]: %q", lines[1])
	}
	if lines[2] != "ACA_CODE=$?; echo '__END__:'$ACA_CODE; unset ACA_CODE" {
		t.Fatalf("unexpected line[2]: %q", lines[2])
	}
}

func TestBuildRunCommandLines_MultiLine_UsesHeredoc(t *testing.T) {
	lines := buildRunCommandLines("__BEGIN__", "__END__:", "ACA_EOF_X", "echo 1\necho 2", "")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d: %#v", len(lines), lines)
	}
	if lines[0] != "echo '__BEGIN__'" {
		t.Fatalf("unexpected line[0]: %q", lines[0])
	}
	if lines[1] != "bash <<'ACA_EOF_X'" {
		t.Fatalf("unexpected line[1]: %q", lines[1])
	}
	if lines[2] != "echo 1" || lines[3] != "echo 2" {
		t.Fatalf("unexpected command lines: %#v", lines[2:4])
	}
	if lines[4] != "ACA_EOF_X" {
		t.Fatalf("unexpected line[4]: %q", lines[4])
	}
	if lines[5] != "ACA_CODE=$?; echo '__END__:'$ACA_CODE; unset ACA_CODE" {
		t.Fatalf("unexpected line[5]: %q", lines[5])
	}
}

func TestBuildRunCommandLines_SingleLine_WithWorkDir_UsesHeredocAndCd(t *testing.T) {
	lines := buildRunCommandLines("__BEGIN__", "__END__:", "ACA_EOF_X", "ls -la", "~/test")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d: %#v", len(lines), lines)
	}
	if lines[1] != "bash <<'ACA_EOF_X'" {
		t.Fatalf("unexpected line[1]: %q", lines[1])
	}
	if lines[2] != "cd -- \"$HOME/test\"" {
		t.Fatalf("unexpected cd line: %q", lines[2])
	}
	if lines[3] != "ls -la" {
		t.Fatalf("unexpected command line: %q", lines[3])
	}
	if lines[4] != "ACA_EOF_X" {
		t.Fatalf("unexpected heredoc marker line: %q", lines[4])
	}
}

