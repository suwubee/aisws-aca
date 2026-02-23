package terminal

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/detector"
	"github.com/google/uuid"
)

const (
	logTypeAINativeInput  = "ai_input_native"
	logTypeAINativeOutput = "ai_output_native"

	nativeSyncPollInterval      = 1200 * time.Millisecond
	nativeResolveInterval       = 3 * time.Second
	nativeInitialReadWindow     = 256 * 1024
	nativeHistoryReadWindow     = 512 * 1024
	nativeMaxSeenKeys           = 2000
	nativeSeenTTL               = 2 * time.Hour
	nativeMaxContentRunes       = 12000
	nativeTerminalLogLookback   = 200
	nativeHistoryInputMatchSkew = 20 * time.Minute
)

type claudeSessionLine struct {
	Type      string `json:"type"`
	UUID      string `json:"uuid"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID      string `json:"id"`
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"message"`
}

type claudeHistoryLine struct {
	Project   string `json:"project"`
	SessionID string `json:"sessionId"`
	Display   string `json:"display"`
	Timestamp int64  `json:"timestamp"`
}

var claudeWorkspaceHintRegex = regexp.MustCompile(`(?is)accessing workspace:\s*([^\r\n]+)`)
var claudeShellPromptPathRegex = regexp.MustCompile(`^(?:\([^)]+\)\s*)?[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+:([^$#%]+)[$#%](?:\s+.*)?$`)
var claudeCDCommandRegex = regexp.MustCompile(`(?i)^cd(?:\s+(.*))?$`)
var claudeCommandRegex = regexp.MustCompile(`(?i)^claude(?:\s|$)`)
var claudeWhitespaceRegex = regexp.MustCompile(`\s+`)

func (s *Session) startNativeAILogSync() {
	if s == nil {
		return
	}
	s.nativeSyncOnce.Do(func() {
		go s.runNativeAILogSync()
	})
}

func (s *Session) runNativeAILogSync() {
	ticker := time.NewTicker(nativeSyncPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.syncClaudeNativeLogs()
		}
	}
}

func (s *Session) syncClaudeNativeLogs() {
	if s == nil || model.DB == nil {
		return
	}

	aiType, sessionID, sessionFile := s.currentAISessionIdentity()
	if aiType != string(detector.AIAgentClaudeCode) && sessionID == "" && sessionFile == "" {
		if s.shouldResolveAISession() {
			s.loadAISessionFromDB()
		}
		aiType, sessionID, sessionFile = s.currentAISessionIdentity()
	}
	if aiType == "" && sessionID == "" && sessionFile == "" {
		return
	}
	if aiType != "" && aiType != string(detector.AIAgentClaudeCode) {
		return
	}

	sessionPath, resolvedSessionID := s.resolveClaudeSessionPath(strings.TrimSpace(sessionID), strings.TrimSpace(sessionFile))
	if sessionPath == "" {
		return
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = strings.TrimSpace(resolvedSessionID)
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = claudeSessionIDFromPath(sessionPath)
	}
	s.persistClaudeSessionIdentity(sessionID, sessionPath)

	lines, err := s.readClaudeNativeLines(sessionPath)
	if err != nil || len(lines) == 0 {
		return
	}

	for _, raw := range lines {
		logType, text, dedupeKey, parsedSessionID, ok := parseClaudeNativeLogLine(raw)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		if parsedSessionID != "" && parsedSessionID != strings.TrimSpace(s.aiSessionCLIID) {
			s.persistClaudeSessionIdentity(parsedSessionID, sessionPath)
		}
		if s.isNativeSeen(dedupeKey) {
			continue
		}
		s.addLog(logType, truncateRunes(text, nativeMaxContentRunes)+"\n")
	}
}

func (s *Session) currentAISessionIdentity() (aiType string, sessionID string, sessionFile string) {
	if s == nil {
		return "", "", ""
	}

	aiType = normalizeAISessionType(s.aiSessionType)
	sessionID = strings.TrimSpace(s.aiSessionCLIID)
	sessionFile = strings.TrimSpace(s.aiSessionFile)
	if aiType != "" {
		return aiType, sessionID, sessionFile
	}

	s.metaMutex.RLock()
	if s.aiAssistant != nil {
		aiType = normalizeAISessionType(s.aiAssistant.Type)
	}
	s.metaMutex.RUnlock()
	return aiType, sessionID, sessionFile
}

