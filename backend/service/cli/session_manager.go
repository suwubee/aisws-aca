package cli

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CLIState string

const (
	CLIStateStarting     CLIState = "starting"
	CLIStateReady        CLIState = "ready"
	CLIStateWorking      CLIState = "working"
	CLIStateWaitingInput CLIState = "waiting_input"
)

type SessionSnapshot struct {
	AISessionID string
	TerminalID  string
	TaskID      string
	CLIType     string
	State       CLIState
	SessionID   string
	SessionFile string
}

type SessionManager struct {
	mu sync.Mutex

	db         *gorm.DB
	detector   *detector.Detector
	aiSession  *model.AISession
	started    bool
	outputTail string
}

const maxTailBytes = 8192

var (
	uuidRegex = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(?:\x07|\x1b\\)`)
	codexFile = regexp.MustCompile(`(?i)(rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl)`)
)

func NewSessionManager(terminalID string, taskID *string, cliType string) (*SessionManager, error) {
	terminalID = strings.TrimSpace(terminalID)
	cliType = strings.TrimSpace(cliType)
	if terminalID == "" {
		return nil, errors.New("missing terminalID")
	}
	if taskID == nil || *taskID == "" {
		return nil, errors.New("missing taskID: AI session must be bound to a task")
	}
	if cliType == "" {
		cliType = string(detector.AIAgentUnknown)
	}
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}

	taskIDStr := *taskID
	now := time.Now()
	aiSession := &model.AISession{
		ID:         uuid.New().String(),
		TerminalID: terminalID,
		TaskID:     taskIDStr,
		AIType:     cliType,
		State:      string(CLIStateStarting),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := model.DB.Create(aiSession).Error; err != nil {
		return nil, err
	}

	return &SessionManager{
		db:       model.DB,
		detector: detector.NewDetector(),
		aiSession: &model.AISession{
			ID:          aiSession.ID,
			TerminalID:  terminalID,
			TaskID:      taskIDStr,
			AIType:      cliType,
			State:       aiSession.State,
			SessionID:   "",
			SessionFile: "",
		},
		started: false,
	}, nil
}

// UpdateFromOutput parses new terminal output chunk, updates state/session fields, and persists changes.
func (m *SessionManager) UpdateFromOutput(output string) (SessionSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	output = stripANSI(output)
	if output == "" {
		return m.snapshotLocked(), nil
	}

	m.outputTail += output
	if len(m.outputTail) > maxTailBytes {
		m.outputTail = m.outputTail[len(m.outputTail)-maxTailBytes:]
	}

	updates := map[string]interface{}{}

	if m.aiSession.AIType == string(detector.AIAgentUnknown) || m.aiSession.AIType == "" {
		if t := detectCLIType(m.detector, output); t != "" {
			m.aiSession.AIType = t
			updates["ai_type"] = t
		}
	}

	maybeUpdateSessionIDs(m.aiSession, m.outputTail, &updates)

	state, _ := m.detector.DetectState(output)
	if !m.started && startupComplete(m.aiSession, state) {
		m.started = true
	}

	oldState := CLIState(m.aiSession.State)
	newState := m.computeStateLocked(state)
	if oldState != newState {
		m.aiSession.State = string(newState)
		updates["state"] = string(newState)
	}

	if len(updates) == 0 {
		return m.snapshotLocked(), nil
	}

	updates["updated_at"] = time.Now()
	if err := m.db.Model(&model.AISession{}).Where("id = ?", m.aiSession.ID).Updates(updates).Error; err != nil {
		return SessionSnapshot{}, err
	}

	return m.snapshotLocked(), nil
}

func (m *SessionManager) snapshotLocked() SessionSnapshot {
	return SessionSnapshot{
		AISessionID: m.aiSession.ID,
		TerminalID:  m.aiSession.TerminalID,
		TaskID:      m.aiSession.TaskID,
		CLIType:     m.aiSession.AIType,
		State:       CLIState(m.aiSession.State),
		SessionID:   m.aiSession.SessionID,
		SessionFile: m.aiSession.SessionFile,
	}
}

func (m *SessionManager) computeStateLocked(detected detector.AIAgentState) CLIState {
	if !m.started {
		return CLIStateStarting
	}

	// prompt/approval => waiting_input
	if detected == detector.StateWaitingInput || detected == detector.StateWaitingApproval {
		return CLIStateWaitingInput
	}

	// once we enter working, stay working until prompt shows up again
	if CLIState(m.aiSession.State) == CLIStateWorking {
		return CLIStateWorking
	}

	if detected == detector.StateWorking {
		return CLIStateWorking
	}

	if CLIState(m.aiSession.State) == CLIStateWaitingInput {
		return CLIStateWorking
	}

	if CLIState(m.aiSession.State) == CLIStateStarting {
		return CLIStateReady
	}

	return CLIState(m.aiSession.State)
}

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func detectCLIType(d *detector.Detector, output string) string {
	if d == nil || output == "" {
		return ""
	}
	agent := d.DetectAgent(output)
	if agent == nil {
		return ""
	}
	switch agent.Type {
	case detector.AIAgentClaudeCode, detector.AIAgentCodex:
		return string(agent.Type)
	default:
		return ""
	}
}

func maybeUpdateSessionIDs(aiSession *model.AISession, output string, updates *map[string]interface{}) {
	if aiSession == nil || output == "" || updates == nil {
		return
	}

	switch aiSession.AIType {
	case string(detector.AIAgentClaudeCode):
		if aiSession.SessionID == "" {
			if id := uuidRegex.FindString(output); id != "" {
				aiSession.SessionID = id
				(*updates)["session_id"] = id
			}
		}
	case string(detector.AIAgentCodex):
		if aiSession.SessionFile == "" || aiSession.SessionID == "" {
			if match := codexFile.FindStringSubmatch(output); len(match) == 3 {
				if aiSession.SessionFile == "" {
					aiSession.SessionFile = match[1]
					(*updates)["session_file"] = match[1]
				}
				if aiSession.SessionID == "" {
					aiSession.SessionID = match[2]
					(*updates)["session_id"] = match[2]
				}
			}
		}
	default:
		// unknown type: prefer codex filename signal (more specific), otherwise UUID if "claude" hinted.
		if aiSession.SessionFile == "" || aiSession.SessionID == "" {
			if match := codexFile.FindStringSubmatch(output); len(match) == 3 {
				aiSession.AIType = string(detector.AIAgentCodex)
				(*updates)["ai_type"] = aiSession.AIType
				if aiSession.SessionFile == "" {
					aiSession.SessionFile = match[1]
					(*updates)["session_file"] = match[1]
				}
				if aiSession.SessionID == "" {
					aiSession.SessionID = match[2]
					(*updates)["session_id"] = match[2]
				}
				return
			}
		}
		if aiSession.SessionID == "" && strings.Contains(strings.ToLower(output), "claude") {
			if id := uuidRegex.FindString(output); id != "" {
				aiSession.AIType = string(detector.AIAgentClaudeCode)
				(*updates)["ai_type"] = aiSession.AIType
				aiSession.SessionID = id
				(*updates)["session_id"] = id
			}
		}
	}
}

func startupComplete(aiSession *model.AISession, state detector.AIAgentState) bool {
	if state == detector.StateWorking || state == detector.StateWaitingInput || state == detector.StateWaitingApproval {
		return true
	}
	if aiSession == nil {
		return false
	}
	switch aiSession.AIType {
	case string(detector.AIAgentClaudeCode):
		return aiSession.SessionID != ""
	case string(detector.AIAgentCodex):
		return aiSession.SessionFile != ""
	default:
		return aiSession.SessionID != "" || aiSession.SessionFile != ""
	}
}
