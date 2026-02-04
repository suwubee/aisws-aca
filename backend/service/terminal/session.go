package terminal

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/approval"
	clisession "github.com/ai-coding-assistant/service/clisession"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/ai-coding-assistant/service/keybinding"
	"github.com/ai-coding-assistant/utils"
	"github.com/creack/pty"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var execCommand = exec.Command
var ptyStart = pty.Start

func applyTerminalEnv(cmd *exec.Cmd) {
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
}

// StreamEventType 流事件类型
type StreamEventType string

const (
	StreamEventData     StreamEventType = "data"
	StreamEventMetadata StreamEventType = "metadata"
	StreamEventExit     StreamEventType = "exit"
	StreamEventApproval StreamEventType = "approval" // 审批事件
	StreamEventMessage  StreamEventType = "message"  // 消息事件
	StreamEventAILog    StreamEventType = "ai_log"   // AI思考日志
)

// StreamEvent 流事件
type StreamEvent struct {
	Type           StreamEventType  `json:"type"`
	Data           string           `json:"data,omitempty"`
	Metadata       *SessionMetadata `json:"metadata,omitempty"`
	ExitCode       int              `json:"exit_code,omitempty"`
	Message        string           `json:"message,omitempty"`
	ApprovalResult *ApprovalEvent   `json:"approval_result,omitempty"`
	AILog          *AILogEvent      `json:"ai_log,omitempty"` // AI日志
}

// AILogEvent AI日志事件
type AILogEvent struct {
	Type      string `json:"type"`                 // thinking, action, decision, error, info
	Message   string `json:"message"`              // 日志内容
	InputType string `json:"input_type,omitempty"` // text, key, command
	InputData string `json:"input_data,omitempty"` // 实际输入的数据
}

// ApprovalEvent 审批事件
type ApprovalEvent struct {
	Action      string  `json:"action"`       // approve, reject, wait, input
	Input       string  `json:"input"`        // 自动输入的内容
	Reasoning   string  `json:"reasoning"`    // 决策理由
	Confidence  float64 `json:"confidence"`   // 置信度
	RuleMatched string  `json:"rule_matched"` // 匹配的规则
	AIDecision  bool    `json:"ai_decision"`  // 是否AI决策
	AutoHandled bool    `json:"auto_handled"` // 是否自动处理
}

// SessionMetadata 会话元数据
type SessionMetadata struct {
	Title          string       `json:"title"`
	PID            int          `json:"pid"`
	Status         string       `json:"status"`
	RunningCommand string       `json:"running_command,omitempty"`
	TaskID         *string      `json:"task_id,omitempty"`
	ServerID       *string      `json:"server_id,omitempty"`
	ServerName     string       `json:"server_name,omitempty"`
	ServerHost     string       `json:"server_host,omitempty"`
	AIAssistant    *AIAssistant `json:"ai_assistant,omitempty"`
	AutomationMode string       `json:"automation_mode,omitempty"` // manual, auto_yes, smart
	TmuxSession    string       `json:"tmux_session,omitempty"`    // tmux 会话名称
}

// AIAssistant AI助手信息
type AIAssistant struct {
	Type           string    `json:"type"`
	DisplayName    string    `json:"display_name"`
	State          string    `json:"state"` // unknown, waiting_input, working, waiting_approval
	StateUpdatedAt time.Time `json:"state_updated_at"`
	Detected       bool      `json:"detected"`
	Version        string    `json:"version,omitempty"`
	ApprovalPrompt string    `json:"approval_prompt,omitempty"` // 当前的审批提示
	// AI CLI 会话（用于 --resume）
	AISessionID string `json:"ai_session_id,omitempty"` // ACA 内部 AISession 主键
	SessionID   string `json:"session_id,omitempty"`    // 外部 CLI 的 session id（claude UUID / codex UUID）
	SessionFile string `json:"session_file,omitempty"`  // 外部 CLI 的会话文件路径/文件名（如 codex rollout jsonl）
	// CLI 状态确认（用于“CLI可选/不强制”的场景：允许人工确认 + AI/启发式预判）
	NeedsConfirm   bool    `json:"needs_confirm,omitempty"`   // 是否需要用户确认 CLI 状态
	ConfirmKind    string  `json:"confirm_kind,omitempty"`    // enter_cli / exit_cli
	ConfirmMessage string  `json:"confirm_message,omitempty"` // 用于弹框展示的证据/原因片段
	Source         string  `json:"source,omitempty"`          // anchor / heuristic / manual / ai
	Confidence     float64 `json:"confidence,omitempty"`      // 0-1，预判置信度
	Manual         bool    `json:"manual,omitempty"`          // 是否处于人工确认/覆盖状态
}

// Session 终端会话
type Session struct {
	id          string
	title       string
	taskID      *string
	serverID    *string
	shell       string
	startDir    string
	backend     sessionBackend
	pty         *os.File
	cmd         *exec.Cmd
	status      string
	subscribers map[string]chan StreamEvent
	subMutex    sync.RWMutex
	scrollback  *ScrollbackBuffer
	metadata    *SessionMetadata
	metaMutex   sync.RWMutex
	aiAssistant *AIAssistant
	createdAt   time.Time
	closedAt    *time.Time
	done        chan struct{}
	doneOnce    sync.Once // 确保 done channel 只关闭一次
	// 日志相关
	logBuffer    []LogEntry
	logMutex     sync.Mutex
	logFlushChan chan struct{}
	// 输入/输出日志聚合（避免按字符记录）
	ioBufMutex             sync.Mutex
	inputLineBuf           []rune
	inputEscState          int    // 0=none, 1=ESC, 2=CSI(ESC[...), 3=SS3(ESCO...)
	outputLineBuf          []rune // 当前输出行（处理 \r/\b/ANSI 清行等）
	outputCursor           int
	outputRemainder        []byte // 残留的半个 UTF-8/ANSI 序列
	outputSavedCursor      int
	outputCursorSaveActive bool // between save/restore (ESC7/ESC8 or CSI s/u)
	outputSuppressWrites   bool // suppress output writes for off-screen UI regions (e.g., status bar)
	outputLineBufSize      int
	// 检测和审批相关
	detector               *detector.Detector
	approvalEngine         *approval.Engine
	cliTrackingEnabled     bool
	expectedAIAgentType    detector.AIAgentType
	cliManualPresent       *bool
	cliManualUntil         time.Time
	cliSessionManager      *clisession.SessionManager
	lastOutput             string // 用于检测状态变化
	lastOutputMu           sync.Mutex
	approvalEvalMu         sync.Mutex
	approvalEvalInProgress bool
	// 节流相关（减少屏闪）
	dataBatchMu      sync.Mutex
	dataBatchBuf     []byte        // 数据批量缓冲
	dataBatchTimer   *time.Timer   // 批量发送定时器
	dataBatchMaxWait time.Duration // 最大等待时间
	metaThrottleMu   sync.Mutex
	metaThrottleLast time.Time // 上次元数据广播时间
	// 日志去重相关（避免动态输出重复记录）
	recentLogsMu     sync.Mutex
	recentLogs       map[string]time.Time // 最近记录的日志内容 -> 时间
	recentLogsWindow time.Duration        // 去重时间窗口

	commandRunMu sync.Mutex
}

type sessionBackend interface {
	Write(data []byte) error
	Resize(cols, rows uint16) error
	Close() error
}

type pipeShellBackend struct {
	cmd         *exec.Cmd
	stdinWriter *os.File
	closeOnce   sync.Once
	closeErr    error
}

func (b *pipeShellBackend) Write(data []byte) error {
	if b == nil || b.stdinWriter == nil {
		return errors.New("stdin pipe is not initialized")
	}
	n, err := b.stdinWriter.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func (b *pipeShellBackend) Resize(_, _ uint16) error {
	// No PTY available: ignore resize requests.
	return nil
}

func (b *pipeShellBackend) Close() error {
	if b == nil {
		return nil
	}

	b.closeOnce.Do(func() {
		var errs []error

		if b.stdinWriter != nil {
			if err := b.stdinWriter.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				errs = append(errs, err)
			}
		}

		if b.cmd != nil && b.cmd.Process != nil {
			_ = b.cmd.Process.Signal(os.Interrupt)
			time.Sleep(100 * time.Millisecond)
			_ = b.cmd.Process.Kill()
		}

		b.closeErr = errors.Join(errs...)
	})

	return b.closeErr
}

// LogEntry 日志条目
type LogEntry struct {
	LogType   string
	Content   string
	CreatedAt time.Time
}

// ScrollbackBuffer 滚动缓冲区
type ScrollbackBuffer struct {
	data    []byte
	maxSize int
	mutex   sync.RWMutex
}

func NewScrollbackBuffer(maxSize int) *ScrollbackBuffer {
	return &ScrollbackBuffer{
		data:    make([]byte, 0, maxSize),
		maxSize: maxSize,
	}
}

func (b *ScrollbackBuffer) Write(p []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.data = append(b.data, p...)
	if len(b.data) > b.maxSize {
		// 截断前面的数据
		b.data = b.data[len(b.data)-b.maxSize:]
	}
}

func (b *ScrollbackBuffer) Read() []byte {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	result := make([]byte, len(b.data))
	copy(result, b.data)
	return result
}

// NewSession 创建新会话
func NewSession(id, shell string, scrollbackSize int) *Session {
	return &Session{
		id:                id,
		shell:             shell,
		status:            "running",
		subscribers:       make(map[string]chan StreamEvent),
		scrollback:        NewScrollbackBuffer(scrollbackSize),
		createdAt:         time.Now(),
		done:              make(chan struct{}),
		logBuffer:         make([]LogEntry, 0, 100),
		logFlushChan:      make(chan struct{}, 1),
		inputLineBuf:      make([]rune, 0, 256),
		outputLineBuf:     make([]rune, 0, 256),
		outputLineBufSize: 8192,
		detector:          detector.NewDetector(),
		approvalEngine:    approval.NewEngine(),
		dataBatchBuf:      make([]byte, 0, 8192),
		dataBatchMaxWait:  16 * time.Millisecond, // 约60fps
		recentLogs:        make(map[string]time.Time),
		recentLogsWindow:  10 * time.Second, // 10秒内相同前缀内容不重复记录
		metadata: &SessionMetadata{
			Title:  "Terminal",
			Status: "running",
		},
	}
}