func (s *Session) shouldResolveAISession() bool {
	now := time.Now()
	s.nativeLogMu.Lock()
	defer s.nativeLogMu.Unlock()
	if now.Sub(s.nativeResolveAt) < nativeResolveInterval {
		return false
	}
	s.nativeResolveAt = now
	return true
}

func (s *Session) loadAISessionFromDB() {
	if s == nil || model.DB == nil {
		return
	}

	var row model.AISession
	if err := model.DB.Where("terminal_id = ?", s.id).Order("updated_at desc").First(&row).Error; err != nil {
		return
	}
	if strings.TrimSpace(s.aiSessionID) == "" {
		s.aiSessionID = strings.TrimSpace(row.ID)
	}
	if normalizeAISessionType(s.aiSessionType) == "" {
		s.aiSessionType = strings.TrimSpace(row.AIType)
	}
	if strings.TrimSpace(s.aiSessionCLIID) == "" {
		s.aiSessionCLIID = strings.TrimSpace(row.SessionID)
	}
	if strings.TrimSpace(s.aiSessionFile) == "" {
		s.aiSessionFile = strings.TrimSpace(row.SessionFile)
	}
	if strings.TrimSpace(s.aiSessionState) == "" {
		s.aiSessionState = strings.TrimSpace(row.State)
	}
	if strings.TrimSpace(s.aiSessionTaskID) == "" {
		s.aiSessionTaskID = derefStringPtr(row.TaskID)
	}
}

func (s *Session) resolveClaudeSessionPath(sessionID, sessionFile string) (path string, resolvedSessionID string) {
	if s == nil {
		return "", ""
	}

	if sessionFile != "" {
		if info, err := os.Stat(sessionFile); err == nil && !info.IsDir() {
			return sessionFile, firstNonEmpty(sessionID, claudeSessionIDFromPath(sessionFile))
		}
	}

	if sessionID != "" {
		if path := findClaudeSessionByID(sessionID); path != "" {
			return path, sessionID
		}
	}

	workDir := s.resolveTaskWorkDir()
	if workDir == "" {
		workDir = s.resolveClaudeWorkspaceHint()
	}
	if workDir == "" {
		workDir = s.resolveWorkDirFromTerminalActivity()
	}
	if workDir != "" {
		projectDir := detector.GetClaudeSessionDir(workDir)
		if path := findNewestJSONLFile(projectDir); path != "" {
			return path, firstNonEmpty(sessionID, claudeSessionIDFromPath(path))
		}

		historySessionID, historyPath := findClaudeSessionFromHistory(workDir)
		if historyPath != "" {
			return historyPath, firstNonEmpty(sessionID, historySessionID, claudeSessionIDFromPath(historyPath))
		}
	}

	historySessionID, historyPath := s.findClaudeSessionFromHistoryByRecentInput(workDir)
	if historyPath != "" {
		return historyPath, firstNonEmpty(sessionID, historySessionID, claudeSessionIDFromPath(historyPath))
	}

	return "", strings.TrimSpace(sessionID)
}

func findClaudeSessionFromHistory(workDir string) (sessionID string, sessionPath string) {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return "", ""
	}

	historyFile := filepath.Join(homeDir, ".claude", "history.jsonl")
	lines, err := readJSONLLinesFromTail(historyFile, nativeHistoryReadWindow)
	if err != nil || len(lines) == 0 {
		return "", ""
	}

	normalizedWorkDir := normalizeWorkDir(workDir)
	if normalizedWorkDir == "" {
		return "", ""
	}
	for i := len(lines) - 1; i >= 0; i-- {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}

		var item claudeHistoryLine
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}

		id := strings.TrimSpace(item.SessionID)
		if id == "" {
			continue
		}

		if normalizedWorkDir != "" {
			project := normalizeWorkDir(item.Project)
			if project == "" || project != normalizedWorkDir {
				continue
			}
		}

		if path := findClaudeSessionByID(id); path != "" {
			return id, path
		}
	}
	return "", ""
}

