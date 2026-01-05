package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/utils"
	"go.uber.org/zap"
)

// ChatMessage OpenAI兼容的消息格式
type ChatMessage struct {
	Role    string `json:"role"`    // system, user, assistant
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
const DefaultApprovalPrompt = `你是一个终端自动化助手。分析以下终端输出，判断是否需要用户审批或输入。

请根据以下规则判断：
1. 如果是常规的文件读写、创建目录等安全操作，返回 approve
2. 如果涉及删除文件（rm）、强制操作（-f）、root权限等危险操作，返回 reject
3. 如果是简单的确认提示（y/n, yes/no），且操作安全，返回 approve 并设置 input 为 "yes" 或 "y"
4. 如果需要用户选择多个选项，返回 wait 等待用户处理
5. 如果不确定，返回 wait

请以JSON格式返回结果：
{
  "action": "approve|reject|wait|input",
  "input": "如果action是input，这里填写需要输入的内容",
  "confidence": 0.0-1.0,
  "reasoning": "简短说明决策理由"
}`

// AnalyzeForApproval 分析终端输出并返回决策
func (p *AIProvider) AnalyzeForApproval(ctx context.Context, config *model.AIProviderConfig, prompt, terminalOutput string) (*DecisionResult, error) {
	if prompt == "" {
		prompt = DefaultApprovalPrompt
	}

	response, err := p.ChatSimple(ctx, config, prompt, terminalOutput)
	if err != nil {
		return nil, err
	}

	// 尝试从响应中提取JSON
	var result DecisionResult

	// 查找JSON开始和结束位置
	start := -1
	end := -1
	depth := 0
	for i, c := range response {
		if c == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		jsonStr := response[start:end]
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			utils.Warn("Failed to parse AI response as JSON", zap.Error(err), zap.String("response", response))
			// 默认返回等待
			return &DecisionResult{
				Action:     "wait",
				Confidence: 0,
				Reasoning:  "无法解析AI响应: " + response,
			}, nil
		}
	} else {
		// 无法找到JSON，返回等待
		return &DecisionResult{
			Action:     "wait",
			Confidence: 0,
			Reasoning:  "AI响应格式不正确: " + response,
		}, nil
	}

	return &result, nil
}