func (s *Session) SetStartDir(dir string) {
	s.startDir = strings.TrimSpace(dir)
}

// GetWorkDir 获取当前工作目录
func (s *Session) GetWorkDir() string {
	return s.startDir
}

// Start 启动会话
func (s *Session) Start() error {
	return s.StartWithTmux(false)
}

// RecoverFromTmux 从已有 tmux 会话恢复连接
func (s *Session) RecoverFromTmux() error {
	return s.StartWithTmux(true)
}

// StartWithTmux 使用 tmux 启动会话
func (s *Session) StartWithTmux(attach bool) error {
	var cmd *exec.Cmd

	if attach {
		// 重新连接到已有的 tmux 会话
		cmd = execCommand("tmux", "attach-session", "-t", s.id)
	} else {
		// 创建新的 tmux 会话
		// 先检查会话是否已存在
		checkCmd := execCommand("tmux", "has-session", "-t", s.id)
		if checkCmd.Run() == nil {
			// 会话已存在，直接 attach
			cmd = execCommand("tmux", "attach-session", "-t", s.id)
		} else {
			// 创建新会话
			args := []string{"new-session", "-d", "-s", s.id}
			if strings.TrimSpace(s.startDir) != "" {
				args = append(args, "-c", s.startDir)
			}
			args = append(args, "-x", "120", "-y", "30")
			cmd = execCommand("tmux", args...)
			applyTerminalEnv(cmd)
			if err := cmd.Run(); err != nil {
				utils.Warn("Failed to create tmux session, falling back to direct shell", zap.Error(err))
				return s.startDirectShell()
			}
			// 然后 attach 到新创建的会话
			cmd = execCommand("tmux", "attach-session", "-t", s.id)
		}
	}

	applyTerminalEnv(cmd)

	ptmx, err := ptyStart(cmd)
	if err != nil {
		return err
	}

	s.pty = ptmx
	s.cmd = cmd
	s.metadata.PID = cmd.Process.Pid
	s.metadata.TmuxSession = s.id

	// 加载自动化配置
	s.loadAutomationConfig()

	// 启动PTY读取goroutine
	go s.readPTY()

	// 启动进程等待goroutine
	go s.wait()

	// 启动日志刷新goroutine
	go s.flushLogs()

	return nil
}

// startDirectShell 直接启动 shell（fallback）
func (s *Session) startDirectShell() error {
	newCmd := func() *exec.Cmd {
		cmd := execCommand(s.shell)
		if strings.TrimSpace(s.startDir) != "" {
			cmd.Dir = s.startDir
		}
		applyTerminalEnv(cmd)
		return cmd
	}

	cmd := newCmd()

	ptmx, err := ptyStart(cmd)
	if err != nil {
		if os.IsPermission(err) || errors.Is(err, pty.ErrUnsupported) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
			// pty.Start may mutate SysProcAttr; create a fresh cmd for pipe mode to avoid ENOTTY.
			return s.startPipeShell(newCmd())
		}
		return err
	}

	s.pty = ptmx
	s.cmd = cmd
	s.metadata.PID = cmd.Process.Pid

	s.loadAutomationConfig()
	go s.readPTY()
	go s.wait()
	go s.flushLogs()

	return nil
}

func (s *Session) startPipeShell(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("command is nil")
	}

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return err
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		return err
	}

	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stdoutWriter

	if err := cmd.Start(); err != nil {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return err
	}

	_ = stdinReader.Close()
	_ = stdoutWriter.Close()

	s.backend = &pipeShellBackend{cmd: cmd, stdinWriter: stdinWriter}
	s.pty = stdoutReader
	s.cmd = cmd
	s.metadata.PID = cmd.Process.Pid

	s.loadAutomationConfig()
	go s.readPTY()
	go s.wait()
	go s.flushLogs()

	return nil
}

// loadAutomationConfig 加载自动化配置
func (s *Session) loadAutomationConfig() {
	if model.DB == nil {
		return
	}
	config, err := s.approvalEngine.GetAutomationConfig(s.id)
	if err != nil {
		return
	}
	s.metaMutex.Lock()
	s.metadata.AutomationMode = config.ApprovalMode
	s.metaMutex.Unlock()
}

// readPTY 读取PTY输出
func (s *Session) readPTY() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.done:
			s.flushDataBatch() // 退出前刷新剩余数据
			return
		default:
			n, err := s.pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					utils.Debug("PTY read error", zap.Error(err))
				}
				s.flushDataBatch()
				return
			}

			if n > 0 {
				data := buf[:n]
				s.scrollback.Write(data)

				// 记录输出日志
				s.addOutputLog(data)

				// 批量广播数据事件（减少屏闪）
				s.batchBroadcastData(data)

				// 更新最后输出
				s.lastOutputMu.Lock()
				s.lastOutput = string(data)
				s.lastOutputMu.Unlock()

				// 检测AI助手状态
				s.detectAndHandle(data)
			}
		}
	}
}

// batchBroadcastData 批量广播数据（减少频繁更新导致的屏闪）
func (s *Session) batchBroadcastData(data []byte) {
	s.dataBatchMu.Lock()
	defer s.dataBatchMu.Unlock()

	s.dataBatchBuf = append(s.dataBatchBuf, data...)

	// 如果缓冲区较大，立即发送
	if len(s.dataBatchBuf) >= 4096 {
		s.flushDataBatchLocked()
		return
	}

	// 否则设置定时器延迟发送
	if s.dataBatchTimer == nil {
		s.dataBatchTimer = time.AfterFunc(s.dataBatchMaxWait, func() {
			s.flushDataBatch()
		})
	}
}

// flushDataBatch 刷新数据批量缓冲
func (s *Session) flushDataBatch() {
	s.dataBatchMu.Lock()
	defer s.dataBatchMu.Unlock()
	s.flushDataBatchLocked()
}

// flushDataBatchLocked 刷新数据批量缓冲（需要持有锁）
func (s *Session) flushDataBatchLocked() {
	if s.dataBatchTimer != nil {
		s.dataBatchTimer.Stop()
		s.dataBatchTimer = nil
	}

	if len(s.dataBatchBuf) == 0 {
		return
	}

	// 发送批量数据
	s.broadcast(StreamEvent{
		Type: StreamEventData,
		// NOTE: do not filter internal markers here.
		// RunCommand relies on markers to capture output deterministically.
		// Filtering for UI clients is handled in the websocket layer.
		Data: base64.StdEncoding.EncodeToString(s.dataBatchBuf),
	})

	s.dataBatchBuf = s.dataBatchBuf[:0]
}

func lastNonEmptyLineLooksLikeShellPrompt(output string) bool {
	cleaned := strings.TrimSpace(stripANSI(output))
	if cleaned == "" {
		return false
	}
	lines := strings.Split(cleaned, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Only treat as shell prompt when the last non-empty line itself looks like a prompt.
		// This is intentionally conservative to avoid misclassifying "$" in logs/snippets as an exit signal.
		return isPromptLine(line) || shellPromptRegex.MatchString(line)
	}
	return false
}

