package terminal

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

func TestBuildTranscriptSnapshotFromLogs_MergesInputOutputChronologically(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_history_snapshot_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	terminalID := "t1"
	taskID := "task-1"

	now := time.Now()
	logs := []*model.Log{
		{ID: "1", TerminalID: &terminalID, TaskID: &taskID, LogType: "input", Content: "ls\n", CreatedAt: now.Add(1 * time.Second)},
		{ID: "2", TerminalID: &terminalID, TaskID: &taskID, LogType: "output", Content: "file1\n", CreatedAt: now.Add(2 * time.Second)},
		{ID: "3", TerminalID: &terminalID, TaskID: &taskID, LogType: "system", Content: "[AI][info] ignored\n", CreatedAt: now.Add(2500 * time.Millisecond)},
		{ID: "4", TerminalID: &terminalID, TaskID: &taskID, LogType: "output", Content: "file2\n", CreatedAt: now.Add(3 * time.Second)},
		{ID: "5", TerminalID: &terminalID, TaskID: &taskID, LogType: "input", Content: "echo hi\n", CreatedAt: now.Add(4 * time.Second)},
		{ID: "6", TerminalID: &terminalID, TaskID: &taskID, LogType: "output", Content: "hi\n", CreatedAt: now.Add(5 * time.Second)},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("insert logs failed: %v", err)
	}

	out, err := BuildTranscriptSnapshotFromLogs(terminalID, 1024*1024)
	if err != nil {
		t.Fatalf("BuildTranscriptSnapshotFromLogs error: %v", err)
	}

	got := string(out)
	want := "ls\nfile1\nfile2\necho hi\nhi\n"
	if got != want {
		t.Fatalf("unexpected snapshot.\nwant=%q\ngot=%q", want, got)
	}
}

func TestCaptureTmuxSnapshot_TruncatesAndFiltersMarkers(t *testing.T) {
	payload := "start\n__ACA_CMD_BEGIN_abc__\n" + strings.Repeat("a", 20*1024) + "\nend\n"
	restore := stubTmuxCapturePane(t, payload)
	defer restore()

	maxBytes := 9 * 1024
	out, err := CaptureTmuxSnapshot("s1", maxBytes)
	if err != nil {
		t.Fatalf("CaptureTmuxSnapshot error: %v", err)
	}
	if len(out) > maxBytes {
		t.Fatalf("expected truncated output <= %d bytes, got %d", maxBytes, len(out))
	}
	if strings.Contains(string(out), "__ACA_CMD_BEGIN_") {
		t.Fatalf("expected internal marker to be filtered out")
	}
}

func stubTmuxCapturePane(t *testing.T, stdout string) func() {
	t.Helper()

	origExec := execCommand
	b64 := base64.StdEncoding.EncodeToString([]byte(stdout))

	execCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestTmuxCaptureHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_TMUX_CAPTURE_HELPER=1",
			"TMUX_CAPTURE_OUTPUT_B64="+b64,
		)
		return cmd
	}

	return func() {
		execCommand = origExec
	}
}

func TestTmuxCaptureHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_TMUX_CAPTURE_HELPER") != "1" {
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
	if name != "tmux" || len(cmdArgs) == 0 || cmdArgs[0] != "capture-pane" {
		os.Exit(2)
	}

	encoded := os.Getenv("TMUX_CAPTURE_OUTPUT_B64")
	raw, _ := base64.StdEncoding.DecodeString(encoded)
	_, _ = os.Stdout.Write(raw)
	os.Exit(0)
}
