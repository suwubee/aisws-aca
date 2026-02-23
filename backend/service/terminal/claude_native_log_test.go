package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
)

func TestParseClaudeNativeLogLine_UserAndAssistant(t *testing.T) {
	userLine := `{"type":"user","sessionId":"sid-1","uuid":"u-1","message":{"role":"user","content":"你好"}}`
	logType, content, key, sessionID, ok := parseClaudeNativeLogLine(userLine)
	if !ok {
		t.Fatalf("expected user line parsed")
	}
	if logType != logTypeAINativeInput {
		t.Fatalf("expected logType %q, got %q", logTypeAINativeInput, logType)
	}
	if content != "你好" {
		t.Fatalf("expected content 你好, got %q", content)
	}
	if key == "" {
		t.Fatalf("expected non-empty dedupe key")
	}
	if sessionID != "sid-1" {
		t.Fatalf("expected session id sid-1, got %q", sessionID)
	}

	assistantLine := `{"type":"assistant","sessionId":"sid-1","uuid":"a-1","message":{"id":"msg-1","role":"assistant","content":[{"type":"text","text":"第一段"},{"type":"thinking","thinking":"第二段"}]}}`
	logType, content, key, sessionID, ok = parseClaudeNativeLogLine(assistantLine)
	if !ok {
		t.Fatalf("expected assistant line parsed")
	}
	if logType != logTypeAINativeOutput {
		t.Fatalf("expected logType %q, got %q", logTypeAINativeOutput, logType)
	}
	if content != "第一段\n\n第二段" {
		t.Fatalf("unexpected assistant content: %q", content)
	}
	if key == "" {
		t.Fatalf("expected non-empty assistant dedupe key")
	}
	if sessionID != "sid-1" {
		t.Fatalf("expected session id sid-1, got %q", sessionID)
	}
}

func TestParseClaudeNativeLogLine_IgnoresNonDialogueLine(t *testing.T) {
	line := `{"type":"file-history-snapshot","messageId":"m1"}`
	if _, _, _, _, ok := parseClaudeNativeLogLine(line); ok {
		t.Fatalf("expected non-dialogue line ignored")
	}
}

func TestSession_IsNativeSeen(t *testing.T) {
	s := NewSession("native-seen-test", "bash", 1024)
	if seen := s.isNativeSeen("k1"); seen {
		t.Fatalf("expected first key unseen")
	}
	if seen := s.isNativeSeen("k1"); !seen {
		t.Fatalf("expected duplicated key seen")
	}
}

func TestSession_IsNativeSeen_ExpiresByTTL(t *testing.T) {
	s := NewSession("native-seen-expire-test", "bash", 1024)
	now := time.Now()

	s.nativeLogMu.Lock()
	s.nativeSeen["k1"] = now.Add(-nativeSeenTTL - time.Minute)
	s.nativeLogMu.Unlock()

	if seen := s.isNativeSeen("k1"); seen {
		t.Fatalf("expected expired native key to be treated as unseen")
	}
	if seen := s.isNativeSeen("k1"); !seen {
		t.Fatalf("expected fresh native key to be treated as seen on second check")
	}
}

func TestBuildClaudeNativeKey_PrefersStableMessageIdentity(t *testing.T) {
	entryA := claudeSessionLine{
		Type:      "assistant",
		SessionID: "sid-1",
		Timestamp: "2026-02-16T00:00:00.000Z",
	}
	entryA.Message.ID = "msg-1"

	entryB := entryA
	entryB.Timestamp = "2026-02-16T00:00:01.000Z"

	keyA := buildClaudeNativeKey(entryA, "hello", `{"a":1}`)
	keyB := buildClaudeNativeKey(entryB, "hello", `{"a":2}`)
	if keyA != keyB {
		t.Fatalf("expected same key for same message identity, got %q vs %q", keyA, keyB)
	}
}