// detectAndHandle 检测AI状态并处理审批
func (s *Session) detectAndHandle(data []byte) {
	output := string(data)

	now := time.Now()

	s.metaMutex.Lock()
	// Expire manual override lazily.
	if s.cliManualPresent != nil && !s.cliManualUntil.IsZero() && now.After(s.cliManualUntil) {
		s.cliManualPresent = nil
		s.cliManualUntil = time.Time{}
		if s.aiAssistant != nil {
			s.aiAssistant.Manual = false
			if s.aiAssistant.Source == "manual" {
				s.aiAssistant.Source = ""
				s.aiAssistant.Confidence = 0
			}
		}
	}
	cliTrackingEnabled := s.cliTrackingEnabled
	expectedType := s.expectedAIAgentType
	currentAssistant := s.aiAssistant
	manualPresent := s.cliManualPresent
	manualUntil := s.cliManualUntil
	s.metaMutex.Unlock()

	manualActive := manualPresent != nil && !manualUntil.IsZero() && now.Before(manualUntil)

	// CLI 检测/确认：CLI 可选（不强制），允许通过“输出锚点预判 + 用户确认”闭环。
	if cliTrackingEnabled {
		// 1) 人工确认覆盖：只要在有效期内，就按用户选择固定 detected（避免误判导致阻塞）。
		if manualActive {
			s.metaMutex.Lock()
			if s.aiAssistant == nil {
				s.aiAssistant = &AIAssistant{
					Type:           string(detector.AIAgentUnknown),
					DisplayName:    "AI CLI",
					State:          string(detector.StateUnknown),
					StateUpdatedAt: now,
					Detected:       false,
				}
			}
			s.aiAssistant.Manual = true
			s.aiAssistant.Source = "manual"
			s.aiAssistant.Confidence = 1
			if manualPresent != nil {
				s.aiAssistant.Detected = *manualPresent
			}
			s.aiAssistant.NeedsConfirm = false
			s.aiAssistant.ConfirmKind = ""
			s.aiAssistant.ConfirmMessage = ""
			s.metadata.AIAssistant = s.aiAssistant
			s.metaMutex.Unlock()
		} else if strings.TrimSpace(string(expectedType)) != "" && expectedType != detector.AIAgentUnknown {
			// 2) 任务明确选择了 CLI 类型：使用类型限定的输出锚点确认进入（detected=true）。
			if currentAssistant == nil || !currentAssistant.Detected {
				if agent := s.detector.DetectAgentWithType(output, expectedType); agent != nil {
					s.metaMutex.Lock()
					if s.aiAssistant == nil {
						s.aiAssistant = &AIAssistant{
							Type:           string(agent.Type),
							DisplayName:    agent.DisplayName,
							State:          string(agent.State),
							StateUpdatedAt: agent.StateUpdatedAt,
							Detected:       true,
							Version:        agent.Version,
							Source:         "anchor",
							Confidence:     1,
							Manual:         false,
						}
					} else {
						s.aiAssistant.Type = string(agent.Type)
						s.aiAssistant.DisplayName = agent.DisplayName
						s.aiAssistant.Detected = true
						s.aiAssistant.Version = agent.Version
						s.aiAssistant.StateUpdatedAt = now
						s.aiAssistant.Source = "anchor"
						s.aiAssistant.Confidence = 1
						s.aiAssistant.NeedsConfirm = false
						s.aiAssistant.ConfirmKind = ""
						s.aiAssistant.ConfirmMessage = ""
						s.aiAssistant.Manual = false
					}
					s.metadata.AIAssistant = s.aiAssistant
					s.metaMutex.Unlock()

					utils.Info("AI CLI entered (detected from output)",
						zap.String("type", string(agent.Type)),
						zap.String("terminal", s.id))

					s.broadcastMetadata()
				}
			}
		} else {
			// 3) CLI 未强制选择：命中输出锚点时先作为候选（needs_confirm=true），由用户确认“是/否/不确定”。
			if currentAssistant == nil {
				currentAssistant = &AIAssistant{
					Type:           string(detector.AIAgentUnknown),
					DisplayName:    "AI CLI",
					State:          string(detector.StateUnknown),
					StateUpdatedAt: now,
					Detected:       false,
				}
			}
			if !currentAssistant.Detected && !currentAssistant.NeedsConfirm {
				if agent := s.detector.DetectAgent(output); agent != nil {
					s.metaMutex.Lock()
					if s.aiAssistant == nil {
						s.aiAssistant = &AIAssistant{
							Type:           string(agent.Type),
							DisplayName:    agent.DisplayName,
							State:          string(agent.State),
							StateUpdatedAt: agent.StateUpdatedAt,
							Detected:       false,
							Version:        agent.Version,
							Source:         "anchor",
							Confidence:     0.7,
							NeedsConfirm:   true,
							ConfirmKind:    "enter_cli",
							ConfirmMessage: "检测到可能已进入 AI CLI，请确认是否进入交互界面（是/否/不确定）",
						}
					} else {
						s.aiAssistant.Type = string(agent.Type)
						s.aiAssistant.DisplayName = agent.DisplayName
						s.aiAssistant.Detected = false
						s.aiAssistant.Version = agent.Version
						s.aiAssistant.StateUpdatedAt = now
						s.aiAssistant.Source = "anchor"
						s.aiAssistant.Confidence = 0.7
						s.aiAssistant.NeedsConfirm = true
						s.aiAssistant.ConfirmKind = "enter_cli"
						s.aiAssistant.ConfirmMessage = "检测到可能已进入 AI CLI，请确认是否进入交互界面（是/否/不确定）"
						s.aiAssistant.Manual = false
					}
					s.metadata.AIAssistant = s.aiAssistant
					s.metaMutex.Unlock()

					utils.Info("AI CLI candidate detected (needs confirmation)",
						zap.String("type", string(agent.Type)),
						zap.String("terminal", s.id))

					s.broadcastMetadata()
				}
			}
		}
	}

	// ===== CLI 会话跟踪（AISession） =====
	// 仅在任务绑定且检测到进入/疑似进入 AI CLI 时创建会话记录，并持续从输出提取 session_id/session_file。
	// 目的：为“一键 resume”与审批审计提供稳定的会话对象。
	if cliTrackingEnabled {
		taskID := ""
		assistantType := ""
		assistantDetected := false
		assistantNeedsConfirm := false
		assistantSource := ""
		s.metaMutex.RLock()
		if s.taskID != nil {
			taskID = strings.TrimSpace(*s.taskID)
		}
		sessionMgr := s.cliSessionManager
		if s.aiAssistant != nil {
			assistantType = strings.TrimSpace(s.aiAssistant.Type)
			assistantDetected = s.aiAssistant.Detected
			assistantNeedsConfirm = s.aiAssistant.NeedsConfirm
			assistantSource = s.aiAssistant.Source
		}
		s.metaMutex.RUnlock()

		// detected=true 或 needs_confirm(由输出锚点触发) 都视为足够强的信号
		shouldTrack := taskID != "" && (assistantDetected || (assistantNeedsConfirm && assistantSource == "anchor"))

		if shouldTrack && sessionMgr == nil {
			if mgr, err := clisession.NewSessionManager(s.id, s.taskID, assistantType); err == nil {
				s.metaMutex.Lock()
				// double check: avoid racing with refreshTaskAutomationTracking
				if s.cliTrackingEnabled && s.cliSessionManager == nil {
					s.cliSessionManager = mgr
				}
				sessionMgr = s.cliSessionManager
				s.metaMutex.Unlock()
			} else {
				utils.Debug("Failed to init CLI session manager", zap.Error(err), zap.String("terminal", s.id))
			}
		}

		if sessionMgr != nil {
			if snap, err := sessionMgr.UpdateFromOutput(output); err == nil {
				s.metaMutex.Lock()
				if s.aiAssistant != nil {
					s.aiAssistant.AISessionID = snap.AISessionID
					s.aiAssistant.SessionID = snap.SessionID
					s.aiAssistant.SessionFile = snap.SessionFile
					s.metadata.AIAssistant = s.aiAssistant
				}
				s.metaMutex.Unlock()
			} else {
				utils.Debug("Failed to update CLI session from output", zap.Error(err), zap.String("terminal", s.id))
			}
		}
	}

	// 检测状态变化
	state, approvalPrompt := s.detector.DetectState(output)

	// 无论是否有AI代理，都检测审批状态
	if state == detector.StateWaitingApproval {
		utils.Info("Detected waiting approval state",
			zap.String("terminal", s.id),
			zap.String("prompt", approvalPrompt))

		// 触发审批流程
		go s.handleApproval(output)
	}

	// 更新AI助手状态（如果存在）
	s.metaMutex.Lock()
	if s.aiAssistant != nil {
		oldState := s.aiAssistant.State
		s.aiAssistant.State = string(state)
		s.aiAssistant.StateUpdatedAt = now
		if approvalPrompt != "" {
			s.aiAssistant.ApprovalPrompt = approvalPrompt
		}
		s.metadata.AIAssistant = s.aiAssistant

		// CLI 退出（疑似）：从可交互/工作态回到 shell prompt。
		// 为了安全默认先将 detected=false（避免把 prompt 当作 shell 命令执行），并交由用户确认闭环。
		if cliTrackingEnabled && s.aiAssistant.Detected &&
			oldState != string(detector.StateIdle) &&
			state == detector.StateIdle &&
			lastNonEmptyLineLooksLikeShellPrompt(output) &&
			!(manualActive && manualPresent != nil && *manualPresent) {
			s.aiAssistant.Detected = false
			s.aiAssistant.ApprovalPrompt = ""
			s.aiAssistant.NeedsConfirm = true
			s.aiAssistant.ConfirmKind = "exit_cli"
			s.aiAssistant.ConfirmMessage = "检测到可能已退出 AI CLI（出现 shell 提示符），请确认当前是否仍在 AI CLI（是/否/不确定）"
			s.aiAssistant.Source = "heuristic"
			s.aiAssistant.Confidence = 0.5
			s.aiAssistant.Manual = false
		}

		// 状态变化时广播
		if oldState != string(state) {
			s.metaMutex.Unlock()
			s.broadcastMetadata()
			return
		}
	}
	s.metaMutex.Unlock()
}

