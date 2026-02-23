package terminal

import "testing"

func TestSessionAddOutputLog_FiltersPromptEchoedInput(t *testing.T) {
	s := NewSession("terminal-echo-test-1", "bash", 1024)

	s.addInputLog([]byte("codex\r"))
	s.addOutputLog([]byte("root@host:~# codex\nOpenAI Codex ready\n"))

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	countByType := map[string]int{}
	for _, entry := range s.logBuffer {
		countByType[entry.LogType]++
	}

	if countByType["input"] != 1 || countByType["input_raw"] != 1 {
		t.Fatalf("expected one input/input_raw log, got input=%d input_raw=%d", countByType["input"], countByType["input_raw"])
	}
	if countByType["output"] != 1 || countByType["output_raw"] != 1 {
		t.Fatalf("expected echoed command to be filtered from output logs, got output=%d output_raw=%d", countByType["output"], countByType["output_raw"])
	}
	if got := s.logBuffer[len(s.logBuffer)-1].Content; got != "OpenAI Codex ready\n" {
		t.Fatalf("expected final output line to be kept, got %q", got)
	}
}

func TestSessionAddOutputLog_FiltersPlainEchoedInput(t *testing.T) {
	s := NewSession("terminal-echo-test-2", "bash", 1024)

	s.addInputLog([]byte("ls -la\r"))
	s.addOutputLog([]byte("ls -la\nfile1\n"))

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	countByType := map[string]int{}
	for _, entry := range s.logBuffer {
		countByType[entry.LogType]++
	}

	if countByType["output"] != 1 || countByType["output_raw"] != 1 {
		t.Fatalf("expected plain echoed input to be filtered, got output=%d output_raw=%d", countByType["output"], countByType["output_raw"])
	}
	if got := s.logBuffer[len(s.logBuffer)-1].Content; got != "file1\n" {
		t.Fatalf("expected only business output to remain, got %q", got)
	}
}

func TestSessionAddOutputLog_FiltersEphemeralSpinnerOnlyLines(t *testing.T) {
	s := NewSession("terminal-echo-test-3", "bash", 1024)

	s.addOutputLog([]byte("✶\n●\n真实输出\n"))

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	countByType := map[string]int{}
	for _, entry := range s.logBuffer {
		countByType[entry.LogType]++
	}

	if countByType["output"] != 1 || countByType["output_raw"] != 1 {
		t.Fatalf("expected spinner-only lines to be filtered, got output=%d output_raw=%d", countByType["output"], countByType["output_raw"])
	}
	if got := s.logBuffer[len(s.logBuffer)-1].Content; got != "真实输出\n" {
		t.Fatalf("expected only real output line to remain, got %q", got)
	}
}

func TestSessionAddOutputLog_FiltersClaudeThinkingFragments(t *testing.T) {
	s := NewSession("terminal-echo-test-4", "bash", 1024)

	s.addInputLog([]byte("claude\r"))
	s.addInputLog([]byte("hello\r"))
	s.addOutputLog([]byte("· Transfiguring…\n✻ Tr nsf\nTr\na\n❯ hello\n● Hey! What can I help you with today?\n"))

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	var outputs []string
	var rawOutputs []string
	for _, entry := range s.logBuffer {
		if entry.LogType == "output" {
			outputs = append(outputs, entry.Content)
		}
		if entry.LogType == "output_raw" {
			rawOutputs = append(rawOutputs, entry.Content)
		}
	}

	if len(outputs) != 1 {
		t.Fatalf("expected exactly one clean output line, got %d (%#v)", len(outputs), outputs)
	}
	if len(rawOutputs) != 1 {
		t.Fatalf("expected exactly one clean raw output line, got %d (%#v)", len(rawOutputs), rawOutputs)
	}
	if outputs[0] != "● Hey! What can I help you with today?\n" {
		t.Fatalf("unexpected clean output: %q", outputs[0])
	}
}

func TestSessionAddOutputLog_DoesNotOverFilterNonAIOutput(t *testing.T) {
	s := NewSession("terminal-echo-test-5", "bash", 1024)

	s.addOutputLog([]byte("ok\nhello world\n"))

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	var outputs []string
	for _, entry := range s.logBuffer {
		if entry.LogType == "output" {
			outputs = append(outputs, entry.Content)
		}
	}
	if len(outputs) != 2 {
		t.Fatalf("expected two normal output lines, got %d (%#v)", len(outputs), outputs)
	}
}
