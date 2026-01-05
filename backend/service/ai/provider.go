package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/utils"
	"go.uber.org/zap"
)

// ChatMessage OpenAI兼容的消息格式
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// ChatRequest OpenAI兼容的请求格式
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatChoice OpenAI响应选项
type ChatChoice struct {
	Index   int         `json:"index"`
	Message ChatMessage `json:"message"`
}

// ChatResponse OpenAI兼容的响应格式
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// AIProvider AI提供商服务
type AIProvider struct {
	httpClient *http.Client
}

// NewAIProvider 创建AI提供商服务
func NewAIProvider() *AIProvider {
	return &AIProvider{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetDefaultConfig 获取默认AI配置
func (p *AIProvider) GetDefaultConfig() (*model.AIProviderConfig, error) {
	var config model.AIProviderConfig
	err := model.DB.Where("is_default = ? AND enabled = ?", true, true).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigByID 根据ID获取AI配置
func (p *AIProvider) GetConfigByID(id string) (*model.AIProviderConfig, error) {
	var config model.AIProviderConfig
	err := model.DB.Where("id = ? AND enabled = ?", id, true).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// Chat 调用AI进行对话
func (p *AIProvider) Chat(ctx context.Context, config *model.AIProviderConfig, messages []ChatMessage) (*ChatResponse, error) {
	// 构建请求
	reqBody := ChatRequest{
		Model:       config.Model,
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 构建URL
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = getDefaultBaseURL(config.Provider)
	}
	url := baseURL + "/chat/completions"

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置头部
	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	utils.Debug("AI request", zap.String("url", url), zap.String("model", config.Model))

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 解析响应
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w, body: %s", err, string(body))
	}

	// 检查错误
	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return &chatResp, nil
}

// ChatSimple 简单对话（系统提示词 + 用户消息）
func (p *AIProvider) ChatSimple(ctx context.Context, config *model.AIProviderConfig, systemPrompt, userMessage string) (string, error) {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	resp, err := p.Chat(ctx, config, messages)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return resp.Choices[0].Message.Content, nil
}

// 获取默认的API基础URL
func getDefaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

// DecisionResult AI决策结果
type DecisionResult struct {
	Action     string  `json:"action"`     // approve, reject, wait, input
	Input      string  `json:"input"`      // 需要输入的内容
	Confidence float64 `json:"confidence"` // 置信度 0-1
	Reasoning  string  `json:"reasoning"`  // 推理说明
}

// DefaultApprovalPrompt 默认的审批判断提示词
const DefaultApprovalPrompt = `你是一个终端自动化审批助手。分析以下终端输出，只做安全判断与需要的输入建议。

判定规则（从上到下匹配）：
1) 安全且明确的操作：action=approve
2) 明确危险操作（如 rm -rf、覆盖写入敏感路径、sudo/root 权限、不可逆删除等）：action=reject
3) 需要确认 (y/n, yes/no, continue/proceed/confirm)：若操作安全，action=approve 并给出 input（如 "y\n" / "yes\n" / "\n"）
4) 需要用户选择多个选项：action=wait
5) 不确定：action=wait

输出字段（必须齐全）：
- action: approve | reject | wait | input
- input: string（可为空；如需要自动输入必须包含换行符）
- confidence: 0~1 的数字（允许用百分比，如 92%）
- reasoning: 简短说明

输出格式要求（优先级从高到低）：
1) 仅输出一个 JSON 对象（不要 Markdown、不要代码块、不要额外文字）
2) 如果无法稳定输出 JSON，则输出 4 行键值对（不要其它内容）：
ACTION: ...
INPUT: ...
CONFIDENCE: ...
REASONING: ...
`

func buildApprovalPrompt(userPrompt string) string {
	userPrompt = strings.TrimSpace(userPrompt)
	if userPrompt == "" {
		return DefaultApprovalPrompt
	}
	return DefaultApprovalPrompt + "\n\n额外规则（高优先级）：\n" + userPrompt + "\n"
}

func normalizeDecisionKey(key string) string {
	k := strings.TrimSpace(strings.ToLower(key))
	switch k {
	case "action", "动作", "决策", "结论", "decision":
		return "action"
	case "input", "输入":
		return "input"
	case "confidence", "置信度":
		return "confidence"
	case "reasoning", "原因", "理由", "说明", "explanation":
		return "reasoning"
	default:
		return ""
	}
}

func normalizeDecisionAction(action string) string {
	a := strings.TrimSpace(strings.ToLower(action))
	switch a {
	case "approve", "allow", "yes", "y", "pass", "ok", "safe":
		return "approve"
	case "reject", "deny", "no", "n", "block", "danger":
		return "reject"
	case "wait", "manual", "ask", "unknown":
		return "wait"
	case "input", "enter":
		return "input"
	}

	// 中文兼容
	switch {
	case strings.Contains(a, "通过") || strings.Contains(a, "允许") || strings.Contains(a, "同意"):
		return "approve"
	case strings.Contains(a, "拒绝") || strings.Contains(a, "禁止") || strings.Contains(a, "阻止"):
		return "reject"
	case strings.Contains(a, "等待") || strings.Contains(a, "人工") || strings.Contains(a, "手动"):
		return "wait"
	case strings.Contains(a, "输入"):
		return "input"
	}

	return ""
}

func normalizeDecisionConfidence(v float64) float64 {
	if v > 1 && v <= 100 {
		v = v / 100
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

var kvDecisionLineRe = regexp.MustCompile(`(?i)^\s*(action|input|confidence|reasoning|动作|输入|置信度|原因|理由|说明|decision|explanation)\s*[:=：]\s*(.*?)\s*$`)
var percentRe = regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?)\s*%\s*$`)

func parseDecisionFromResponse(response string) *DecisionResult {
	text := strings.ReplaceAll(response, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)

	// 1) 尝试解析响应中的 JSON（取最后一个可解析且 action 合法的对象）
	var best *DecisionResult
	for _, obj := range extractJSONObjectCandidates(text) {
		var parsed DecisionResult
		if err := json.Unmarshal([]byte(obj), &parsed); err != nil {
			continue
		}
		normalizedAction := normalizeDecisionAction(parsed.Action)
		if normalizedAction == "" {
			continue
		}
		parsed.Action = normalizedAction
		parsed.Confidence = normalizeDecisionConfidence(parsed.Confidence)
		if strings.TrimSpace(parsed.Reasoning) == "" {
			parsed.Reasoning = "parsed_from_json"
		}
		best = &parsed
	}
	if best != nil {
		return best
	}

	// 2) 解析键值对（支持 action: / ACTION: 等）
	kv := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• \t"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "```") {
			continue
		}
		if m := kvDecisionLineRe.FindStringSubmatch(line); len(m) == 3 {
			key := normalizeDecisionKey(m[1])
			if key == "" {
				continue
			}
			kv[key] = strings.TrimSpace(m[2])
			continue
		}
	}
	if len(kv) > 0 {
		action := normalizeDecisionAction(kv["action"])
		if action != "" {
			conf := 0.0
			if s := strings.TrimSpace(kv["confidence"]); s != "" {
				if m := percentRe.FindStringSubmatch(s); len(m) == 2 {
					if f, err := strconv.ParseFloat(m[1], 64); err == nil {
						conf = f
					}
				} else if f, err := strconv.ParseFloat(s, 64); err == nil {
					conf = f
				}
				conf = normalizeDecisionConfidence(conf)
			}
			return &DecisionResult{
				Action:     action,
				Input:      kv["input"],
				Confidence: conf,
				Reasoning:  strings.TrimSpace(kv["reasoning"]),
			}
		}
	}

	// 3) 兜底：基于关键词猜测 action
	action := normalizeDecisionAction(text)
	if action != "" {
		return &DecisionResult{
			Action:     action,
			Confidence: 0.3,
			Reasoning:  "parsed_from_keywords",
		}
	}

	// 4) 无法解析
	preview := text
	if len(preview) > 800 {
		preview = preview[:800] + "…"
	}
	return &DecisionResult{
		Action:     "wait",
		Confidence: 0,
		Reasoning:  "无法解析AI响应: " + preview,
	}
}

func extractJSONObjectCandidates(text string) []string {
	candidates := make([]string, 0, 4)
	start := -1
	depth := 0
	inString := false
	escape := false

	for i := 0; i < len(text); i++ {
		ch := text[i]

		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				candidates = append(candidates, text[start:i+1])
				start = -1
			}
		}
	}
	return candidates
}

// AnalyzeForApproval 分析终端输出并返回决策
func (p *AIProvider) AnalyzeForApproval(ctx context.Context, config *model.AIProviderConfig, prompt, terminalOutput string) (*DecisionResult, error) {
	systemPrompt := buildApprovalPrompt(prompt)
	response, err := p.ChatSimple(ctx, config, systemPrompt, terminalOutput)
	if err != nil {
		return nil, err
	}

	decision := parseDecisionFromResponse(response)
	if decision.Action != "" {
		decision.Action = normalizeDecisionAction(decision.Action)
	}
	decision.Confidence = normalizeDecisionConfidence(decision.Confidence)
	if decision.Action == "" {
		decision.Action = "wait"
	}
	return decision, nil
}