// handleApproval 处理审批流程
func (s *Session) handleApproval(output string) {
	s.approvalEvalMu.Lock()
	if s.approvalEvalInProgress {
		s.approvalEvalMu.Unlock()
		return
	}
	s.approvalEvalInProgress = true
	s.approvalEvalMu.Unlock()
	defer func() {
		s.approvalEvalMu.Lock()
		s.approvalEvalInProgress = false
		s.approvalEvalMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取更多上下文
	scrollbackData := s.scrollback.Read()
	contextLines := 50
	cfg, cfgErr := s.approvalEngine.GetAutomationConfig(s.id)
	if cfgErr == nil && cfg != nil {
		if cfg.ContextLines > 0 {
			contextLines = cfg.ContextLines
		}
	}
	if contextLines > 200 {
		contextLines = 200
	}
	fullContext := detector.GetRecentContext(scrollbackData, contextLines)
	if fullContext == "" {
		fullContext = output
	}

	// 添加调试日志
	utils.Debug("handleApproval called",
		zap.String("terminal", s.id),
		zap.Int("context_len", len(fullContext)),
		zap.Int("output_len", len(output)))

	// 评估审批 - 传入原始output用于检测，fullContext用于AI分析
	result, err := s.approvalEngine.EvaluateWithContext(ctx, s.id, output, fullContext)
	if err != nil {
		utils.Error("Approval evaluation failed", zap.Error(err))
		return
	}

	// 添加调试日志
	utils.Info("Approval evaluation result",
		zap.String("terminal", s.id),
		zap.String("action", string(result.Action)),
		zap.String("input", result.Input),
		zap.String("reasoning", result.Reasoning),
		zap.Float64("confidence", result.Confidence),
		zap.Bool("ai_decision", result.AIDecision))

	if result.Reasoning == "不是审批提示" {
		utils.Info("Skipping - not approval prompt", zap.String("terminal", s.id))
		return
	}

	// 构建审批事件
	approvalEvent := &ApprovalEvent{
		Action:      string(result.Action),
		Input:       result.Input,
		Reasoning:   result.Reasoning,
		Confidence:  result.Confidence,
		RuleMatched: result.RuleMatched,
		AIDecision:  result.AIDecision,
		AutoHandled: false,
	}

	// 如果是自动通过/自动输入，执行输入
	shouldExecute := (result.Action == approval.ActionApprove || result.Action == approval.ActionInput) && result.Input != ""
	utils.Info("Checking execution condition",
		zap.String("terminal", s.id),
		zap.String("action", string(result.Action)),
		zap.Bool("input_not_empty", result.Input != ""),
		zap.Bool("should_execute", shouldExecute))

	if shouldExecute {
		utils.Info("Executing auto approval",
			zap.String("terminal", s.id),
			zap.String("action", string(result.Action)),
			zap.String("input", result.Input))

		if err := s.sendApprovalInput(result.Input); err != nil {
			utils.Error("Failed to write approval input", zap.Error(err))
		} else {
			approvalEvent.AutoHandled = true
			utils.Info("Auto-handled approval success",
				zap.String("terminal", s.id),
				zap.String("action", string(result.Action)),
				zap.String("input", result.Input),
				zap.String("reasoning", result.Reasoning))
		}
	} else {
		utils.Info("Not auto-handling approval",
			zap.String("terminal", s.id),
			zap.String("action", string(result.Action)),
			zap.String("input", result.Input),
			zap.Bool("input_empty", result.Input == ""))
	}

	// 记录审批操作
	var aiSessionID *string
	s.metaMutex.RLock()
	if s.aiAssistant != nil && strings.TrimSpace(s.aiAssistant.AISessionID) != "" {
		id := strings.TrimSpace(s.aiAssistant.AISessionID)
		aiSessionID = &id
	}
	s.metaMutex.RUnlock()

	promptType := "unknown"
	if s.detector.IsApprovalPrompt(fullContext) {
		promptType = "approval"
	}

	recordPrompt := output
	if fullContext != "" {
		// 优先存储提取后的审批提示（更易读，避免只截到一段 ANSI/片段）
		if _, extracted := s.detector.DetectState(fullContext); extracted != "" {
			recordPrompt = extracted
		} else {
			recordPrompt = fullContext
		}
	}

	s.approvalEngine.RecordApproval(
		s.id,
		aiSessionID,
		promptType,
		recordPrompt,
		result.Input,
		approvalEvent.AutoHandled,
		result.RuleMatched,
		result.Reasoning,
	)

	// 广播审批事件
	s.broadcast(StreamEvent{
		Type:           StreamEventApproval,
		Message:        recordPrompt,
		ApprovalResult: approvalEvent,
	})
}

// sendApprovalInput sends auto-approval input to the terminal.
//
// For tmux-backed sessions, prefer `tmux send-keys` to avoid edge cases where
// writing raw control bytes does not trigger selection UIs (e.g., Claude Code
// trust prompt: "Enter to confirm").
func (s *Session) sendApprovalInput(input string) error {
	normalized := normalizeApprovalInput(input)

	tmuxSession := s.currentTmuxSession()
	if tmuxSession != "" {
		target := tmuxSession + ":0.0"
		if err := s.sendTmuxInput(target, normalized); err == nil {
			// Keep input logs consistent even when bypassing PTY write.
			s.addInputLog([]byte(normalized))
			utils.Info("Approval input sent via tmux",
				zap.String("terminal", s.id),
				zap.String("target", target))
			return nil
		} else {
			utils.Warn("tmux send-keys for approval failed, falling back to PTY write",
				zap.String("terminal", s.id),
				zap.String("target", target),
				zap.Error(err))
		}
	}

	return s.Write([]byte(normalized))
}

func (s *Session) currentTmuxSession() string {
	s.metaMutex.RLock()
	tmuxSession := strings.TrimSpace(s.metadata.TmuxSession)
	s.metaMutex.RUnlock()
	return tmuxSession
}

func normalizeApprovalInput(input string) string {
	// Normalize common variants (some callers may pass \r\n).
	return strings.ReplaceAll(input, "\r\n", "\r")
}

// SendKeyAction sends a configured key binding preset to the terminal.
func (s *Session) SendKeyAction(actionID string) error {
	id := keybinding.Alias(actionID)
	if id == "" {
		return errors.New("key action is required")
	}

	item, err := keybinding.Get(id)
	if err != nil {
		return err
	}
	decoded, err := keybinding.DecodePtyInput(item.PtyInput)
	if err != nil {
		return err
	}
	if decoded == "" {
		return errors.New("key action resolved to empty input")
	}

	tmuxSession := s.currentTmuxSession()
	if tmuxSession != "" {
		target := tmuxSession + ":0.0"
		tmuxKeys := strings.TrimSpace(item.TmuxKeys)
		if tmuxKeys != "" && len(decoded) == 1 {
			if err := sendTmuxKeys(target, tmuxKeys, item.TmuxLiteral); err == nil {
				s.addInputLog([]byte(decoded))
				return nil
			}
		}

		if err := s.sendTmuxInput(target, decoded); err == nil {
			s.addInputLog([]byte(decoded))
			return nil
		}
	}

	return s.Write([]byte(decoded))
}

func (s *Session) sendTmuxInput(target string, input string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("tmux target is required")
	}
	if input == "" {
		return nil
	}

	enterKey := tmuxKeyOrFallback(keybinding.IDEnter, "C-m")
	newlineKey := tmuxKeyOrFallback(keybinding.IDNewline, "C-j")
	escKey := tmuxKeyOrFallback(keybinding.IDEsc, "Escape")
	ctrlCKey := tmuxKeyOrFallback(keybinding.IDCtrlC, "C-c")
	ctrlDKey := tmuxKeyOrFallback(keybinding.IDCtrlD, "C-d")
	tabKey := tmuxKeyOrFallback(keybinding.IDTab, "Tab")

	var buf strings.Builder
	flushText := func() error {
		if buf.Len() == 0 {
			return nil
		}
		text := buf.String()
		buf.Reset()
		return sendTmuxKeys(target, text, true)
	}

	for i := 0; i < len(input); i++ {
		b := input[i]
		switch b {
		case '\r':
			if err := flushText(); err != nil {
				return err
			}
			if err := sendTmuxKeys(target, enterKey, false); err != nil {
				return err
			}
		case '\n':
			if err := flushText(); err != nil {
				return err
			}
			if err := sendTmuxKeys(target, newlineKey, false); err != nil {
				return err
			}
		case '\t':
			if err := flushText(); err != nil {
				return err
			}
			if err := sendTmuxKeys(target, tabKey, false); err != nil {
				return err
			}
		case 0x1b:
			if err := flushText(); err != nil {
				return err
			}
			if err := sendTmuxKeys(target, escKey, false); err != nil {
				return err
			}
		case 0x03:
			if err := flushText(); err != nil {
				return err
			}
			if err := sendTmuxKeys(target, ctrlCKey, false); err != nil {
				return err
			}
		case 0x04:
			if err := flushText(); err != nil {
				return err
			}
			if err := sendTmuxKeys(target, ctrlDKey, false); err != nil {
				return err
			}
		default:
			if b < 0x20 {
				return errors.New("unsupported control byte for tmux send")
			}
			buf.WriteByte(b)
		}
	}

	return flushText()
}

func tmuxKeyOrFallback(bindingID string, fallback string) string {
	item, err := keybinding.Get(bindingID)
	if err != nil {
		return fallback
	}
	key := strings.TrimSpace(item.TmuxKeys)
	if key == "" {
		return fallback
	}
	return key
}

func sendTmuxKeys(target string, keys string, literal bool) error {
	args := []string{"send-keys", "-t", target}
	if literal {
		args = append(args, "-l")
	}
	args = append(args, "--", keys)

	cmd := execCommand("tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		utils.Warn("tmux send-keys failed",
			zap.String("target", target),
			zap.String("keys", keys),
			zap.String("output", string(out)),
			zap.Error(err))
		return err
	}
	return nil
}

// wait 等待进程退出
func (s *Session) wait() {
	if s.cmd != nil && s.cmd.Process != nil {
		state, _ := s.cmd.Process.Wait()
		exitCode := 0
		if state != nil {
			exitCode = state.ExitCode()
		}

		s.metaMutex.Lock()
		s.status = "exited"
		s.metadata.Status = "exited"
		now := time.Now()
		s.closedAt = &now
		s.metaMutex.Unlock()

		s.broadcast(StreamEvent{
			Type:     StreamEventExit,
			ExitCode: exitCode,
			Message:  "Process exited",
		})

		s.doneOnce.Do(func() { close(s.done) })
	}
}

// Subscribe 订阅会话事件
func (s *Session) Subscribe() (string, chan StreamEvent) {
	return s.SubscribeWithBuffer(256)
}

// SubscribeWithBuffer 订阅会话事件（可指定缓冲区大小）。
func (s *Session) SubscribeWithBuffer(buffer int) (string, chan StreamEvent) {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	id := uuid.New().String()
	if buffer <= 0 {
		buffer = 256
	}
	ch := make(chan StreamEvent, buffer)
	s.subscribers[id] = ch
	return id, ch
}

// Unsubscribe 取消订阅
func (s *Session) Unsubscribe(id string) {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	if ch, ok := s.subscribers[id]; ok {
		close(ch)
		delete(s.subscribers, id)
	}
}

// broadcast 广播事件
func (s *Session) broadcast(event StreamEvent) {
	s.subMutex.RLock()
	defer s.subMutex.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// 通道已满，丢弃事件
		}
	}
}

// broadcastMetadata 广播元数据（带节流）
func (s *Session) broadcastMetadata() {
	s.metaThrottleMu.Lock()
	// 节流：最少间隔100ms
	if time.Since(s.metaThrottleLast) < 100*time.Millisecond {
		s.metaThrottleMu.Unlock()
		return
	}
	s.metaThrottleLast = time.Now()
	s.metaThrottleMu.Unlock()

	s.metaMutex.RLock()
	metadata := *s.metadata
	s.metaMutex.RUnlock()

	s.broadcast(StreamEvent{
		Type:     StreamEventMetadata,
		Metadata: &metadata,
	})
}

// Write 写入数据到PTY
func (s *Session) Write(data []byte) error {
	// 记录输入日志
	s.addInputLog(data)

	if s.backend != nil {
		err := s.backend.Write(data)
		utils.Info("Terminal backend write result",
			zap.String("terminal", s.id),
			zap.Int("bytes", len(data)),
			zap.String("data", string(data)),
			zap.Error(err))
		return err
	}

	if s.pty == nil {
		utils.Warn("Write failed: pty is nil", zap.String("terminal", s.id))
		return nil
	}
	n, err := s.pty.Write(data)
	utils.Info("PTY Write result",
		zap.String("terminal", s.id),
		zap.Int("bytes_written", n),
		zap.String("data", string(data)),
		zap.Error(err))
	return err
}