func (s *Session) findClaudeSessionFromHistoryByRecentInput(workDir string) (sessionID string, sessionPath string) {
	if s == nil || model.DB == nil {
		return "", ""
	}

	inputsByText, hasClaudeCommand := s.loadRecentTerminalInputs(nativeTerminalLogLookback)
	if !hasClaudeCommand || len(inputsByText) == 0 {
		return "", ""
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return "", ""
	}

	historyFile := filepath.Join(homeDir, ".claude", "history.jsonl")
	lines, err := readJSONLLinesFromTail(historyFile, nativeHistoryReadWindow)
	if err != nil || len(lines) == 0 {
		return "", ""
	}

	normalizedWorkDir := normalizeWorkDir(workDir)
	for i := len(lines) - 1; i >= 0; i-- {
		raw := strings.TrimSpace(lines[i])
		if raw == "" {
			continue
		}

		var item claudeHistoryLine
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}

		id := strings.TrimSpace(item.SessionID)
		if id == "" {
			continue
		}

		if normalizedWorkDir != "" {
			project := normalizeWorkDir(item.Project)
			if project != "" && project != normalizedWorkDir {
				continue
			}
		}

		display := normalizeHistoryMatchText(item.Display)
		if display == "" {
			continue
		}
		inputAt, ok := inputsByText[display]
		if !ok {
			continue
		}
		if !isHistoryTimestampClose(inputAt, item.Timestamp) {
			continue
		}

		if path := findClaudeSessionByID(id); path != "" {
			return id, path
		}
	}
	return "", ""
}

func (s *Session) loadRecentTerminalInputs(limit int) (map[string]time.Time, bool) {
	if s == nil || model.DB == nil {
		return nil, false
	}
	if limit <= 0 {
		limit = nativeTerminalLogLookback
	}

	var rows []model.Log
	if err := model.DB.
		Select("log_type", "content", "created_at").
		Where("terminal_id = ?", s.id).
		Where("log_type IN ?", []string{"input", "input_raw"}).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil || len(rows) == 0 {
		return nil, false
	}

	inputsByText := make(map[string]time.Time, len(rows))
	hasClaudeCommand := false
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		rawLines := strings.Split(strings.ReplaceAll(row.Content, "\r", "\n"), "\n")
		for _, rawLine := range rawLines {
			line := normalizeHistoryMatchText(rawLine)
			if line == "" {
				continue
			}
			if isClaudeCommandInput(line) {
				hasClaudeCommand = true
				continue
			}
			if prev, exists := inputsByText[line]; !exists || row.CreatedAt.After(prev) {
				inputsByText[line] = row.CreatedAt
			}
		}
	}
	return inputsByText, hasClaudeCommand
}

func normalizeHistoryMatchText(raw string) string {
	text := strings.TrimSpace(stripANSI(raw))
	if text == "" {
		return ""
	}
	text = strings.Trim(text, "`")
	text = claudeWhitespaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func isClaudeCommandInput(line string) bool {
	text := strings.TrimSpace(line)
	if text == "" {
		return false
	}
	return claudeCommandRegex.MatchString(text)
}

func isHistoryTimestampClose(inputAt time.Time, historyTimestampMillis int64) bool {
	if inputAt.IsZero() || historyTimestampMillis <= 0 {
		return true
	}
	historyAt := time.UnixMilli(historyTimestampMillis)
	diff := historyAt.Sub(inputAt)
	if diff < 0 {
		diff = -diff
	}
	return diff <= nativeHistoryInputMatchSkew
}

func (s *Session) resolveWorkDirFromTerminalActivity() string {
	if s == nil || model.DB == nil {
		return ""
	}

	baseDir := resolveExistingWorkDir(s.startDir)
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			baseDir = resolveExistingWorkDir(homeDir)
		}
	}
	if baseDir == "" {
		return ""
	}

	var rows []model.Log
	if err := model.DB.
		Select("log_type", "content", "created_at").
		Where("terminal_id = ?", s.id).
		Where("log_type IN ?", []string{"input", "input_raw", "output", "output_raw", "system"}).
		Order("created_at asc").
		Limit(nativeTerminalLogLookback).
		Find(&rows).Error; err != nil || len(rows) == 0 {
		return ""
	}

	cwd := baseDir
	hasHint := false
	for _, row := range rows {
		rawLines := strings.Split(strings.ReplaceAll(row.Content, "\r", "\n"), "\n")
		for _, rawLine := range rawLines {
			line := strings.TrimSpace(stripANSI(rawLine))
			if line == "" {
				continue
			}

			if promptDir := extractWorkDirFromPromptLine(line); promptDir != "" {
				cwd = promptDir
				hasHint = true
				continue
			}

			if row.LogType != "input" && row.LogType != "input_raw" {
				continue
			}

			if nextDir, ok := resolveWorkDirFromCDCommand(line, cwd); ok {
				cwd = nextDir
				hasHint = true
			}
		}
	}

	if !hasHint {
		return ""
	}
	return normalizeWorkDir(cwd)
}

