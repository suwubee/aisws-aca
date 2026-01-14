package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	promptsvc "github.com/ai-coding-assistant/service/prompt"
)

// ToolDefinition defines an AI-callable tool
type ToolDefinition struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Parameters  map[string]ToolParam `json:"parameters"`
	Required    []string             `json:"required,omitempty"`
}

// ToolParam defines a tool parameter
type ToolParam struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// ToolCall represents an AI tool call request
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	Data       any    `json:"data,omitempty"`
}

// GetAvailableTools returns all available tool definitions
func GetAvailableTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "list_servers",
			Description: "列出所有可用的服务器",
			Parameters:  map[string]ToolParam{},
		},
		{
			Name:        "select_server",
			Description: "选择一个服务器作为后续操作的目标",
			Parameters: map[string]ToolParam{
				"server_id": {Type: "string", Description: "服务器ID"},
			},
			Required: []string{"server_id"},
		},
		{
			Name:        "create_task",
			Description: "创建一个新任务",
			Parameters: map[string]ToolParam{
				"title":          {Type: "string", Description: "任务标题"},
				"description":    {Type: "string", Description: "任务描述"},
				"server_id":      {Type: "string", Description: "目标服务器ID（可选）"},
				"work_dir":       {Type: "string", Description: "工作目录"},
				"cli_type":       {Type: "string", Description: "CLI类型", Enum: []string{"claude", "codex", "gemini"}, Default: "claude"},
				"initial_prompt": {Type: "string", Description: "初始提示词"},
			},
			Required: []string{"title"},
		},
		{
			Name:        "start_task",
			Description: "启动一个已创建的任务",
			Parameters: map[string]ToolParam{
				"task_id": {Type: "string", Description: "任务ID"},
			},
			Required: []string{"task_id"},
		},
		{
			Name:        "execute_command",
			Description: "在服务器上执行命令（不允许隐式本地执行；必须显式选择服务器，本地也需要作为服务器记录配置）",
			Parameters: map[string]ToolParam{
				"command":   {Type: "string", Description: "要执行的命令"},
				"server_id": {Type: "string", Description: "目标服务器ID（可选，不填则使用当前选择的服务器）"},
				"work_dir":  {Type: "string", Description: "工作目录（可选）"},
			},
			Required: []string{"command"},
		},
		{
			Name:        "batch_execute_command",
			Description: "在多台服务器上批量执行同一条命令（可用于巡检/批量运维）；不允许隐式本地执行",
			Parameters: map[string]ToolParam{
				"command":    {Type: "string", Description: "要执行的命令"},
				"server_ids": {Type: "array", Description: "目标服务器ID列表（可选，不填则使用当前任务上下文）"},
				"work_dir":   {Type: "string", Description: "工作目录（可选）"},
			},
			Required: []string{"command"},
		},
		{
			Name:        "git_operation",
			Description: "执行Git操作",
			Parameters: map[string]ToolParam{
				"operation": {Type: "string", Description: "Git操作", Enum: []string{"clone", "pull", "push", "commit", "status", "branch", "checkout"}},
				"repo_url":  {Type: "string", Description: "仓库URL（clone时需要）"},
				"branch":    {Type: "string", Description: "分支名（可选）"},
				"message":   {Type: "string", Description: "提交信息（commit时需要）"},
				"work_dir":  {Type: "string", Description: "工作目录"},
				"server_id": {Type: "string", Description: "目标服务器ID（可选）"},
			},
			Required: []string{"operation"},
		},
		{
			Name:        "check_task_status",
			Description: "检查任务执行状态",
			Parameters: map[string]ToolParam{
				"task_id": {Type: "string", Description: "任务ID"},
			},
			Required: []string{"task_id"},
		},
		{
			Name:        "get_terminal_logs",
			Description: "获取终端日志",
			Parameters: map[string]ToolParam{
				"terminal_id": {Type: "string", Description: "终端ID"},
				"lines":       {Type: "integer", Description: "获取最近N行日志", Default: 100},
			},
			Required: []string{"terminal_id"},
		},
		{
			Name:        "wait",
			Description: "等待指定时间",
			Parameters: map[string]ToolParam{
				"seconds": {Type: "integer", Description: "等待秒数", Default: 5},
				"reason":  {Type: "string", Description: "等待原因"},
			},
			Required: []string{"seconds"},
		},
		{
			Name:        "ask_user",
			Description: "当信息不足或需要用户确认时，暂停工作流并向用户提问（用户回复后可继续）",
			Parameters: map[string]ToolParam{
				"question": {Type: "string", Description: "要向用户确认的问题（尽量给出明确选项）"},
				"context":  {Type: "string", Description: "补充上下文（可选）"},
			},
			Required: []string{"question"},
		},
		{
			Name:        "complete_workflow",
			Description: "标记工作流完成",
			Parameters: map[string]ToolParam{
				"summary": {Type: "string", Description: "完成总结"},
				"status":  {Type: "string", Description: "完成状态", Enum: []string{"success", "partial", "failed"}, Default: "success"},
			},
			Required: []string{"summary"},
		},
	}
}

