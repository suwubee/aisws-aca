package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	terminalsvc "github.com/ai-coding-assistant/service/terminal"
)

type recordingTerminalManager struct {
	sessions        map[string]terminalSession
	createSSHServer []string
}

type interactiveMetaTerminalSession struct {
	id              string
	writes          []string
	runCommandCalls int
	meta            *terminalsvc.SessionMetadata
}

func (s *interactiveMetaTerminalSession) ID() string { return s.id }

func (s *interactiveMetaTerminalSession) Write(data []byte) error {
	s.writes = append(s.writes, string(data))
	return nil
}

func (s *interactiveMetaTerminalSession) BroadcastAILog(logType, message string) {}
func (s *interactiveMetaTerminalSession) BroadcastAILogWithInput(logType, message, inputType, inputData string) {
}
func (s *interactiveMetaTerminalSession) InjectOutput([]byte) {}
func (s *interactiveMetaTerminalSession) RunCommand(command, workDir string, timeout time.Duration) (string, int, error) {
	s.runCommandCalls++
	return "", 0, nil
}
func (s *interactiveMetaTerminalSession) Metadata() *terminalsvc.SessionMetadata { return s.meta }

func (m *recordingTerminalManager) CreateSession(title string, taskID *string) (terminalSession, error) {
	if m.sessions == nil {
		m.sessions = map[string]terminalSession{}
	}
	id := fmt.Sprintf("local-%d", len(m.sessions)+1)
	s := &fakeTerminalSession{id: id}
	m.sessions[id] = s
	return s, nil
}

func (m *recordingTerminalManager) CreateSSHSession(serverID string) (terminalSession, error) {
	m.createSSHServer = append(m.createSSHServer, serverID)
	if m.sessions == nil {
		m.sessions = map[string]terminalSession{}
	}
	id := fmt.Sprintf("ssh-%d", len(m.createSSHServer))
	s := &fakeTerminalSession{id: id}
	m.sessions[id] = s
	return s, nil
}

func (m *recordingTerminalManager) RenameSession(id, title string) error { return nil }
func (m *recordingTerminalManager) LinkTask(id string, taskID *string) error {
	return nil
}

func (m *recordingTerminalManager) GetOrResumeSession(id string) (terminalSession, error) {
	if m.sessions == nil {
		return nil, nil
	}
	return m.sessions[id], nil
}

