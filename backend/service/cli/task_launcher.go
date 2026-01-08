package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/ai-coding-assistant/service/terminal"
)

type TaskLauncher struct {
	terminalManager *terminal.Manager
}

func NewTaskLauncher(tm *terminal.Manager) *TaskLauncher {
	return &TaskLauncher{terminalManager: tm}
}

// StartCLI starts a new terminal and launches Claude/Codex CLI inside {workDir}.
func (l *TaskLauncher) StartCLI(cliType, workDir, serverID string) (string, error) {
	cmd, err := buildStartCLICommand(cliType, workDir)
	if err != nil {
		return "", err
	}
	return l.startInNewTerminal(cmd, cliType, serverID)
}

// ResumeSession starts a new terminal and resumes an existing CLI session inside {workDir}.
func (l *TaskLauncher) ResumeSession(cliType, sessionID, workDir string) (string, error) {
	cmd, err := buildResumeCLICommand(cliType, sessionID, workDir)
	if err != nil {
		return "", err
	}
	return l.startInNewTerminal(cmd, cliType, "")
}

// WaitForReady waits until CLI enters an interactive/working state, or times out.
func (l *TaskLauncher) WaitForReady(terminalID string, timeout time.Duration) error {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return errors.New("missing terminalID")
	}
	if l == nil || l.terminalManager == nil {
		return errors.New("terminal manager not configured")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	session, err := l.getSession(terminalID)
	if err != nil {
		return err
	}

	d := detector.NewDetector()
	if meta := session.Metadata(); meta != nil && meta.AIAssistant != nil && meta.AIAssistant.Detected {
		if isReadyAssistantState(meta.AIAssistant.State) {
			return nil
		}
	}

	tail := stripANSI(string(session.Scrollback()))
	if len(tail) > maxTailBytes {
		tail = tail[len(tail)-maxTailBytes:]
	}
	if isReadyOutput(d, tail) {
		return nil
	}

	subID, ch := session.Subscribe()
	defer session.Unsubscribe(subID)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timeout waiting for CLI ready (%s)", timeout)
		case evt, ok := <-ch:
			if !ok {
				return errors.New("terminal stream closed")
			}
			switch evt.Type {
			case terminal.StreamEventMetadata:
				if evt.Metadata != nil && evt.Metadata.AIAssistant != nil && evt.Metadata.AIAssistant.Detected {
					if isReadyAssistantState(evt.Metadata.AIAssistant.State) {
						return nil
					}
				}
			case terminal.StreamEventData:
				chunk, err := decodeTerminalData(evt.Data)
				if err != nil || chunk == "" {
					continue
				}
				tail += stripANSI(chunk)
				if len(tail) > maxTailBytes {
					tail = tail[len(tail)-maxTailBytes:]
				}
				if isReadyOutput(d, tail) {
					return nil
				}
			case terminal.StreamEventExit:
				msg := strings.TrimSpace(evt.Message)
				if msg == "" {
					msg = "terminal exited"
				}
				return errors.New(msg)
			}
		}
	}
}

// SendTask sends a prompt to the specified terminal (with a trailing enter).
func (l *TaskLauncher) SendTask(terminalID, prompt string) error {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return errors.New("missing terminalID")
	}
	if strings.TrimSpace(prompt) == "" {
		return errors.New("missing prompt")
	}
	if l == nil || l.terminalManager == nil {
		return errors.New("terminal manager not configured")
	}

	session, err := l.getSession(terminalID)
	if err != nil {
		return err
	}

	payload := ensureEnter(prompt)
	return session.Write([]byte(payload))
}

func (l *TaskLauncher) getSession(id string) (*terminal.Session, error) {
	if s := l.terminalManager.GetSession(id); s != nil {
		return s, nil
	}
	if model.DB == nil {
		return nil, errors.New("terminal session not found")
	}
	s, err := l.terminalManager.GetOrResumeSession(id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("terminal session not found")
	}
	return s, nil
}

func (l *TaskLauncher) startInNewTerminal(command, cliType, serverID string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("missing command")
	}
	if l == nil || l.terminalManager == nil {
		return "", errors.New("terminal manager not configured")
	}

	serverID = strings.TrimSpace(serverID)

	var session *terminal.Session
	var err error

	if serverID != "" {
		session, err = l.terminalManager.CreateSSHSession(serverID)
	} else {
		title := fmt.Sprintf("[%s] CLI", normalizeCLIType(cliType))
		session, err = l.terminalManager.CreateSession(title, nil)
	}
	if err != nil {
		return "", err
	}

	if err := session.Write([]byte(ensureEnter(command))); err != nil {
		return session.ID(), err
	}
	return session.ID(), nil
}

func buildStartCLICommand(cliType, workDir string) (string, error) {
	switch normalizeCLIType(cliType) {
	case "claude":
		return buildInWorkDir(workDir, "claude"), nil
	case "codex":
		return buildInWorkDir(workDir, "codex"), nil
	default:
		return "", fmt.Errorf("unsupported cliType: %s", strings.TrimSpace(cliType))
	}
}

func buildResumeCLICommand(cliType, sessionID, workDir string) (string, error) {
	switch normalizeCLIType(cliType) {
	case "claude":
		return buildInWorkDir(workDir, "claude --continue"), nil
	case "codex":
		sessionID = strings.TrimSpace(sessionID)
		if sessionID == "" {
			return "", errors.New("missing sessionID")
		}
		return buildInWorkDir(workDir, fmt.Sprintf("codex --resume %s", sessionID)), nil
	default:
		return "", fmt.Errorf("unsupported cliType: %s", strings.TrimSpace(cliType))
	}
}

func buildInWorkDir(workDir, cmd string) string {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return strings.TrimSpace(cmd)
	}
	return fmt.Sprintf("cd %s && %s", workDir, strings.TrimSpace(cmd))
}

func normalizeCLIType(cliType string) string {
	t := strings.ToLower(strings.TrimSpace(cliType))
	switch t {
	case "", "claude", "claude-code", "claude_code":
		return "claude"
	case "codex", "openai-codex":
		return "codex"
	default:
		return t
	}
}

func isReadyAssistantState(state string) bool {
	switch strings.TrimSpace(state) {
	case string(detector.StateWorking), string(detector.StateWaitingInput), string(detector.StateWaitingApproval):
		return true
	default:
		return false
	}
}

func isReadyOutput(d *detector.Detector, output string) bool {
	if d == nil {
		return false
	}
	state, _ := d.DetectState(output)
	if state == detector.StateWorking || state == detector.StateWaitingInput || state == detector.StateWaitingApproval {
		return true
	}
	return d.DetectAgent(output) != nil
}

func decodeTerminalData(encoded string) (string, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ensureEnter(s string) string {
	if strings.HasSuffix(s, "\r\n") || strings.HasSuffix(s, "\r") {
		return s
	}
	if strings.HasSuffix(s, "\n") {
		return strings.TrimSuffix(s, "\n") + "\r"
	}
	return s + "\r"
}
