package terminal

import (
	"context"
	"encoding/base64"
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
	"unsafe"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/approval"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/ai-coding-assistant/utils"
	"github.com/creack/pty"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StreamEventType 流事件类型
type StreamEventType string

const (
	StreamEventData     StreamEventType = "data"
	StreamEventMetadata StreamEventType = "metadata"
	StreamEventExit     StreamEventType = "exit"
	StreamEventApproval StreamEventType = "approval" // 新增：审批事件
	StreamEventMessage  StreamEventType = "message"  // 新增：消息事件
)

// StreamEvent 流事件
type StreamEvent struct {
	Type           StreamEventType  `json:"type"`
	Data           string           `json:"data,omitempty"`
	Metadata       *SessionMetadata `json:"metadata,omitempty"`
	ExitCode       int              `json:"exit_code,omitempty"`
	Message        string           `json:"message,omitempty"`
	ApprovalResult *ApprovalEvent   `json:"approval_result,omitempty"` // 审批结果
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
}

// Session 终端会话
type Session struct {
	id          string
	title       string
	taskID      *string
	shell       string
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
	// 日志相关
	logBuffer    []LogEntry
	logMutex     sync.Mutex
	logFlushChan chan struct{}
	// 输入/输出日志聚合（避免按字符记录）
	ioBufMutex        sync.Mutex
	inputLineBuf      []rune
	inputEscState     int    // 0=none, 1=ESC, 2=CSI(ESC[...), 3=SS3(ESCO...)
	outputLineBuf     []rune // 当前输出行（处理 \r/\b/ANSI 清行等）
	outputCursor      int
	outputRemainder   []byte // 残留的半个 UTF-8/ANSI 序列
	outputSavedCursor int
	outputLineBufSize int
	// 检测和审批相关
	detector               *detector.Detector
	approvalEngine         *approval.Engine
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
		metadata: &SessionMetadata{
			Title:  "Terminal",
			Status: "running",
		},
	}
}

// Start 启动会话
func (s *Session) Start() error {
	return s.StartWithTmux(false)
}

// StartWithTmux 使用 tmux 启动会话
func (s *Session) StartWithTmux(attach bool) error {
	var cmd *exec.Cmd

	if attach {
		// 重新连接到已有的 tmux 会话
		cmd = exec.Command("tmux", "attach-session", "-t", s.id)
	} else {
		// 创建新的 tmux 会话
		// 先检查会话是否已存在
		checkCmd := exec.Command("tmux", "has-session", "-t", s.id)
		if checkCmd.Run() == nil {
			// 会话已存在，直接 attach
			cmd = exec.Command("tmux", "attach-session", "-t", s.id)
		} else {
			// 创建新会话
			cmd = exec.Command("tmux", "new-session", "-d", "-s", s.id, "-x", "120", "-y", "30")
			cmd.Env = append(os.Environ(), "TERM=xterm-256color")
			if err := cmd.Run(); err != nil {
				utils.Warn("Failed to create tmux session, falling back to direct shell", zap.Error(err))
				return s.startDirectShell()
			}
			// 然后 attach 到新创建的会话
			cmd = exec.Command("tmux", "attach-session", "-t", s.id)
		}
	}

	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
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
	cmd := exec.Command(s.shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
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

// loadAutomationConfig 加载自动化配置
func (s *Session) loadAutomationConfig() {
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
		Data: base64.StdEncoding.EncodeToString(s.dataBatchBuf),
	})

	s.dataBatchBuf = s.dataBatchBuf[:0]
}

// detectAndHandle 检测AI状态并处理审批
func (s *Session) detectAndHandle(data []byte) {
	output := string(data)

	// 检测AI代理
	if s.aiAssistant == nil || !s.aiAssistant.Detected {
		if agent := s.detector.DetectAgent(output); agent != nil {
			s.metaMutex.Lock()
			s.aiAssistant = &AIAssistant{
				Type:           string(agent.Type),
				DisplayName:    agent.DisplayName,
				State:          string(agent.State),
				StateUpdatedAt: agent.StateUpdatedAt,
				Detected:       agent.Detected,
				Version:        agent.Version,
			}
			s.metadata.AIAssistant = s.aiAssistant
			s.metaMutex.Unlock()

			utils.Info("AI agent detected",
				zap.String("type", string(agent.Type)),
				zap.String("terminal", s.id))

			s.broadcastMetadata()
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
		s.aiAssistant.StateUpdatedAt = time.Now()
		if approvalPrompt != "" {
			s.aiAssistant.ApprovalPrompt = approvalPrompt
		}
		s.metadata.AIAssistant = s.aiAssistant

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

		if err := s.Write([]byte(result.Input)); err != nil {
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
	if s.aiAssistant != nil {
		// 这里可以关联AI会话ID
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
		ApprovalResult: approvalEvent,
	})
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

		close(s.done)
	}
}

// Subscribe 订阅会话事件
func (s *Session) Subscribe() (string, chan StreamEvent) {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	id := uuid.New().String()
	ch := make(chan StreamEvent, 256)
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
	if s.pty == nil {
		utils.Warn("Write failed: pty is nil", zap.String("terminal", s.id))
		return nil
	}
	// 记录输入日志
	s.addInputLog(data)
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
	if s.pty == nil {
		return nil
	}
	ws := &struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{Row: rows, Col: cols}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		s.pty.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Close 关闭会话
func (s *Session) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		s.cmd.Process.Kill()
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

// Setters
func (s *Session) SetTitle(title string) {
	s.metaMutex.Lock()
	s.title = title
	s.metadata.Title = title
	s.metaMutex.Unlock()
}

func (s *Session) SetTaskID(taskID *string) {
	s.metaMutex.Lock()
	s.taskID = taskID
	s.metadata.TaskID = taskID
	s.metaMutex.Unlock()
}

// ToDBModel 转换为数据库模型
func (s *Session) ToDBModel() *model.TerminalSession {
	return &model.TerminalSession{
		ID:        s.id,
		Title:     s.title,
		TaskID:    s.taskID,
		Shell:     s.shell,
		Status:    s.status,
		PID:       s.metadata.PID,
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
	s.addLog("input", line+"\n")
}

// addOutputLog 将PTY输出按行聚合，过滤提示符/回显后写入日志缓冲
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
			}
			i += seqLen
			continue
		}

		switch b {
		case '\n':
			s.flushOutputLineLocked(&lines)
			i++
			continue
		case '\r':
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
			// 退格：仅移动光标（是否擦除由后续输出决定）
			if s.outputCursor > 0 {
				s.outputCursor--
			}
			i++
			continue
		case '\t':
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
	case 's': // Save cursor
		s.outputSavedCursor = s.outputCursor
	case 'u': // Restore cursor
		s.outputCursor = s.outputSavedCursor
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

	s.logMutex.Lock()
	s.logBuffer = append(s.logBuffer, LogEntry{
		LogType:   logType,
		Content:   cleanContent,
		CreatedAt: time.Now(),
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
