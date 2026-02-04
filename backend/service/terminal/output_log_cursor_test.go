package terminal

import (
	"strings"
	"testing"
)

func TestSession_consumeOutputLinesLocked_ESC7_ESC8_SaveRestoreCursor(t *testing.T) {
	s := NewSession("t-1", "/bin/bash", 1024)

	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked([]byte("abc\n"))
	s.ioBufMutex.Unlock()
	if len(lines) != 1 || lines[0] != "abc" {
		t.Fatalf("unexpected lines: %#v", lines)
	}

	// Simulate a status line painted far to the right while saving/restoring cursor.
	// Many TUI libs use ESC 7/8 (DECSC/DECRC) rather than CSI s/u.
	payload := "\x1b7\x1b[50GSTATUS\x1b8hello\n"

	s.ioBufMutex.Lock()
	lines = s.consumeOutputLinesLocked([]byte(payload))
	s.ioBufMutex.Unlock()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %#v", lines)
	}
	if !strings.HasPrefix(lines[0], "hello") {
		t.Fatalf("expected line to start with hello, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "STATUS") {
		t.Fatalf("expected line to contain STATUS, got %q", lines[0])
	}
}

func TestSession_consumeOutputLinesLocked_CursorPosition_CUP_H(t *testing.T) {
	s := NewSession("t-1", "/bin/bash", 1024)

	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked([]byte("\x1b[1;10HHi\n"))
	s.ioBufMutex.Unlock()

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %#v", lines)
	}
	if idx := strings.Index(lines[0], "Hi"); idx != 9 {
		t.Fatalf("expected 'Hi' to appear at col 10 (idx 9), got idx=%d line=%q", idx, lines[0])
	}
}

func TestSession_consumeOutputLinesLocked_SuppressesOffscreenWritesWithinSaveRestore(t *testing.T) {
	s := NewSession("t-1", "/bin/bash", 1024)

	// Start a line without flushing.
	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked([]byte("hello"))
	s.ioBufMutex.Unlock()
	if len(lines) != 0 {
		t.Fatalf("expected no flushed lines, got %#v", lines)
	}

	// Simulate a bottom status-bar repaint:
	// - save cursor
	// - move to a different row (CUP row=30) => triggers suppression
	// - clear line + paint status text (should not affect current log line buffer)
	// - restore cursor
	// - continue printing on the original line
	payload := "\x1b7\x1b[30;1H\x1b[2KSTATUS\x1b8 world\n"
	s.ioBufMutex.Lock()
	lines = s.consumeOutputLinesLocked([]byte(payload))
	s.ioBufMutex.Unlock()

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %#v", lines)
	}
	if strings.TrimSpace(lines[0]) != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", lines[0])
	}
}