func extractWorkDirFromPromptLine(line string) string {
	match := claudeShellPromptPathRegex.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 2 {
		return ""
	}

	dir := normalizeWorkDir(match[1])
	if dir == "" {
		return ""
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

func resolveWorkDirFromCDCommand(line, currentDir string) (string, bool) {
	match := claudeCDCommandRegex.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 2 {
		return "", false
	}

	rawTarget := strings.TrimSpace(match[1])
	if idx := strings.IndexAny(rawTarget, ";&|"); idx >= 0 {
		rawTarget = strings.TrimSpace(rawTarget[:idx])
	}
	rawTarget = strings.Trim(rawTarget, `"'`)
	if rawTarget == "-" {
		return "", false
	}

	target := rawTarget
	if target == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		target = homeDir
	}

	var candidate string
	switch {
	case strings.HasPrefix(target, "~"):
		candidate = normalizeWorkDir(target)
	case filepath.IsAbs(target):
		candidate = normalizeWorkDir(target)
	default:
		base := normalizeWorkDir(currentDir)
		if base == "" {
			base, _ = os.UserHomeDir()
		}
		candidate = normalizeWorkDir(filepath.Join(base, target))
	}

	if candidate == "" {
		return "", false
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return candidate, true
}

func (s *Session) resolveClaudeWorkspaceHint() string {
	if s == nil {
		return ""
	}
	if output := strings.TrimSpace(s.aiSessionOutput); output != "" {
		if match := claudeWorkspaceHintRegex.FindStringSubmatch(output); len(match) >= 2 {
			return normalizeWorkDir(match[1])
		}
	}

	if model.DB == nil {
		return ""
	}

	var rows []model.Log
	if err := model.DB.
		Select("content", "created_at").
		Where("terminal_id = ?", s.id).
		Where("log_type IN ?", []string{"output", "output_raw", "system"}).
		Order("created_at desc").
		Limit(80).
		Find(&rows).Error; err != nil || len(rows) == 0 {
		return ""
	}

	var contentBuilder strings.Builder
	for i := len(rows) - 1; i >= 0; i-- {
		contentBuilder.WriteString(rows[i].Content)
		contentBuilder.WriteByte('\n')
	}

	match := claudeWorkspaceHintRegex.FindStringSubmatch(contentBuilder.String())
	if len(match) >= 2 {
		return normalizeWorkDir(match[1])
	}
	return ""
}

func readJSONLLinesFromTail(path string, window int64) ([]string, error) {
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, os.ErrInvalid
	}

	size := info.Size()
	offset := int64(0)
	startMidLine := false
	if window > 0 && size > window {
		offset = size - window
		startMidLine = true
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	text := string(data)
	lines := strings.Split(text, "\n")
	if !strings.HasSuffix(text, "\n") && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	if startMidLine && len(lines) > 0 {
		lines = lines[1:]
	}
	return lines, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" {
			return text
		}
	}
	return ""
}

