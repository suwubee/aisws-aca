package terminal

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestSession_StartWithTmux_NewSessionUsesStartDir(t *testing.T) {
	origExec := execCommand
	origPty := ptyStart
	defer func() {
		execCommand = origExec
		ptyStart = origPty
	}()

	var calls [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))

		cmd := exec.Command(os.Args[0], "-test.run=TestTerminalHelperProcessExitCode")
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "ACA_HELPER_EXIT=0"}
		if len(args) > 0 && args[0] == "has-session" {
			cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "ACA_HELPER_EXIT=1"}
		}
		return cmd
	}

	ptyStart = func(cmd *exec.Cmd) (*os.File, error) {
		return nil, errors.New("stop")
	}

	s := NewSession("term-1", "/bin/bash", 1024)
	s.SetStartDir("/tmp")

	_ = s.StartWithTmux(false)

	foundNew := false
	for _, args := range calls {
		if len(args) == 0 || args[0] != "new-session" {
			continue
		}
		foundNew = true
		if !reflect.DeepEqual(args[0:4], []string{"new-session", "-d", "-s", "term-1"}) {
			t.Fatalf("unexpected new-session prefix: %#v", args)
		}

		hasDir := false
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-c" && args[i+1] == "/tmp" {
				hasDir = true
				break
			}
		}
		if !hasDir {
			t.Fatalf("expected tmux new-session to include -c /tmp, got %#v", args)
		}
	}

	if !foundNew {
		t.Fatalf("expected tmux new-session to be invoked, calls=%#v", calls)
	}
}

func TestTerminalHelperProcessExitCode(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if os.Getenv("ACA_HELPER_EXIT") == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

