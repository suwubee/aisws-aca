package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

const localPTYReadBufferSize = 4096

var errTerminalNotStarted = errors.New("terminal not started")

// LocalPTYTerminal 本地PTY终端实现。
type LocalPTYTerminal struct {
	id  string
	cmd *exec.Cmd

	mu     sync.RWMutex
	pty    *os.File
	status TerminalStatus

	startedAt *time.Time
	closedAt  *time.Time
	exitCode  *int
	cols      uint16
	rows      uint16

	ptyCloseOnce sync.Once
	ptyCloseErr  error
}

var _ Terminal = (*LocalPTYTerminal)(nil)

// NewLocalPTYTerminal 创建本地PTY终端。
// 传入的 cmd 会在 Start() 中启动；若 id 为空则自动生成。
func NewLocalPTYTerminal(id string, cmd *exec.Cmd) *LocalPTYTerminal {
	if id == "" {
		id = uuid.New().String()
	}
	return &LocalPTYTerminal{
		id:     id,
		cmd:    cmd,
		status: TerminalStatusNew,
	}
}

func (t *LocalPTYTerminal) ID() string {
	return t.id
}

func (t *LocalPTYTerminal) Type() TerminalType {
	return TerminalTypeLocal
}

func (t *LocalPTYTerminal) Start() error {
	t.mu.Lock()
	if t.status == TerminalStatusRunning {
		t.mu.Unlock()
		return fmt.Errorf("terminal already started")
	}
	if t.status == TerminalStatusClosed || t.status == TerminalStatusExited {
		t.mu.Unlock()
		return fmt.Errorf("terminal already closed")
	}
	cmd := t.cmd
	t.mu.Unlock()

	if cmd == nil {
		return fmt.Errorf("terminal command is nil")
	}

	ensureLocalPTYCmdDefaults(cmd)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	now := time.Now()

	t.mu.Lock()
	t.pty = ptmx
	t.status = TerminalStatusRunning
	t.startedAt = &now
	t.mu.Unlock()

	go t.wait()

	return nil
}

func (t *LocalPTYTerminal) Close() error {
	t.mu.Lock()
	if t.status == TerminalStatusClosed || t.status == TerminalStatusExited {
		t.mu.Unlock()
		return nil
	}
	t.status = TerminalStatusClosed
	now := time.Now()
	t.closedAt = &now
	cmd := t.cmd
	t.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_ = cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Process.Kill()
	}

	return t.closePTY()
}

func (t *LocalPTYTerminal) Read() ([]byte, error) {
	t.mu.RLock()
	ptmx := t.pty
	status := t.status
	t.mu.RUnlock()

	if ptmx == nil {
		if status == TerminalStatusClosed || status == TerminalStatusExited {
			return nil, io.EOF
		}
		return nil, errTerminalNotStarted
	}

	buf := make([]byte, localPTYReadBufferSize)
	n, err := ptmx.Read(buf)
	if n > 0 {
		return buf[:n], nil
	}
	if err != nil {
		if errors.Is(err, io.EOF) || isErrEIO(err) {
			t.markExitedIfRunning()
			return nil, io.EOF
		}
		return nil, err
	}
	return nil, nil
}

func (t *LocalPTYTerminal) Write(data []byte) error {
	t.mu.RLock()
	ptmx := t.pty
	status := t.status
	t.mu.RUnlock()

	if ptmx == nil {
		if status == TerminalStatusClosed || status == TerminalStatusExited {
			return io.EOF
		}
		return errTerminalNotStarted
	}

	n, err := ptmx.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func (t *LocalPTYTerminal) Resize(cols, rows uint16) error {
	t.mu.RLock()
	ptmx := t.pty
	status := t.status
	t.mu.RUnlock()

	if ptmx == nil {
		if status == TerminalStatusClosed || status == TerminalStatusExited {
			return io.EOF
		}
		return errTerminalNotStarted
	}

	if err := pty.Setsize(ptmx, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		return err
	}

	t.mu.Lock()
	t.cols = cols
	t.rows = rows
	t.mu.Unlock()

	return nil
}

func (t *LocalPTYTerminal) Status() TerminalStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *LocalPTYTerminal) Metadata() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	metadata := map[string]interface{}{
		"status": t.status,
		"cols":   t.cols,
		"rows":   t.rows,
	}

	if t.cmd != nil {
		metadata["args"] = append([]string(nil), t.cmd.Args...)
		if t.cmd.Process != nil {
			metadata["pid"] = t.cmd.Process.Pid
		}
	}
	if t.startedAt != nil {
		metadata["started_at"] = *t.startedAt
	}
	if t.closedAt != nil {
		metadata["closed_at"] = *t.closedAt
	}
	if t.exitCode != nil {
		metadata["exit_code"] = *t.exitCode
	}

	return metadata
}

func (t *LocalPTYTerminal) wait() {
	cmd := t.cmd
	if cmd == nil {
		return
	}

	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	t.mu.Lock()
	if t.status == TerminalStatusRunning {
		t.status = TerminalStatusExited
		now := time.Now()
		t.closedAt = &now
	}
	t.exitCode = &exitCode
	t.mu.Unlock()

	_ = t.closePTY()
}

func (t *LocalPTYTerminal) closePTY() error {
	t.ptyCloseOnce.Do(func() {
		t.mu.Lock()
		ptmx := t.pty
		t.pty = nil
		t.mu.Unlock()

		if ptmx != nil {
			t.ptyCloseErr = ptmx.Close()
		}
	})

	return t.ptyCloseErr
}

func (t *LocalPTYTerminal) markExitedIfRunning() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status == TerminalStatusRunning {
		t.status = TerminalStatusExited
	}
}

func ensureLocalPTYCmdDefaults(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	if cmd.Env == nil {
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		return
	}
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
}

func isErrEIO(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		err = pathErr.Err
	}
	return errors.Is(err, syscall.EIO)
}
