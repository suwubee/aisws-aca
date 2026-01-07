package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	sshservice "github.com/ai-coding-assistant/service/ssh"
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
	ErrWorkflowEngineNil           = errors.New("workflow engine is nil")
	ErrWorkflowIDRequired          = errors.New("workflowID is required")
	ErrWorkflowNotFound            = errors.New("workflow not found")
	ErrWorkflowQueryFailed         = errors.New("failed to query workflow")
	ErrWorkflowStartFailed         = errors.New("failed to start workflow")
	ErrSSHManagerNotConfigured     = errors.New("ssh manager is not configured")
	ErrLocalExecutorNotConfigured  = errors.New("local executor is not configured")
	ErrTaskAutomationNotConfigured = errors.New("task automation is not configured")
	ErrTaskNotFound                = errors.New("task not found")
	ErrTaskQueryFailed             = errors.New("failed to query task")
	ErrServerNotFound              = errors.New("server not found")
	ErrServerQueryFailed           = errors.New("failed to query server")
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

// WorkflowEngine executes workflow definitions and updates WorkflowRun status/logs.
type WorkflowEngine struct {
	sshManager sshCommandExecutor
	automation automationService

	localExecutor commandExecutor
	now           func() time.Time
	startAsync    func(fn func())
	newAgent      func(engine *WorkflowEngine) *WorkflowAgent
}

// NewWorkflowEngine creates a workflow engine.
func NewWorkflowEngine(sshManager *sshservice.SSHManager, automation *taskservice.AutomationService) *WorkflowEngine {
	engine := &WorkflowEngine{
		sshManager:    sshManager,
		automation:    automation,
		localExecutor: localShellExecutor{},
		now:           time.Now,
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
	defer func() {
		if recovered := recover(); recovered != nil {
			e.failRun(runID, "", fmt.Errorf("workflow engine panic"))
		}
	}()

	if workflow == nil {
		e.failRun(runID, "", errors.New("workflow is nil"))
		return
	}

	nodes, edges, err := parseWorkflowDefinition(workflow.Nodes, workflow.Edges)
	if err != nil {
		e.failRun(runID, "", err)
		return
	}

	if len(nodes) == 0 {
		e.completeRun(runID)
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
			e.failRun(runID, current, errors.New("workflow node not found"))
			return
		}

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

		result, err := e.executeNode(runID, node, ctx)
		if err != nil {
			e.appendRunLog(runID, runLogEntry{
				Timestamp: e.now(),
				Level:     "error",
				Message:   "Node failed",
				NodeID:    node.ID,
				NodeType:  node.Type,
				Error:     safeError(err),
				Output:    truncateString(result.Output, 4096),
			})
			e.failRun(runID, node.ID, err)
			return
		}

		ctx.lastCondition = result.ConditionResult

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
}

func (e *WorkflowEngine) executeNode(runID string, node workflowNode, ctx *executionContext) (nodeExecutionResult, error) {
	switch node.Type {
	case model.WorkflowNodeTypeServer:
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
		output, err := e.runCommand(node, cmd, ctx)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output}, nil
	case model.WorkflowNodeTypeTask:
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
	case model.WorkflowNodeTypeAI:
		return nodeExecutionResult{Output: "AI node not implemented"}, nil
	case model.WorkflowNodeTypeCondition:
		ok, output, err := e.evaluateCondition(node, ctx)
		if err != nil {
			return nodeExecutionResult{Output: output}, err
		}
		return nodeExecutionResult{Output: output, ConditionResult: &ok}, nil
	default:
		return nodeExecutionResult{}, fmt.Errorf("unsupported node type: %s", node.Type)
	}
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
