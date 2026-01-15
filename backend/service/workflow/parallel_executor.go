package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ParallelTask represents a single MCP-driven CLI task.
// For Codex MCP, it maps to: tool=mcp__codex__codex, args={PROMPT, cd, sandbox}.
type ParallelTask struct {
	WorkDir string // maps to MCP arg: cd
	Prompt  string // maps to MCP arg: PROMPT
	CLIType string // claude/codex
}

// ParallelTaskResult contains the execution outcome for a single task.
type ParallelTaskResult struct {
	Task     ParallelTask
	Success  bool
	Output   string
	Error    string
	TimedOut bool
	Duration time.Duration
}

// MCPCallFunc executes an MCP tool call (e.g. mcp__codex__codex).
type MCPCallFunc func(ctx context.Context, tool string, args map[string]any) (string, error)

// ParallelExecutor runs multiple MCP tasks concurrently and aggregates results.
type ParallelExecutor struct {
	Call MCPCallFunc

	Timeout time.Duration
	Sandbox string

	CodexTool  string
	ClaudeTool string
}

// ExecuteParallel runs tasks concurrently and returns results in the same order as inputs.
func (e *ParallelExecutor) ExecuteParallel(tasks []ParallelTask) []ParallelTaskResult {
	if len(tasks) == 0 {
		return nil
	}

	results := make([]ParallelTaskResult, len(tasks))
	if e == nil {
		for i, task := range tasks {
			results[i] = ParallelTaskResult{Task: task, Error: "parallel executor is nil"}
		}
		return results
	}

	call := e.Call
	if call == nil {
		for i, task := range tasks {
			results[i] = ParallelTaskResult{Task: task, Error: "mcp call function not configured"}
		}
		return results
	}

	sandbox := strings.TrimSpace(e.Sandbox)
	if sandbox == "" {
		sandbox = "workspace-write"
	}

	codexTool := strings.TrimSpace(e.CodexTool)
	if codexTool == "" {
		codexTool = "mcp__codex__codex"
	}

	claudeTool := strings.TrimSpace(e.ClaudeTool)
	if claudeTool == "" {
		claudeTool = "mcp__claude__claude"
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if e.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	type indexed struct {
		idx int
		res ParallelTaskResult
	}

	ch := make(chan indexed, len(tasks))
	for i, task := range tasks {
		i := i
		task := task

		go func() {
			tool, err := toolForCLIType(task.CLIType, codexTool, claudeTool)
			if err != nil {
				ch <- indexed{idx: i, res: ParallelTaskResult{Task: task, Error: err.Error()}}
				return
			}
			ch <- indexed{idx: i, res: runParallelTask(ctx, call, tool, sandbox, task)}
		}()
	}

	done := make([]bool, len(tasks))
	received := 0
	for received < len(tasks) {
		select {
		case r := <-ch:
			results[r.idx] = r.res
			done[r.idx] = true
			received++
		case <-ctx.Done():
			for i, task := range tasks {
				if done[i] {
					continue
				}
				results[i] = ParallelTaskResult{
					Task:     task,
					Error:    ctx.Err().Error(),
					TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded),
				}
			}
			return results
		}
	}

	return results
}

func runParallelTask(ctx context.Context, call MCPCallFunc, tool, sandbox string, task ParallelTask) ParallelTaskResult {
	start := time.Now()
	res := ParallelTaskResult{Task: task}
	defer func() { res.Duration = time.Since(start) }()

	prompt := strings.TrimSpace(task.Prompt)
	if prompt == "" {
		res.Error = "prompt is required"
		return res
	}

	args := map[string]any{
		"PROMPT":  prompt,
		"cd":      strings.TrimSpace(task.WorkDir),
		"sandbox": sandbox,
	}

	out, err := call(ctx, tool, args)
	if err != nil {
		res.Error = err.Error()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			res.TimedOut = true
		}
		return res
	}

	res.Success = true
	res.Output = out
	return res
}

func toolForCLIType(cliType, codexTool, claudeTool string) (string, error) {
	switch normalizeCLIType(cliType) {
	case "codex":
		return codexTool, nil
	case "claude":
		return claudeTool, nil
	default:
		return "", fmt.Errorf("unsupported cliType: %s", strings.TrimSpace(cliType))
	}
}

func normalizeCLIType(cliType string) string {
	t := strings.ToLower(strings.TrimSpace(cliType))
	switch t {
	case "", "claude", "claude-code", "claude_code":
		return "claude"
	case "codex", "openai-codex":
		return "codex"
	default:
		return t
	}
}
