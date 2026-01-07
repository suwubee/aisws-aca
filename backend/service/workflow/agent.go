package workflow

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/ai-coding-assistant/utils"
	"go.uber.org/zap"
)

type AgentStatus string

const (
	AgentStatusIdle       AgentStatus = "idle"
	AgentStatusMonitoring AgentStatus = "monitoring"
	AgentStatusDeciding   AgentStatus = "deciding"
	AgentStatusExecuting  AgentStatus = "executing"
)

type AgentAction string

const (
	AgentActionWait    AgentAction = "wait"
	AgentActionAdvance AgentAction = "advance"
	AgentActionFail    AgentAction = "fail"
)

type AgentDecision struct {
	Action         AgentAction `json:"action"`
	Reasoning      string      `json:"reasoning"`
	MatchedPattern string      `json:"matched_pattern,omitempty"`
	ApprovalPrompt string      `json:"approval_prompt,omitempty"`
}

type WorkflowAgent struct {
	engine       *WorkflowEngine
	detector     *detector.Detector
	pollInterval time.Duration
	logLimit     int
	maxWait      time.Duration

	statusMu sync.RWMutex
	status   AgentStatus

	now   func() time.Time
	sleep func(time.Duration)

	loadTerminalLogs   func(ctx context.Context, terminalID string, limit int) (string, error)
	loadTaskStatus     func(ctx context.Context, taskID string) (string, error)
	loadTerminalStatus func(ctx context.Context, terminalID string) (string, error)

	completionPatterns []*regexp.Regexp
	failurePatterns    []*regexp.Regexp
}

func NewWorkflowAgent(engine *WorkflowEngine) *WorkflowAgent {
	agent := &WorkflowAgent{
		engine:       engine,
		detector:     detector.NewDetector(),
		pollInterval: 2 * time.Second,
		logLimit:     200,
		maxWait:      0,
		status:       AgentStatusIdle,
		now:          time.Now,
		sleep:        time.Sleep,
	}

	agent.completionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(task|workflow)\s+(completed|complete|finished|done)\b`),
		regexp.MustCompile(`(?i)\bcompleted\s+successfully\b`),
		regexp.MustCompile(`(?i)\ball\s+done\b`),
		regexp.MustCompile(`(?i)\bexit\s+code\s*0\b`),
		regexp.MustCompile(`(?i)\bexited\s+with\s+code\s*0\b`),
	}

	agent.failurePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpanic:`),
		regexp.MustCompile(`(?i)\bfatal:`),
		regexp.MustCompile(`(?i)\btraceback\b`),
		regexp.MustCompile(`(?i)\bexception\b`),
		regexp.MustCompile(`(?i)\bsegmentation\s+fault\b`),
		regexp.MustCompile(`(?i)\berror:`),
		regexp.MustCompile(`(?i)\bfailed\b`),
		regexp.MustCompile(`(?i)\bfailure\b`),
		regexp.MustCompile(`(?i)\b(exit\s+code|exit\s+status)\s*[1-9]\d*\b`),
	}

	agent.loadTerminalLogs = agent.loadTerminalLogsFromDB
	agent.loadTaskStatus = agent.loadTaskStatusFromDB
	agent.loadTerminalStatus = agent.loadTerminalStatusFromDB

	return agent
}

func (a *WorkflowAgent) Status() AgentStatus {
	a.statusMu.RLock()
	defer a.statusMu.RUnlock()
	return a.status
}

func (a *WorkflowAgent) setStatus(status AgentStatus) {
	a.statusMu.Lock()
	a.status = status
	a.statusMu.Unlock()
}

