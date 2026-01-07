package ssh

import (
	"errors"
	"io"
	"os"
	"sync"

	cryptossh "golang.org/x/crypto/ssh"
)

// SSHTerminalSession 适配 SSH Session，使其能够复用终端会话的输入/输出接口。
// 通过 stdin/stdout 管道与远端交互，并支持窗口大小调整与关闭。
type SSHTerminalSession struct {
	Session *cryptossh.Session

	stdinReader  *os.File
	stdinWriter  *os.File
	stdoutReader *os.File
	stdoutWriter *os.File

	closeOnce sync.Once
	closeErr  error
}

func NewSSHTerminalSession(session *cryptossh.Session, stdinReader, stdinWriter, stdoutReader, stdoutWriter *os.File) *SSHTerminalSession {
	return &SSHTerminalSession{
		Session:       session,
		stdinReader:  stdinReader,
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		stdoutWriter: stdoutWriter,
	}
}

func (s *SSHTerminalSession) StdoutPipe() *os.File {
	return s.stdoutReader
}

func (s *SSHTerminalSession) Write(data []byte) error {
	if s.stdinWriter == nil {
		return errors.New("stdin pipe is not initialized")
	}
	n, err := s.stdinWriter.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func (s *SSHTerminalSession) Resize(cols, rows uint16) error {
	if s.Session == nil {
		return errors.New("ssh session is nil")
	}
	return s.Session.WindowChange(int(rows), int(cols))
}

func (s *SSHTerminalSession) Close() error {
	s.closeOnce.Do(func() {
		var errs []error

		if s.stdinWriter != nil {
			if err := s.stdinWriter.Close(); err != nil && !isIgnorableCloseErr(err) {
				errs = append(errs, err)
			}
		}
		if s.stdinReader != nil {
			if err := s.stdinReader.Close(); err != nil && !isIgnorableCloseErr(err) {
				errs = append(errs, err)
			}
		}
		if s.stdoutWriter != nil {
			if err := s.stdoutWriter.Close(); err != nil && !isIgnorableCloseErr(err) {
				errs = append(errs, err)
			}
		}
		if s.stdoutReader != nil {
			if err := s.stdoutReader.Close(); err != nil && !isIgnorableCloseErr(err) {
				errs = append(errs, err)
			}
		}
		if s.Session != nil {
			if err := s.Session.Close(); err != nil && !isIgnorableCloseErr(err) {
				errs = append(errs, err)
			}
		}

		s.closeErr = errors.Join(errs...)
	})

	return s.closeErr
}

func isIgnorableCloseErr(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed)
}