func TestSession_SyncClaudeNativeLogs(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_native_sync_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"user","sessionId":"sid-1","uuid":"u-1","message":{"role":"user","content":"你好"}}
{"type":"file-history-snapshot","messageId":"u-1"}
{"type":"assistant","sessionId":"sid-1","uuid":"a-1","message":{"role":"assistant","content":[{"type":"text","text":"你好，我是 Claude"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write temp session file failed: %v", err)
	}

	s := NewSession("native-sync-test", "bash", 1024)
	s.aiSessionType = "claude-code"
	s.aiSessionFile = path

	s.syncClaudeNativeLogs()

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	if len(s.logBuffer) != 2 {
		t.Fatalf("expected 2 native logs, got %d", len(s.logBuffer))
	}
	if s.logBuffer[0].LogType != logTypeAINativeInput {
		t.Fatalf("expected first log type %q, got %q", logTypeAINativeInput, s.logBuffer[0].LogType)
	}
	if s.logBuffer[1].LogType != logTypeAINativeOutput {
		t.Fatalf("expected second log type %q, got %q", logTypeAINativeOutput, s.logBuffer[1].LogType)
	}

	s.syncClaudeNativeLogs()
	if len(s.logBuffer) != 2 {
		t.Fatalf("expected native logs deduped on second sync, got %d", len(s.logBuffer))
	}
}

func TestResolveClaudeSessionPath_FromWorkspaceHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionID := "3999291c-eb34-4ccf-b618-0367019aa52c"
	projectDir := filepath.Join(home, ".claude", "projects", "-root-testcc")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}

	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte("{\"type\":\"user\",\"message\":{\"content\":\"hi\"}}\n"), 0o600); err != nil {
		t.Fatalf("write session file failed: %v", err)
	}

	s := NewSession("native-history-resolve", "bash", 1024)
	s.aiSessionType = "claude-code"
	s.aiSessionOutput = "Accessing workspace:\n /root/testcc"

	path, resolvedSessionID := s.resolveClaudeSessionPath("", "")
	if strings.TrimSpace(path) != strings.TrimSpace(sessionPath) {
		t.Fatalf("expected path %q, got %q", sessionPath, path)
	}
	if strings.TrimSpace(resolvedSessionID) != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, resolvedSessionID)
	}
}

func TestSession_SyncClaudeNativeLogs_FromHistoryFallback(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_native_history_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	sessionID := "3999291c-eb34-4ccf-b618-0367019aa52c"
	projectDir := filepath.Join(home, ".claude", "projects", "-root-testcc")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}

	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	content := `{"type":"user","sessionId":"3999291c-eb34-4ccf-b618-0367019aa52c","uuid":"u-1","message":{"role":"user","content":"你是谁"}}
{"type":"assistant","sessionId":"3999291c-eb34-4ccf-b618-0367019aa52c","uuid":"a-1","message":{"role":"assistant","content":[{"type":"text","text":"我是 Claude Code"}]}}
`
	if err := os.WriteFile(sessionPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write session file failed: %v", err)
	}

	historyPath := filepath.Join(home, ".claude", "history.jsonl")
	historyLine := fmt.Sprintf(`{"display":"你是谁","timestamp":1771167628692,"project":"/root/testcc","sessionId":"%s"}`+"\n", sessionID)
	if err := os.WriteFile(historyPath, []byte(historyLine), 0o600); err != nil {
		t.Fatalf("write history file failed: %v", err)
	}

	s := NewSession("native-history-sync", "bash", 1024)
	s.aiSessionType = "claude-code"
	s.aiSessionOutput = "Accessing workspace:\n /root/testcc"

	s.syncClaudeNativeLogs()

	s.logMutex.Lock()
	defer s.logMutex.Unlock()

	if len(s.logBuffer) != 2 {
		t.Fatalf("expected 2 native logs, got %d", len(s.logBuffer))
	}
	if s.logBuffer[0].LogType != logTypeAINativeInput {
		t.Fatalf("expected first log type %q, got %q", logTypeAINativeInput, s.logBuffer[0].LogType)
	}
	if s.logBuffer[1].LogType != logTypeAINativeOutput {
		t.Fatalf("expected second log type %q, got %q", logTypeAINativeOutput, s.logBuffer[1].LogType)
	}
	if strings.TrimSpace(s.aiSessionCLIID) != sessionID {
		t.Fatalf("expected synced ai session id %q, got %q", sessionID, s.aiSessionCLIID)
	}
}

func TestFindClaudeSessionFromHistory_WithWorkDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := filepath.Join(home, ".claude", "projects", "-root-testcc")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}

	sessionID := "3999291c-eb34-4ccf-b618-0367019aa52c"
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte("{\"type\":\"user\",\"message\":{\"content\":\"hello\"}}\n"), 0o600); err != nil {
		t.Fatalf("write session file failed: %v", err)
	}

	historyPath := filepath.Join(home, ".claude", "history.jsonl")
	lines := []string{
		`{"display":"foo","timestamp":1,"project":"/root/other","sessionId":"11111111-1111-1111-1111-111111111111"}`,
		fmt.Sprintf(`{"display":"bar","timestamp":2,"project":"/root/testcc","sessionId":"%s"}`, sessionID),
	}
	if err := os.WriteFile(historyPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write history file failed: %v", err)
	}

	foundID, foundPath := findClaudeSessionFromHistory("/root/testcc")
	if foundID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, foundID)
	}
	if strings.TrimSpace(foundPath) != strings.TrimSpace(sessionPath) {
		t.Fatalf("expected session path %q, got %q", sessionPath, foundPath)
	}
}