func (a *WorkflowAgent) MonitorTask(ctx context.Context, runID, nodeID, nodeType, taskID, terminalID string) (*AgentDecision, error) {
	if a == nil {
		return nil, errors.New("workflow agent is nil")
	}

	tid := strings.TrimSpace(terminalID)
	if tid == "" {
		return nil, errors.New("terminalID is required")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	a.setStatus(AgentStatusMonitoring)
	defer a.setStatus(AgentStatusIdle)

	start := a.now()
	lastDecisionKey := ""

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if a.maxWait > 0 && a.now().Sub(start) > a.maxWait {
			decision := &AgentDecision{
				Action:    AgentActionFail,
				Reasoning: "timeout waiting for task to complete",
			}
			a.logDecision(runID, nodeID, nodeType, taskID, tid, decision)
			return decision, errors.New("workflow agent timeout waiting for task completion")
		}

		logText, err := a.loadTerminalLogs(ctx, tid, a.logLimit)
		if err != nil {
			utils.Warn("Workflow agent failed to load terminal logs",
				zap.String("run_id", runID),
				zap.String("node_id", nodeID),
				zap.String("terminal_id", tid),
				zap.Error(err))
		}

		taskStatus, _ := a.loadTaskStatus(ctx, taskID)
		terminalStatus, _ := a.loadTerminalStatus(ctx, tid)

		a.setStatus(AgentStatusDeciding)
		decision := a.shouldStartNextNode(logText, taskStatus, terminalStatus)

		key := string(decision.Action) + ":" + decision.MatchedPattern
		if key != lastDecisionKey {
			a.logDecision(runID, nodeID, nodeType, taskID, tid, decision)
			lastDecisionKey = key
		}

		switch decision.Action {
		case AgentActionAdvance:
			a.setStatus(AgentStatusExecuting)
			return decision, nil
		case AgentActionFail:
			a.setStatus(AgentStatusExecuting)
			return decision, errors.New("task failed: " + decision.Reasoning)
		default:
			a.setStatus(AgentStatusMonitoring)
			interval := a.pollInterval
			if interval <= 0 {
				interval = 1 * time.Second
			}
			a.sleep(interval)
		}
	}
}

func (a *WorkflowAgent) logDecision(runID, nodeID, nodeType, taskID, terminalID string, decision *AgentDecision) {
	if a == nil || decision == nil {
		return
	}

	fields := []zap.Field{
		zap.String("run_id", runID),
		zap.String("node_id", nodeID),
		zap.String("node_type", nodeType),
		zap.String("task_id", strings.TrimSpace(taskID)),
		zap.String("terminal_id", strings.TrimSpace(terminalID)),
		zap.String("agent_status", string(a.Status())),
		zap.String("decision", string(decision.Action)),
	}

	if strings.TrimSpace(decision.MatchedPattern) != "" {
		fields = append(fields, zap.String("matched_pattern", decision.MatchedPattern))
	}
	if strings.TrimSpace(decision.ApprovalPrompt) != "" {
		fields = append(fields, zap.String("approval_prompt", truncateString(decision.ApprovalPrompt, 256)))
	}

	msg := strings.TrimSpace(decision.Reasoning)
	if msg == "" {
		msg = "workflow agent decision"
	}

	switch decision.Action {
	case AgentActionFail:
		utils.Warn(msg, fields...)
	default:
		utils.Info(msg, fields...)
	}

	if a.engine == nil || strings.TrimSpace(runID) == "" {
		return
	}

	level := "info"
	if decision.Action == AgentActionFail {
		level = "error"
	}
	if decision.Action == AgentActionWait && strings.TrimSpace(decision.ApprovalPrompt) != "" {
		level = "warn"
	}

	a.engine.appendRunLog(runID, runLogEntry{
		Timestamp: a.now(),
		Level:     level,
		Message:   "Agent decision",
		NodeID:    strings.TrimSpace(nodeID),
		NodeType:  strings.TrimSpace(nodeType),
		Output:    truncateString(decision.Reasoning, 2048),
		Error:     truncateString(decision.MatchedPattern, 512),
	})
}

func (a *WorkflowAgent) detectTaskCompletion(logs string) (bool, string) {
	text := strings.TrimSpace(logs)
	if text == "" {
		return false, ""
	}

	matched, pattern, _ := lastMatch(text, a.completionPatterns)
	return matched, pattern
}

func (a *WorkflowAgent) detectTaskFailure(logs string) (bool, string) {
	text := strings.TrimSpace(logs)
	if text == "" {
		return false, ""
	}

	matched, pattern, _ := lastMatch(text, a.failurePatterns)
	return matched, pattern
}

func (a *WorkflowAgent) detectApprovalNeeded(logs string) (bool, string) {
	text := strings.TrimSpace(logs)
	if text == "" {
		return false, ""
	}

	state, prompt := a.detector.DetectState(text)
	if state == detector.StateWaitingApproval {
		return true, prompt
	}
	return false, ""
}

