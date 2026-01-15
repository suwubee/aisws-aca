package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

type recordingTerminalManager struct {
	sessions        map[string]terminalSession
	createSSHServer []string
}

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

