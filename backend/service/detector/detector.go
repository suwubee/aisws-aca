package detector

import (
	"regexp"
	"strings"
	"time"
)

// AIAgentType AI代理类型
type AIAgentType string

const (
	AIAgentClaudeCode AIAgentType = "claude-code"
	AIAgentCodex      AIAgentType = "codex"
	AIAgentGemini     AIAgentType = "gemini"
	AIAgentCopilot    AIAgentType = "copilot"
	AIAgentCursor     AIAgentType = "cursor"
	AIAgentUnknown    AIAgentType = "unknown"
)

// AIAgentState AI代理状态
type AIAgentState string

const (
	StateUnknown         AIAgentState = "unknown"
	StateWaitingInput    AIAgentState = "waiting_input"
	StateWorking         AIAgentState = "working"
	StateWaitingApproval AIAgentState = "waiting_approval"
	StateIdle            AIAgentState = "idle"
)

// DetectedAgent 检测到的AI代理信息
type DetectedAgent struct {
	Type           AIAgentType  `json:"type"`
	DisplayName    string       `json:"display_name"`
	State          AIAgentState `json:"state"`
	StateUpdatedAt time.Time    `json:"state_updated_at"`
	Detected       bool         `json:"detected"`
	Version        string       `json:"version,omitempty"`
	ApprovalPrompt string       `json:"approval_prompt,omitempty"` // 当前的审批提示内容
}

// AgentPattern AI代理检测模式
type AgentPattern struct {
	Type        AIAgentType
	DisplayName string
	// 命令行检测模式
	CommandPatterns []string
	// 输出检测模式
	OutputPatterns []string
	// 版本提取模式
	VersionPattern string
}

// StatePattern 状态检测模式
type StatePattern struct {
	State    AIAgentState
	Patterns []string
	Priority int // 优先级，数字越大优先级越高
}

// Detector AI代理检测器
type Detector struct {
	agentPatterns []AgentPattern
	statePatterns []StatePattern
}

// NewDetector 创建检测器
func NewDetector() *Detector {
	return &Detector{
		agentPatterns: defaultAgentPatterns,
		statePatterns: defaultStatePatterns,
	}
}

// DetectAgent 从输出检测AI代理
func (d *Detector) DetectAgent(output string) *DetectedAgent {
	for _, pattern := range d.agentPatterns {
		for _, p := range pattern.OutputPatterns {
			if matched, _ := regexp.MatchString(p, output); matched {
				agent := &DetectedAgent{
					Type:           pattern.Type,
					DisplayName:    pattern.DisplayName,
					State:          StateWorking,
					StateUpdatedAt: time.Now(),
					Detected:       true,
				}
				// 尝试提取版本
				if pattern.VersionPattern != "" {
					if re, err := regexp.Compile(pattern.VersionPattern); err == nil {
						if matches := re.FindStringSubmatch(output); len(matches) > 1 {
							agent.Version = matches[1]
						}
					}
				}
				return agent
			}
		}
	}
	return nil
}

// DetectAgentFromCommand 从命令检测AI代理
func (d *Detector) DetectAgentFromCommand(cmd string) *DetectedAgent {
	cmdLower := strings.ToLower(cmd)
	for _, pattern := range d.agentPatterns {
		for _, p := range pattern.CommandPatterns {
			if matched, _ := regexp.MatchString(p, cmdLower); matched {
				return &DetectedAgent{
					Type:           pattern.Type,
					DisplayName:    pattern.DisplayName,
					State:          StateIdle,
					StateUpdatedAt: time.Now(),
					Detected:       true,
				}
			}
		}
	}
	return nil
}

// DetectState 检测AI代理状态
func (d *Detector) DetectState(output string) (AIAgentState, string) {
	bestState := StateUnknown
	bestPriority := -1
	matchedPrompt := ""

	for _, sp := range d.statePatterns {
		for _, p := range sp.Patterns {
			if matched, _ := regexp.MatchString(p, output); matched {
				if sp.Priority > bestPriority {
					bestState = sp.State
					bestPriority = sp.Priority
					// 提取匹配的提示内容
					if sp.State == StateWaitingApproval {
						matchedPrompt = extractApprovalPrompt(output, p)
					}
				}
			}
		}
	}

	return bestState, matchedPrompt
}

// IsApprovalPrompt 检测是否是审批提示
func (d *Detector) IsApprovalPrompt(output string) bool {
	state, _ := d.DetectState(output)
	return state == StateWaitingApproval
}

// GetRecentContext 获取最近的上下文（用于AI分析）
func GetRecentContext(scrollback []byte, lines int) string {
	if len(scrollback) == 0 {
		return ""
	}

	content := string(scrollback)
	allLines := strings.Split(content, "\n")

	// 获取最后N行
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	return strings.Join(allLines[start:], "\n")
}

