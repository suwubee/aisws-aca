package workflow

import "github.com/ai-coding-assistant/service/terminal"

type terminalManagerAdapter struct {
	manager *terminal.Manager
}

// NewTerminalManagerAdapter adapts terminal.Manager to workflow's internal terminalManager interface.
// Returns nil when manager is nil.
func NewTerminalManagerAdapter(manager *terminal.Manager) terminalManager {
	if manager == nil {
		return nil
	}
	return terminalManagerAdapter{manager: manager}
}

func (a terminalManagerAdapter) CreateSession(title string, taskID *string) (terminalSession, error) {
	return a.manager.CreateSession(title, taskID)
}

func (a terminalManagerAdapter) CreateSSHSession(serverID string) (terminalSession, error) {
	return a.manager.CreateSSHSession(serverID)
}

func (a terminalManagerAdapter) RenameSession(id, title string) error {
	return a.manager.RenameSession(id, title)
}

func (a terminalManagerAdapter) LinkTask(id string, taskID *string) error {
	return a.manager.LinkTask(id, taskID)
}

func (a terminalManagerAdapter) GetOrResumeSession(id string) (terminalSession, error) {
	session, err := a.manager.GetOrResumeSession(id)
	if err != nil || session == nil {
		return nil, err
	}
	return session, nil
}