// Resize 调整PTY大小
func (s *Session) Resize(cols, rows uint16) error {
	if s.backend != nil {
		return s.backend.Resize(cols, rows)
	}
	if s.pty == nil {
		return nil
	}
	err := pty.Setsize(s.pty, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if errors.Is(err, pty.ErrUnsupported) {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

// Close 关闭会话
func (s *Session) Close() error {
	// 关闭 done channel，通知所有 goroutine 退出
	s.doneOnce.Do(func() { close(s.done) })

	if s.backend != nil {
		err := s.backend.Close()
		if s.pty != nil {
			_ = s.pty.Close()
			s.pty = nil
		}
		return err
	}

	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(os.Interrupt)
		time.Sleep(100 * time.Millisecond)
		_ = s.cmd.Process.Kill()
	}
	if s.pty != nil {
		s.pty.Close()
	}
	return nil
}

// SendApprovalResponse 手动发送审批响应
func (s *Session) SendApprovalResponse(input string) error {
	return s.Write([]byte(input))
}

// GetAutomationConfig 获取自动化配置
func (s *Session) GetAutomationConfig() (*approval.EffectiveConfig, error) {
	return s.approvalEngine.GetAutomationConfig(s.id)
}

// RefreshAutomationConfig 刷新自动化配置元数据
func (s *Session) RefreshAutomationConfig() error {
	config, err := s.approvalEngine.GetAutomationConfig(s.id)
	if err != nil {
		return err
	}

	// 更新元数据
	s.metaMutex.Lock()
	s.metadata.AutomationMode = config.ApprovalMode
	s.metaMutex.Unlock()

	s.broadcastMetadata()
	return nil
}

// ReevaluateApprovalIfWaiting 当终端处于等待审批状态时，按最新规则重新评估一次
func (s *Session) ReevaluateApprovalIfWaiting() {
	s.metaMutex.RLock()
	state := ""
	if s.aiAssistant != nil {
		state = s.aiAssistant.State
	}
	s.metaMutex.RUnlock()

	if state != string(detector.StateWaitingApproval) {
		return
	}

	s.lastOutputMu.Lock()
	output := s.lastOutput
	s.lastOutputMu.Unlock()

	go s.handleApproval(output)
}

// Getters
func (s *Session) ID() string           { return s.id }
func (s *Session) Title() string        { return s.title }
func (s *Session) TaskID() *string      { return s.taskID }
func (s *Session) Status() string       { return s.status }
func (s *Session) CreatedAt() time.Time { return s.createdAt }
func (s *Session) ClosedAt() *time.Time { return s.closedAt }
func (s *Session) Scrollback() []byte   { return s.scrollback.Read() }
func (s *Session) Metadata() *SessionMetadata {
	s.metaMutex.RLock()
	defer s.metaMutex.RUnlock()
	meta := *s.metadata
	return &meta
}

// RunCommand 在当前终端会话中执行命令，并通过标记符采集命令输出。
// 注意：该方法会将命令“真实输入”到终端里（用户可实时看到），并返回命令输出与退出码。
func (s *Session) RunCommand(command, workDir string, timeout time.Duration) (string, int, error) {
	if s == nil {
		return "", -1, errors.New("terminal session is nil")
	}

	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", -1, errors.New("command is required")
	}

	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	s.commandRunMu.Lock()
	defer s.commandRunMu.Unlock()

	select {
	case <-s.done:
		return "", -1, errors.New("terminal session already closed")
	default:
	}

	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	startMarker := "__ACA_CMD_BEGIN_" + id + "__"
	endMarkerPrefix := "__ACA_CMD_END_" + id + "__:"
	heredocMarker := "ACA_EOF_" + id

	subID, events := s.SubscribeWithBuffer(4096)
	defer s.Unsubscribe(subID)

	lines := buildRunCommandLines(startMarker, endMarkerPrefix, heredocMarker, cmd, strings.TrimSpace(workDir))
	for _, line := range lines {
		if err := s.Write([]byte(line + "\r")); err != nil {
			return "", -1, err
		}
		// 模拟人类输入节奏，避免远端 shell/网络缓冲导致丢字符
		time.Sleep(20 * time.Millisecond)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	started := false
	exitCode := -1
	truncated := false

	const maxCapturedBytes = 200_000
	var captured strings.Builder
	captured.Grow(16 * 1024)

	pending := ""

	appendCaptured := func(chunk string) {
		if truncated || chunk == "" {
			return
		}
		remaining := maxCapturedBytes - captured.Len()
		if remaining <= 0 {
			truncated = true
			return
		}
		if len(chunk) <= remaining {
			captured.WriteString(chunk)
			return
		}
		captured.WriteString(chunk[:remaining])
		truncated = true
	}

	findOutputLineMarker := func(text, marker string) (int, bool) {
		searchFrom := 0
		for {
			idx := strings.Index(text[searchFrom:], marker)
			if idx == -1 {
				return -1, false
			}
			abs := searchFrom + idx
			nextPos := abs + len(marker)
			if nextPos >= len(text) {
				return abs, true
			}
			next := text[nextPos]
			if next == '\n' || next == '\r' {
				return abs, true
			}
			searchFrom = abs + 1
			if searchFrom >= len(text) {
				return -1, false
			}
		}
	}

	for {
		select {
		case <-timer.C:
			out := strings.ToValidUTF8(stripANSI(captured.String()), "")
			if truncated {
				out = out + "\n…(truncated)…"
			}
			return strings.TrimSpace(out), exitCode, fmt.Errorf("command timeout after %s", timeout)
		case <-s.done:
			out := strings.ToValidUTF8(stripANSI(captured.String()), "")
			if truncated {
				out = out + "\n…(truncated)…"
			}
			return strings.TrimSpace(out), exitCode, errors.New("terminal session closed")
		case event, ok := <-events:
			if !ok {
				out := strings.ToValidUTF8(stripANSI(captured.String()), "")
				if truncated {
					out = out + "\n…(truncated)…"
				}
				return strings.TrimSpace(out), exitCode, errors.New("terminal subscription closed")
			}
			if event.Type != StreamEventData || strings.TrimSpace(event.Data) == "" {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil || len(raw) == 0 {
				continue
			}
			pending += string(raw)

			if !started {
				idx, ok := findOutputLineMarker(pending, startMarker)
				if !ok {
					// 保留尾部，避免 marker 跨 chunk 丢失
					keep := len(startMarker) + 64
					if len(pending) > keep {
						pending = pending[len(pending)-keep:]
					}
					continue
				}
				pending = pending[idx+len(startMarker):]
				pending = strings.TrimLeft(pending, "\r\n")
				started = true
			}

			endIdx := strings.Index(pending, endMarkerPrefix)
			if endIdx == -1 {
				keep := len(endMarkerPrefix) + 64
				if len(pending) > keep {
					appendCaptured(pending[:len(pending)-keep])
					pending = pending[len(pending)-keep:]
				}
				continue
			}

			appendCaptured(pending[:endIdx])

			rest := pending[endIdx+len(endMarkerPrefix):]
			rest = strings.TrimLeft(rest, " \t\r\n")
			codeStr := ""
			for _, r := range rest {
				if r < '0' || r > '9' {
					break
				}
				codeStr += string(r)
				if len(codeStr) > 6 {
					break
				}
			}
			if codeStr != "" {
				if code, err := strconv.Atoi(codeStr); err == nil {
					exitCode = code
				}
			} else {
				exitCode = 0
			}

			out := strings.ToValidUTF8(stripANSI(captured.String()), "")
			if truncated {
				out = out + "\n…(truncated)…"
			}
			out = strings.TrimSpace(out)

			if exitCode != 0 {
				return out, exitCode, fmt.Errorf("command exit code %d", exitCode)
			}
			return out, exitCode, nil
		}
	}
}

func buildRunCommandLines(startMarker, endMarkerPrefix, heredocMarker, command, workDir string) []string {
	snippet := strings.ReplaceAll(command, "\r\n", "\n")
	snippet = strings.ReplaceAll(snippet, "\r", "\n")
	snippet = strings.TrimRight(snippet, "\n")

	snippetLines := strings.Split(snippet, "\n")
	singleLine := len(snippetLines) == 1 && strings.TrimSpace(workDir) == ""

	lines := make([]string, 0, 4+len(snippetLines))
	lines = append(lines, "echo '"+startMarker+"'")

	if singleLine {
		lines = append(lines, snippetLines[0])
		lines = append(lines, fmt.Sprintf("ACA_CODE=$?; echo '%s'$ACA_CODE; unset ACA_CODE", endMarkerPrefix))
		return lines
	}

	// 多行命令 / 指定 workDir 时：使用 heredoc 交给子 bash 执行，避免改变交互 shell 的工作目录/环境。
	lines = append(lines, fmt.Sprintf("bash <<'%s'", heredocMarker))
	if strings.TrimSpace(workDir) != "" {
		lines = append(lines, "cd -- "+quoteShellPathForTerminal(workDir))
	}
	lines = append(lines, snippetLines...)
	lines = append(lines, heredocMarker)
	lines = append(lines, fmt.Sprintf("ACA_CODE=$?; echo '%s'$ACA_CODE; unset ACA_CODE", endMarkerPrefix))
	return lines
}

func quoteShellPathForTerminal(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "''"
	}

	if trimmed == "~" {
		return "\"$HOME\""
	}
	if strings.HasPrefix(trimmed, "~/") {
		rest := strings.TrimPrefix(trimmed, "~/")
		return "\"$HOME/" + escapeDoubleQuotedForTerminal(rest) + "\""
	}

	return "'" + strings.ReplaceAll(trimmed, "'", "'\\''") + "'"
}

func escapeDoubleQuotedForTerminal(text string) string {
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "$", "\\$")
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	return escaped
}

// InjectOutput 将文本注入到终端输出流（仅用于展示/观测，不会写入到底层 PTY/SSH）。
// 典型用途：AI 托管后台执行的命令/结果，需要在工作台终端实时可见。
func (s *Session) InjectOutput(data []byte) {
	if s == nil {
		return
	}
	if len(data) == 0 {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}

	s.scrollback.Write(data)
	s.batchBroadcastData(data)
}

// BroadcastAILog 广播AI日志事件
func (s *Session) BroadcastAILog(logType, message string) {
	s.broadcast(StreamEvent{
		Type: StreamEventAILog,
		AILog: &AILogEvent{
			Type:    logType,
			Message: message,
		},
	})
}

// BroadcastAILogWithInput 广播带输入信息的AI日志事件
func (s *Session) BroadcastAILogWithInput(logType, message, inputType, inputData string) {
	s.broadcast(StreamEvent{
		Type: StreamEventAILog,
		AILog: &AILogEvent{
			Type:      logType,
			Message:   message,
			InputType: inputType,
			InputData: inputData,
		},
	})
}

// Setters
func (s *Session) SetTitle(title string) {
	s.metaMutex.Lock()
	s.title = title
	s.metadata.Title = title
	s.metaMutex.Unlock()
}

func (s *Session) SetTaskID(taskID *string) {
	s.metaMutex.Lock()
	// NOTE: fiber/fasthttp may return strings backed by an internal buffer (zero-copy).
	// Do not retain those string headers/pointers beyond the request lifecycle.
	// Always copy the value we store in the session to avoid later corruption.
	if taskID == nil || strings.TrimSpace(*taskID) == "" {
		s.taskID = nil
		s.metadata.TaskID = nil
		s.metaMutex.Unlock()
		s.refreshTaskCLISelection()
		return
	}

	copied := strings.Clone(strings.TrimSpace(*taskID))
	s.taskID = &copied
	s.metadata.TaskID = s.taskID
	s.metaMutex.Unlock()

	s.refreshTaskCLISelection()
}

func (s *Session) SetServerInfo(serverID *string, name, host string) {
	s.metaMutex.Lock()
	// Same reason as SetTaskID: copy values to avoid retaining unsafe string headers.
	if serverID == nil || strings.TrimSpace(*serverID) == "" {
		s.serverID = nil
		s.metadata.ServerID = nil
	} else {
		copied := strings.Clone(strings.TrimSpace(*serverID))
		s.serverID = &copied
		s.metadata.ServerID = s.serverID
	}
	s.metadata.ServerName = strings.Clone(strings.TrimSpace(name))
	s.metadata.ServerHost = strings.Clone(strings.TrimSpace(host))
	s.metaMutex.Unlock()
}

func (s *Session) refreshTaskCLISelection() {
	if s == nil {
		return
	}

	taskID := ""
	s.metaMutex.RLock()
	if s.taskID != nil {
		taskID = strings.TrimSpace(*s.taskID)
	}
	s.metaMutex.RUnlock()

	enabled := false
	expectedType := detector.AIAgentUnknown
	displayName := ""
	taskMode := ""

	if taskID != "" && model.DB != nil {
		var task model.Task
		if err := model.DB.Select("automation_mode", "ai_managed", "cli_type").First(&task, "id = ?", taskID).Error; err == nil {
			taskMode = strings.ToLower(strings.TrimSpace(task.AutomationMode))
			enabled = task.AIManaged || taskMode == "cli"

			// CLI 可选：只有 automation_mode=cli 时才“强绑定” cli_type；否则允许 unknown，由输出锚点/人工确认决定。
			if taskMode == "cli" {
				cliType := strings.ToLower(strings.TrimSpace(task.CLIType))
				switch cliType {
				case "claude":
					expectedType = detector.AIAgentClaudeCode
					displayName = "Claude Code"
				case "codex":
					expectedType = detector.AIAgentCodex
					displayName = "OpenAI Codex"
				case "gemini":
					expectedType = detector.AIAgentGemini
					displayName = "Gemini CLI"
				default:
					expectedType = detector.AIAgentUnknown
					displayName = ""
				}
			} else {
				expectedType = detector.AIAgentUnknown
				displayName = "AI CLI"
			}
		}
	}

	if !enabled || (taskMode == "cli" && expectedType == detector.AIAgentUnknown) {
		s.metaMutex.Lock()
		s.cliTrackingEnabled = false
		s.expectedAIAgentType = detector.AIAgentUnknown
		s.aiAssistant = nil
		s.metadata.AIAssistant = nil
		s.cliManualPresent = nil
		s.cliManualUntil = time.Time{}
		s.cliSessionManager = nil
		s.metaMutex.Unlock()
		s.broadcastMetadata()
		return
	}

	s.metaMutex.Lock()
	s.cliTrackingEnabled = true
	s.expectedAIAgentType = expectedType

	if s.aiAssistant == nil || strings.TrimSpace(s.aiAssistant.Type) != string(expectedType) {
		// 任务关键字段变化（automation_mode/cli_type）时，清理旧的会话跟踪器，避免把旧会话误关联到新 CLI。
		s.cliSessionManager = nil
		s.aiAssistant = &AIAssistant{
			Type:           string(expectedType),
			DisplayName:    displayName,
			State:          string(detector.StateUnknown),
			StateUpdatedAt: time.Now(),
			Detected:       false,
			Version:        "",
			ApprovalPrompt: "",
			NeedsConfirm:   false,
			ConfirmKind:    "",
			ConfirmMessage: "",
			Source:         "",
			Confidence:     0,
			Manual:         false,
		}
	} else if s.aiAssistant != nil && s.aiAssistant.DisplayName != displayName {
		s.aiAssistant.DisplayName = displayName
	}

	s.metadata.AIAssistant = s.aiAssistant
	s.metaMutex.Unlock()

	s.broadcastMetadata()
}

func normalizeAIAssistantType(value string) (detector.AIAgentType, string) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "claude", "claude-code", "claude_code":
		return detector.AIAgentClaudeCode, "Claude Code"
	case "codex", "openai-codex":
		return detector.AIAgentCodex, "OpenAI Codex"
	case "gemini":
		return detector.AIAgentGemini, "Gemini CLI"
	default:
		return detector.AIAgentUnknown, ""
	}
}

