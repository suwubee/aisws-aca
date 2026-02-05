package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/utils"
)

func TestMain(m *testing.M) {
	_ = utils.InitLogger("error", "")
	os.Exit(m.Run())
}

func TestSessionRecoverFromTmux_UsesAttach(t *testing.T) {
	restore := stubTerminalExec(t, []string{"s1"})
	defer restore()

	dsn := fmt.Sprintf("file:terminal_session_recover_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	s := NewSession("s1", "/bin/bash", 1024)
	t.Cleanup(func() { _ = s.Close() })

	if err := s.RecoverFromTmux(); err != nil {
		t.Fatalf("RecoverFromTmux() error: %v", err)
	}
	if !s.RecoveredFromTmux() {
		t.Fatalf("expected RecoveredFromTmux()=true")
	}

	meta := s.Metadata()
	if meta.TmuxSession != "s1" {
		t.Fatalf("expected TmuxSession %q, got %q", "s1", meta.TmuxSession)
	}
	if meta.PID == 0 {
		t.Fatalf("expected non-zero PID")
	}
}

func TestManagerRecoverSessions_AttachesExistingAndMarksExitedMissing(t *testing.T) {
	restore := stubTerminalExec(t, []string{"s-exists"})
	defer restore()

	dsn := fmt.Sprintf("file:terminal_recover_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	now := time.Now()
	if err := model.DB.Create(&model.TerminalSession{
		ID:        "s-exists",
		UserID:    "u1",
		Title:     "exists",
		Status:    "running",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create TerminalSession s-exists failed: %v", err)
	}
	if err := model.DB.Create(&model.TerminalSession{
		ID:        "s-missing",
		UserID:    "u1",
		Title:     "missing",
		Status:    "running",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create TerminalSession s-missing failed: %v", err)
	}

	mgr := &Manager{
		config: &config.TerminalConfig{
			DefaultShell:    "/bin/bash",
			ScrollbackBytes: 1024,
		},
	}

	if err := mgr.RecoverSessions(); err != nil {
		t.Fatalf("RecoverSessions() error: %v", err)
	}

	recovered := mgr.GetSession("s-exists")
	if recovered == nil {
		t.Fatalf("expected session s-exists to be recovered")
	}
	t.Cleanup(func() { _ = recovered.Close() })

	if mgr.GetSession("s-missing") != nil {
		t.Fatalf("expected session s-missing to not be recovered")
	}

	var exists model.TerminalSession
	if err := model.DB.First(&exists, "id = ?", "s-exists").Error; err != nil {
		t.Fatalf("query TerminalSession s-exists failed: %v", err)
	}
	if exists.Status != "running" {
		t.Fatalf("expected s-exists status %q, got %q", "running", exists.Status)
	}

	var missing model.TerminalSession
	if err := model.DB.First(&missing, "id = ?", "s-missing").Error; err != nil {
		t.Fatalf("query TerminalSession s-missing failed: %v", err)
	}
	if missing.Status != "exited" {
		t.Fatalf("expected s-missing status %q, got %q", "exited", missing.Status)
	}
	if missing.ClosedAt == nil {
		t.Fatalf("expected s-missing ClosedAt to be set")
	}
}

func stubTerminalExec(t *testing.T, existingSessions []string) func() {
	t.Helper()

	origExec := execCommand
	origPtyStart := ptyStart

	execCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestTerminalExecHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_TERMINAL_EXEC_HELPER=1",
			"TERMINAL_TEST_EXISTING_TMUX_SESSIONS="+strings.Join(existingSessions, ","),
		)
		return cmd
	}

	ptyStart = func(cmd *exec.Cmd) (*os.File, error) {
		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}

		cmd.Stdout = w
		cmd.Stderr = w

		if err := cmd.Start(); err != nil {
			_ = r.Close()
			_ = w.Close()
			return nil, err
		}

		_ = w.Close()
		return r, nil
	}

	return func() {
		execCommand = origExec
		ptyStart = origPtyStart
	}
}

func TestTerminalExecHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_TERMINAL_EXEC_HELPER") != "1" {
		return
	}

	args := os.Args
	for i := range args {
		if args[i] == "--" && i+1 < len(args) {
			args = args[i+1:]
			break
		}
	}

	if len(args) == 0 {
		os.Exit(2)
	}

	name := args[0]
	cmdArgs := args[1:]
	if name != "tmux" || len(cmdArgs) == 0 {
		os.Exit(2)
	}

	switch cmdArgs[0] {
	case "has-session":
		target := ""
		for i := 1; i < len(cmdArgs)-1; i++ {
			if cmdArgs[i] == "-t" {
				target = cmdArgs[i+1]
				break
			}
		}

		existing := os.Getenv("TERMINAL_TEST_EXISTING_TMUX_SESSIONS")
		if target != "" && containsSession(existing, target) {
			os.Exit(0)
		}
		os.Exit(1)
	case "attach-session":
		_, _ = os.Stdout.WriteString("attached")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

		select {
		case <-sigCh:
		case <-time.After(2 * time.Second):
		}
		os.Exit(0)
	case "new-session":
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func containsSession(list string, target string) bool {
	for _, item := range strings.Split(list, ",") {
		if item == target {
			return true
		}
	}
	return false
}
