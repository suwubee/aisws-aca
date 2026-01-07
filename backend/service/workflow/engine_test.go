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

func initWorkflowEngineTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:workflow_engine_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
}

func TestWorkflowEngine_RunWorkflow_WorkflowNotFound(t *testing.T) {
	initWorkflowEngineTestDB(t)

	engine := NewWorkflowEngine(nil, nil)
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

	engine := NewWorkflowEngine(nil, nil)
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

	engine := NewWorkflowEngine(nil, nil)
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

	engine := NewWorkflowEngine(nil, nil)
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

	automation := &fakeAutomationService{}

	engine := NewWorkflowEngine(nil, nil)
	engine.automation = automation
	engine.startAsync = func(fn func()) { fn() }

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

