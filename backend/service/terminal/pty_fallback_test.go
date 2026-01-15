package terminal

import (
	"os"
	"os/exec"
	"testing"

	"github.com/creack/pty"
)

func TestSessionStart_FallsBackToPipesWhenPTYDenied(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	origExec := execCommand
	origPty := ptyStart
	defer func() {
		execCommand = origExec
		ptyStart = origPty
	}()

	// Make tmux unavailable to force direct shell mode, then deny PTY so we fall back to pipe backend.
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "tmux" {
			cmd := exec.Command(os.Args[0], "-test.run=TestTerminalHelperProcessExitCode")
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "ACA_HELPER_EXIT=1"}
			return cmd
		}
		return origExec(name, args...)
	}
	ptyStart = func(cmd *exec.Cmd) (*os.File, error) {
		return nil, pty.ErrUnsupported
	}

	s := NewSession("term-pipe-fallback", bashPath, 1024)
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if s.backend == nil {
		t.Fatalf("expected backend to be set when PTY is unavailable")
	}
	if _, ok := s.backend.(*pipeShellBackend); !ok {
		t.Fatalf("expected pipeShellBackend, got %T", s.backend)
	}
}
