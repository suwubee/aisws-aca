package terminal

import "testing"

func TestSessionAddLog_DedupeRulesForRawAndAggregatedLogs(t *testing.T) {
	s := NewSession("terminal-log-test", "bash", 1024)

	s.addLog("output", "same output line\n")
	s.addLog("output", "same output line\n")

	s.addLog("system", "same system line\n")
	s.addLog("system", "same system line\n")

	s.addLog("input", "run command\n")
	s.addLog("input", "run command\n")

	s.addLog("input_raw", "run command\n")
	s.addLog("input_raw", "run command\n")

	s.addLog("output_raw", "raw output\n")
	s.addLog("output_raw", "raw output\n")

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	countByType := map[string]int{}
	for _, entry := range s.logBuffer {
		countByType[entry.LogType]++
	}

	if countByType["output"] != 1 {
		t.Fatalf("expected deduped output count 1, got %d", countByType["output"])
	}
	if countByType["system"] != 1 {
		t.Fatalf("expected deduped system count 1, got %d", countByType["system"])
	}
	if countByType["input"] != 2 {
		t.Fatalf("expected input not deduped (count 2), got %d", countByType["input"])
	}
	if countByType["input_raw"] != 2 {
		t.Fatalf("expected input_raw not deduped (count 2), got %d", countByType["input_raw"])
	}
	if countByType["output_raw"] != 2 {
		t.Fatalf("expected output_raw not deduped (count 2), got %d", countByType["output_raw"])
	}
}
