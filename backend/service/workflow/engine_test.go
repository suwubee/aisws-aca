package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	taskservice "github.com/ai-coding-assistant/service/task"
)

type fakeCommandExecutor struct {
	results map[string]struct {
		output string
		err    error
	}
	calls []string
}

func (f *fakeCommandExecutor) Execute(command string) (string, error) {
	cmd := strings.TrimSpace(command)
	f.calls = append(f.calls, cmd)
	if f.results == nil {
		return "", nil
	}
	if res, ok := f.results[cmd]; ok {
		return res.output, res.err
	}
	return "", nil
}

type flakyCommandExecutor struct {
	calls []string
	tries int
}

func (f *flakyCommandExecutor) Execute(command string) (string, error) {
	cmd := strings.TrimSpace(command)
	f.calls = append(f.calls, cmd)
	f.tries++
	if f.tries == 1 {
		return "network issue\n", errors.New("i/o timeout")
	}
	return "ok\n", nil
}

type fakeTaskTerminal struct {
	id string
}

func (t fakeTaskTerminal) ID() string { return t.id }
func (t fakeTaskTerminal) Write([]byte) error {
	return nil
}

type fakeAutomationService struct {
	calls      int
	lastTaskID string
}

func (s *fakeAutomationService) StartTask(task *model.Task) (*taskservice.StartTaskResult, error) {
	s.calls++
	if task == nil {
		return nil, errors.New("task is nil")
	}
	s.lastTaskID = task.ID
	return &taskservice.StartTaskResult{
		Task:       task,
		Terminal:   fakeTaskTerminal{id: "term-1"},
		CLIStarted: true,
	}, nil
}

type fakeSSHExecutor struct {
	calls []struct {
		serverID string
		command  string
	}
	results map[string]struct {
		output string
		err    error
	}
}

func (f *fakeSSHExecutor) ExecuteCommand(serverID, command string) (string, error) {
	f.calls = append(f.calls, struct {
		serverID string
		command  string
	}{serverID: strings.TrimSpace(serverID), command: strings.TrimSpace(command)})

	if f.results == nil {
		return "", nil
	}

	key := strings.TrimSpace(serverID) + "|" + strings.TrimSpace(command)
	if res, ok := f.results[key]; ok {
		return res.output, res.err
	}

	return "", nil
}

type fakeTerminalSession struct {
	id     string
	writes []string
}

func (s *fakeTerminalSession) ID() string { return s.id }

func (s *fakeTerminalSession) Write(data []byte) error {
	s.writes = append(s.writes, string(data))
	return nil
}

type fakeTerminalManager struct {
	localSessionsCreated int
	sshSessionsCreated   int

	lastCreateTitle  string
	lastCreateTaskID *string
	lastSSHServerID  string

	renameCalls []struct {
		terminalID string
		title      string
	}

	nextSession terminalSession
}

func (m *fakeTerminalManager) CreateSession(title string, taskID *string) (terminalSession, error) {
	m.localSessionsCreated++
	m.lastCreateTitle = title
	m.lastCreateTaskID = taskID
	if m.nextSession != nil {
		return m.nextSession, nil
	}
	return &fakeTerminalSession{id: "local-1"}, nil
}

func (m *fakeTerminalManager) CreateSSHSession(serverID string) (terminalSession, error) {
	m.sshSessionsCreated++
	m.lastSSHServerID = serverID
	if m.nextSession != nil {
		return m.nextSession, nil
	}
	return &fakeTerminalSession{id: "ssh-1"}, nil
}

func (m *fakeTerminalManager) RenameSession(id, title string) error {
	m.renameCalls = append(m.renameCalls, struct {
		terminalID string
		title      string
	}{terminalID: id, title: title})
	return nil
}

func (m *fakeTerminalManager) GetOrResumeSession(id string) (terminalSession, error) {
	if m.nextSession != nil {
		return m.nextSession, nil
	}
	return nil, nil
}

func initWorkflowEngineTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:workflow_engine_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
}

func TestWorkflowEngine_RunWorkflow_WorkflowNotFound(t *testing.T) {
	initWorkflowEngineTestDB(t)

	engine := NewWorkflowEngine(nil, nil, nil)
	_, err := engine.RunWorkflow("missing")
	if !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestWorkflowEngine_RunWorkflow_ExecutesCommandsSequentially(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "n1", "type": "command", "name": "cmd-1", "config": map[string]any{"command": "echo one"}},
		{"id": "n2", "type": "command", "name": "cmd-2", "config": map[string]any{"command": "echo two"}},
	})
	edges, _ := json.Marshal([]map[string]any{
		{"id": "e1", "source": "n1", "target": "n2"},
	})
	workflow := model.Workflow{
		ID:    "wf-1",
		Name:  "wf",
		Nodes: string(nodes),
		Edges: string(edges),
	}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec := &fakeCommandExecutor{
		results: map[string]struct {
			output string
			err    error
		}{
			"echo one": {output: "one\n"},
			"echo two": {output: "two\n"},
		},
	}

	engine := NewWorkflowEngine(nil, nil, nil)
	engine.localExecutor = exec
	engine.startAsync = func(fn func()) { fn() }

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}
	if dbRun.CompletedAt == nil {
		t.Fatalf("expected completed_at to be set")
	}

	if len(exec.calls) != 2 || exec.calls[0] != "echo one" || exec.calls[1] != "echo two" {
		t.Fatalf("expected commands executed in order, got %v", exec.calls)
	}

	var logs []runLogEntry
	if err := json.Unmarshal([]byte(dbRun.Logs), &logs); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected logs to be non-empty")
	}
}

