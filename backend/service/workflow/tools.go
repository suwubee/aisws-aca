package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolDefinition defines an AI-callable tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]ToolParam   `json:"parameters"`
	Required    []string               `json:"required,omitempty"`
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
			Description: "在终端中执行命令",
			Parameters: map[string]ToolParam{
				"command":   {Type: "string", Description: "要执行的命令"},
				"server_id": {Type: "string", Description: "目标服务器ID（可选，不填则本地执行）"},
				"work_dir":  {Type: "string", Description: "工作目录（可选）"},
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

	// ReActSystemPrompt returns the ReAct framework system prompt
	const ReActSystemPrompt = `你是一个智能DevOps助手，使用ReAct（Reasoning + Acting）框架来完成任务。

## 工作模式
你需要按照以下循环工作：
1. **Thought**: 分析当前状态，思考下一步该做什么
2. **Action**: 调用工具执行操作
3. **Observation**: 观察执行结果（系统会自动提供）
4. 重复以上步骤直到任务完成

## 输出格式
每次回复必须严格按照以下格式：

<thought>
你的思考过程...分析当前状态，决定下一步行动
</thought>

<action>
{"tool": "工具名称", "args": {"参数名": "参数值"}}
</action>

或者当任务完成时：

<thought>
总结完成情况...
</thought>

<complete>
{"status": "success|partial|failed", "summary": "完成总结"}
</complete>

	## 重要规则
	1. 每次只能执行一个action
	2. 必须等待observation后再继续下一步
	3. 遇到错误时，分析原因并尝试修复
	4. 不要假设执行结果，必须等待实际反馈
	5. 任务完成后必须使用<complete>标签结束

	## ACA 平台背景（必须遵守）
	- 你运行在 AI Coding Assistant（ACA）的“AI 工作流”中，可通过工具创建任务/启动任务/查看日志。
	- 若用户明确要求使用某个 AI Coding Agent CLI（如 Claude Code / Codex / Gemini CLI）：
	  - 必须优先使用 create_task + start_task，让对应 CLI 在终端中执行；不要用 execute_command 直接绕过 CLI 完成交付（除非用户明确要求“直接执行命令”）。
	  - Claude Code 常见启动命令是 claude；如果环境未安装，可能需要 npx claude。当你无法确认应使用哪一个，先用 ask_user 向用户确认，而不是自行猜测。
	  - start_task 之后必须通过 check_task_status / get_terminal_logs 验证任务确实在运行；若 CLI 未启动或卡住，使用 ask_user 请求用户补充信息/确认安装情况。

	`

// ParsedResponse represents parsed AI response
type ParsedResponse struct {
	Thought    string         `json:"thought"`
	Action     *ActionCall    `json:"action,omitempty"`
	Complete   *CompleteCall  `json:"complete,omitempty"`
	RawContent string         `json:"raw_content"`
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
		completeJSON := strings.TrimSpace(response[completeStart+10 : completeEnd])
		var complete CompleteCall
		if err := json.Unmarshal([]byte(completeJSON), &complete); err == nil {
			result.Complete = &complete
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
func FormatToolsForPrompt() string {
	tools := GetAvailableTools()
	var sb strings.Builder
	sb.WriteString(ReActSystemPrompt)
	sb.WriteString("## 可用工具\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("### %s\n", tool.Name))
		sb.WriteString(fmt.Sprintf("%s\n", tool.Description))
		if len(tool.Parameters) > 0 {
			sb.WriteString("参数：\n")
			for name, param := range tool.Parameters {
				required := ""
				for _, r := range tool.Required {
					if r == name {
						required = " (必填)"
						break
					}
				}
				enumStr := ""
				if len(param.Enum) > 0 {
					enumStr = fmt.Sprintf(" [可选值: %s]", strings.Join(param.Enum, ", "))
				}
				sb.WriteString(fmt.Sprintf("- %s: %s%s%s\n", name, param.Description, required, enumStr))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
