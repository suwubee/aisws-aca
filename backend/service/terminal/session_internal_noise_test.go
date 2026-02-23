package terminal

import "testing"

func TestIsRunCommandInternalLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "begin marker",
			line: "__ACA_CMD_BEGIN_804ba0b6afbf4b36879df66a0126a4d8__",
			want: true,
		},
		{
			name: "end marker",
			line: "__ACA_CMD_END_804ba0b6afbf4b36879df66a0126a4d8__:0",
			want: true,
		},
		{
			name: "echo wrapper command",
			line: "echo '__ACA_CMD_BEGIN_804ba0b6afbf4b36879df66a0126a4d8__'",
			want: true,
		},
		{
			name: "exit code wrapper command",
			line: "ACA_CODE=$?; echo '__ACA_CMD_END_804ba0b6afbf4b36879df66a0126a4d8__:'$ACA_CODE; unset ACA_CODE",
			want: true,
		},
		{
			name: "heredoc marker",
			line: "ACA_EOF_804ba0b6afbf4b36879df66a0126a4d8",
			want: true,
		},
		{
			name: "normal command",
			line: "cat /proc/loadavg",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRunCommandInternalLine(tt.line)
			if got != tt.want {
				t.Fatalf("isRunCommandInternalLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestFlushInputLineLocked_SkipsInternalRunCommandNoise(t *testing.T) {
	s := NewSession("terminal-noise-test", "bash", 1024)

	s.inputLineBuf = []rune("echo '__ACA_CMD_BEGIN_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa__'")
	s.flushInputLineLocked()
	if len(s.logBuffer) != 0 {
		t.Fatalf("expected no logs for internal run-command wrapper, got %d", len(s.logBuffer))
	}

	s.inputLineBuf = []rune("pwd")
	s.flushInputLineLocked()
	if len(s.logBuffer) == 0 {
		t.Fatalf("expected normal input to be logged")
	}
}
