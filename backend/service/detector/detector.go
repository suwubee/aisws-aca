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
	// 清理并去重进度条类型的输出
	content = cleanProgressOutput(content)
	allLines := strings.Split(content, "\n")

	// 获取最后N行
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	return strings.Join(allLines[start:], "\n")
}

// cleanProgressOutput 清理进度条类型的输出，模拟终端的行覆盖行为
func cleanProgressOutput(content string) string {
	// 移除ANSI转义序列
	content = stripANSISequences(content)

	// 处理 \r 回车符（进度条覆盖）
	var result strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// 处理行内的 \r，只保留最后一个版本
		if strings.Contains(line, "\r") {
			parts := strings.Split(line, "\r")
			// 取最后一个非空部分
			for i := len(parts) - 1; i >= 0; i-- {
				if strings.TrimSpace(parts[i]) != "" {
					line = parts[i]
					break
				}
			}
			// 如果全是空的，取最后一个
			if strings.TrimSpace(line) == "" && len(parts) > 0 {
				line = parts[len(parts)-1]
			}
		}

		// 跳过进度条特征行（避免记录大量重复的进度信息）
		if isProgressLine(line) {
			continue
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n")
}

// stripANSISequences 移除ANSI转义序列
func stripANSISequences(s string) string {
	// ANSI转义序列正则
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[PX^_].*?\x1b\\|\x1b\[[\?]?[0-9;]*[hlm]`)
	s = ansiRegex.ReplaceAllString(s, "")

	// 移除其他控制字符（保留换行、制表符、回车）
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)

	return s
}

// isProgressLine 检测是否是进度条行
func isProgressLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	// 常见进度条模式
	progressPatterns := []string{
		`^\s*\d+%`,                          // 百分比开头: "50%..."
		`\[\s*=*>?\s*\]`,                    // 进度条: [===>    ]
		`\d+\s*[KMG]?B/s`,                   // 下载速度: 1.5MB/s
		`\d+:\d+:\d+`,                       // 时间格式: 00:01:23
		`ETA\s*\d+`,                         // ETA时间
		`\d+\s*[KMG]?B\s*/\s*\d+\s*[KMG]?B`, // 下载进度: 100KB / 1MB
		`^\s*[\-\\|/]+\s*$`,                 // 旋转动画: - \ | /
		`^[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏\s]+$`,                 // Unicode旋转动画
	}

	for _, pattern := range progressPatterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			return true
		}
	}

	return false
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
			// 通用确认提示（更宽松的匹配）
			`(?i)\(y/n\)`,
			`(?i)\[y/n\]`,
			`(?i)\(yes/no\)`,
			`(?i)\[yes/no\]`,
			`(?i)yes\s+or\s+no`,
			`(?i)continue\s*\?`,
			`(?i)proceed\s*\?`,
			`(?i)confirm\s*\?`,
			`(?i)y/n`,
			// Claude Code 特有
			`(?i)allow\s+tool`,
			`(?i)allow\s+read`,
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
			`(?i)enter\s+to\s+confirm`,
			`(?i)esc\s+to\s+cancel`,
			`❯\s*\d+\.`,
			`(?i)\d+\.\s*(yes|no)`,
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
			// Claude Code 特有 - 欢迎界面后等待输入
			`(?i)>\s*Try\s+"`,
			`(?i)\?\s+for\s+shortcuts`,
			`(?i)Welcome\s+to\s+.*Claude`,
			`(?i)Claude\s+Code\s+v\d+`,
			// Claude Code 任务完成后等待新输入
			`(?i)What would you like to do`,
			`(?i)How can I help`,
			`(?i)What.*next`,
			`(?i)Anything else`,
			`(?i)Is there anything`,
			`(?i)Let me know if`,
			`(?i)Feel free to ask`,
			// Codex 特有
			`(?i)codex\s+v\d+`,
			`(?i)Enter\s+a\s+prompt`,
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