// ConfirmCLIState allows user/AI to manually confirm whether the terminal is currently inside an AI CLI.
//
// decision: yes/no/unknown (also accepts y/n/true/false/unsure).
// assistantType: optional hint (claude/codex/gemini).
func (s *Session) ConfirmCLIState(decision string, assistantType string, ttl time.Duration) error {
	if s == nil {
		return errors.New("session is nil")
	}

	d := strings.ToLower(strings.TrimSpace(decision))
	var present *bool
	switch d {
	case "yes", "y", "true", "in", "inside":
		v := true
		present = &v
	case "no", "n", "false", "out", "outside":
		v := false
		present = &v
	case "unknown", "unsure", "maybe", "idk", "":
		present = nil
	default:
		return fmt.Errorf("invalid decision: %s", decision)
	}

	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	until := time.Now().Add(ttl)

	agentType, agentDisplay := normalizeAIAssistantType(assistantType)

	s.metaMutex.Lock()
	s.cliManualPresent = present
	if present == nil {
		s.cliManualUntil = time.Time{}
	} else {
		s.cliManualUntil = until
	}

	if s.aiAssistant == nil {
		s.aiAssistant = &AIAssistant{
			Type:           string(detector.AIAgentUnknown),
			DisplayName:    "AI CLI",
			State:          string(detector.StateUnknown),
			StateUpdatedAt: time.Now(),
			Detected:       false,
		}
	}

	// Apply type hint if provided and known.
	if agentType != detector.AIAgentUnknown && strings.TrimSpace(string(agentType)) != "" {
		s.aiAssistant.Type = string(agentType)
		if agentDisplay != "" {
			s.aiAssistant.DisplayName = agentDisplay
		}
	}

	s.aiAssistant.Manual = present != nil
	s.aiAssistant.Source = "manual"
	s.aiAssistant.Confidence = 1
	s.aiAssistant.NeedsConfirm = false
	s.aiAssistant.ConfirmKind = ""
	s.aiAssistant.ConfirmMessage = ""

	if present != nil {
		s.aiAssistant.Detected = *present
	} else {
		// unknown => do not force detected=true; keep current value (but safe default is false).
		s.aiAssistant.Detected = false
		s.aiAssistant.Manual = false
		s.aiAssistant.Source = ""
		s.aiAssistant.Confidence = 0
	}

	s.metadata.AIAssistant = s.aiAssistant
	s.metaMutex.Unlock()

	s.broadcastMetadata()
	return nil
}

// ToDBModel 转换为数据库模型
func (s *Session) ToDBModel() *model.TerminalSession {
	return &model.TerminalSession{
		ID:        s.id,
		Title:     s.title,
		TaskID:    s.taskID,
		ServerID:  s.serverID,
		Shell:     s.shell,
		Status:    s.status,
		PID:       s.metadata.PID,
		Hidden:    false,
		CreatedAt: s.createdAt,
		ClosedAt:  s.closedAt,
	}
}

// addInputLog 将用户/自动化输入按行聚合后写入日志缓冲（避免按字符记录）
func (s *Session) addInputLog(data []byte) {
	s.ioBufMutex.Lock()
	defer s.ioBufMutex.Unlock()

	for _, r := range string(data) {
		// 处理方向键等转义序列，避免污染输入日志
		switch s.inputEscState {
		case 1: // 刚收到 ESC
			switch r {
			case '[':
				s.inputEscState = 2
				continue
			case 'O':
				s.inputEscState = 3
				continue
			default:
				s.inputEscState = 0
				continue
			}
		case 2: // CSI(ESC[...)
			// 以字母或 ~ 结束
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '~' {
				s.inputEscState = 0
			}
			continue
		case 3: // SS3(ESCO...)
			s.inputEscState = 0
			continue
		}

		switch r {
		case '\x1b':
			s.inputEscState = 1
			continue
		case '\r', '\n':
			s.flushInputLineLocked()
			continue
		case '\b', '\x7f':
			if len(s.inputLineBuf) > 0 {
				s.inputLineBuf = s.inputLineBuf[:len(s.inputLineBuf)-1]
			}
			continue
		}

		// 忽略控制字符（保留制表符）
		if r < 32 && r != '\t' {
			continue
		}

		s.inputLineBuf = append(s.inputLineBuf, r)
		// 防止极端情况下输入缓冲无限增长
		if len(s.inputLineBuf) > 4096 {
			s.inputLineBuf = s.inputLineBuf[len(s.inputLineBuf)-4096:]
		}
	}
}

func (s *Session) flushInputLineLocked() {
	if len(s.inputLineBuf) == 0 {
		return
	}
	line := strings.TrimSpace(string(s.inputLineBuf))
	s.inputLineBuf = s.inputLineBuf[:0]
	if line == "" {
		return
	}
	// 过滤内部命令标记（ACA_CMD_BEGIN/END、ACA_CODE、ACA_EOF等）
	if isInternalMarkerLine(line) {
		return
	}

	s.addLog("input", line+"\n")
}

// addOutputLog 将PTY输出按行聚合，过滤提示符/回显/内部标记后写入日志缓冲
func (s *Session) addOutputLog(data []byte) {
	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked(data)
	s.ioBufMutex.Unlock()

	for _, line := range lines {
		// 保留前导空白（可能有缩进），仅去掉右侧空白
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isShellPromptOrCommandLine(line) {
			continue
		}
		// 过滤内部命令标记行（ACA_CMD_BEGIN/END、ACA_CODE、ACA_EOF等）
		if isInternalMarkerLine(line) {
			continue
		}
		s.addLog("output", line+"\n")
	}
}