func (a *WorkflowAgent) shouldStartNextNode(logs, taskStatus, terminalStatus string) *AgentDecision {
	text := strings.TrimSpace(logs)
	completed, completionPattern, completionPos := lastMatch(text, a.completionPatterns)
	failed, failurePattern, failurePos := lastMatch(text, a.failurePatterns)
	if completed || failed {
		if completed && (!failed || completionPos > failurePos) {
			return &AgentDecision{
				Action:         AgentActionAdvance,
				Reasoning:      "task completion detected in logs",
				MatchedPattern: completionPattern,
			}
		}
		return &AgentDecision{
			Action:         AgentActionFail,
			Reasoning:      "task failure detected in logs",
			MatchedPattern: failurePattern,
		}
	}

	status := strings.ToLower(strings.TrimSpace(taskStatus))
	if status == "done" || status == "archived" {
		return &AgentDecision{
			Action:    AgentActionAdvance,
			Reasoning: fmt.Sprintf("task status indicates completion (%s)", status),
		}
	}

	tStatus := strings.ToLower(strings.TrimSpace(terminalStatus))
	if tStatus == "exited" {
		return &AgentDecision{
			Action:    AgentActionAdvance,
			Reasoning: "terminal session exited; assuming task completed",
		}
	}

	if approvalNeeded, prompt := a.detectApprovalNeeded(logs); approvalNeeded {
		return &AgentDecision{
			Action:         AgentActionWait,
			Reasoning:      "approval prompt detected; waiting for resolution",
			ApprovalPrompt: prompt,
		}
	}

	return &AgentDecision{
		Action:    AgentActionWait,
		Reasoning: "no completion/failure detected; continue monitoring",
	}
}

func lastMatch(text string, patterns []*regexp.Regexp) (matched bool, pattern string, position int) {
	if strings.TrimSpace(text) == "" || len(patterns) == 0 {
		return false, "", -1
	}

	bestPos := -1
	bestPattern := ""
	for _, re := range patterns {
		if re == nil {
			continue
		}
		indices := re.FindAllStringIndex(text, -1)
		if len(indices) == 0 {
			continue
		}
		last := indices[len(indices)-1]
		if len(last) != 2 {
			continue
		}
		if last[0] > bestPos {
			bestPos = last[0]
			bestPattern = re.String()
		}
	}

	if bestPos < 0 {
		return false, "", -1
	}
	return true, bestPattern, bestPos
}

func (a *WorkflowAgent) loadTerminalLogsFromDB(_ context.Context, terminalID string, limit int) (string, error) {
	if strings.TrimSpace(terminalID) == "" {
		return "", errors.New("terminalID is required")
	}
	if model.DB == nil {
		return "", errors.New("database not initialized")
	}

	max := limit
	if max <= 0 {
		max = 200
	}

	var rows []model.Log
	if err := model.DB.
		Where("terminal_id = ? AND log_type IN ?", terminalID, []string{"output", "system"}).
		Order("created_at desc").
		Limit(max).
		Find(&rows).Error; err != nil {
		return "", err
	}

	if len(rows) == 0 {
		return "", nil
	}

	var b strings.Builder
	for i := len(rows) - 1; i >= 0; i-- {
		content := strings.TrimSpace(rows[i].Content)
		if content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(content)
	}
	return b.String(), nil
}

func (a *WorkflowAgent) loadTaskStatusFromDB(_ context.Context, taskID string) (string, error) {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return "", nil
	}
	if model.DB == nil {
		return "", errors.New("database not initialized")
	}

	var task model.Task
	if err := model.DB.Select("status").First(&task, "id = ?", id).Error; err != nil {
		return "", err
	}
	return task.Status, nil
}

func (a *WorkflowAgent) loadTerminalStatusFromDB(_ context.Context, terminalID string) (string, error) {
	id := strings.TrimSpace(terminalID)
	if id == "" {
		return "", nil
	}
	if model.DB == nil {
		return "", errors.New("database not initialized")
	}

	var session model.TerminalSession
	if err := model.DB.Select("status").First(&session, "id = ?", id).Error; err != nil {
		return "", err
	}
	return session.Status, nil
}