func claudeSessionIDFromPath(path string) string {
	base := strings.TrimSpace(filepath.Base(path))
	if !strings.HasSuffix(strings.ToLower(base), ".jsonl") {
		return ""
	}
	id := strings.TrimSpace(strings.TrimSuffix(base, ".jsonl"))
	if id == "" {
		return ""
	}
	if _, err := uuid.Parse(id); err != nil {
		return ""
	}
	return id
}

func findClaudeSessionByID(sessionID string) string {
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return ""
	}
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}
	pattern := filepath.Join(homeDir, ".claude", "projects", "*", id+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	type candidate struct {
		path string
		mt   time.Time
	}
	items := make([]candidate, 0, len(matches))
	for _, path := range matches {
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			continue
		}
		items = append(items, candidate{path: path, mt: info.ModTime()})
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].mt.After(items[j].mt)
	})
	return items[0].path
}

func findNewestJSONLFile(dir string) string {
	root := strings.TrimSpace(dir)
	if root == "" {
		return ""
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	type candidate struct {
		path string
		mt   time.Time
	}
	items := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasSuffix(strings.ToLower(name), ".jsonl") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		items = append(items, candidate{
			path: filepath.Join(root, name),
			mt:   info.ModTime(),
		})
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].mt.After(items[j].mt)
	})
	return items[0].path
}

func (s *Session) resolveTaskWorkDir() string {
	if s == nil || model.DB == nil || s.taskID == nil {
		return ""
	}
	taskID := strings.TrimSpace(*s.taskID)
	if taskID == "" {
		return ""
	}

	var task model.Task
	if err := model.DB.Select("work_dir").First(&task, "id = ?", taskID).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(task.WorkDir)
}

func (s *Session) persistClaudeSessionIdentity(sessionID, sessionPath string) {
	if s == nil || model.DB == nil {
		return
	}

	id := strings.TrimSpace(sessionID)
	path := strings.TrimSpace(sessionPath)
	now := time.Now()

	if strings.TrimSpace(s.aiSessionID) == "" {
		record := &model.AISession{
			ID:          uuid.New().String(),
			TerminalID:  s.id,
			TaskID:      cloneStringPtr(s.taskID),
			AIType:      string(detector.AIAgentClaudeCode),
			State:       defaultString(strings.TrimSpace(s.aiSessionState), string(detector.StateUnknown)),
			SessionID:   id,
			SessionFile: path,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := model.DB.Create(record).Error; err != nil {
			return
		}
		s.aiSessionID = record.ID
		s.aiSessionType = record.AIType
		s.aiSessionState = record.State
		s.aiSessionCLIID = record.SessionID
		s.aiSessionFile = record.SessionFile
		s.aiSessionTaskID = derefStringPtr(record.TaskID)
		return
	}

	updates := map[string]any{
		"updated_at": now,
	}
	if normalizeAISessionType(s.aiSessionType) != string(detector.AIAgentClaudeCode) {
		updates["ai_type"] = string(detector.AIAgentClaudeCode)
		s.aiSessionType = string(detector.AIAgentClaudeCode)
	}
	if id != "" && id != strings.TrimSpace(s.aiSessionCLIID) {
		updates["session_id"] = id
		s.aiSessionCLIID = id
	}
	if path != "" && path != strings.TrimSpace(s.aiSessionFile) {
		updates["session_file"] = path
		s.aiSessionFile = path
	}
	if len(updates) == 1 { // only updated_at
		return
	}
	_ = model.DB.Model(&model.AISession{}).Where("id = ?", s.aiSessionID).Updates(updates).Error
}

func (s *Session) readClaudeNativeLines(path string) ([]string, error) {
	if s == nil {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, os.ErrInvalid
	}
	size := info.Size()

	s.nativeLogMu.Lock()
	pathChanged := strings.TrimSpace(s.nativeLogPath) != strings.TrimSpace(path)
	if pathChanged {
		s.nativeLogPath = path
		s.nativeLogOffset = 0
		s.nativeLogTail = ""
	}
	offset := s.nativeLogOffset
	tail := s.nativeLogTail
	s.nativeLogMu.Unlock()

	startMidLine := false
	if pathChanged && size > nativeInitialReadWindow {
		offset = size - nativeInitialReadWindow
		startMidLine = true
	}
	if size < offset {
		offset = 0
		tail = ""
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	text := tail + string(data)
	lines := strings.Split(text, "\n")
	newTail := ""
	if !strings.HasSuffix(text, "\n") {
		newTail = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}
	if startMidLine && len(lines) > 0 {
		// 起始 offset 可能落在半行，丢弃第一行避免解析脏 JSON。
		lines = lines[1:]
	}

	s.nativeLogMu.Lock()
	s.nativeLogOffset = offset + int64(len(data))
	s.nativeLogTail = newTail
	s.nativeLogMu.Unlock()

	return lines, nil
}

func parseClaudeNativeLogLine(line string) (logType string, content string, key string, sessionID string, ok bool) {
	raw := strings.TrimSpace(line)
	if raw == "" {
		return "", "", "", "", false
	}

	var entry claudeSessionLine
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return "", "", "", "", false
	}

	entryType := strings.ToLower(strings.TrimSpace(entry.Type))
	if entryType != "user" && entryType != "assistant" {
		return "", "", "", "", false
	}

	text := extractClaudeMessageText(entry.Message.Content)
	if text == "" {
		return "", "", "", "", false
	}

	if entryType == "user" {
		logType = logTypeAINativeInput
	} else {
		logType = logTypeAINativeOutput
	}

	sessionID = strings.TrimSpace(entry.SessionID)
	key = buildClaudeNativeKey(entry, text, raw)
	return logType, text, key, sessionID, true
}

func extractClaudeMessageText(content any) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType := strings.ToLower(strings.TrimSpace(stringFromAny(block["type"])))
			switch blockType {
			case "text":
				if text := strings.TrimSpace(stringFromAny(block["text"])); text != "" {
					parts = append(parts, text)
				}
			case "thinking":
				if text := strings.TrimSpace(stringFromAny(block["thinking"])); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	case map[string]any:
		if text := strings.TrimSpace(stringFromAny(v["text"])); text != "" {
			return text
		}
		if text := strings.TrimSpace(stringFromAny(v["content"])); text != "" {
			return text
		}
	}
	return ""
}