func (s *Session) consumeOutputLinesLocked(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	// 拼接上次残留（ANSI/UTF-8 可能跨 chunk）
	buf := append(s.outputRemainder, data...)
	s.outputRemainder = nil

	lines := make([]string, 0, 8)

	for i := 0; i < len(buf); {
		b := buf[i]

		// ANSI / VT 序列处理（尽量模拟行内覆盖，避免日志叠字）
		if b == 0x1b { // ESC
			seqLen, seqType, ok := scanEscapeSequence(buf[i:])
			if !ok {
				// 不完整序列，留待下次
				s.outputRemainder = append(s.outputRemainder, buf[i:]...)
				break
			}
			seq := buf[i : i+seqLen]
			switch seqType {
			case "CSI":
				s.applyCSISequenceLocked(seq)
			case "ESC":
				s.applyESCSequenceLocked(seq)
			}
			i += seqLen
			continue
		}

		switch b {
		case '\n':
			if s.outputSuppressWrites {
				i++
				continue
			}
			s.flushOutputLineLocked(&lines)
			i++
			continue
		case '\r':
			if s.outputSuppressWrites {
				i++
				continue
			}
			// 回车：回到行首
			// 检查下一个字符是否是 \n（\r\n 是正常换行）
			if i+1 < len(buf) && buf[i+1] != '\n' {
				// 不是 \r\n，说明是进度条等动态输出，清空当前行避免叠加
				s.outputLineBuf = s.outputLineBuf[:0]
			}
			s.outputCursor = 0
			i++
			continue
		case '\b', 0x7f:
			if s.outputSuppressWrites {
				i++
				continue
			}
			// 退格：仅移动光标（是否擦除由后续输出决定）
			if s.outputCursor > 0 {
				s.outputCursor--
			}
			i++
			continue
		case '\t':
			if s.outputSuppressWrites {
				i++
				continue
			}
			s.writeOutputRuneLocked('\t')
			i++
			continue
		default:
			// 忽略其他控制字符
			if b < 32 {
				i++
				continue
			}
		}

		// UTF-8 解码（确保中文不乱码；遇到半个 rune 留待下次）
		r, size := utf8.DecodeRune(buf[i:])
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(buf[i:]) {
				s.outputRemainder = append(s.outputRemainder, buf[i:]...)
				break
			}
			// 非法字节，跳过
			i++
			continue
		}
		if s.outputSuppressWrites {
			i += size
			continue
		}
		s.writeOutputRuneLocked(r)
		i += size
	}

	return lines
}

func (s *Session) flushOutputLineLocked(lines *[]string) {
	if len(s.outputLineBuf) == 0 {
		s.outputCursor = 0
		return
	}

	line := string(s.outputLineBuf)
	// 重置行缓冲
	s.outputLineBuf = s.outputLineBuf[:0]
	s.outputCursor = 0

	if strings.TrimSpace(line) == "" {
		return
	}

	*lines = append(*lines, line)
}

func (s *Session) writeOutputRuneLocked(r rune) {
	// 过长保护（极端情况下避免无限增长）
	if s.outputLineBufSize > 0 && len(s.outputLineBuf) > s.outputLineBufSize {
		overflow := len(s.outputLineBuf) - s.outputLineBufSize
		s.outputLineBuf = s.outputLineBuf[overflow:]
		if s.outputCursor >= overflow {
			s.outputCursor -= overflow
		} else {
			s.outputCursor = 0
		}
	}

	// 光标可能超出当前内容，补空格到光标位置
	for len(s.outputLineBuf) < s.outputCursor {
		s.outputLineBuf = append(s.outputLineBuf, ' ')
	}

	if s.outputCursor < len(s.outputLineBuf) {
		s.outputLineBuf[s.outputCursor] = r
	} else {
		s.outputLineBuf = append(s.outputLineBuf, r)
	}
	s.outputCursor++
}

func (s *Session) applyCSISequenceLocked(seq []byte) {
	// seq 形如: ESC [ ... final
	if len(seq) < 3 || seq[0] != 0x1b || seq[1] != '[' {
		return
	}
	final := seq[len(seq)-1]
	paramStr := string(seq[2 : len(seq)-1])
	// 去掉私有前缀 '?'
	paramStr = strings.TrimPrefix(paramStr, "?")

	params := parseCSIParams(paramStr)
	get1 := func(defaultVal int) int {
		if len(params) == 0 {
			return defaultVal
		}
		if params[0] == 0 {
			return defaultVal
		}
		return params[0]
	}

	// When we are in an off-screen UI write section (e.g. status bar rendered via save+cursorTo+restore),
	// ignore all mutations to the current line buffer until cursor restore arrives.
	if s.outputSuppressWrites {
		switch final {
		case 'u': // Restore cursor
			s.outputCursor = s.outputSavedCursor
			s.outputCursorSaveActive = false
			s.outputSuppressWrites = false
		}
		return
	}

	// Heuristic: if a program uses save/restore and then issues clear-line / clear-screen,
	// it's very likely updating a transient UI region (status bar / spinner). Our log line
	// buffer is single-line only, so applying these clears would corrupt the current line.
	if s.outputCursorSaveActive {
		switch final {
		case 'K', 'J':
			s.outputSuppressWrites = true
			return
		}
	}

	switch final {
	case 'K': // EL - Erase in Line
		mode := 0
		if len(params) > 0 {
			mode = params[0]
		}
		switch mode {
		case 0: // clear from cursor to end
			if s.outputCursor < len(s.outputLineBuf) {
				s.outputLineBuf = s.outputLineBuf[:s.outputCursor]
			}
		case 1: // clear from start to cursor (用空格覆盖)
			if s.outputCursor > len(s.outputLineBuf) {
				s.outputCursor = len(s.outputLineBuf)
			}
			for i := 0; i < s.outputCursor && i < len(s.outputLineBuf); i++ {
				s.outputLineBuf[i] = ' '
			}
		case 2: // clear entire line
			s.outputLineBuf = s.outputLineBuf[:0]
			s.outputCursor = 0
		}
	case 'J': // ED - Erase in Display（日志里按清行处理）
		s.outputLineBuf = s.outputLineBuf[:0]
		s.outputCursor = 0
	case 'D': // Cursor Left
		n := get1(1)
		s.outputCursor -= n
		if s.outputCursor < 0 {
			s.outputCursor = 0
		}
	case 'C': // Cursor Right
		n := get1(1)
		s.outputCursor += n
	case 'G': // CHA - Cursor Horizontal Absolute
		n := get1(1)
		if n < 1 {
			n = 1
		}
		s.outputCursor = n - 1
	case 'A', 'B', 'E', 'F': // Cursor up/down/next/prev line
		// We don't model row changes for log lines. When a TUI uses save+move+restore
		// to paint a status bar, treat vertical cursor moves as an off-screen update.
		if s.outputCursorSaveActive {
			s.outputSuppressWrites = true
		}
	case 'H', 'f': // CUP/HVP - Cursor Position
		// ESC[row;colH : default 1;1
		row := 1
		col := 1
		if len(params) >= 1 && params[0] != 0 {
			row = params[0]
		}
		if len(params) >= 2 && params[1] != 0 {
			col = params[1]
		}
		if s.outputCursorSaveActive && row > 1 {
			// Treat cursor move to another row during a save/restore block as a transient UI update
			// (common in AI CLIs). Suppress writes & buffer mutations until restore.
			s.outputSuppressWrites = true
		}
		if col < 1 {
			col = 1
		}
		s.outputCursor = col - 1
	case 's': // Save cursor
		s.outputSavedCursor = s.outputCursor
		s.outputCursorSaveActive = true
	case 'u': // Restore cursor
		s.outputCursor = s.outputSavedCursor
		s.outputCursorSaveActive = false
		s.outputSuppressWrites = false
	case 'm':
		// SGR - ignore
	default:
		// ignore other sequences
	}

	// 防止光标越界
	if s.outputCursor < 0 {
		s.outputCursor = 0
	}
	if s.outputLineBufSize > 0 && s.outputCursor > s.outputLineBufSize {
		s.outputCursor = s.outputLineBufSize
	}
}

func (s *Session) applyESCSequenceLocked(seq []byte) {
	// Common 2-byte ESC sequences used by TUI libraries (e.g. ansi-escapes in Node):
	// - ESC 7 : DECSC (Save cursor)
	// - ESC 8 : DECRC (Restore cursor)
	if len(seq) != 2 || seq[0] != 0x1b {
		return
	}

	switch seq[1] {
	case '7':
		s.outputSavedCursor = s.outputCursor
		s.outputCursorSaveActive = true
	case '8':
		s.outputCursor = s.outputSavedCursor
		s.outputCursorSaveActive = false
		s.outputSuppressWrites = false
	}
}

func scanEscapeSequence(buf []byte) (seqLen int, seqType string, ok bool) {
	if len(buf) < 2 || buf[0] != 0x1b {
		return 0, "", false
	}

	switch buf[1] {
	case '[': // CSI: ESC [ ... final(0x40-0x7E)
		for i := 2; i < len(buf); i++ {
			if buf[i] >= 0x40 && buf[i] <= 0x7e {
				return i + 1, "CSI", true
			}
		}
		return 0, "CSI", false

	case ']': // OSC: ESC ] ... (BEL or ST)
		for i := 2; i < len(buf); i++ {
			if buf[i] == 0x07 { // BEL
				return i + 1, "OSC", true
			}
			if buf[i] == 0x1b && i+1 < len(buf) && buf[i+1] == '\\' { // ST
				return i + 2, "OSC", true
			}
		}
		return 0, "OSC", false

	case 'P', 'X', '^', '_': // DCS/SOS/PM/APC: ESC <type> ... ST
		for i := 2; i < len(buf); i++ {
			if buf[i] == 0x1b && i+1 < len(buf) && buf[i+1] == '\\' {
				return i + 2, "ST", true
			}
		}
		return 0, "ST", false

	default:
		// 其他 ESC 序列通常是 2 字节
		return 2, "ESC", true
	}
}

func parseCSIParams(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	params := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			params = append(params, 0)
			continue
		}
		// 忽略非数字前缀（例如 ?25l 里已提前 TrimPrefix）
		if n, err := strconv.Atoi(p); err == nil {
			params = append(params, n)
		} else {
			params = append(params, 0)
		}
	}
	return params
}