func TestWorkflowEngine_RunWorkflow_CommandFailureMarksFailed(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "n1", "type": "command", "config": map[string]any{"command": "ok"}},
		{"id": "n2", "type": "command", "config": map[string]any{"command": "boom"}},
	})
	edges, _ := json.Marshal([]map[string]any{
		{"source": "n1", "target": "n2"},
	})
	workflow := model.Workflow{ID: "wf-err", Name: "wf", Nodes: string(nodes), Edges: string(edges)}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec := &fakeCommandExecutor{
		results: map[string]struct {
			output string
			err    error
		}{
			"ok":   {output: "ok\n"},
			"boom": {output: "bad\n", err: errors.New("command failed")},
		},
	}

	engine := NewWorkflowEngine(nil, nil, nil)
	engine.localExecutor = exec
	engine.startAsync = func(fn func()) { fn() }

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "failed" {
		t.Fatalf("expected status %q, got %q", "failed", dbRun.Status)
	}
	if dbRun.CompletedAt == nil {
		t.Fatalf("expected completed_at to be set")
	}
	if dbRun.CurrentNodeID == nil || *dbRun.CurrentNodeID != "n2" {
		t.Fatalf("expected current_node_id %q, got %v", "n2", dbRun.CurrentNodeID)
	}
}

func TestWorkflowEngine_RunWorkflow_ConditionBranching(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "cond", "type": "condition", "config": map[string]any{"value": true}},
		{"id": "t", "type": "command", "config": map[string]any{"command": "true-cmd"}},
		{"id": "f", "type": "command", "config": map[string]any{"command": "false-cmd"}},
	})
	edges, _ := json.Marshal([]map[string]any{
		{"source": "cond", "target": "t", "sourceHandle": "true"},
		{"source": "cond", "target": "f", "sourceHandle": "false"},
	})
	workflow := model.Workflow{ID: "wf-cond", Name: "wf", Nodes: string(nodes), Edges: string(edges)}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec := &fakeCommandExecutor{
		results: map[string]struct {
			output string
			err    error
		}{
			"true-cmd":  {output: "ok\n"},
			"false-cmd": {output: "no\n"},
		},
	}

	engine := NewWorkflowEngine(nil, nil, nil)
	engine.localExecutor = exec
	engine.startAsync = func(fn func()) { fn() }

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}

	if len(exec.calls) != 1 || exec.calls[0] != "true-cmd" {
		t.Fatalf("expected only true branch command executed, got %v", exec.calls)
	}
}

func TestWorkflowEngine_RunWorkflow_TaskNodeCreatesAndStartsTask(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "task1", "type": "task", "config": map[string]any{"title": "Example", "cli_type": "codex"}},
	})
	workflow := model.Workflow{ID: "wf-task", Name: "wf", Nodes: string(nodes), Edges: "[]"}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	termID := "term-1"
	if err := model.DB.Create(&model.Log{
		ID:         "log-1",
		TerminalID: &termID,
		LogType:    "output",
		Content:    "Task completed",
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	automation := &fakeAutomationService{}

	engine := NewWorkflowEngine(nil, nil, nil)
	engine.automation = automation
	engine.startAsync = func(fn func()) { fn() }
	engine.newAgent = func(engine *WorkflowEngine) *WorkflowAgent {
		agent := NewWorkflowAgent(engine)
		agent.pollInterval = 1 * time.Millisecond
		agent.maxWait = 250 * time.Millisecond
		return agent
	}

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	if automation.calls != 1 {
		t.Fatalf("expected automation StartTask called once, got %d", automation.calls)
	}

	var taskCount int64
	if err := model.DB.Model(&model.Task{}).Count(&taskCount).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("expected 1 task, got %d", taskCount)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}
}

func TestWorkflowEngine_RunWorkflow_TerminalNodeUsesTerminalManagerAndCurrentServer(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "srv", "type": "server", "config": map[string]any{"server_id": "srv-1"}},
		{"id": "term", "type": "terminal", "name": "Run cmd", "config": map[string]any{"command": "echo hi"}},
	})
	workflow := model.Workflow{ID: "wf-term", Name: "wf", Nodes: string(nodes), Edges: "[]"}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	session := &fakeTerminalSession{id: "ssh-9"}
	tm := &fakeTerminalManager{nextSession: session}

	engine := NewWorkflowEngine(nil, nil, tm)
	engine.startAsync = func(fn func()) { fn() }
	engine.sleep = func(time.Duration) {}

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	if tm.sshSessionsCreated != 1 || tm.lastSSHServerID != "srv-1" {
		t.Fatalf("expected ssh terminal session created for %q, got created=%d server=%q", "srv-1", tm.sshSessionsCreated, tm.lastSSHServerID)
	}

	if len(session.writes) != 1 || session.writes[0] != "echo hi\r" {
		t.Fatalf("expected command written to terminal, got %v", session.writes)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}
}