func TestResolveClaudeSessionPath_WithoutWorkDirSkipsHistoryFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := filepath.Join(home, ".claude", "projects", "-root-testcc")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}

	sessionID := "3999291c-eb34-4ccf-b618-0367019aa52c"
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte("{\"type\":\"user\",\"message\":{\"content\":\"hello\"}}\n"), 0o600); err != nil {
		t.Fatalf("write session file failed: %v", err)
	}

	historyPath := filepath.Join(home, ".claude", "history.jsonl")
	historyLine := fmt.Sprintf(`{"display":"hello","timestamp":1771167628692,"project":"/root/testcc","sessionId":"%s"}`+"\n", sessionID)
	if err := os.WriteFile(historyPath, []byte(historyLine), 0o600); err != nil {
		t.Fatalf("write history file failed: %v", err)
	}

	s := NewSession("native-no-workdir-history", "bash", 1024)
	s.aiSessionType = "claude-code"

	path, resolvedSessionID := s.resolveClaudeSessionPath("", "")
	if strings.TrimSpace(path) != "" {
		t.Fatalf("expected empty path when workdir is unknown, got %q", path)
	}
	if strings.TrimSpace(resolvedSessionID) != "" {
		t.Fatalf("expected empty resolved session id when workdir is unknown, got %q", resolvedSessionID)
	}
}

func TestResolveClaudeWorkspaceHint_FromLogs(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_native_workspace_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	s := NewSession("workspace-hint-log", "bash", 1024)
	terminalID := s.ID()
	now := time.Now()

	logs := []*model.Log{
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			LogType:    "output",
			Content:    " Accessing workspace:\n",
			CreatedAt:  now,
		},
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			LogType:    "output",
			Content:    " /root/testcc_native_unique\n",
			CreatedAt:  now.Add(time.Second),
		},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}

	hint := s.resolveClaudeWorkspaceHint()
	if hint != "/root/testcc_native_unique" {
		t.Fatalf("expected workspace hint /root/testcc_native_unique, got %q", hint)
	}
}

func TestResolveWorkDirFromTerminalActivity_FromCDInput(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_native_workdir_activity_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	workDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir failed: %v", err)
	}

	s := NewSession("terminal-workdir-activity", "bash", 1024)
	terminalID := s.ID()
	now := time.Now()
	logs := []*model.Log{
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			LogType:    "input",
			Content:    "cd " + workDir + "\n",
			CreatedAt:  now,
		},
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			LogType:    "input",
			Content:    "claude\n",
			CreatedAt:  now.Add(time.Second),
		},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}

	resolved := s.resolveWorkDirFromTerminalActivity()
	if resolved != normalizeWorkDir(workDir) {
		t.Fatalf("expected resolved workdir %q, got %q", workDir, resolved)
	}
}

func TestResolveClaudeSessionPath_WithoutWorkDirUsesRecentInputHistoryMatch(t *testing.T) {
	dsn := fmt.Sprintf("file:terminal_native_recent_input_history_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	projectWorkDir := filepath.Join(t.TempDir(), "testcc")
	if err := os.MkdirAll(projectWorkDir, 0o755); err != nil {
		t.Fatalf("mkdir project workdir failed: %v", err)
	}

	projectName := strings.ReplaceAll(projectWorkDir, "/", "-")
	if !strings.HasPrefix(projectName, "-") {
		projectName = "-" + projectName
	}
	projectDir := filepath.Join(home, ".claude", "projects", projectName)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}

	sessionID := "3999291c-eb34-4ccf-b618-0367019aa52c"
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(sessionPath, []byte("{\"type\":\"user\",\"message\":{\"content\":\"你好你是谁\"}}\n"), 0o600); err != nil {
		t.Fatalf("write session file failed: %v", err)
	}

	now := time.Now()
	historyPath := filepath.Join(home, ".claude", "history.jsonl")
	historyLine := fmt.Sprintf(
		`{"display":"你好你是谁","timestamp":%d,"project":"%s","sessionId":"%s"}`+"\n",
		now.UnixMilli(),
		projectWorkDir,
		sessionID,
	)
	if err := os.WriteFile(historyPath, []byte(historyLine), 0o600); err != nil {
		t.Fatalf("write history file failed: %v", err)
	}

	s := NewSession("native-recent-input-history", "bash", 1024)
	s.aiSessionType = "claude-code"

	terminalID := s.ID()
	logs := []*model.Log{
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			LogType:    "input",
			Content:    "claude\n",
			CreatedAt:  now.Add(-2 * time.Second),
		},
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID,
			LogType:    "input",
			Content:    "你好你是谁\n",
			CreatedAt:  now,
		},
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}

	path, resolvedSessionID := s.resolveClaudeSessionPath("", "")
	if strings.TrimSpace(path) != strings.TrimSpace(sessionPath) {
		t.Fatalf("expected session path %q, got %q", sessionPath, path)
	}
	if strings.TrimSpace(resolvedSessionID) != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, resolvedSessionID)
	}
}