// addLog 添加日志到缓冲区
func (s *Session) addLog(logType, content string) {
	// 清理ANSI转义码
	cleanContent := stripANSI(content)
	// 过滤空内容
	if strings.TrimSpace(cleanContent) == "" {
		return
	}

	// 过滤shell提示符（仅对输出类型）
	if logType == "output" && isShellPromptOnly(cleanContent) {
		return
	}

	// 日志去重：在时间窗口内相同或相似内容不重复记录
	now := time.Now()
	trimmedContent := strings.TrimSpace(cleanContent)

	// 生成去重key：取前50个字符（rune）作为前缀匹配，避免UTF-8截断
	dedupeKey := logType + ":"
	runes := []rune(trimmedContent)
	if len(runes) > 50 {
		dedupeKey += string(runes[:50])
	} else {
		dedupeKey += trimmedContent
	}

	s.recentLogsMu.Lock()
	if lastTime, exists := s.recentLogs[dedupeKey]; exists {
		if now.Sub(lastTime) < s.recentLogsWindow {
			s.recentLogsMu.Unlock()
			return // 重复内容，跳过
		}
	}
	s.recentLogs[dedupeKey] = now
	// 清理过期条目（避免内存泄漏）
	if len(s.recentLogs) > 100 {
		for k, t := range s.recentLogs {
			if now.Sub(t) > s.recentLogsWindow {
				delete(s.recentLogs, k)
			}
		}
	}
	s.recentLogsMu.Unlock()

	s.logMutex.Lock()
	s.logBuffer = append(s.logBuffer, LogEntry{
		LogType:   logType,
		Content:   cleanContent,
		CreatedAt: now,
	})
	shouldFlush := len(s.logBuffer) >= 50 // 缓冲区达到50条时触发刷新
	s.logMutex.Unlock()

	if shouldFlush {
		select {
		case s.logFlushChan <- struct{}{}:
		default:
		}
	}
}

// ANSI转义码正则表达式
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[PX^_].*?\x1b\\|\x1b\[[\?]?[0-9;]*[hlm]`)

// Shell提示符正则表达式
var shellPromptRegex = regexp.MustCompile(`^[\s\r\n]*([a-zA-Z0-9_-]+@[a-zA-Z0-9._-]+:[~\/][^\$#]*[\$#]\s*)+[\s\r\n]*$`)

// 提示符+命令行（回显）过滤：例如 "root@host:~/path# ls -la"
var shellPromptWithCommandRegex = regexp.MustCompile(`^(?:\([^)]+\)\s*)?[a-zA-Z0-9_-]+@[a-zA-Z0-9._-]+:[^$#%]*[$#%]\s+.+$`)
var simplePromptWithCommandRegex = regexp.MustCompile(`^[$#%>]\s+.+$`)

// isShellPromptOnly 检查内容是否只是shell提示符
func isShellPromptOnly(content string) bool {
	// 去除首尾空白
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}

	// 检查是否匹配常见的shell提示符模式
	// 例如: "root@hostname:~/path# " 或 "user@host:~$ "
	if shellPromptRegex.MatchString(content) {
		return true
	}

	// 检查是否只包含换行和提示符
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 如果有任何非提示符行，返回false
		if !isPromptLine(line) {
			return false
		}
	}
	return true
}

// isPromptLine 检查单行是否是shell提示符
func isPromptLine(line string) bool {
	// 常见的提示符模式
	patterns := []string{
		`^[a-zA-Z0-9_-]+@[a-zA-Z0-9._-]+:[^$#]*[$#]\s*$`, // user@host:path$
		`^[$#>]\s*$`,             // 简单提示符
		`^\([^)]+\)\s*[$#>]\s*$`, // (venv) $
		`^.*[$#>%]\s*$`,          // 任何以提示符结尾的行（仅当行很短时）
	}

	// 如果行太长，不太可能只是提示符
	if len(line) > 100 {
		return false
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			return true
		}
	}
	return false
}

// isShellPromptOrCommandLine 检查单行是否为提示符或提示符回显的命令行
func isShellPromptOrCommandLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if isPromptLine(trimmed) {
		return true
	}
	if shellPromptWithCommandRegex.MatchString(trimmed) {
		return true
	}
	if simplePromptWithCommandRegex.MatchString(trimmed) {
		return true
	}
	return false
}

// isInternalMarkerLine 检查单行是否包含内部命令标记
func isInternalMarkerLine(line string) bool {
	// 直接包含标记
	if strings.Contains(line, "__ACA_CMD_BEGIN_") ||
		strings.Contains(line, "__ACA_CMD_END_") ||
		strings.Contains(line, "ACA_CODE=$?") ||
		strings.Contains(line, "ACA_EOF_") ||
		strings.Contains(line, "ACA_TASK_EXIT_CODE:") ||
		strings.Contains(line, "ACA_TASK_DONE") ||
		strings.Contains(line, "ACA_TASK_PAUSE") {
		return true
	}
	// echo 命令中包含标记
	if strings.Contains(line, "echo '") {
		if strings.Contains(line, "__ACA_CMD_") ||
			strings.Contains(line, "ACA_CODE") ||
			strings.Contains(line, "ACA_EOF_") ||
			strings.Contains(line, "ACA_TASK_") {
			return true
		}
	}
	// bash heredoc 命令
	if strings.Contains(line, "bash <<'ACA_") {
		return true
	}
	// unset ACA_CODE
	if strings.Contains(line, "unset ACA_CODE") {
		return true
	}
	return false
}

// stripANSI 去除ANSI转义码
func stripANSI(s string) string {
	// 移除ANSI转义序列
	result := ansiRegex.ReplaceAllString(s, "")
	// 移除其他控制字符（保留换行和制表符）
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, result)
	return cleaned
}

// flushLogs 定期刷新日志到数据库
func (s *Session) flushLogs() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			// 会话结束时刷新剩余日志
			s.doFlushLogs()
			return
		case <-ticker.C:
			s.doFlushLogs()
		case <-s.logFlushChan:
			s.doFlushLogs()
		}
	}
}

// doFlushLogs 实际执行日志刷新
func (s *Session) doFlushLogs() {
	s.logMutex.Lock()
	if len(s.logBuffer) == 0 {
		s.logMutex.Unlock()
		return
	}

	logs := make([]LogEntry, len(s.logBuffer))
	copy(logs, s.logBuffer)
	s.logBuffer = s.logBuffer[:0]
	s.logMutex.Unlock()

	// 批量写入数据库
	terminalID := s.id
	var taskID *string
	s.metaMutex.RLock()
	if s.taskID != nil {
		taskIDCopy := *s.taskID
		taskID = &taskIDCopy
	}
	s.metaMutex.RUnlock()

	dbLogs := make([]*model.Log, 0, len(logs))
	for _, log := range logs {
		dbLogs = append(dbLogs, &model.Log{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			TaskID:     taskID,
			LogType:    log.LogType,
			Content:    log.Content,
			CreatedAt:  log.CreatedAt,
		})
	}

	if len(dbLogs) > 0 {
		if err := model.DB.CreateInBatches(dbLogs, 100).Error; err != nil {
			utils.Warn("Failed to save terminal logs", zap.Error(err))
		}
	}
}

// FilterInternalMarkers 过滤内部命令标记
// FilterInternalMarkers removes internal command markers from a terminal byte stream.
// It is intended for UI rendering only; internal consumers (e.g. RunCommand) may rely on
// these markers to delimit captured output.
func FilterInternalMarkers(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// 快速检查是否包含标记
	if !bytes.Contains(data, []byte("__ACA_CMD_")) &&
		!bytes.Contains(data, []byte("ACA_CODE")) &&
		!bytes.Contains(data, []byte("ACA_EOF_")) &&
		!bytes.Contains(data, []byte("ACA_TASK_")) {
		return data
	}

	// 使用正则替换方式过滤标记（更可靠，能处理跨行情况）
	result := data

	// 过滤 echo '__ACA_CMD_BEGIN_xxx__' 命令
	result = bytes.ReplaceAll(result, []byte("echo '"), []byte("\x00ECHO_"))
	for {
		beginIdx := bytes.Index(result, []byte("\x00ECHO___ACA_"))
		if beginIdx == -1 {
			break
		}
		endIdx := bytes.Index(result[beginIdx:], []byte("'"))
		if endIdx == -1 {
			break
		}
		// 删除整个 echo 命令
		endIdx = beginIdx + endIdx + 1
		// 检查是否有 && 或 ; 后续
		if endIdx < len(result) && (result[endIdx] == '&' || result[endIdx] == ';' || result[endIdx] == ' ') {
			// 继续查找行尾
			lineEnd := bytes.IndexByte(result[endIdx:], '\n')
			if lineEnd != -1 {
				endIdx = endIdx + lineEnd + 1
			} else {
				endIdx = len(result)
			}
		}
		result = append(result[:beginIdx], result[endIdx:]...)
	}
	// 恢复普通 echo
	result = bytes.ReplaceAll(result, []byte("\x00ECHO_"), []byte("echo '"))

	// 按行过滤
	lines := bytes.Split(result, []byte("\n"))
	filtered := make([][]byte, 0, len(lines))

	for _, line := range lines {
		// 跳过包含内部标记的行
		if bytes.Contains(line, []byte("__ACA_CMD_BEGIN_")) ||
			bytes.Contains(line, []byte("__ACA_CMD_END_")) ||
			bytes.Contains(line, []byte("ACA_CODE=$?")) ||
			bytes.Contains(line, []byte("ACA_CODE=")) ||
			bytes.Contains(line, []byte("unset ACA_CODE")) ||
			bytes.Contains(line, []byte("ACA_EOF_")) ||
			bytes.Contains(line, []byte("ACA_TASK_EXIT_CODE")) ||
			bytes.Contains(line, []byte("ACA_TASK_DONE")) ||
			bytes.Contains(line, []byte("ACA_TASK_PAUSE")) ||
			bytes.Contains(line, []byte("bash <<'ACA_")) {
			continue
		}
		// 跳过以 echo '__ACA 开头的行
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("echo '")) && bytes.Contains(trimmed, []byte("__ACA_")) {
			continue
		}
		filtered = append(filtered, line)
	}

	return bytes.Join(filtered, []byte("\n"))
}
