package terminal

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
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
	// 检测和审批相关
	detector       *detector.Detector
	approvalEngine *approval.Engine
	lastOutput     string      // 用于检测状态变化
	lastOutputMu   sync.Mutex
}

// LogEntry 日志条目
type LogEntry struct {
	LogType   string
	Content   string
	CreatedAt time.Time
}

// ScrollbackBuffer 滚动缓冲区
type ScrollbackBuffer struct {
	data     []byte
	maxSize  int
	mutex    sync.RWMutex
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
		id:             id,
		shell:          shell,
		status:         "running",
		subscribers:    make(map[string]chan StreamEvent),
		scrollback:     NewScrollbackBuffer(scrollbackSize),
		createdAt:      time.Now(),
		done:           make(chan struct{}),
		logBuffer:      make([]LogEntry, 0, 100),
		logFlushChan:   make(chan struct{}, 1),
		detector:       detector.NewDetector(),
		approvalEngine: approval.NewEngine(),
		metadata: &SessionMetadata{
			Title:  "Terminal",
			Status: "running",
		},
	}
}

// Start 启动会话
func (s *Session) Start() error {
	cmd := exec.Command(s.shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	s.pty = ptmx
	s.cmd = cmd
	s.metadata.PID = cmd.Process.Pid

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
			return
		default:
			n, err := s.pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					utils.Debug("PTY read error", zap.Error(err))
				}
				return
			}

			if n > 0 {
				data := buf[:n]
				s.scrollback.Write(data)

				// 记录输出日志
				s.addLog("output", string(data))

				// 广播数据事件
				s.broadcast(StreamEvent{
					Type: StreamEventData,
					Data: base64.StdEncoding.EncodeToString(data),
				})

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

			// 如果进入等待审批状态，触发审批流程
			if state == detector.StateWaitingApproval {
				go s.handleApproval(output)
			}
			return
		}
	}
	s.metaMutex.Unlock()
}

// handleApproval 处理审批流程
func (s *Session) handleApproval(output string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取更多上下文
	scrollbackData := s.scrollback.Read()
	context := detector.GetRecentContext(scrollbackData, 50)
	if context == "" {
		context = output
	}

	// 评估审批
	result, err := s.approvalEngine.Evaluate(ctx, s.id, context)
	if err != nil {
		utils.Error("Approval evaluation failed", zap.Error(err))
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

	// 如果是自动通过，执行输入
	if result.Action == approval.ActionApprove && result.Input != "" {
		if err := s.Write([]byte(result.Input)); err != nil {
			utils.Error("Failed to write approval input", zap.Error(err))
		} else {
			approvalEvent.AutoHandled = true
			utils.Info("Auto-approved",
				zap.String("terminal", s.id),
				zap.String("input", result.Input),
				zap.String("reasoning", result.Reasoning))
		}
	}

	// 记录审批操作
	var aiSessionID *string
	s.metaMutex.RLock()
	if s.aiAssistant != nil {
		// 这里可以关联AI会话ID
	}
	s.metaMutex.RUnlock()

	promptType := "unknown"
	if s.detector.IsApprovalPrompt(output) {
		promptType = "approval"
	}

	s.approvalEngine.RecordApproval(
		s.id,
		aiSessionID,
		promptType,
		output,
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

// broadcastMetadata 广播元数据
func (s *Session) broadcastMetadata() {
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
		return nil
	}
	// 记录输入日志
	s.addLog("input", string(data))
	_, err := s.pty.Write(data)
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
func (s *Session) GetAutomationConfig() (*model.TerminalAutomation, error) {
	return s.approvalEngine.GetAutomationConfig(s.id)
}

// SetAutomationConfig 设置自动化配置
func (s *Session) SetAutomationConfig(config *model.TerminalAutomation) error {
	config.TerminalID = s.id
	err := s.approvalEngine.SaveAutomationConfig(config)
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

// addLog 添加日志到缓冲区
func (s *Session) addLog(logType, content string) {
	s.logMutex.Lock()
	s.logBuffer = append(s.logBuffer, LogEntry{
		LogType:   logType,
		Content:   content,
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
