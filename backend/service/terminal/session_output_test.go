package terminal

import (
	"testing"
)

func TestConsumeOutputLinesLocked_CarriageReturnOverwriteAndClear(t *testing.T) {
	s := NewSession("t1", "/bin/bash", 1024)

	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked([]byte("hello\rhi\x1b[K\n"))
	s.ioBufMutex.Unlock()

	if len(lines) != 1 || lines[0] != "hi" {
		t.Fatalf("expected [hi], got %#v", lines)
	}
}

func TestConsumeOutputLinesLocked_UTF8AcrossChunks(t *testing.T) {
	s := NewSession("t2", "/bin/bash", 1024)

	// "中文\n" 的 UTF-8 字节序列在 chunk 边界处切开
	part1 := []byte{0xe4, 0xb8, 0xad, 0xe6} // "中" + "文" 的首字节
	part2 := []byte{0x96, 0x87, '\n'}       // "文" 剩余 + newline

	s.ioBufMutex.Lock()
	lines1 := s.consumeOutputLinesLocked(part1)
	lines2 := s.consumeOutputLinesLocked(part2)
	s.ioBufMutex.Unlock()

	lines := append(lines1, lines2...)
	if len(lines) != 1 || lines[0] != "中文" {
		t.Fatalf("expected [中文], got %#v", lines)
	}
}

func TestConsumeOutputLinesLocked_CarriageReturnKeepsMeaningfulLineBeforeSpinner(t *testing.T) {
	s := NewSession("t3", "/bin/bash", 1024)

	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked([]byte("我是 Claude Code\r✶\n"))
	s.ioBufMutex.Unlock()

	if len(lines) != 1 || lines[0] != "我是 Claude Code" {
		t.Fatalf("expected [我是 Claude Code], got %#v", lines)
	}
}

func TestConsumeOutputLinesLocked_CSIEraseLinePreservesLineBeforeClear(t *testing.T) {
	s := NewSession("t4", "/bin/bash", 1024)

	s.ioBufMutex.Lock()
	lines := s.consumeOutputLinesLocked([]byte("Claude 正在回答\x1b[2K\n"))
	s.ioBufMutex.Unlock()

	if len(lines) != 1 || lines[0] != "Claude 正在回答" {
		t.Fatalf("expected [Claude 正在回答], got %#v", lines)
	}
}
