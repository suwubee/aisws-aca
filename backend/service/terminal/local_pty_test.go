package terminal

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLocalPTYTerminal_NotStartedErrors(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	term := NewLocalPTYTerminal("t1", exec.Command(bashPath, "-c", "printf hello"))

	if term.Type() != TerminalTypeLocal {
		t.Fatalf("expected TerminalTypeLocal, got %q", term.Type())
	}
	if term.Status() != TerminalStatusNew {
		t.Fatalf("expected status %q, got %q", TerminalStatusNew, term.Status())
	}

	if _, err := term.Read(); err == nil {
		t.Fatalf("expected Read error before Start")
	}
	if err := term.Write([]byte("x")); err == nil {
		t.Fatalf("expected Write error before Start")
	}
	if err := term.Resize(80, 24); err == nil {
		t.Fatalf("expected Resize error before Start")
	}

	if err := term.Close(); err != nil {
		t.Fatalf("expected Close to succeed, got %v", err)
	}
	if err := term.Close(); err != nil {
		t.Fatalf("expected Close to be idempotent, got %v", err)
	}
}

func TestLocalPTYTerminal_StartTwice(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	term := NewLocalPTYTerminal("t2", exec.Command(bashPath, "-c", "printf hello"))
	startOrSkipPTYPermissionDenied(t, term)
	defer func() { _ = term.Close() }()

	if err := term.Start(); err == nil {
		t.Fatalf("expected Start() to fail when called twice")
	}
}

func TestLocalPTYTerminal_ReadOutput(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	term := NewLocalPTYTerminal("t3", exec.Command(bashPath, "-c", "printf hello"))
	startOrSkipPTYPermissionDenied(t, term)
	defer func() { _ = term.Close() }()

	output, err := readUntil(t, term, []string{"hello"}, 2*time.Second)
	if err != nil {
		t.Fatalf("readUntil error: %v (output=%q)", err, output)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("expected output to contain %q, got %q", "hello", output)
	}

	meta := term.Metadata()
	if _, ok := meta["pid"].(int); !ok {
		t.Fatalf("expected metadata pid int, got %#v", meta["pid"])
	}
}

func TestLocalPTYTerminal_WriteAndRead(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	cmd := exec.Command(bashPath, "-c", `stty -echo; printf READY; read -r line; printf "got:%s" "$line"`)
	term := NewLocalPTYTerminal("t4", cmd)
	startOrSkipPTYPermissionDenied(t, term)
	defer func() { _ = term.Close() }()

	if err := term.Resize(100, 40); err != nil {
		t.Fatalf("Resize() error: %v", err)
	}

	_, err = readUntil(t, term, []string{"READY"}, 2*time.Second)
	if err != nil {
		t.Fatalf("expected READY output, got %v", err)
	}

	if err := term.Write([]byte("hi\n")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	output, err := readUntil(t, term, []string{"got:hi"}, 2*time.Second)
	if err != nil {
		t.Fatalf("expected got:hi output, got %v (output=%q)", err, output)
	}
	if !strings.Contains(output, "got:hi") {
		t.Fatalf("expected output to contain %q, got %q", "got:hi", output)
	}
}

func readUntil(t *testing.T, term Terminal, wants []string, timeout time.Duration) (string, error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var out strings.Builder

	for time.Now().Before(deadline) {
		data, err := readOnceWithTimeout(term, 500*time.Millisecond)
		if len(data) > 0 {
			out.Write(data)
			s := out.String()
			ok := true
			for _, w := range wants {
				if !strings.Contains(s, w) {
					ok = false
					break
				}
			}
			if ok {
				return s, nil
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return out.String(), io.EOF
			}
			return out.String(), err
		}
	}

	return out.String(), errors.New("timeout")
}

func readOnceWithTimeout(term Terminal, timeout time.Duration) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		data, err := term.Read()
		ch <- result{data: data, err: err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-time.After(timeout):
		return nil, errors.New("read timeout")
	}
}

func startOrSkipPTYPermissionDenied(t *testing.T, term *LocalPTYTerminal) {
	t.Helper()

	if err := term.Start(); err != nil {
		if os.IsPermission(err) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
			t.Skipf("pty permission denied: %v", err)
		}
		t.Fatalf("Start() error: %v", err)
	}
}
