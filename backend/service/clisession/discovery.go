package clisession

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DiscoveredSession struct {
	Tool        string    `json:"tool"`
	AIType      string    `json:"ai_type"`
	SessionID   string    `json:"session_id"`
	SessionFile string    `json:"session_file"`
	ProjectKey  string    `json:"project_key,omitempty"`
	CWD         string    `json:"cwd,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DiscoverOptions struct {
	Tool    string
	WorkDir string
	Scope   string // task, all
	Limit   int
}

type SSHExecutor interface {
	ExecuteCommand(serverID, command string) (string, error)
}

func DiscoverSessions(serverID string, exec SSHExecutor, opts DiscoverOptions) ([]DiscoveredSession, error) {
	tool := strings.ToLower(strings.TrimSpace(opts.Tool))
	scope := strings.ToLower(strings.TrimSpace(opts.Scope))
	if scope == "" {
		scope = "task"
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	workDir := strings.TrimSpace(opts.WorkDir)

	if strings.TrimSpace(serverID) != "" {
		if exec == nil {
			return nil, errors.New("ssh executor is required")
		}
		return discoverRemote(strings.TrimSpace(serverID), exec, tool, workDir, scope, limit)
	}

	return discoverLocal(tool, workDir, scope, limit)
}

func discoverLocal(tool, workDir, scope string, limit int) ([]DiscoveredSession, error) {
	var sessions []DiscoveredSession

	switch tool {
	case "", "auto":
		claudeSessions, _ := discoverClaudeLocal(workDir, scope, limit)
		codexSessions, _ := discoverCodexLocal(workDir, scope, limit)
		sessions = append(sessions, claudeSessions...)
		sessions = append(sessions, codexSessions...)
	case "claude":
		found, err := discoverClaudeLocal(workDir, scope, limit)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, found...)
	case "codex":
		found, err := discoverCodexLocal(workDir, scope, limit)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, found...)
	default:
		return nil, errors.New("unsupported tool: " + tool)
	}

	sortDiscoveredSessions(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func discoverRemote(serverID string, exec SSHExecutor, tool, workDir, scope string, limit int) ([]DiscoveredSession, error) {
	var sessions []DiscoveredSession

	switch tool {
	case "", "auto":
		claudeSessions, _ := discoverClaudeRemote(serverID, exec, workDir, scope, limit)
		codexSessions, _ := discoverCodexRemote(serverID, exec, workDir, scope, limit)
		sessions = append(sessions, claudeSessions...)
		sessions = append(sessions, codexSessions...)
	case "claude":
		found, err := discoverClaudeRemote(serverID, exec, workDir, scope, limit)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, found...)
	case "codex":
		found, err := discoverCodexRemote(serverID, exec, workDir, scope, limit)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, found...)
	default:
		return nil, errors.New("unsupported tool: " + tool)
	}

	sortDiscoveredSessions(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func sortDiscoveredSessions(items []DiscoveredSession) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func encodeClaudeProjectKey(workDir string) string {
	normalized := strings.TrimSpace(workDir)
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	normalized = strings.TrimRight(normalized, "/")
	if normalized == "" {
		normalized = "/"
	}

	replacer := strings.NewReplacer(
		":", "-",
		"/", "-",
		"_", "-",
	)
	return replacer.Replace(normalized)
}

func discoverClaudeLocal(workDir, scope string, limit int) ([]DiscoveredSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	root := filepath.Join(home, ".claude", "projects")
	if strings.ToLower(scope) == "task" && strings.TrimSpace(workDir) != "" {
		projectKey := encodeClaudeProjectKey(workDir)
		if projectKey == "" {
			return nil, nil
		}
		projectDir := filepath.Join(root, projectKey)
		return discoverClaudeProjectDir(projectDir, projectKey, limit)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []DiscoveredSession
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectKey := entry.Name()
		projectDir := filepath.Join(root, projectKey)
		found, err := discoverClaudeProjectDir(projectDir, projectKey, limit)
		if err != nil {
			continue
		}
		sessions = append(sessions, found...)
	}

	sortDiscoveredSessions(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func discoverClaudeProjectDir(projectDir string, projectKey string, limit int) ([]DiscoveredSession, error) {
	info, err := os.Stat(projectDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var sessions []DiscoveredSession
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		id := uuidRegex.FindString(name)
		if id == "" {
			continue
		}
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, DiscoveredSession{
			Tool:        "claude",
			AIType:      "claude-code",
			SessionID:   id,
			SessionFile: filepath.Join(projectDir, name),
			ProjectKey:  projectKey,
			UpdatedAt:   fileInfo.ModTime(),
		})
	}

	sortDiscoveredSessions(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func discoverCodexLocal(workDir, scope string, limit int) ([]DiscoveredSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	root := filepath.Join(home, ".codex", "sessions")
	pattern := filepath.Join(root, "*", "*", "*", "rollout-*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	normalizedWorkDir := normalizeComparePath(workDir)
	var sessions []DiscoveredSession
	for _, filePath := range files {
		base := filepath.Base(filePath)
		match := codexFile.FindStringSubmatch(base)
		if len(match) != 3 {
			continue
		}
		sessionID := strings.TrimSpace(match[2])
		if sessionID == "" {
			continue
		}
		stat, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		cwd := ""
		if normalizedWorkDir != "" || strings.ToLower(scope) != "task" {
			if line, err := readFirstLineLimited(filePath, 512*1024); err == nil {
				cwd = extractJSONStringValue(line, "cwd")
			}
		}

		sessions = append(sessions, DiscoveredSession{
			Tool:        "codex",
			AIType:      "codex",
			SessionID:   sessionID,
			SessionFile: filePath,
			CWD:         cwd,
			UpdatedAt:   stat.ModTime(),
		})
	}

	sortDiscoveredSessions(sessions)

	if normalizedWorkDir != "" {
		filtered := make([]DiscoveredSession, 0, len(sessions))
		for _, item := range sessions {
			if relatedPaths(normalizedWorkDir, normalizeComparePath(item.CWD)) {
				filtered = append(filtered, item)
			}
		}
		sessions = filtered
	}

	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}

func discoverClaudeRemote(serverID string, exec SSHExecutor, workDir, scope string, limit int) ([]DiscoveredSession, error) {
	projectKey := ""
	if strings.ToLower(scope) == "task" && strings.TrimSpace(workDir) != "" {
		projectKey = encodeClaudeProjectKey(workDir)
		if projectKey == "" {
			return nil, nil
		}
	}

	var cmd string
	if projectKey != "" {
		cmd = "find ~/.claude/projects/" + projectKey + " -maxdepth 1 -type f -name '*.jsonl' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n " + strconv.Itoa(limit)
	} else {
		cmd = "find ~/.claude/projects -maxdepth 2 -type f -name '*.jsonl' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n " + strconv.Itoa(limit)
	}

	output, err := exec.ExecuteCommand(serverID, cmd)
	if err != nil && strings.TrimSpace(output) == "" {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var sessions []DiscoveredSession
	for _, line := range lines {
		parsed := parseFindLine(line)
		if parsed.Path == "" {
			continue
		}
		base := path.Base(parsed.Path)
		id := uuidRegex.FindString(base)
		if id == "" {
			continue
		}

		projectKey := path.Base(path.Dir(parsed.Path))
		sessions = append(sessions, DiscoveredSession{
			Tool:        "claude",
			AIType:      "claude-code",
			SessionID:   id,
			SessionFile: parsed.Path,
			ProjectKey:  projectKey,
			UpdatedAt:   parsed.ModTime,
		})
	}

	sortDiscoveredSessions(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func discoverCodexRemote(serverID string, exec SSHExecutor, workDir, scope string, limit int) ([]DiscoveredSession, error) {
	_ = workDir
	_ = scope

	cmd := "find ~/.codex/sessions -type f -name 'rollout-*.jsonl' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n " + strconv.Itoa(limit)
	output, err := exec.ExecuteCommand(serverID, cmd)
	if err != nil && strings.TrimSpace(output) == "" {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var sessions []DiscoveredSession
	for _, line := range lines {
		parsed := parseFindLine(line)
		if parsed.Path == "" {
			continue
		}
		base := path.Base(parsed.Path)
		match := codexFile.FindStringSubmatch(base)
		if len(match) != 3 {
			continue
		}
		sessionID := strings.TrimSpace(match[2])
		if sessionID == "" {
			continue
		}
		sessions = append(sessions, DiscoveredSession{
			Tool:        "codex",
			AIType:      "codex",
			SessionID:   sessionID,
			SessionFile: parsed.Path,
			UpdatedAt:   parsed.ModTime,
		})
	}

	sortDiscoveredSessions(sessions)
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

type findLine struct {
	Path    string
	ModTime time.Time
}

func parseFindLine(line string) findLine {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return findLine{}
	}
	idx := strings.IndexByte(trimmed, ' ')
	if idx == -1 {
		return findLine{Path: trimmed}
	}

	ts := strings.TrimSpace(trimmed[:idx])
	p := strings.TrimSpace(trimmed[idx+1:])
	if p == "" {
		return findLine{}
	}

	if ts == "" {
		return findLine{Path: p}
	}

	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return findLine{Path: p}
	}
	seconds := int64(f)
	return findLine{Path: p, ModTime: time.Unix(seconds, 0)}
}

func normalizeComparePath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	if trimmed != "/" {
		trimmed = strings.TrimRight(trimmed, "/")
	}
	return trimmed
}

func relatedPaths(a, b string) bool {
	a = normalizeComparePath(a)
	b = normalizeComparePath(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
		return true
	}
	return false
}

func readFirstLineLimited(filePath string, maxBytes int64) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader := bufio.NewReader(io.LimitReader(f, maxBytes))
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func extractJSONStringValue(line string, key string) string {
	needle := `"` + key + `":`
	idx := strings.Index(line, needle)
	if idx == -1 {
		return ""
	}
	rest := strings.TrimLeft(line[idx+len(needle):], " \t")
	if !strings.HasPrefix(rest, "\"") {
		return ""
	}
	rest = rest[1:]

	end := -1
	escaped := false
	for i := 0; i < len(rest); i++ {
		if escaped {
			escaped = false
			continue
		}
		switch rest[i] {
		case '\\':
			escaped = true
		case '"':
			end = i
			i = len(rest)
		}
	}
	if end == -1 {
		return ""
	}

	raw := rest[:end]
	decoded, err := strconv.Unquote(`"` + raw + `"`)
	if err != nil {
		return raw
	}
	return decoded
}
