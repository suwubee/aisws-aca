package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	taskservice "github.com/ai-coding-assistant/service/task"
	"github.com/ai-coding-assistant/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type commandExecutor interface {
	Execute(command string) (string, error)
}

var (
	ErrWorkflowEngineNil            = errors.New("workflow engine is nil")
	ErrWorkflowIDRequired           = errors.New("workflowID is required")
	ErrWorkflowNotFound             = errors.New("workflow not found")
	ErrWorkflowQueryFailed          = errors.New("failed to query workflow")
	ErrWorkflowStartFailed          = errors.New("failed to start workflow")
	ErrSSHManagerNotConfigured      = errors.New("ssh manager is not configured")
	ErrLocalExecutorNotConfigured   = errors.New("local executor is not configured")
	ErrTaskAutomationNotConfigured  = errors.New("task automation is not configured")
	ErrTerminalManagerNotConfigured = errors.New("terminal manager is not configured")
	ErrTaskNotFound                 = errors.New("task not found")
	ErrTaskQueryFailed              = errors.New("failed to query task")
	ErrServerNotFound               = errors.New("server not found")
	ErrServerQueryFailed            = errors.New("failed to query server")
)

type localShellExecutor struct{}

func (localShellExecutor) Execute(command string) (string, error) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", errors.New("command is required")
	}

	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out), err
}

type sshCommandExecutor interface {
	ExecuteCommand(serverID, command string) (string, error)
}

type automationService interface {
	StartTask(task *model.Task) (*taskservice.StartTaskResult, error)
}

type terminalSession interface {
	ID() string
	Write(data []byte) error
}

type terminalManager interface {
	CreateSession(title string, taskID *string) (terminalSession, error)
	CreateSSHSession(serverID string) (terminalSession, error)
	RenameSession(id, title string) error
	GetOrResumeSession(id string) (terminalSession, error)
}

type workflowPauseError struct {
	decision *AgentDecision
	command  string
}

func (e *workflowPauseError) Error() string {
	if e == nil || e.decision == nil {
		return "workflow paused"
	}
	msg := strings.TrimSpace(e.decision.Reasoning)
	if msg == "" {
		msg = "workflow paused"
	}
	return msg
}

// WorkflowEngine executes workflow definitions and updates WorkflowRun status/logs.
type WorkflowEngine struct {
	sshManager sshCommandExecutor
	automation automationService
	terminal   terminalManager

	localExecutor commandExecutor
	now           func() time.Time
	sleep         func(time.Duration)
	startAsync    func(fn func())
	newAgent      func(engine *WorkflowEngine) *WorkflowAgent
}

// NewWorkflowEngine creates a workflow engine.
func NewWorkflowEngine(sshManager sshCommandExecutor, automation automationService, terminalMgr terminalManager) *WorkflowEngine {
	engine := &WorkflowEngine{
		sshManager:    sshManager,
		automation:    automation,
		terminal:      terminalMgr,
		localExecutor: localShellExecutor{},
		now:           time.Now,
		sleep:         time.Sleep,
		startAsync:    func(fn func()) { go fn() },
		newAgent:      NewWorkflowAgent,
	}
	return engine
}

type workflowNode struct {
	ID       string
	Type     string
	Name     string
	ServerID string
	Config   map[string]any
	raw      map[string]any
}

type workflowEdge struct {
	ID           string
	Source       string
	Target       string
	SourceHandle string
	Label        string
	Condition    string
}

type runLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	NodeID    string    `json:"node_id,omitempty"`
	NodeType  string    `json:"node_type,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type executionReport struct {
	Status         string    `json:"status"`
	RunID          string    `json:"run_id"`
	WorkflowID     string    `json:"workflow_id,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMs     int64     `json:"duration_ms"`
	NodesTotal     int       `json:"nodes_total"`
	NodesCompleted int       `json:"nodes_completed"`
	LastNodeID     string    `json:"last_node_id,omitempty"`
	LastNodeType   string    `json:"last_node_type,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	AgentSummary   string    `json:"agent_summary,omitempty"`
	RiskyCommand   string    `json:"risky_command,omitempty"`
}

type executionContext struct {
	currentServerID string
	lastCondition   *bool
	agent           *WorkflowAgent
}

type nodeExecutionResult struct {
	Output          string
	ConditionResult *bool
}

type taskExecutionInfo struct {
	TaskID     string
	TerminalID string
	Output     string
}

// RunWorkflow creates a WorkflowRun and starts workflow execution in background.
func (e *WorkflowEngine) RunWorkflow(workflowID string) (*model.WorkflowRun, error) {
	if e == nil {
		return nil, ErrWorkflowEngineNil
	}

	id := strings.TrimSpace(workflowID)
	if id == "" {
		return nil, ErrWorkflowIDRequired
	}

	var workflow model.Workflow
	if err := model.DB.First(&workflow, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, ErrWorkflowQueryFailed
	}

	now := e.now()
	run := model.WorkflowRun{
		ID:         uuid.New().String(),
		WorkflowID: id,
		Status:     "running",
		Logs:       "[]",
		StartedAt:  &now,
	}

	if err := model.DB.Create(&run).Error; err != nil {
		return nil, ErrWorkflowStartFailed
	}

	e.appendRunLog(run.ID, runLogEntry{
		Timestamp: now,
		Level:     "info",
		Message:   "Workflow started",
	})

	utils.Info("Workflow run started", zap.String("workflow_id", id), zap.String("run_id", run.ID))

	execution := func() {
		e.executeWorkflowRun(&workflow, run.ID)
	}

	if e.startAsync != nil {
		e.startAsync(execution)
	} else {
		go execution()
	}

	return &run, nil
}

func (e *WorkflowEngine) executeWorkflowRun(workflow *model.Workflow, runID string) {
	startedAt := e.now()
	nodesCompleted := 0
	nodesTotal := 0
	lastNodeID := ""
	lastNodeType := ""

	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("workflow engine panic")
			e.failRun(runID, lastNodeID, err)
			e.appendExecutionReport(runID, executionReport{
				Status:         "failed",
				RunID:          runID,
				WorkflowID:     workflowIDOrEmpty(workflow),
				StartedAt:      startedAt,
				FinishedAt:     e.now(),
				DurationMs:     e.now().Sub(startedAt).Milliseconds(),
				NodesTotal:     nodesTotal,
				NodesCompleted: nodesCompleted,
				LastNodeID:     lastNodeID,
				LastNodeType:   lastNodeType,
				LastError:      safeError(err),
			})
		}
	}()

	if workflow == nil {
		err := errors.New("workflow is nil")
		e.failRun(runID, "", err)
		e.appendExecutionReport(runID, executionReport{
			Status:     "failed",
			RunID:      runID,
			StartedAt:  startedAt,
			FinishedAt: e.now(),
			DurationMs: e.now().Sub(startedAt).Milliseconds(),
			LastError:  safeError(err),
		})
		return
	}

	nodes, edges, err := parseWorkflowDefinition(workflow.Nodes, workflow.Edges)
	if err != nil {
		e.failRun(runID, "", err)
		e.appendExecutionReport(runID, executionReport{
			Status:     "failed",
			RunID:      runID,
			WorkflowID: workflowIDOrEmpty(workflow),
			StartedAt:  startedAt,
			FinishedAt: e.now(),
			DurationMs: e.now().Sub(startedAt).Milliseconds(),
			LastError:  safeError(err),
		})
		return
	}

	nodesTotal = len(nodes)
	if len(nodes) == 0 {
		e.completeRun(runID)
		e.appendExecutionReport(runID, executionReport{
			Status:         "completed",
			RunID:          runID,
			WorkflowID:     workflow.ID,
			StartedAt:      startedAt,
			FinishedAt:     e.now(),
			DurationMs:     e.now().Sub(startedAt).Milliseconds(),
			NodesTotal:     0,
			NodesCompleted: 0,
		})
		return
	}

	nodesByID := make(map[string]workflowNode, len(nodes))
	order := make([]string, 0, len(nodes))
	for _, n := range nodes {
		nodesByID[n.ID] = n
		order = append(order, n.ID)
	}

	outgoing := buildOutgoingEdges(edges)
	startNodeID := findStartNodeID(nodes, edges)

	ctx := &executionContext{}
	if e.newAgent != nil {
		ctx.agent = e.newAgent(e)
	} else {
		ctx.agent = NewWorkflowAgent(e)
	}
	maxSteps := len(nodes) * 4
	steps := 0

	current := startNodeID
	for strings.TrimSpace(current) != "" {
		steps++
		if steps > maxSteps {
			e.failRun(runID, current, errors.New("workflow exceeded max steps (possible loop)"))
			return
		}

		node, ok := nodesByID[current]
		if !ok {
			err := errors.New("workflow node not found")
			e.failRun(runID, current, err)
			e.appendExecutionReport(runID, executionReport{
				Status:         "failed",
				RunID:          runID,
				WorkflowID:     workflow.ID,
				StartedAt:      startedAt,
				FinishedAt:     e.now(),
				DurationMs:     e.now().Sub(startedAt).Milliseconds(),
				NodesTotal:     nodesTotal,
				NodesCompleted: nodesCompleted,
				LastNodeID:     current,
				LastNodeType:   node.Type,
				LastError:      safeError(err),
			})
			return
		}
		lastNodeID = node.ID
		lastNodeType = node.Type

		if err := e.updateRun(runID, map[string]any{"current_node_id": node.ID}); err != nil {
			utils.Warn("Failed to update workflow current node", zap.String("run_id", runID), zap.Error(err))
		}

		e.appendRunLog(runID, runLogEntry{
			Timestamp: e.now(),
			Level:     "info",
			Message:   "Node started",
			NodeID:    node.ID,
			NodeType:  node.Type,
		})

		var result nodeExecutionResult
		var execErr error

		maxRetries := 1
		for attempt := 0; attempt <= maxRetries; attempt++ {
			result, execErr = e.executeNode(runID, node, ctx)
			if execErr == nil {
				break
			}

			var pauseErr *workflowPauseError
			if errors.As(execErr, &pauseErr) {
				e.pauseRun(runID, node, pauseErr, ctx)
				e.appendExecutionReport(runID, executionReport{
					Status:         "paused",
					RunID:          runID,
					WorkflowID:     workflow.ID,
					StartedAt:      startedAt,
					FinishedAt:     e.now(),
					DurationMs:     e.now().Sub(startedAt).Milliseconds(),
					NodesTotal:     nodesTotal,
					NodesCompleted: nodesCompleted,
					LastNodeID:     node.ID,
					LastNodeType:   node.Type,
					AgentSummary:   agentSummaryFromDecision(pauseErr.decision),
					RiskyCommand:   truncateString(redactSensitive(pauseErr.command), 1024),
				})
				return
			}

			if attempt >= maxRetries || ctx == nil || ctx.agent == nil || !isRetryableNode(node) {
				break
			}

			decision := ctx.agent.generateNextStepSuggestion(agentSuggestionContext{
				phase:    suggestionPhaseOnError,
				nodeType: node.Type,
				logs:     result.Output,
				err:      execErr,
			})
			ctx.agent.logDecision(runID, node.ID, node.Type, "", "", decision)

			if decision == nil || decision.Action != AgentActionAdvance {
				break
			}

			e.appendRunLog(runID, runLogEntry{
				Timestamp: e.now(),
				Level:     "warn",
				Message:   fmt.Sprintf("Retrying node (attempt %d/%d)", attempt+1, maxRetries),
				NodeID:    node.ID,
				NodeType:  node.Type,
				Output:    truncateString(decision.Reasoning, 2048),
				Error:     safeError(execErr),
			})
			e.sleepFor(time.Duration(attempt+1) * 500 * time.Millisecond)
		}

		err := execErr
		if err != nil {
			agentReasoning := ""
			if ctx != nil && ctx.agent != nil {
				decision := ctx.agent.generateNextStepSuggestion(agentSuggestionContext{
					phase:    suggestionPhaseOnError,
					nodeType: node.Type,
					logs:     result.Output,
					err:      err,
				})
				if decision != nil {
					ctx.agent.logDecision(runID, node.ID, node.Type, "", "", decision)
					agentReasoning = strings.TrimSpace(decision.Reasoning)
				}
			}

			output := strings.TrimSpace(result.Output)
			if agentReasoning != "" {
				if output == "" {
					output = "Agent: " + agentReasoning
				} else {
					output = strings.TrimSpace(output + "\nAgent: " + agentReasoning)
				}
			}

			e.appendRunLog(runID, runLogEntry{
				Timestamp: e.now(),
				Level:     "error",
				Message:   "Node failed",
				NodeID:    node.ID,
				NodeType:  node.Type,
				Error:     safeError(err),
				Output:    truncateString(output, 4096),
			})
			e.failRun(runID, node.ID, err)
			e.appendExecutionReport(runID, executionReport{
				Status:         "failed",
				RunID:          runID,
				WorkflowID:     workflow.ID,
				StartedAt:      startedAt,
				FinishedAt:     e.now(),
				DurationMs:     e.now().Sub(startedAt).Milliseconds(),
				NodesTotal:     nodesTotal,
				NodesCompleted: nodesCompleted,
				LastNodeID:     node.ID,
				LastNodeType:   node.Type,
				LastError:      safeError(err),
				AgentSummary:   agentReasoning,
			})
			return
		}

		ctx.lastCondition = result.ConditionResult
		nodesCompleted++

		if output := strings.TrimSpace(result.Output); output != "" {
			e.appendRunLog(runID, runLogEntry{
				Timestamp: e.now(),
				Level:     "info",
				Message:   "Node output",
				NodeID:    node.ID,
				NodeType:  node.Type,
				Output:    truncateString(output, 4096),
			})
		}

		e.appendRunLog(runID, runLogEntry{
			Timestamp: e.now(),
			Level:     "info",
			Message:   "Node completed",
			NodeID:    node.ID,
			NodeType:  node.Type,
		})

		next, ok := resolveNextNode(node, result, outgoing[node.ID])
		if ok {
			current = next
			continue
		}

		if len(edges) == 0 {
			current = nextNodeInOrder(node.ID, order)
			continue
		}

		break
	}

	e.completeRun(runID)
	e.appendExecutionReport(runID, executionReport{
		Status:         "completed",
		RunID:          runID,
		WorkflowID:     workflow.ID,
		StartedAt:      startedAt,
		FinishedAt:     e.now(),
		DurationMs:     e.now().Sub(startedAt).Milliseconds(),
		NodesTotal:     nodesTotal,
		NodesCompleted: nodesCompleted,
		LastNodeID:     lastNodeID,
		LastNodeType:   lastNodeType,
	})
}

func (e *WorkflowEngine) executeNode(runID string, node workflowNode, ctx *executionContext) (nodeExecutionResult, error) {
	nodeType := strings.TrimSpace(node.Type)
	if nodeType == model.WorkflowNodeTypeAI {
		nodeType = model.NodeTypeAIAgent
	}

	switch nodeType {
	case model.NodeTypeServer:
		serverID := strings.TrimSpace(node.ServerID)
		if serverID == "" {
			serverID = getString(node.Config, "server_id", "serverId", "id")
		}
		if serverID == "" {
			return nodeExecutionResult{}, errors.New("server_id is required")
		}
		ctx.currentServerID = serverID
		return nodeExecutionResult{Output: fmt.Sprintf("Selected server: %s", serverID)}, nil
	case model.WorkflowNodeTypeCommand:
		cmd := getString(node.Config, "command", "cmd", "shell")
		if cmd == "" {
			return nodeExecutionResult{}, errors.New("command is required")
		}
		if ctx != nil && ctx.agent != nil {
			decision := ctx.agent.generateNextStepSuggestion(agentSuggestionContext{
				phase:    suggestionPhasePreExecute,
				nodeType: nodeType,
				command:  cmd,
			})
			ctx.agent.logDecision(runID, node.ID, node.Type, "", "", decision)
			if decision != nil {
				switch decision.Action {
				case AgentActionWait:
					return nodeExecutionResult{}, &workflowPauseError{decision: decision, command: cmd}
				case AgentActionFail:
					return nodeExecutionResult{}, errors.New(strings.TrimSpace(decision.Reasoning))
				}
			}
		}
		output, err := e.runCommand(node, cmd, ctx)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output}, nil
	case model.NodeTypeTerminal:
		output, err := e.runTerminal(runID, node, ctx)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output}, nil
	case model.NodeTypeGit:
		output, err := e.runGit(runID, node, ctx)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output}, nil
	case model.NodeTypeTask:
		info, err := e.runTask(node, ctx)
		if err != nil {
			return nodeExecutionResult{Output: info.Output}, err
		}

		if ctx == nil || ctx.agent == nil {
			return nodeExecutionResult{Output: info.Output}, nil
		}

		decision, monitorErr := ctx.agent.MonitorTask(context.Background(), runID, node.ID, node.Type, info.TaskID, info.TerminalID)
		output := info.Output
		if decision != nil && strings.TrimSpace(decision.Reasoning) != "" {
			output = strings.TrimSpace(output + "\nAgent: " + decision.Reasoning)
		}
		if monitorErr != nil {
			return nodeExecutionResult{Output: output}, monitorErr
		}
		return nodeExecutionResult{Output: output}, nil
	case model.NodeTypeAIAgent:
		return nodeExecutionResult{Output: "AI agent node placeholder"}, nil
	case model.NodeTypeParallel:
		return nodeExecutionResult{Output: "Parallel node placeholder"}, nil
	case model.NodeTypeWait:
		output, err := e.runWait(node)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output}, nil
	case model.NodeTypeCondition:
		ok, output, err := e.evaluateCondition(node, ctx)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output, ConditionResult: &ok}, nil
	default:
		return nodeExecutionResult{}, fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

func (e *WorkflowEngine) runTerminal(runID string, node workflowNode, ctx *executionContext) (string, error) {
	if e.terminal == nil {
		return "", ErrTerminalManagerNotConfigured
	}

	cmd := strings.TrimSpace(getString(node.Config, "command", "cmd", "shell"))
	if cmd == "" {
		return "", errors.New("command is required")
	}

	serverID := strings.TrimSpace(node.ServerID)
	if serverID == "" {
		serverID = getString(node.Config, "server_id", "serverId")
	}
	if serverID == "" && ctx != nil {
		serverID = strings.TrimSpace(ctx.currentServerID)
	}

	title := strings.TrimSpace(getString(node.Config, "title"))
	if title == "" {
		title = strings.TrimSpace(node.Name)
	}
	if title == "" {
		title = "Workflow Terminal"
	}

	workDir := strings.TrimSpace(getString(node.Config, "work_dir", "workDir"))

	fullCmd := cmd
	if workDir != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", workDir, cmd)
	}

	if ctx != nil && ctx.agent != nil {
		decision := ctx.agent.generateNextStepSuggestion(agentSuggestionContext{
			phase:    suggestionPhasePreExecute,
			nodeType: model.NodeTypeTerminal,
			command:  fullCmd,
		})
		ctx.agent.logDecision(runID, node.ID, node.Type, "", "", decision)
		if decision != nil {
			switch decision.Action {
			case AgentActionWait:
				return "", &workflowPauseError{decision: decision, command: fullCmd}
			case AgentActionFail:
				return "", errors.New(strings.TrimSpace(decision.Reasoning))
			}
		}
	}

	var session terminalSession
	var err error
	if serverID != "" {
		session, err = e.terminal.CreateSSHSession(serverID)
	} else {
		session, err = e.terminal.CreateSession(title, nil)
	}
	if err != nil {
		return "", err
	}

	if serverID != "" && title != "" {
		_ = e.terminal.RenameSession(session.ID(), title)
	}

	if workDir != "" {
		if err := session.Write([]byte(fmt.Sprintf("cd %s\r", workDir))); err != nil {
			return fmt.Sprintf("terminal=%s", session.ID()), err
		}
		e.sleepFor(300 * time.Millisecond)
	}

	if err := session.Write([]byte(cmd + "\r")); err != nil {
		return fmt.Sprintf("terminal=%s", session.ID()), err
	}

	return fmt.Sprintf("Terminal command dispatched: %s (terminal=%s)", cmd, session.ID()), nil
}

func (e *WorkflowEngine) runGit(runID string, node workflowNode, ctx *executionContext) (string, error) {
	command := strings.TrimSpace(getString(node.Config, "command", "cmd", "shell"))
	if command == "" {
		args := strings.TrimSpace(getString(node.Config, "args", "git_args", "gitArgs"))
		if args != "" {
			command = "git " + args
		}
	}

	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("git command is required")
	}

	if command != "git" && !strings.HasPrefix(command, "git ") && !strings.HasPrefix(command, "git\t") {
		command = "git " + command
	}

	workDir := strings.TrimSpace(getString(node.Config, "work_dir", "workDir", "repo_dir", "repoDir"))
	if workDir != "" {
		command = fmt.Sprintf("cd %s && %s", workDir, command)
	}

	if ctx != nil && ctx.agent != nil {
		decision := ctx.agent.generateNextStepSuggestion(agentSuggestionContext{
			phase:    suggestionPhasePreExecute,
			nodeType: model.NodeTypeGit,
			command:  command,
		})
		ctx.agent.logDecision(runID, node.ID, node.Type, "", "", decision)
		if decision != nil {
			switch decision.Action {
			case AgentActionWait:
				return "", &workflowPauseError{decision: decision, command: command}
			case AgentActionFail:
				return "", errors.New(strings.TrimSpace(decision.Reasoning))
			}
		}
	}

	output, err := e.runCommand(node, command, ctx)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (e *WorkflowEngine) runWait(node workflowNode) (string, error) {
	d, err := parseWaitDuration(node.Config)
	if err != nil {
		return "", err
	}
	if d < 0 {
		return "", errors.New("wait duration must be non-negative")
	}
	e.sleepFor(d)
	return fmt.Sprintf("Waited %s", d), nil
}

func (e *WorkflowEngine) sleepFor(d time.Duration) {
	if d <= 0 {
		return
	}
	if e != nil && e.sleep != nil {
		e.sleep(d)
		return
	}
	time.Sleep(d)
}

func (e *WorkflowEngine) runCommand(node workflowNode, command string, ctx *executionContext) (string, error) {
	serverID := strings.TrimSpace(node.ServerID)
	if serverID == "" {
		serverID = getString(node.Config, "server_id", "serverId")
	}
	if serverID == "" {
		serverID = strings.TrimSpace(ctx.currentServerID)
	}

	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", errors.New("command is required")
	}

	if serverID == "" {
		if e.localExecutor == nil {
			return "", ErrLocalExecutorNotConfigured
		}
		return e.localExecutor.Execute(cmd)
	}

	if e.sshManager == nil {
		return "", ErrSSHManagerNotConfigured
	}
	return e.sshManager.ExecuteCommand(serverID, cmd)
}

func (e *WorkflowEngine) runTask(node workflowNode, ctx *executionContext) (taskExecutionInfo, error) {
	if e.automation == nil {
		return taskExecutionInfo{}, ErrTaskAutomationNotConfigured
	}

	taskID := getString(node.Config, "task_id", "taskId")
	if taskID != "" {
		var existing model.Task
		if err := model.DB.First(&existing, "id = ?", taskID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return taskExecutionInfo{}, ErrTaskNotFound
			}
			return taskExecutionInfo{}, ErrTaskQueryFailed
		}
		result, err := e.automation.StartTask(&existing)
		if err != nil {
			return taskExecutionInfo{TaskID: existing.ID}, err
		}
		terminalID := ""
		if result != nil && result.Terminal != nil {
			terminalID = result.Terminal.ID()
		}
		output := fmt.Sprintf("Task started: %s (terminal=%s)", existing.ID, terminalID)
		return taskExecutionInfo{TaskID: existing.ID, TerminalID: terminalID, Output: output}, nil
	}

	title := strings.TrimSpace(getString(node.Config, "title"))
	if title == "" {
		title = strings.TrimSpace(node.Name)
	}
	if title == "" {
		title = "Workflow Task"
	}

	cliType := strings.TrimSpace(getString(node.Config, "cli_type", "cliType"))
	if cliType == "" {
		cliType = "claude"
	}

	workDir := strings.TrimSpace(getString(node.Config, "work_dir", "workDir"))
	initialPrompt := strings.TrimSpace(getString(node.Config, "initial_prompt", "initialPrompt"))
	autoCreateDir := getBool(node.Config, true, "auto_create_dir", "autoCreateDir")

	var serverID *string
	sid := strings.TrimSpace(node.ServerID)
	if sid == "" {
		sid = getString(node.Config, "server_id", "serverId")
	}
	if sid == "" {
		sid = strings.TrimSpace(ctx.currentServerID)
	}
	if sid != "" {
		var server model.SSHServer
		if err := model.DB.Select("id").First(&server, "id = ?", sid).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return taskExecutionInfo{}, ErrServerNotFound
			}
			return taskExecutionInfo{}, ErrServerQueryFailed
		}
		serverID = &sid
	}

	taskModel := model.Task{
		ID:            uuid.New().String(),
		Title:         title,
		Description:   getString(node.Config, "description"),
		Status:        "todo",
		Priority:      0,
		OrderIndex:    float64(e.now().UnixNano()),
		ServerID:      serverID,
		WorkDir:       workDir,
		CLIType:       cliType,
		InitialPrompt: initialPrompt,
		AutoStart:     true,
		AutoCreateDir: autoCreateDir,
	}

	if err := model.DB.Create(&taskModel).Error; err != nil {
		return taskExecutionInfo{}, errors.New("failed to create task")
	}

	result, err := e.automation.StartTask(&taskModel)
	if err != nil {
		return taskExecutionInfo{TaskID: taskModel.ID}, err
	}

	terminalID := ""
	if result != nil && result.Terminal != nil {
		terminalID = result.Terminal.ID()
	}
	output := fmt.Sprintf("Task created and started: %s (terminal=%s)", taskModel.ID, terminalID)
	return taskExecutionInfo{TaskID: taskModel.ID, TerminalID: terminalID, Output: output}, nil
}

func parseWaitDuration(cfg map[string]any) (time.Duration, error) {
	if cfg == nil {
		return 0, errors.New("wait config is required")
	}

	if raw := strings.TrimSpace(getString(cfg, "duration")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err == nil {
			return d, nil
		}
	}

	if seconds, ok := getNumber(cfg, "seconds", "sec", "s"); ok {
		return time.Duration(seconds * float64(time.Second)), nil
	}

	if ms, ok := getNumber(cfg, "duration_ms", "durationMs", "ms", "milliseconds"); ok {
		return time.Duration(ms * float64(time.Millisecond)), nil
	}

	return 0, errors.New("wait duration is required")
}

func getNumber(m map[string]any, keys ...string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		case json.Number:
			parsed, err := v.Float64()
			if err == nil {
				return parsed, true
			}
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				continue
			}
			parsed, err := strconv.ParseFloat(trimmed, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func (e *WorkflowEngine) evaluateCondition(node workflowNode, ctx *executionContext) (bool, string, error) {
	if raw, ok := node.Config["value"]; ok {
		switch v := raw.(type) {
		case bool:
			return v, "", nil
		case string:
			parsed, err := parseBool(v)
			if err == nil {
				return parsed, "", nil
			}
		}
	}

	expr := getString(node.Config, "expression", "expr", "condition")
	if expr != "" {
		parsed, err := parseBool(expr)
		if err == nil {
			return parsed, "", nil
		}
	}

	cmd := getString(node.Config, "command", "cmd", "shell")
	if cmd == "" {
		return false, "", errors.New("condition requires value/expression/command")
	}

	output, err := e.runCommand(node, cmd, ctx)
	if err != nil {
		if isCommandExitError(err) {
			return false, output, nil
		}
		return false, output, err
	}
	return true, output, nil
}

func isCommandExitError(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.ExitError
	if errors.As(err, &execErr) {
		return true
	}
	var sshErr *cryptossh.ExitError
	if errors.As(err, &sshErr) {
		return true
	}
	return false
}

func resolveNextNode(node workflowNode, result nodeExecutionResult, edges []workflowEdge) (string, bool) {
	if len(edges) == 0 {
		return "", false
	}

	if node.Type != model.WorkflowNodeTypeCondition || result.ConditionResult == nil {
		return edges[0].Target, strings.TrimSpace(edges[0].Target) != ""
	}

	wantTrue := *result.ConditionResult
	for _, edge := range edges {
		if matchConditionEdge(edge, wantTrue) {
			return edge.Target, strings.TrimSpace(edge.Target) != ""
		}
	}

	if len(edges) == 1 {
		return edges[0].Target, strings.TrimSpace(edges[0].Target) != ""
	}
	if len(edges) >= 2 {
		if wantTrue {
			return edges[0].Target, strings.TrimSpace(edges[0].Target) != ""
		}
		return edges[1].Target, strings.TrimSpace(edges[1].Target) != ""
	}

	return "", false
}

func matchConditionEdge(edge workflowEdge, wantTrue bool) bool {
	candidates := []string{
		edge.Condition,
		edge.SourceHandle,
		edge.Label,
	}

	for _, raw := range candidates {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if wantTrue && (value == "true" || value == "yes" || value == "1" || value == "success") {
			return true
		}
		if !wantTrue && (value == "false" || value == "no" || value == "0" || value == "fail" || value == "failure") {
			return true
		}
	}

	return false
}

func nextNodeInOrder(current string, order []string) string {
	for i, id := range order {
		if id != current {
			continue
		}
		if i+1 < len(order) {
			return order[i+1]
		}
		return ""
	}
	return ""
}

func findStartNodeID(nodes []workflowNode, edges []workflowEdge) string {
	inDegree := make(map[string]int, len(nodes))
	for _, n := range nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range edges {
		target := strings.TrimSpace(e.Target)
		if target == "" {
			continue
		}
		if _, ok := inDegree[target]; ok {
			inDegree[target]++
		}
	}

	for _, n := range nodes {
		if inDegree[n.ID] == 0 {
			return n.ID
		}
	}
	if len(nodes) > 0 {
		return nodes[0].ID
	}
	return ""
}

func buildOutgoingEdges(edges []workflowEdge) map[string][]workflowEdge {
	outgoing := make(map[string][]workflowEdge)
	for _, e := range edges {
		src := strings.TrimSpace(e.Source)
		if src == "" {
			continue
		}
		outgoing[src] = append(outgoing[src], e)
	}
	return outgoing
}

func parseWorkflowDefinition(nodesJSON, edgesJSON string) ([]workflowNode, []workflowEdge, error) {
	nodes, err := parseNodes(nodesJSON)
	if err != nil {
		return nil, nil, errors.New("invalid workflow nodes")
	}
	edges, err := parseEdges(edgesJSON)
	if err != nil {
		return nil, nil, errors.New("invalid workflow edges")
	}
	return nodes, edges, nil
}

func parseNodes(raw string) ([]workflowNode, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "[]"
	}

	var items []any
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, err
	}

	nodes := make([]workflowNode, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		node, err := parseNode(obj)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func parseNode(raw map[string]any) (workflowNode, error) {
	id := getString(raw, "id")
	if id == "" {
		return workflowNode{}, errors.New("node id is required")
	}

	nodeType := getString(raw, "type", "node_type", "nodeType")
	nodeType = strings.TrimSpace(nodeType)
	if nodeType == "" {
		nodeType = model.WorkflowNodeTypeCommand
	}

	name := strings.TrimSpace(getString(raw, "name"))
	if name == "" {
		if dataMap, ok := raw["data"].(map[string]any); ok {
			name = strings.TrimSpace(getString(dataMap, "label", "name"))
		}
	}

	serverID := strings.TrimSpace(getString(raw, "server_id", "serverId"))
	if serverID == "" {
		if dataMap, ok := raw["data"].(map[string]any); ok {
			serverID = strings.TrimSpace(getString(dataMap, "server_id", "serverId"))
		}
	}

	config := extractConfig(raw)

	return workflowNode{
		ID:       id,
		Type:     nodeType,
		Name:     name,
		ServerID: serverID,
		Config:   config,
		raw:      raw,
	}, nil
}

func extractConfig(raw map[string]any) map[string]any {
	if cfg, ok := decodeObject(raw["config"]); ok {
		return cfg
	}

	if data, ok := raw["data"].(map[string]any); ok {
		if cfg, ok := decodeObject(data["config"]); ok {
			return cfg
		}
	}

	return map[string]any{}
}

func decodeObject(value any) (map[string]any, bool) {
	switch v := value.(type) {
	case map[string]any:
		return v, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, false
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func parseEdges(raw string) ([]workflowEdge, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "[]"
	}

	var items []any
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, err
	}

	edges := make([]workflowEdge, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		edge, err := parseEdge(obj)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

func parseEdge(raw map[string]any) (workflowEdge, error) {
	source := strings.TrimSpace(getString(raw, "source"))
	target := strings.TrimSpace(getString(raw, "target"))
	if source == "" || target == "" {
		return workflowEdge{}, errors.New("edge source/target is required")
	}

	return workflowEdge{
		ID:           getString(raw, "id"),
		Source:       source,
		Target:       target,
		SourceHandle: getString(raw, "sourceHandle", "source_handle"),
		Label:        getString(raw, "label"),
		Condition:    getString(raw, "condition"),
	}, nil
}

func getString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case string:
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(v.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func getBool(m map[string]any, defaultValue bool, keys ...string) bool {
	if m == nil {
		return defaultValue
	}
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case bool:
			return v
		case string:
			parsed, err := parseBool(v)
			if err == nil {
				return parsed
			}
		}
	}
	return defaultValue
}

func parseBool(value string) (bool, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "true", "1", "yes", "y", "ok", "success":
		return true, nil
	case "false", "0", "no", "n", "fail", "failure":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool: %s", value)
	}
}

func (e *WorkflowEngine) appendRunLog(runID string, entry runLogEntry) {
	if strings.TrimSpace(runID) == "" {
		return
	}

	var run model.WorkflowRun
	if err := model.DB.Select("id", "logs").First(&run, "id = ?", runID).Error; err != nil {
		return
	}

	var logs []runLogEntry
	raw := strings.TrimSpace(run.Logs)
	if raw == "" {
		raw = "[]"
	}
	_ = json.Unmarshal([]byte(raw), &logs)
	logs = append(logs, entry)

	encoded, err := json.Marshal(logs)
	if err != nil {
		return
	}

	_ = model.DB.Model(&model.WorkflowRun{}).Where("id = ?", runID).Update("logs", string(encoded)).Error
}

func (e *WorkflowEngine) updateRun(runID string, updates map[string]any) error {
	if strings.TrimSpace(runID) == "" {
		return errors.New("runID is required")
	}
	if len(updates) == 0 {
		return nil
	}
	return model.DB.Model(&model.WorkflowRun{}).Where("id = ?", runID).Updates(updates).Error
}

func (e *WorkflowEngine) failRun(runID, nodeID string, err error) {
	now := e.now()

	e.appendRunLog(runID, runLogEntry{
		Timestamp: now,
		Level:     "error",
		Message:   "Workflow failed",
		NodeID:    strings.TrimSpace(nodeID),
		Error:     safeError(err),
	})

	updates := map[string]any{
		"status":       "failed",
		"completed_at": &now,
	}
	_ = e.updateRun(runID, updates)

	utils.Error("Workflow run failed", zap.String("run_id", runID), zap.String("node_id", nodeID), zap.Error(err))
}

func (e *WorkflowEngine) completeRun(runID string) {
	now := e.now()

	e.appendRunLog(runID, runLogEntry{
		Timestamp: now,
		Level:     "info",
		Message:   "Workflow completed",
	})

	updates := map[string]any{
		"status":          "completed",
		"current_node_id": nil,
		"completed_at":    &now,
	}
	_ = e.updateRun(runID, updates)

	utils.Info("Workflow run completed", zap.String("run_id", runID))
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "unknown error"
	}
	return truncateString(msg, 1024)
}

func truncateString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func (e *WorkflowEngine) pauseRun(runID string, node workflowNode, pauseErr *workflowPauseError, ctx *executionContext) {
	if strings.TrimSpace(runID) == "" {
		return
	}

	now := e.now()
	reason := "workflow paused"
	command := ""
	var decision *AgentDecision
	if pauseErr != nil {
		command = pauseErr.command
		decision = pauseErr.decision
	}
	if decision != nil && strings.TrimSpace(decision.Reasoning) != "" {
		reason = strings.TrimSpace(decision.Reasoning)
	}

	output := reason
	if strings.TrimSpace(command) != "" {
		output = strings.TrimSpace(output + "\nCommand: " + redactSensitive(command))
	}

	e.appendRunLog(runID, runLogEntry{
		Timestamp: now,
		Level:     "warn",
		Message:   "Workflow paused",
		NodeID:    node.ID,
		NodeType:  node.Type,
		Output:    truncateString(output, 4096),
	})

	updates := map[string]any{
		"status": "paused",
	}
	_ = e.updateRun(runID, updates)

	utils.Warn("Workflow run paused",
		zap.String("run_id", runID),
		zap.String("node_id", node.ID),
		zap.String("node_type", node.Type),
		zap.String("reason", truncateString(reason, 512)))

	e.notifyRiskyOperation(runID, node, decision, command, ctx)
}

func (e *WorkflowEngine) notifyRiskyOperation(runID string, node workflowNode, decision *AgentDecision, command string, ctx *executionContext) {
	if model.DB == nil || strings.TrimSpace(runID) == "" {
		return
	}

	now := e.now()
	title := "Workflow paused: risky operation"
	content := "Workflow paused pending manual review."
	if decision != nil && strings.TrimSpace(decision.Reasoning) != "" {
		content = strings.TrimSpace(decision.Reasoning)
	}

	var serverID *string
	sid := strings.TrimSpace(node.ServerID)
	if sid == "" {
		sid = getString(node.Config, "server_id", "serverId")
	}
	if sid == "" && ctx != nil {
		sid = strings.TrimSpace(ctx.currentServerID)
	}
	if sid != "" {
		serverID = &sid
	}

	msg := &model.Message{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		Type:      "approval_needed",
		Title:     title,
		Content:   truncateString(content, 2048),
		Context:   truncateString(fmt.Sprintf("run_id=%s node_id=%s node_type=%s command=%s", runID, node.ID, node.Type, redactSensitive(command)), 4096),
		Status:    "unread",
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := model.DB.Create(msg).Error; err != nil {
		utils.Warn("Failed to save workflow pause notification",
			zap.String("run_id", runID),
			zap.Error(err))
	}
}

func workflowIDOrEmpty(workflow *model.Workflow) string {
	if workflow == nil {
		return ""
	}
	return strings.TrimSpace(workflow.ID)
}

func isRetryableNode(node workflowNode) bool {
	nodeType := strings.TrimSpace(node.Type)
	switch nodeType {
	case model.WorkflowNodeTypeCommand, model.NodeTypeTerminal, model.NodeTypeGit:
		return true
	default:
		return false
	}
}

func agentSummaryFromDecision(decision *AgentDecision) string {
	if decision == nil {
		return ""
	}
	return truncateString(strings.TrimSpace(decision.Reasoning), 1024)
}

func (e *WorkflowEngine) appendExecutionReport(runID string, report executionReport) {
	if strings.TrimSpace(runID) == "" {
		return
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		return
	}

	level := "info"
	switch strings.ToLower(strings.TrimSpace(report.Status)) {
	case "failed":
		level = "error"
	case "paused":
		level = "warn"
	}

	e.appendRunLog(runID, runLogEntry{
		Timestamp: report.FinishedAt,
		Level:     level,
		Message:   "Execution report",
		NodeID:    report.LastNodeID,
		NodeType:  report.LastNodeType,
		Output:    truncateString(string(encoded), 4096),
	})
}

var sensitiveTokenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|token|api[_-]?key|secret)\s*=\s*[^\s]+`),
	regexp.MustCompile(`(?i)--(password|token|api[_-]?key|secret)(=|\s+)([^\s]+)`),
	regexp.MustCompile(`(?i)authorization:\s*bearer\s+[^\s]+`),
}

func redactSensitive(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	redacted := text
	for _, re := range sensitiveTokenPatterns {
		if re == nil {
			continue
		}
		redacted = re.ReplaceAllStringFunc(redacted, func(match string) string {
			lower := strings.ToLower(match)
			if strings.HasPrefix(lower, "authorization:") {
				return "authorization: bearer ***"
			}
			if strings.Contains(match, "=") {
				parts := strings.SplitN(match, "=", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[0]) + "=***"
				}
			}
			if strings.Contains(lower, "--") {
				fields := strings.Fields(match)
				if len(fields) >= 1 {
					return fields[0] + " ***"
				}
			}
			return "***"
		})
	}
	return redacted
}
