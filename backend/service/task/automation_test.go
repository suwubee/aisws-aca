package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

type fakeTerminalSession struct {
	id          string
	writes      []string
	writeErrFor map[string]error
}

func (s *fakeTerminalSession) ID() string { return s.id }

func (s *fakeTerminalSession) Write(data []byte) error {
	if s.writeErrFor != nil {
		if err, ok := s.writeErrFor[string(data)]; ok {
			return err
		}
	}
	s.writes = append(s.writes, string(data))
	return nil
}

type fakeTerminalManager struct {
	localSessionsCreated int
	sshSessionsCreated   int

	lastCreateTitle  string
	lastCreateTaskID *string
	lastSSHServerID  string

	linkCalls []struct {
		terminalID string
		taskID     *string
	}
	renameCalls []struct{ terminalID, title string }

	nextSession taskTerminal
	createErr   error
}

func (m *fakeTerminalManager) GetSession(id string) taskTerminal {
	if m.nextSession != nil && m.nextSession.ID() == id {
		return m.nextSession
	}
	return nil
}

func (m *fakeTerminalManager) CreateSession(title string, taskID *string) (taskTerminal, error) {
	m.localSessionsCreated++
	m.lastCreateTitle = title
	m.lastCreateTaskID = taskID
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.nextSession != nil {
		return m.nextSession, nil
	}
	return &fakeTerminalSession{id: "local-1"}, nil
}

func (m *fakeTerminalManager) CreateSSHSession(serverID string) (taskTerminal, error) {
	m.sshSessionsCreated++
	m.lastSSHServerID = serverID
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.nextSession != nil {
		return m.nextSession, nil
	}
	return &fakeTerminalSession{id: "ssh-1"}, nil
}

func (m *fakeTerminalManager) RenameSession(id, title string) error {
	m.renameCalls = append(m.renameCalls, struct{ terminalID, title string }{terminalID: id, title: title})
	return nil
}

func (m *fakeTerminalManager) LinkTask(id string, taskID *string) error {
	m.linkCalls = append(m.linkCalls, struct {
		terminalID string
		taskID     *string
	}{terminalID: id, taskID: taskID})
	return nil
}

func initTestDB(t *testing.T) {
	t.Helper()

	dsn := fmt.Sprintf("file:automation_task_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
}

func TestAutomationService_StartTask_UsesLocalTerminalWhenServerIDNil(t *testing.T) {
	initTestDB(t)

	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "work")

	taskModel := model.Task{
		ID:            "task-1",
		Title:         "Example",
		Status:        "todo",
		WorkDir:       workDir,
		CLIType:       "codex",
		InitialPrompt: "hello",
		AutoCreateDir: true,
	}
	if err := model.DB.Create(&taskModel).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	session := &fakeTerminalSession{id: "local-1"}
	tm := &fakeTerminalManager{nextSession: session}
	svc := &AutomationService{terminalManager: tm}

	origSleep := sleep
	sleep = func(d time.Duration) { time.Sleep(time.Millisecond) }
	t.Cleanup(func() { sleep = origSleep })

	result, err := svc.StartTask(&taskModel)
	if err != nil {
		t.Fatalf("StartTask error: %v", err)
	}
	if !result.CLIStarted {
		t.Fatalf("expected CLIStarted true")
	}
	if tm.localSessionsCreated != 1 || tm.sshSessionsCreated != 0 {
		t.Fatalf("expected local session created once, got local=%d ssh=%d", tm.localSessionsCreated, tm.sshSessionsCreated)
	}

	expectedTitle := "[codex] Example"
	if tm.lastCreateTitle != expectedTitle {
		t.Fatalf("expected CreateSession title %q, got %q", expectedTitle, tm.lastCreateTitle)
	}

	expectedWrites := []string{
		"cd " + workDir + "\r",
		"codex\r",
		"hello\r",
	}
	if len(session.writes) != len(expectedWrites) {
		t.Fatalf("expected %d writes, got %d: %v", len(expectedWrites), len(session.writes), session.writes)
	}
	for i := range expectedWrites {
		if session.writes[i] != expectedWrites[i] {
			t.Fatalf("expected write[%d] %q, got %q", i, expectedWrites[i], session.writes[i])
		}
	}

	if info, err := os.Stat(workDir); err != nil || !info.IsDir() {
		t.Fatalf("expected workDir to exist as directory, err=%v", err)
	}

	var updated model.Task
	if err := model.DB.First(&updated, "id = ?", taskModel.ID).Error; err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("expected status %q, got %q", "in_progress", updated.Status)
	}
}