// ParsedResponse represents parsed AI response
type ParsedResponse struct {
	Thought    string        `json:"thought"`
	Action     *ActionCall   `json:"action,omitempty"`
	Complete   *CompleteCall `json:"complete,omitempty"`
	RawContent string        `json:"raw_content"`
}

// ActionCall represents a tool action call
type ActionCall struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// CompleteCall represents workflow completion
type CompleteCall struct {
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// ParseAIResponse parses AI response in ReAct format
func ParseAIResponse(response string) (*ParsedResponse, error) {
	result := &ParsedResponse{RawContent: response}

	// Extract thought
	thoughtStart := strings.Index(response, "<thought>")
	thoughtEnd := strings.Index(response, "</thought>")
	if thoughtStart != -1 && thoughtEnd != -1 && thoughtEnd > thoughtStart {
		result.Thought = strings.TrimSpace(response[thoughtStart+9 : thoughtEnd])
	}

	// Check for complete tag first
	completeStart := strings.Index(response, "<complete>")
	completeEnd := strings.Index(response, "</complete>")
	if completeStart != -1 && completeEnd != -1 && completeEnd > completeStart {
		completeBody := strings.TrimSpace(response[completeStart+10 : completeEnd])
		var complete CompleteCall

		// 1) 标准 JSON
		if err := json.Unmarshal([]byte(completeBody), &complete); err == nil {
			result.Complete = &complete
			return result, nil
		}

		// 2) 宽松解析：尝试从文本中提取 JSON 对象（例如 ```json ...``` 或混入多余文本）
		if obj := extractJSONObject(completeBody); obj != "" {
			if err := json.Unmarshal([]byte(obj), &complete); err == nil {
				result.Complete = &complete
				return result, nil
			}
		}

		// 3) 最后兜底：允许 <complete> 内为纯文本（将其作为 summary）
		if summary := strings.TrimSpace(completeBody); summary != "" {
			result.Complete = &CompleteCall{
				Status:  "success",
				Summary: summary,
			}
			return result, nil
		}
	}

	// Extract action
	actionStart := strings.Index(response, "<action>")
	actionEnd := strings.Index(response, "</action>")
	if actionStart != -1 && actionEnd != -1 && actionEnd > actionStart {
		actionJSON := strings.TrimSpace(response[actionStart+8 : actionEnd])
		var action ActionCall
		if err := json.Unmarshal([]byte(actionJSON), &action); err != nil {
			return result, fmt.Errorf("invalid action JSON: %w", err)
		}
		result.Action = &action
	}

	return result, nil
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

// FormatObservation formats tool execution result as observation
func FormatObservation(result *ToolResult) string {
	var sb strings.Builder
	sb.WriteString("<observation>\n")
	if result.Success {
		sb.WriteString(fmt.Sprintf("执行成功\n%s", result.Output))
	} else {
		sb.WriteString(fmt.Sprintf("执行失败: %s\n%s", result.Error, result.Output))
	}
	sb.WriteString("\n</observation>")
	return sb.String()
}

// FormatToolsForPrompt formats tools for AI prompt using ReAct framework
func FormatToolsForPrompt() (string, error) {
	return FormatToolsForPromptWithTemplate(promptsvc.TemplateKeyAIWorkflowSystemPrompt, nil)
}

func FormatToolsForPromptWithTemplate(templateKey string, extraVars map[string]any) (string, error) {
	tools := GetAvailableTools()

	type promptParam struct {
		Name        string
		Description string
		Required    bool
		Enum        []string
	}

	type promptTool struct {
		Name        string
		Description string
		Params      []promptParam
	}

	formatted := make([]promptTool, 0, len(tools))
	for _, tool := range tools {
		paramNames := make([]string, 0, len(tool.Parameters))
		for name := range tool.Parameters {
			paramNames = append(paramNames, name)
		}
		sort.Strings(paramNames)

		params := make([]promptParam, 0, len(paramNames))
		for _, name := range paramNames {
			param := tool.Parameters[name]
			required := false
			for _, r := range tool.Required {
				if r == name {
					required = true
					break
				}
			}
			params = append(params, promptParam{
				Name:        name,
				Description: param.Description,
				Required:    required,
				Enum:        append([]string(nil), param.Enum...),
			})
		}

		formatted = append(formatted, promptTool{
			Name:        tool.Name,
			Description: tool.Description,
			Params:      params,
		})
	}

	vars := map[string]any{
		"tools": formatted,
	}
	for k, v := range extraVars {
		if strings.TrimSpace(k) == "" {
			continue
		}
		vars[k] = v
	}

	key := strings.TrimSpace(templateKey)
	if key == "" {
		key = promptsvc.TemplateKeyAIWorkflowSystemPrompt
	}

	return promptsvc.RenderTemplate(key, vars)
}