func TestToolExecutor_EnsureTerminalForServer_ReusesCurrentTerminal(t *testing.T) {
	initWorkflowEngineTestDB(t)

	sid := "srv-1"
	termID := "term-1"
	if err := model.DB.Create(&model.TerminalSession{
		ID:        termID,
		Title:     "SSH",
		ServerID:  &sid,
		Status:    "running",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create terminal session: %v", err)
	}

	tm := &recordingTerminalManager{
		sessions: map[string]terminalSession{
			termID: &fakeTerminalSession{id: termID},
		},
	}

	executor := &ToolExecutor{terminal: tm}
	sessionCtx := map[string]any{
		"terminal_id": termID,
	}

	got := executor.ensureTerminalForServer(sessionCtx, sid)
	if got != termID {
		t.Fatalf("expected terminal %q, got %q", termID, got)
	}
	if len(tm.createSSHServer) != 0 {
		t.Fatalf("expected no CreateSSHSession calls, got %d", len(tm.createSSHServer))
	}
	if v := getStringFromMap(sessionCtx, "terminal_id"); v != termID {
		t.Fatalf("expected sessionCtx terminal_id %q, got %q", termID, v)
	}
	byServer := getStringMapFromContext(sessionCtx, "terminal_ids_by_server")
	if byServer == nil || byServer[sid] != termID {
		t.Fatalf("expected terminal_ids_by_server[%q]=%q, got %#v", sid, termID, byServer)
	}
}

func TestToolExecutor_EnsureTerminalForServer_CreatesNewWhenMismatch(t *testing.T) {
	initWorkflowEngineTestDB(t)

	currentServer := "other"
	desiredServer := "srv-1"
	termID := "term-1"
	if err := model.DB.Create(&model.TerminalSession{
		ID:        termID,
		Title:     "SSH",
		ServerID:  &currentServer,
		Status:    "running",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create terminal session: %v", err)
	}

	tm := &recordingTerminalManager{
		sessions: map[string]terminalSession{
			termID: &fakeTerminalSession{id: termID},
		},
	}

	executor := &ToolExecutor{terminal: tm}
	sessionCtx := map[string]any{
		"terminal_id": termID,
		"task_id":     "task-1",
	}

	got := executor.ensureTerminalForServer(sessionCtx, desiredServer)
	if got == termID || got == "" {
		t.Fatalf("expected a new terminal id, got %q", got)
	}
	if len(tm.createSSHServer) != 1 || tm.createSSHServer[0] != desiredServer {
		t.Fatalf("expected CreateSSHSession(%q) once, got %#v", desiredServer, tm.createSSHServer)
	}
	if v := getStringFromMap(sessionCtx, "terminal_id"); v != got {
		t.Fatalf("expected sessionCtx terminal_id updated to %q, got %q", got, v)
	}
}

func TestToolExecutor_EnsureTerminalForServer_TerminalModeReusesCurrent(t *testing.T) {
	initWorkflowEngineTestDB(t)

	currentServer := "other"
	desiredServer := "srv-1"
	termID := "term-1"
	if err := model.DB.Create(&model.TerminalSession{
		ID:        termID,
		Title:     "SSH",
		ServerID:  &currentServer,
		Status:    "running",
		CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create terminal session: %v", err)
	}

	tm := &recordingTerminalManager{
		sessions: map[string]terminalSession{
			termID: &fakeTerminalSession{id: termID},
		},
	}

	executor := &ToolExecutor{terminal: tm}
	sessionCtx := map[string]any{
		"terminal_id":            termID,
		"command_execution_mode": "terminal",
	}

	got := executor.ensureTerminalForServer(sessionCtx, desiredServer)
	if got != termID {
		t.Fatalf("expected terminal mode to keep terminal %q, got %q", termID, got)
	}
	if len(tm.createSSHServer) != 0 {
		t.Fatalf("expected no CreateSSHSession calls in terminal mode, got %#v", tm.createSSHServer)
	}
}

func TestToolExecutor_Execute_BlocksTaskToolsInTerminalMode(t *testing.T) {
	executor := &ToolExecutor{}
	sessionCtx := map[string]any{
		"command_execution_mode": "terminal",
	}

	createRes := executor.Execute(context.Background(), "create_task", map[string]any{
		"title": "blocked",
	}, sessionCtx)
	if createRes.Success {
		t.Fatalf("expected create_task to be blocked in terminal mode")
	}
	if !strings.Contains(createRes.Error, "disabled in terminal mode") {
		t.Fatalf("unexpected create_task error: %s", createRes.Error)
	}

	startRes := executor.Execute(context.Background(), "start_task", map[string]any{
		"task_id": "task-1",
	}, sessionCtx)
	if startRes.Success {
		t.Fatalf("expected start_task to be blocked in terminal mode")
	}
	if !strings.Contains(startRes.Error, "disabled in terminal mode") {
		t.Fatalf("unexpected start_task error: %s", startRes.Error)
	}
}

func TestToolExecutor_ExecuteCommand_BlocksServerSwitchInTerminalMode(t *testing.T) {
	executor := &ToolExecutor{
		terminal: &recordingTerminalManager{
			sessions: map[string]terminalSession{
				"term-1": &fakeTerminalSession{id: "term-1"},
			},
		},
	}

	result := executor.executeCommand(map[string]any{
		"command":   "pwd",
		"server_id": "srv-2",
	}, map[string]any{
		"command_execution_mode": "terminal",
		"terminal_id":            "term-1",
		"current_server_id":      "srv-1",
	})

	if result.Success {
		t.Fatalf("expected execute_command to fail when switching server in terminal mode")
	}
	if !strings.Contains(result.Error, "locks server") {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestToolExecutor_ExecuteCommand_TerminalMode_BlocksOneShotCLI(t *testing.T) {
	executor := &ToolExecutor{}
	result := executor.executeCommand(map[string]any{
		"command":   "claude -p \"hello\"",
		"server_id": "srv-1",
	}, map[string]any{
		"command_execution_mode": "terminal",
		"terminal_id":            "term-1",
		"current_server_id":      "srv-1",
	})
	if result.Success {
		t.Fatalf("expected one-shot CLI command to be blocked in terminal mode")
	}
	if !strings.Contains(strings.ToLower(result.Error), "one-shot cli command") {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestToolExecutor_ExecuteCommand_TerminalMode_InteractivePromptUsesWrite(t *testing.T) {
	initWorkflowEngineTestDB(t)

	session := &interactiveMetaTerminalSession{
		id: "term-1",
		meta: &terminalsvc.SessionMetadata{
			AIAssistant: &terminalsvc.AIAssistant{
				Detected: true,
				State:    "waiting_input",
			},
		},
	}
	executor := &ToolExecutor{
		terminal: &recordingTerminalManager{
			sessions: map[string]terminalSession{"term-1": session},
		},
		sshManager: &fakeSSHExecutor{},
	}

	result := executor.executeCommand(map[string]any{
		"command":   "请创建一个简单 HTML 文件",
		"server_id": "srv-1",
	}, map[string]any{
		"command_execution_mode": "terminal",
		"terminal_id":            "term-1",
		"current_server_id":      "srv-1",
	})

	if !result.Success {
		t.Fatalf("expected prompt to be sent to interactive CLI, got error: %s", result.Error)
	}
	if session.runCommandCalls != 0 {
		t.Fatalf("expected RunCommand not to be called, got %d", session.runCommandCalls)
	}
	if len(session.writes) == 0 {
		t.Fatalf("expected prompt to be written to terminal")
	}
	got := session.writes[len(session.writes)-1]
	if !strings.Contains(got, "请创建一个简单 HTML 文件") {
		t.Fatalf("unexpected prompt payload: %q", got)
	}
	if !strings.HasSuffix(got, "\r") {
		t.Fatalf("expected prompt write to end with carriage return, got %q", got)
	}
}

func TestToolExecutor_ExecuteCommand_TerminalMode_InteractiveShellFallsBackToBackend(t *testing.T) {
	initWorkflowEngineTestDB(t)

	session := &interactiveMetaTerminalSession{
		id: "term-1",
		meta: &terminalsvc.SessionMetadata{
			AIAssistant: &terminalsvc.AIAssistant{
				Detected: true,
				State:    "working",
			},
		},
	}
	ssh := &fakeSSHExecutor{
		results: map[string]struct {
			output string
			err    error
		}{
			"srv-1|cd /root/test && ls -la": {output: "ok", err: nil},
		},
	}
	executor := &ToolExecutor{
		terminal: &recordingTerminalManager{
			sessions: map[string]terminalSession{"term-1": session},
		},
		sshManager: ssh,
	}

	result := executor.executeCommand(map[string]any{
		"command":   "ls -la",
		"work_dir":  "/root/test",
		"server_id": "srv-1",
	}, map[string]any{
		"command_execution_mode": "terminal",
		"terminal_id":            "term-1",
		"current_server_id":      "srv-1",
	})

	if !result.Success {
		t.Fatalf("expected backend fallback execution success, got error: %s", result.Error)
	}
	if session.runCommandCalls != 0 {
		t.Fatalf("expected RunCommand not to be used for interactive CLI shell fallback, got %d", session.runCommandCalls)
	}
	if len(ssh.calls) != 1 {
		t.Fatalf("expected one backend ssh execution, got %#v", ssh.calls)
	}
	if ssh.calls[0].serverID != "srv-1" || ssh.calls[0].command != "cd /root/test && ls -la" {
		t.Fatalf("unexpected backend fallback command: %#v", ssh.calls[0])
	}
}