func TestWorkflowEngine_RunWorkflow_GitNodeUsesSSHWhenCurrentServerSelected(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "srv", "type": "server", "config": map[string]any{"server_id": "srv-1"}},
		{"id": "git", "type": "git", "config": map[string]any{"command": "status", "work_dir": "/repo"}},
	})
	workflow := model.Workflow{ID: "wf-git", Name: "wf", Nodes: string(nodes), Edges: "[]"}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	ssh := &fakeSSHExecutor{
		results: map[string]struct {
			output string
			err    error
		}{
			"srv-1|cd /repo && git status": {output: "ok\n"},
		},
	}

	engine := NewWorkflowEngine(ssh, nil, nil)
	engine.startAsync = func(fn func()) { fn() }

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	if len(ssh.calls) != 1 {
		t.Fatalf("expected ssh ExecuteCommand called once, got %d", len(ssh.calls))
	}
	if ssh.calls[0].serverID != "srv-1" {
		t.Fatalf("expected serverID %q, got %q", "srv-1", ssh.calls[0].serverID)
	}
	if ssh.calls[0].command != "cd /repo && git status" {
		t.Fatalf("expected git command %q, got %q", "cd /repo && git status", ssh.calls[0].command)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}
}

func TestWorkflowEngine_RunWorkflow_WaitNodeSleepsForDuration(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "wait", "type": "wait", "config": map[string]any{"duration": "10ms"}},
	})
	workflow := model.Workflow{ID: "wf-wait", Name: "wf", Nodes: string(nodes), Edges: "[]"}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	var slept []time.Duration
	engine := NewWorkflowEngine(nil, nil, nil)
	engine.startAsync = func(fn func()) { fn() }
	engine.sleep = func(d time.Duration) { slept = append(slept, d) }

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	if len(slept) != 1 || slept[0] != 10*time.Millisecond {
		t.Fatalf("expected sleep 10ms once, got %v", slept)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}
}

func TestWorkflowEngine_RunWorkflow_RiskyCommandPausesRun(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "n1", "type": "command", "config": map[string]any{"command": "rm -rf /tmp/test"}},
	})
	workflow := model.Workflow{ID: "wf-pause", Name: "wf", Nodes: string(nodes), Edges: "[]"}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec := &fakeCommandExecutor{}
	engine := NewWorkflowEngine(nil, nil, nil)
	engine.localExecutor = exec
	engine.startAsync = func(fn func()) { fn() }

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "paused" {
		t.Fatalf("expected status %q, got %q", "paused", dbRun.Status)
	}
	if dbRun.CompletedAt != nil {
		t.Fatalf("expected completed_at to be nil for paused run")
	}
	if len(exec.calls) != 0 {
		t.Fatalf("expected command not executed, got %v", exec.calls)
	}

	var logs []runLogEntry
	if err := json.Unmarshal([]byte(dbRun.Logs), &logs); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	foundPaused := false
	foundReport := false
	for _, entry := range logs {
		if entry.Message == "Workflow paused" {
			foundPaused = true
		}
		if entry.Message == "Execution report" {
			foundReport = true
		}
	}
	if !foundPaused {
		t.Fatalf("expected Workflow paused log entry")
	}
	if !foundReport {
		t.Fatalf("expected Execution report log entry")
	}

	var msgCount int64
	if err := model.DB.Model(&model.Message{}).Where("type = ?", "approval_needed").Count(&msgCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if msgCount != 1 {
		t.Fatalf("expected 1 approval_needed message, got %d", msgCount)
	}
}

func TestWorkflowEngine_RunWorkflow_RetriesTransientCommandFailure(t *testing.T) {
	initWorkflowEngineTestDB(t)

	nodes, _ := json.Marshal([]map[string]any{
		{"id": "n1", "type": "command", "config": map[string]any{"command": "flaky"}},
	})
	workflow := model.Workflow{ID: "wf-retry", Name: "wf", Nodes: string(nodes), Edges: "[]"}
	if err := model.DB.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	exec := &flakyCommandExecutor{}
	engine := NewWorkflowEngine(nil, nil, nil)
	engine.localExecutor = exec
	engine.startAsync = func(fn func()) { fn() }
	engine.sleep = func(time.Duration) {}

	run, err := engine.RunWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow error: %v", err)
	}

	var dbRun model.WorkflowRun
	if err := model.DB.First(&dbRun, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("query run: %v", err)
	}
	if dbRun.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", dbRun.Status)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("expected 2 command attempts, got %v", exec.calls)
	}

	var logs []runLogEntry
	if err := json.Unmarshal([]byte(dbRun.Logs), &logs); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	foundRetry := false
	for _, entry := range logs {
		if strings.Contains(entry.Message, "Retrying node") {
			foundRetry = true
			break
		}
	}
	if !foundRetry {
		t.Fatalf("expected retry log entry")
	}
}
