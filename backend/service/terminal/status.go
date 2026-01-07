package terminal

type TerminalStatus string

const (
	TerminalStatusNew     TerminalStatus = "new"
	TerminalStatusRunning TerminalStatus = "running"
	TerminalStatusExited  TerminalStatus = "exited"
	TerminalStatusClosed  TerminalStatus = "closed"
)