func buildClaudeNativeKey(entry claudeSessionLine, content, rawLine string) string {
	entryType := strings.ToLower(strings.TrimSpace(entry.Type))
	if entryType == "" {
		entryType = "unknown"
	}
	contentHash := hashNativeContent(content)

	if id := strings.TrimSpace(entry.UUID); id != "" {
		return strings.Join([]string{entryType, "uuid", id, contentHash}, "|")
	}
	if id := strings.TrimSpace(entry.Message.ID); id != "" {
		return strings.Join([]string{entryType, "message", id, contentHash}, "|")
	}

	sessionID := strings.TrimSpace(entry.SessionID)
	timestamp := strings.TrimSpace(entry.Timestamp)
	switch {
	case sessionID != "" && timestamp != "":
		return strings.Join([]string{entryType, "session", sessionID, timestamp, contentHash}, "|")
	case sessionID != "":
		return strings.Join([]string{entryType, "session", sessionID, contentHash}, "|")
	case timestamp != "":
		return strings.Join([]string{entryType, "timestamp", timestamp, contentHash}, "|")
	}

	sum := sha1.Sum([]byte(strings.TrimSpace(rawLine)))
	return "sha1|" + hex.EncodeToString(sum[:])
}

func hashNativeContent(content string) string {
	normalized := strings.TrimSpace(content)
	if normalized == "" {
		return ""
	}
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func (s *Session) isNativeSeen(key string) bool {
	k := strings.TrimSpace(key)
	if k == "" {
		return false
	}
	now := time.Now()
	s.nativeLogMu.Lock()
	defer s.nativeLogMu.Unlock()
	if seenAt, exists := s.nativeSeen[k]; exists {
		if now.Sub(seenAt) <= nativeSeenTTL {
			return true
		}
	}
	s.nativeSeen[k] = now
	if len(s.nativeSeen) > nativeMaxSeenKeys {
		for item, ts := range s.nativeSeen {
			if now.Sub(ts) > nativeSeenTTL {
				delete(s.nativeSeen, item)
			}
		}
	}
	return false
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit])
}
