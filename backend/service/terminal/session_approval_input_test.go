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

func TestSession_sendApprovalInput_UsesTmuxForMacroWithEnter(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()

	var calls [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))
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

	if len(calls) != 2 {
		t.Fatalf("expected 2 tmux calls, got %d", len(calls))
	}

	wantFirst := []string{"send-keys", "-t", "term-1:0.0", "-l", "--", "yes"}
	if !reflect.DeepEqual(calls[0], wantFirst) {
		t.Fatalf("unexpected tmux args (1st call).\nwant: %#v\ngot:  %#v", wantFirst, calls[0])
	}
	wantSecond := []string{"send-keys", "-t", "term-1:0.0", "--", "C-m"}
	if !reflect.DeepEqual(calls[1], wantSecond) {
		t.Fatalf("unexpected tmux args (2nd call).\nwant: %#v\ngot:  %#v", wantSecond, calls[1])
	}
}

func TestTerminalHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}