func TestAutomationService_StartTask_UsesSSHTerminalWhenServerIDSet(t *testing.T) {
	initTestDB(t)

	if err := model.DB.Create(&model.SSHServer{
		ID:   "srv-1",
		Name: "example",
	}).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	tmp := t.TempDir()
	workDir := filepath.Join(tmp, "remote-work")
	serverID := "srv-1"

	taskModel := model.Task{
		ID:            "task-2",
		Title:         "Remote",
		Status:        "todo",
		ServerID:      &serverID,
		WorkDir:       workDir,
		CLIType:       "claude",
		AutoCreateDir: true,
	}
	if err := model.DB.Create(&taskModel).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	session := &fakeTerminalSession{id: "ssh-1"}
	tm := &fakeTerminalManager{nextSession: session}
	svc := &AutomationService{terminalManager: tm}

	origSleep := sleep
	sleep = func(d time.Duration) { time.Sleep(time.Millisecond) }
	t.Cleanup(func() { sleep = origSleep })

	result, err := svc.StartTask(&taskModel)
	if err != nil {
		t.Fatalf("StartTask error: %v", err)
	}
	if !result.CLIStarted {
		t.Fatalf("expected CLIStarted true")
	}

	if tm.localSessionsCreated != 0 || tm.sshSessionsCreated != 1 {
		t.Fatalf("expected ssh session created once, got local=%d ssh=%d", tm.localSessionsCreated, tm.sshSessionsCreated)
	}
	if tm.lastSSHServerID != "srv-1" {
		t.Fatalf("expected CreateSSHSession serverID %q, got %q", "srv-1", tm.lastSSHServerID)
	}

	expectedTitle := "[claude] Remote @ example"
	if len(tm.renameCalls) != 1 || tm.renameCalls[0].title != expectedTitle {
		t.Fatalf("expected RenameSession called with title %q, got %+v", expectedTitle, tm.renameCalls)
	}
	if len(tm.linkCalls) != 1 || tm.linkCalls[0].taskID == nil || *tm.linkCalls[0].taskID != taskModel.ID {
		t.Fatalf("expected LinkTask called with taskID %q, got %+v", taskModel.ID, tm.linkCalls)
	}

	expectedWrites := []string{
		"mkdir -p " + workDir + "\r",
		"cd " + workDir + "\r",
		"claude\r",
	}
	if len(session.writes) != len(expectedWrites) {
		t.Fatalf("expected %d writes, got %d: %v", len(expectedWrites), len(session.writes), session.writes)
	}
	for i := range expectedWrites {
		if session.writes[i] != expectedWrites[i] {
			t.Fatalf("expected write[%d] %q, got %q", i, expectedWrites[i], session.writes[i])
		}
	}

	if _, err := os.Stat(workDir); err == nil {
		t.Fatalf("expected workDir not to be created locally for SSH task")
	}
}

func TestAutomationService_StartTask_ReturnsErrorWhenServerMissing(t *testing.T) {
	initTestDB(t)

	missing := "srv-missing"
	taskModel := model.Task{
		ID:            "task-3",
		Title:         "Remote",
		Status:        "todo",
		ServerID:      &missing,
		WorkDir:       "/tmp/remote",
		CLIType:       "claude",
		AutoCreateDir: true,
	}
	if err := model.DB.Create(&taskModel).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	tm := &fakeTerminalManager{}
	svc := &AutomationService{terminalManager: tm}

	origSleep := sleep
	sleep = func(d time.Duration) { time.Sleep(time.Millisecond) }
	t.Cleanup(func() { sleep = origSleep })

	result, err := svc.StartTask(&taskModel)
	if err == nil {
		t.Fatalf("expected error")
	}
	if result.Error != "Server not found" {
		t.Fatalf("expected result.Error %q, got %q", "Server not found", result.Error)
	}
	if tm.localSessionsCreated != 0 || tm.sshSessionsCreated != 0 {
		t.Fatalf("expected no terminal sessions created, got local=%d ssh=%d", tm.localSessionsCreated, tm.sshSessionsCreated)
	}
}

func TestAutomationService_StartTask_DoesNotCreateTerminalWhenLocalDirCreationFails(t *testing.T) {
	initTestDB(t)

	taskModel := model.Task{
		ID:            "task-4",
		Title:         "Local",
		Status:        "todo",
		WorkDir:       "/dev/null/work",
		CLIType:       "claude",
		AutoCreateDir: true,
	}
	if err := model.DB.Create(&taskModel).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	tm := &fakeTerminalManager{}
	svc := &AutomationService{terminalManager: tm}

	origSleep := sleep
	sleep = func(d time.Duration) { time.Sleep(time.Millisecond) }
	t.Cleanup(func() { sleep = origSleep })

	_, err := svc.StartTask(&taskModel)
	if err == nil {
		t.Fatalf("expected error")
	}
	if tm.localSessionsCreated != 0 {
		t.Fatalf("expected no terminal created, got %d", tm.localSessionsCreated)
	}
}

func TestAutomationService_StartTask_ReturnsErrorWhenRemoteMkdirFails(t *testing.T) {
	initTestDB(t)

	if err := model.DB.Create(&model.SSHServer{
		ID:   "srv-2",
		Name: "example",
	}).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	workDir := "/tmp/remote-work"
	serverID := "srv-2"

	taskModel := model.Task{
		ID:            "task-5",
		Title:         "Remote",
		Status:        "todo",
		ServerID:      &serverID,
		WorkDir:       workDir,
		CLIType:       "claude",
		AutoCreateDir: true,
	}
	if err := model.DB.Create(&taskModel).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	mkdirErr := errors.New("mkdir failed")
	session := &fakeTerminalSession{
		id: "ssh-2",
		writeErrFor: map[string]error{
			"mkdir -p " + workDir + "\r": mkdirErr,
		},
	}

	tm := &fakeTerminalManager{nextSession: session}
	svc := &AutomationService{terminalManager: tm}

	origSleep := sleep
	sleep = func(d time.Duration) { time.Sleep(time.Millisecond) }
	t.Cleanup(func() { sleep = origSleep })

	result, err := svc.StartTask(&taskModel)
	if err == nil {
		t.Fatalf("expected error")
	}
	if result.CLIStarted {
		t.Fatalf("expected CLIStarted false")
	}
	if result.Error == "" || result.Error == "Server not found" {
		t.Fatalf("expected remote mkdir error message, got %q", result.Error)
	}
	if tm.sshSessionsCreated != 1 {
		t.Fatalf("expected ssh session created, got %d", tm.sshSessionsCreated)
	}
}
