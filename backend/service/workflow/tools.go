package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
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
				"command":                 {Type: "string", Description: "要执行的命令"},
				"server_id":               {Type: "string", Description: "目标服务器ID（可选，不填则使用当前选择的服务器）"},
				"work_dir":                {Type: "string", Description: "工作目录（可选）"},
				"run_review":              {Type: "boolean", Description: "是否在主命令后启动并行 review 子执行（可选）", Default: false},
				"review_command":          {Type: "string", Description: "review 子执行命令（可选）"},
				"review_command_template": {Type: "string", Description: "review 命令模板，可用变量 {{command}}/{{server_id}}/{{work_dir}}（可选）"},
				"review_work_dir":         {Type: "string", Description: "review 工作目录（可选，默认沿用 work_dir）"},
				"review_cli_type":         {Type: "string", Description: "review 执行器标识（claude/codex/gemini/shell，可选）"},
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
		actionBody := strings.TrimSpace(response[actionStart+8 : actionEnd])
		action, err := parseActionCall(actionBody)
		if err != nil {
			return result, fmt.Errorf("invalid action JSON: %w", err)
		}
		result.Action = action
	}

	return result, nil
}

var (
	trailingCommaJSONRE = regexp.MustCompile(`,(\s*[}\]])`)
	bareKeyJSONRE       = regexp.MustCompile(`([{\[,]\s*)([A-Za-z_][A-Za-z0-9_-]*)(\s*:)`)
	singleQuotedJSONRE  = regexp.MustCompile(`'([^'\\]*(?:\\.[^'\\]*)*)'`)
	toolLineRE          = regexp.MustCompile(`(?i)\btool\b\s*[:=]\s*["']?([a-zA-Z0-9_]+)["']?`)
)

func parseActionCall(raw string) (*ActionCall, error) {
	candidates := collectActionCandidates(raw)
	var lastErr error
	for _, candidate := range candidates {
		if action, err := decodeActionCall(candidate); err == nil {
			return action, nil
		} else {
			lastErr = err
		}

		normalized := normalizeJSONLike(candidate)
		if normalized != candidate {
			if action, err := decodeActionCall(normalized); err == nil {
				return action, nil
			} else {
				lastErr = err
			}
		}
	}

	if action, ok := parseMinimalAction(raw); ok {
		return action, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unsupported action payload")
	}
	return nil, lastErr
}

func collectActionCandidates(raw string) []string {
	out := make([]string, 0, 4)
	seen := map[string]struct{}{}
	push := func(value string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	push(raw)
	stripped := stripMarkdownCodeFence(raw)
	push(stripped)
	push(extractJSONObject(raw))
	push(extractJSONObject(stripped))
	return out
}

func decodeActionCall(raw string) (*ActionCall, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("empty action payload")
	}

	var direct ActionCall
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil && strings.TrimSpace(direct.Tool) != "" {
		if direct.Args == nil {
			direct.Args = map[string]any{}
		}
		direct.Tool = strings.TrimSpace(direct.Tool)
		return &direct, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, err
	}

	if action, ok := actionCallFromMap(payload); ok {
		return action, nil
	}
	if nested, ok := payload["action"].(map[string]any); ok {
		if action, ok := actionCallFromMap(nested); ok {
			return action, nil
		}
	}

	return nil, fmt.Errorf("missing tool field")
}

func actionCallFromMap(payload map[string]any) (*ActionCall, bool) {
	if payload == nil {
		return nil, false
	}

	tool := strings.TrimSpace(getStringFromAnyMap(payload, "tool"))
	if tool == "" {
		tool = strings.TrimSpace(getStringFromAnyMap(payload, "name"))
	}
	if tool == "" {
		return nil, false
	}

	args := map[string]any{}
	switch rawArgs := payload["args"].(type) {
	case map[string]any:
		args = rawArgs
	case map[string]string:
		converted := make(map[string]any, len(rawArgs))
		for k, v := range rawArgs {
			converted[k] = v
		}
		args = converted
	case string:
		argText := strings.TrimSpace(rawArgs)
		if argText != "" {
			var decoded map[string]any
			if err := json.Unmarshal([]byte(normalizeJSONLike(argText)), &decoded); err == nil {
				args = decoded
			}
		}
	case nil:
		// keep empty map
	default:
		if m, ok := rawArgs.(map[string]any); ok {
			args = m
		}
	}

	return &ActionCall{
		Tool: tool,
		Args: args,
	}, true
}

func getStringFromAnyMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func parseMinimalAction(raw string) (*ActionCall, bool) {
	trimmed := strings.TrimSpace(stripMarkdownCodeFence(raw))
	if trimmed == "" {
		return nil, false
	}

	match := toolLineRE.FindStringSubmatch(trimmed)
	if len(match) < 2 {
		return nil, false
	}
	tool := strings.TrimSpace(match[1])
	if tool == "" {
		return nil, false
	}

	args := map[string]any{}
	lower := strings.ToLower(trimmed)
	if idx := strings.Index(lower, "args"); idx >= 0 {
		if obj := extractJSONObject(trimmed[idx:]); obj != "" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(normalizeJSONLike(obj)), &parsed); err == nil {
				args = parsed
			}
		}
	}

	return &ActionCall{Tool: tool, Args: args}, true
}

func stripMarkdownCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	body := strings.TrimPrefix(trimmed, "```")
	body = strings.TrimSpace(body)
	if idx := strings.Index(body, "\n"); idx >= 0 {
		firstLine := strings.TrimSpace(body[:idx])
		if !strings.HasPrefix(firstLine, "{") && !strings.HasPrefix(firstLine, "[") {
			body = body[idx+1:]
		}
	}

	if end := strings.LastIndex(body, "```"); end >= 0 {
		body = body[:end]
	}
	return strings.TrimSpace(body)
}

func normalizeJSONLike(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.TrimPrefix(trimmed, "\ufeff")
	replacer := strings.NewReplacer(
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
	)
	normalized := replacer.Replace(trimmed)
	normalized = trailingCommaJSONRE.ReplaceAllString(normalized, "$1")
	normalized = bareKeyJSONRE.ReplaceAllString(normalized, `$1"$2"$3`)
	normalized = singleQuotedJSONRE.ReplaceAllStringFunc(normalized, func(match string) string {
		if len(match) < 2 {
			return match
		}
		content := match[1 : len(match)-1]
		content = strings.ReplaceAll(content, `"`, `\"`)
		return `"` + content + `"`
	})
	normalized = trailingCommaJSONRE.ReplaceAllString(normalized, "$1")
	return strings.TrimSpace(normalized)
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			depth++
			continue
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1])
			}
		}
	}

	// fallback: best-effort slice
	end := strings.LastIndex(s, "}")
	if end <= start {
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
	tools = filterToolsForPrompt(tools, extraVars)

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

func filterToolsForPrompt(tools []ToolDefinition, extraVars map[string]any) []ToolDefinition {
	if len(tools) == 0 || extraVars == nil {
		return tools
	}

	allowed := mergeToolNameSets(
		readToolNameSet(extraVars, "allowed_tools"),
		readToolNameSet(extraVars, "tool_whitelist"),
	)
	disabled := mergeToolNameSets(
		readToolNameSet(extraVars, "disabled_tools"),
		readToolNameSet(extraVars, "tool_blacklist"),
		readToolNameSet(extraVars, "forbidden_tools"),
	)

	if len(allowed) == 0 && len(disabled) == 0 {
		return tools
	}

	filtered := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		name := strings.ToLower(strings.TrimSpace(tool.Name))
		if name == "" {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[name]; !ok {
				continue
			}
		}
		if _, denied := disabled[name]; denied {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func mergeToolNameSets(sets ...map[string]struct{}) map[string]struct{} {
	merged := map[string]struct{}{}
	for _, set := range sets {
		for name := range set {
			merged[name] = struct{}{}
		}
	}
	return merged
}

func readToolNameSet(vars map[string]any, key string) map[string]struct{} {
	out := map[string]struct{}{}
	if vars == nil {
		return out
	}
	raw, ok := vars[key]
	if !ok || raw == nil {
		return out
	}

	appendName := func(name string) {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if normalized == "" {
			return
		}
		out[normalized] = struct{}{}
	}

	switch value := raw.(type) {
	case []string:
		for _, item := range value {
			appendName(item)
		}
	case []any:
		for _, item := range value {
			if s, ok := item.(string); ok {
				appendName(s)
			}
		}
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return out
		}
		for _, part := range strings.Split(trimmed, ",") {
			appendName(part)
		}
	}

	return out
}
