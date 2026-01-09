package terminal

import (
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestSession_sendApprovalInput_UsesTmuxForSingleEnter(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var gotName string
	var gotArgs []string

	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)

		cmd := exec.Command(os.Args[0], "-test.run=TestTerminalHelperProcess")
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	s := NewSession("term-1", "/bin/bash", 1024)
	s.metaMutex.Lock()
	s.metadata.TmuxSession = "term-1"
	s.metaMutex.Unlock()

	if err := s.sendApprovalInput("\r"); err != nil {
		t.Fatalf("sendApprovalInput returned error: %v", err)
	}

	if gotName != "tmux" {
		t.Fatalf("expected tmux to be invoked, got %q", gotName)
	}

	wantArgs := []string{"send-keys", "-t", "term-1:0.0", "--", "C-m"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected tmux args.\nwant: %#v\ngot:  %#v", wantArgs, gotArgs)
	}
}

func TestSession_sendApprovalInput_DoesNotUseTmuxForText(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	called := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		called = true
		cmd := exec.Command(os.Args[0], "-test.run=TestTerminalHelperProcess")
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	s := NewSession("term-1", "/bin/bash", 1024)
	s.metaMutex.Lock()
	s.metadata.TmuxSession = "term-1"
	s.metaMutex.Unlock()

	if err := s.sendApprovalInput("yes\r"); err != nil {
		t.Fatalf("sendApprovalInput returned error: %v", err)
	}

	if called {
		t.Fatalf("expected tmux not to be called for non-single-enter input")
	}
}

func TestTerminalHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

