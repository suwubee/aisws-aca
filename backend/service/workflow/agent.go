package workflow

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
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
	Confidence     float64     `json:"confidence,omitempty"`
	SuggestedInput string      `json:"suggested_input,omitempty"`
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
	sendTerminalInput  func(ctx context.Context, terminalID, input string) error

	completionPatterns []*regexp.Regexp
	failurePatterns    []*regexp.Regexp
	progressPatterns   []*regexp.Regexp
	errorPatterns      []errorPattern
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

	agent.progressPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?m)(\d{1,3})\s*%`),
		regexp.MustCompile(`(?mi)\b(?:step|steps?|phase|phases?|item|items?)\s*(\d{1,6})\s*(?:/|of)\s*(\d{1,6})\b`),
		regexp.MustCompile(`(?m)\b(\d{1,6})\s*/\s*(\d{1,6})\b`),
	}

	agent.errorPatterns = defaultErrorPatterns()

	agent.loadTerminalLogs = agent.loadTerminalLogsFromDB
	agent.loadTaskStatus = agent.loadTaskStatusFromDB
	agent.loadTerminalStatus = agent.loadTerminalStatusFromDB
	agent.sendTerminalInput = agent.sendTerminalInputViaEngine

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
	lastAutoInputKey := ""

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

		if decision != nil && decision.Action == AgentActionWait && decision.SuggestedInput != "" {
			autoKey := truncateString(decision.ApprovalPrompt, 128) + "|" + formatSuggestedInput(decision.SuggestedInput)
			if autoKey != lastAutoInputKey && a.sendTerminalInput != nil {
				if err := a.sendTerminalInput(ctx, tid, decision.SuggestedInput); err != nil {
					utils.Warn("Workflow agent failed to send suggested input",
						zap.String("run_id", runID),
						zap.String("node_id", nodeID),
						zap.String("terminal_id", tid),
						zap.Error(err))
				} else {
					a.logDecision(runID, nodeID, nodeType, taskID, tid, decision)
					lastAutoInputKey = autoKey
					lastDecisionKey = string(decision.Action) + ":" + decision.MatchedPattern
					a.setStatus(AgentStatusMonitoring)
					interval := a.pollInterval
					if interval <= 0 {
						interval = 1 * time.Second
					}
					a.sleep(interval)
					continue
				}
			}
		}

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
	if decision.Confidence > 0 {
		fields = append(fields, zap.Float64("confidence", decision.Confidence))
	}
	if decision.SuggestedInput != "" {
		fields = append(fields, zap.String("suggested_input", truncateString(formatSuggestedInput(decision.SuggestedInput), 64)))
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
				Confidence:     0.9,
				MatchedPattern: completionPattern,
			}
		}

		errorInsight := a.detectErrors(text)
		reasoning := "task failure detected in logs"
		if errorInsight != nil {
			reasoning = strings.TrimSpace(reasoning + ": " + errorInsight.summary)
			if strings.TrimSpace(errorInsight.suggestion) != "" {
				reasoning = strings.TrimSpace(reasoning + " (suggestion: " + errorInsight.suggestion + ")")
			}
		}
		return &AgentDecision{
			Action:         AgentActionFail,
			Reasoning:      reasoning,
			Confidence:     confidenceOrDefault(errorInsight, 0.8),
			MatchedPattern: failurePattern,
		}
	}

	status := strings.ToLower(strings.TrimSpace(taskStatus))
	if status == "done" || status == "archived" {
		return &AgentDecision{
			Action:    AgentActionAdvance,
			Reasoning: fmt.Sprintf("task status indicates completion (%s)", status),
			Confidence: func() float64 {
				if status == "done" {
					return 0.85
				}
				return 0.75
			}(),
		}
	}

	tStatus := strings.ToLower(strings.TrimSpace(terminalStatus))
	if tStatus == "exited" {
		return &AgentDecision{
			Action:     AgentActionAdvance,
			Reasoning:  "terminal session exited; assuming task completed",
			Confidence: 0.6,
		}
	}

	if approvalNeeded, prompt := a.detectApprovalNeeded(logs); approvalNeeded {
		return a.generateNextStepSuggestion(agentSuggestionContext{
			phase:          suggestionPhaseApproval,
			logs:           text,
			approvalPrompt: prompt,
		})
	}

	if progress, confidence, evidence := a.analyzeTaskProgress(text); progress > 0 {
		reasoning := "no completion/failure detected; continue monitoring"
		if evidence != "" {
			reasoning = fmt.Sprintf("task in progress (~%d%%, %s); continue monitoring", progress, evidence)
		} else {
			reasoning = fmt.Sprintf("task in progress (~%d%%); continue monitoring", progress)
		}
		return &AgentDecision{
			Action:     AgentActionWait,
			Reasoning:  reasoning,
			Confidence: confidence,
		}
	}

	return &AgentDecision{
		Action:     AgentActionWait,
		Reasoning:  "no completion/failure detected; continue monitoring",
		Confidence: 0.4,
	}
}

type suggestionPhase string

const (
	suggestionPhaseApproval   suggestionPhase = "approval"
	suggestionPhasePreExecute suggestionPhase = "pre_execute"
	suggestionPhaseOnError    suggestionPhase = "on_error"
)

type agentSuggestionContext struct {
	phase          suggestionPhase
	nodeType       string
	command        string
	logs           string
	approvalPrompt string
	err            error
}

type errorPattern struct {
	re         *regexp.Regexp
	suggestion string
	confidence float64
}

type errorInsight struct {
	summary    string
	suggestion string
	confidence float64
	pattern    string
}

func defaultErrorPatterns() []errorPattern {
	return []errorPattern{
		{re: regexp.MustCompile(`(?i)permission denied`), suggestion: "check permissions or credentials; avoid running destructive commands as root", confidence: 0.9},
		{re: regexp.MustCompile(`(?i)no such file or directory`), suggestion: "verify the file/path exists and the working directory is correct", confidence: 0.9},
		{re: regexp.MustCompile(`(?i)command not found`), suggestion: "install the missing command or ensure it is on PATH", confidence: 0.85},
		{re: regexp.MustCompile(`(?i)fatal:\s+not a git repository`), suggestion: "run the command in the correct repo directory (set work_dir) or initialize a repo", confidence: 0.9},
		{re: regexp.MustCompile(`(?i)could not resolve host|name or service not known`), suggestion: "check network/DNS and verify the host name", confidence: 0.8},
		{re: regexp.MustCompile(`(?i)connection refused|connection reset|broken pipe|i/o timeout|context deadline exceeded`), suggestion: "check network connectivity and retry; consider backoff if the service is unstable", confidence: 0.75},
		{re: regexp.MustCompile(`(?i)permission denied \\(publickey\\)|authentication failed`), suggestion: "verify SSH keys/credentials and remote access permissions", confidence: 0.8},
		{re: regexp.MustCompile(`(?i)panic:|traceback|exception`), suggestion: "inspect the stack trace and fix the underlying error; retry only after addressing the root cause", confidence: 0.7},
	}
}

func (a *WorkflowAgent) analyzeTaskProgress(logs string) (int, float64, string) {
	text := strings.TrimSpace(logs)
	if text == "" || len(a.progressPatterns) == 0 {
		return 0, 0, ""
	}

	bestPos := -1
	bestPercent := 0
	bestEvidence := ""
	bestConfidence := 0.0

	for _, re := range a.progressPatterns {
		if re == nil {
			continue
		}

		matches := re.FindAllStringSubmatchIndex(text, -1)
		if len(matches) == 0 {
			continue
		}
		last := matches[len(matches)-1]
		if len(last) < 4 {
			continue
		}

		switch len(last) {
		case 4:
			percentText := text[last[2]:last[3]]
			percent, err := strconv.Atoi(strings.TrimSpace(percentText))
			if err != nil || percent < 0 || percent > 100 {
				continue
			}
			if last[0] > bestPos {
				bestPos = last[0]
				bestPercent = percent
				bestEvidence = fmt.Sprintf("matched %q", strings.TrimSpace(text[last[0]:last[1]]))
				bestConfidence = 0.85
			}
		default:
			if len(last) < 6 {
				continue
			}
			numText := strings.TrimSpace(text[last[2]:last[3]])
			denText := strings.TrimSpace(text[last[4]:last[5]])
			num, numErr := strconv.Atoi(numText)
			den, denErr := strconv.Atoi(denText)
			if numErr != nil || denErr != nil || den <= 0 {
				continue
			}
			if num < 0 || num > den {
				continue
			}
			percent := int(float64(num) * 100 / float64(den))
			if last[0] > bestPos {
				bestPos = last[0]
				bestPercent = percent
				bestEvidence = fmt.Sprintf("matched %q", strings.TrimSpace(text[last[0]:last[1]]))
				bestConfidence = 0.75
			}
		}
	}

	if bestPos < 0 {
		return 0, 0, ""
	}

	if bestPercent < 0 {
		bestPercent = 0
	}
	if bestPercent > 100 {
		bestPercent = 100
	}

	return bestPercent, bestConfidence, bestEvidence
}

func (a *WorkflowAgent) detectErrors(logs string) *errorInsight {
	text := strings.TrimSpace(logs)
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	if len(lines) > 80 {
		lines = lines[len(lines)-80:]
		text = strings.Join(lines, "\n")
	}

	bestPos := -1
	var bestPattern errorPattern
	bestPatternStr := ""

	for _, p := range a.errorPatterns {
		if p.re == nil {
			continue
		}
		indices := p.re.FindAllStringIndex(text, -1)
		if len(indices) == 0 {
			continue
		}
		last := indices[len(indices)-1]
		if len(last) != 2 {
			continue
		}
		if last[0] > bestPos {
			bestPos = last[0]
			bestPattern = p
			bestPatternStr = p.re.String()
		}
	}

	if bestPos < 0 {
		return nil
	}

	summary := extractLineAt(text, bestPos)
	if summary == "" {
		summary = "error detected"
	}

	return &errorInsight{
		summary:    truncateString(summary, 200),
		suggestion: strings.TrimSpace(bestPattern.suggestion),
		confidence: bestPattern.confidence,
		pattern:    bestPatternStr,
	}
}

func extractLineAt(text string, pos int) string {
	if strings.TrimSpace(text) == "" || pos < 0 || pos >= len(text) {
		return ""
	}
	start := strings.LastIndex(text[:pos], "\n")
	if start < 0 {
		start = 0
	} else {
		start++
	}
	end := strings.Index(text[pos:], "\n")
	if end < 0 {
		end = len(text)
	} else {
		end = pos + end
	}
	return strings.TrimSpace(text[start:end])
}

func (a *WorkflowAgent) shouldRetry(err error) (bool, string, float64) {
	if err == nil {
		return false, "", 0
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false, "", 0
	}

	retrySignals := []string{
		"timeout",
		"timed out",
		"temporarily unavailable",
		"temporary failure",
		"try again",
		"connection reset",
		"connection refused",
		"broken pipe",
		"i/o timeout",
		"context deadline exceeded",
		"no route to host",
		"network is unreachable",
		"unexpected eof",
		"tls handshake timeout",
	}

	for _, s := range retrySignals {
		if strings.Contains(msg, s) {
			return true, fmt.Sprintf("transient error detected (%s)", s), 0.7
		}
	}

	return false, "error does not look retryable", 0.6
}

func (a *WorkflowAgent) generateNextStepSuggestion(ctx agentSuggestionContext) *AgentDecision {
	switch ctx.phase {
	case suggestionPhaseApproval:
		prompt := strings.TrimSpace(ctx.approvalPrompt)
		if prompt == "" {
			return &AgentDecision{Action: AgentActionWait, Reasoning: "approval prompt detected; waiting for resolution", Confidence: 0.6}
		}

		risky, factors, riskScore := assessPromptRisk(prompt, ctx.logs)
		if risky {
			return &AgentDecision{
				Action:         AgentActionWait,
				Reasoning:      fmt.Sprintf("approval prompt detected but appears risky (%s); waiting for manual review", factors),
				Confidence:     riskScore,
				ApprovalPrompt: prompt,
			}
		}

		input := suggestApprovalInput(prompt)
		reasoning := "approval prompt detected; auto-approving safe prompt"
		if strings.TrimSpace(input) == "" {
			reasoning = "approval prompt detected; unable to determine safe input; waiting for manual review"
		}

		return &AgentDecision{
			Action:         AgentActionWait,
			Reasoning:      reasoning,
			Confidence:     0.65,
			SuggestedInput: input,
			ApprovalPrompt: prompt,
		}

	case suggestionPhasePreExecute:
		cmd := strings.TrimSpace(ctx.command)
		if cmd == "" {
			return &AgentDecision{Action: AgentActionFail, Reasoning: "missing command", Confidence: 0.9}
		}

		risky, factors, confidence := assessCommandRisk(ctx.nodeType, cmd)
		if risky {
			return &AgentDecision{
				Action:     AgentActionWait,
				Reasoning:  fmt.Sprintf("risky operation detected (%s); pausing for manual approval", factors),
				Confidence: confidence,
			}
		}

		return &AgentDecision{
			Action:     AgentActionAdvance,
			Reasoning:  "operation considered safe; auto-approved",
			Confidence: confidence,
		}

	case suggestionPhaseOnError:
		retry, retryReason, retryConfidence := a.shouldRetry(ctx.err)
		if retry {
			return &AgentDecision{
				Action:     AgentActionAdvance,
				Reasoning:  "retry suggested: " + retryReason,
				Confidence: retryConfidence,
			}
		}

		insight := a.detectErrors(ctx.logs)
		reasoning := "step failed; not retrying automatically"
		confidence := retryConfidence
		if insight != nil {
			reasoning = strings.TrimSpace(reasoning + ": " + insight.summary)
			if strings.TrimSpace(insight.suggestion) != "" {
				reasoning = strings.TrimSpace(reasoning + " (suggestion: " + insight.suggestion + ")")
			}
			confidence = insight.confidence
		}

		return &AgentDecision{
			Action:     AgentActionFail,
			Reasoning:  reasoning,
			Confidence: confidence,
		}
	default:
		return &AgentDecision{Action: AgentActionWait, Reasoning: "no suggestion available", Confidence: 0.4}
	}
}

func assessPromptRisk(prompt, logs string) (bool, string, float64) {
	text := strings.ToLower(prompt)
	contextText := strings.ToLower(logs)
	score := 0.0
	var factors []string

	add := func(delta float64, reason string) {
		score += delta
		factors = append(factors, reason)
	}

	riskyTokens := []struct {
		token  string
		score  float64
		reason string
	}{
		{"delete", 0.6, "delete"},
		{"remove", 0.4, "remove"},
		{"overwrite", 0.6, "overwrite"},
		{"destroy", 0.7, "destroy"},
		{"drop", 0.7, "drop"},
		{"format", 0.8, "format"},
		{"wipe", 0.8, "wipe"},
		{"reset", 0.5, "reset"},
		{"--force", 0.5, "force flag"},
		{"-f", 0.3, "force flag"},
		{"rm -rf", 0.9, "rm -rf"},
		{"sudo", 0.6, "sudo"},
		{"git push", 0.6, "git push"},
		{"hard", 0.4, "hard"},
	}

	for _, tok := range riskyTokens {
		if strings.Contains(text, tok.token) || strings.Contains(contextText, tok.token) {
			add(tok.score, tok.reason)
		}
	}

	if len(factors) == 0 {
		return false, "no risky indicators", 0.55
	}

	if score >= 0.6 {
		return true, strings.Join(factors, ", "), minFloat64(score, 0.95)
	}
	return false, strings.Join(factors, ", "), minFloat64(score, 0.8)
}

func suggestApprovalInput(prompt string) string {
	p := strings.ToLower(prompt)
	switch {
	case strings.Contains(p, "enter to confirm") || strings.Contains(p, "press enter"):
		return "\r"
	case strings.Contains(p, "(y/n)") || strings.Contains(p, "[y/n]") || strings.Contains(p, "y/n"):
		return "y\r"
	case strings.Contains(p, "(yes/no)") || strings.Contains(p, "[yes/no]") || strings.Contains(p, "yes/no"):
		return "yes\r"
	case strings.Contains(p, "1.") || strings.Contains(p, "1)") || strings.Contains(p, "option 1"):
		return "1\r"
	default:
		return "y\r"
	}
}

func formatSuggestedInput(value string) string {
	if value == "" {
		return ""
	}
	formatted := strings.ReplaceAll(value, "\r", `\\r`)
	formatted = strings.ReplaceAll(formatted, "\n", `\\n`)
	formatted = strings.ReplaceAll(formatted, "\t", `\\t`)
	return formatted
}

func assessCommandRisk(nodeType, command string) (bool, string, float64) {
	_ = nodeType
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false, "", 0
	}

	score := 0.0
	var factors []string

	add := func(delta float64, reason string) {
		score += delta
		factors = append(factors, reason)
	}

	blacklist := []struct {
		token  string
		score  float64
		reason string
	}{
		{"rm -rf /", 1.0, "rm -rf /"},
		{"rm -r /", 1.0, "rm -r /"},
		{":(){ :|:& };:", 1.0, "fork bomb"},
		{"dd if=/dev/zero", 1.0, "disk overwrite"},
		{"mkfs", 1.0, "filesystem format"},
		{"fdisk", 1.0, "disk partitioning"},
		{"> /dev/sda", 1.0, "raw disk write"},
		{"curl", 0.45, "network download"},
		{"wget", 0.45, "network download"},
		{"| sh", 0.9, "pipe to shell"},
		{"|bash", 0.9, "pipe to shell"},
		{"sudo", 0.6, "privilege escalation"},
		{"chmod", 0.5, "permission change"},
		{"chown", 0.5, "ownership change"},
		{"reboot", 0.9, "system reboot"},
		{"shutdown", 0.9, "system shutdown"},
		{"systemctl", 0.7, "service control"},
		{"service ", 0.6, "service control"},
		{"git push", 0.7, "git push"},
		{"git reset --hard", 0.7, "git hard reset"},
		{"git clean -f", 0.6, "git clean"},
		{"git clean -d", 0.6, "git clean"},
		{"git clean -x", 0.6, "git clean"},
	}

	for _, item := range blacklist {
		if strings.Contains(cmd, item.token) {
			add(item.score, item.reason)
		}
	}

	if strings.Contains(cmd, ">") || strings.Contains(cmd, ">>") {
		add(0.35, "output redirection")
	}

	if len(factors) == 0 {
		return false, "no risky indicators", 0.7
	}

	confidence := minFloat64(0.95, score)
	return score >= 0.6, strings.Join(factors, ", "), confidence
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func confidenceOrDefault(insight *errorInsight, defaultValue float64) float64 {
	if insight == nil {
		return defaultValue
	}
	if insight.confidence <= 0 {
		return defaultValue
	}
	return insight.confidence
}

func (a *WorkflowAgent) sendTerminalInputViaEngine(ctx context.Context, terminalID, input string) error {
	if strings.TrimSpace(terminalID) == "" {
		return errors.New("terminalID is required")
	}
	if input == "" {
		return errors.New("input is required")
	}
	if a == nil || a.engine == nil || a.engine.terminal == nil {
		return errors.New("terminal manager is not configured")
	}
	session, err := a.engine.terminal.GetOrResumeSession(terminalID)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("terminal session not found")
	}
	return session.Write([]byte(input))
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