// extractApprovalPrompt 提取审批提示内容
func extractApprovalPrompt(output, pattern string) string {
	// 获取匹配位置周围的文本
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			// 返回匹配行及其前后各2行
			start := i - 2
			if start < 0 {
				start = 0
			}
			end := i + 3
			if end > len(lines) {
				end = len(lines)
			}
			return strings.Join(lines[start:end], "\n")
		}
	}
	return ""
}

// 默认的AI代理检测模式
var defaultAgentPatterns = []AgentPattern{
	{
		Type:        AIAgentClaudeCode,
		DisplayName: "Claude Code",
		CommandPatterns: []string{
			`^claude\s`,
			`npx.*claude`,
			`claude-code`,
			`@anthropic`,
		},
		OutputPatterns: []string{
			`(?i)claude\s*code`,
			`(?i)anthropic`,
			`(?i)╭─.*claude`,
			`(?i)claude.*>`,
			`(?i)\[claude\]`,
		},
		VersionPattern: `claude.*?(\d+\.\d+\.\d+)`,
	},
	{
		Type:        AIAgentCodex,
		DisplayName: "OpenAI Codex",
		CommandPatterns: []string{
			`^codex\s`,
			`openai.*codex`,
		},
		OutputPatterns: []string{
			`(?i)codex`,
			`(?i)openai.*cli`,
			`(?i)\[codex\]`,
		},
	},
	{
		Type:        AIAgentGemini,
		DisplayName: "Gemini CLI",
		CommandPatterns: []string{
			`^gemini\s`,
			`google.*gemini`,
			`gcloud.*gemini`,
		},
		OutputPatterns: []string{
			`(?i)gemini`,
			`(?i)google\s*ai`,
			`(?i)\[gemini\]`,
		},
	},
	{
		Type:        AIAgentCopilot,
		DisplayName: "GitHub Copilot",
		CommandPatterns: []string{
			`gh\s+copilot`,
			`copilot.*cli`,
		},
		OutputPatterns: []string{
			`(?i)github\s*copilot`,
			`(?i)copilot\s*cli`,
			`(?i)\[copilot\]`,
		},
	},
	{
		Type:        AIAgentCursor,
		DisplayName: "Cursor",
		CommandPatterns: []string{
			`cursor`,
		},
		OutputPatterns: []string{
			`(?i)cursor\s*ai`,
			`(?i)\[cursor\]`,
		},
	},
}

// 默认的状态检测模式
var defaultStatePatterns = []StatePattern{
	// 等待审批 - 最高优先级
	{
		State: StateWaitingApproval,
		Patterns: []string{
			// 通用确认提示
			`(?i)\(y/n\)\s*$`,
			`(?i)\[y/n\]\s*$`,
			`(?i)\(yes/no\)\s*$`,
			`(?i)\[yes/no\]\s*$`,
			`(?i)yes\s+or\s+no\s*\?`,
			`(?i)continue\s*\?\s*\(y/n\)`,
			`(?i)proceed\s*\?\s*$`,
			`(?i)confirm\s*\?\s*$`,
			// Claude Code 特有
			`(?i)allow\s+tool\s+use\s*\?`,
			`(?i)allow\s+read\s*\?`,
			`(?i)allow\s+write\s*\?`,
			`(?i)allow\s+execute\s*\?`,
			`(?i)allow\s+bash\s*\?`,
			`(?i)allow\s+this\s+action\s*\?`,
			`(?i)approve\s*\?`,
			`(?i)grant\s+permission\s*\?`,
			// 权限相关
			`(?i)password\s*:`,
			`(?i)sudo.*password`,
			`(?i)enter\s+passphrase`,
			// 危险操作确认
			`(?i)are\s+you\s+sure\s*\?`,
			`(?i)this\s+cannot\s+be\s+undone`,
			`(?i)permanently\s+delete`,
			// 选择提示
			`\[\d+\]\s+.*\[\d+\]`,
			`\d+\)\s+.*\d+\)`,
		},
		Priority: 100,
	},
	// 等待输入
	{
		State: StateWaitingInput,
		Patterns: []string{
			`>\s*$`,
			`\?\s*$`,
			`:\s*$`,
			`(?i)enter\s+.*:`,
			`(?i)input\s*:`,
			`(?i)type\s+.*:`,
			`(?i)please\s+enter`,
			`(?i)waiting\s+for\s+input`,
		},
		Priority: 50,
	},
	// 工作中
	{
		State: StateWorking,
		Patterns: []string{
			`(?i)thinking`,
			`(?i)processing`,
			`(?i)analyzing`,
			`(?i)generating`,
			`(?i)writing`,
			`(?i)reading`,
			`(?i)searching`,
			`(?i)loading`,
			`⠋|⠙|⠹|⠸|⠼|⠴|⠦|⠧|⠇|⠏`, // 常见的加载动画字符
			`\.{3,}`,                    // 省略号
		},
		Priority: 30,
	},
	// 空闲
	{
		State: StateIdle,
		Patterns: []string{
			`\$\s*$`,
			`#\s*$`,
			`%\s*$`,
		},
		Priority: 10,
	},
}
