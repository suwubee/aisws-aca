package detector

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo Claude Code 会话信息
type SessionInfo struct {
	ID         string    `json:"id"`
	ModTime    time.Time `json:"mod_time"`
	Size       int64     `json:"size"`
	WorkDir    string    `json:"work_dir"`
	SessionDir string    `json:"session_dir"`
}

// GetClaudeSessionDir 获取 Claude Code 会话目录
func GetClaudeSessionDir(workDir string) string {
	// 将路径转换为 Claude Code 的项目目录名格式
	// /root/test2 -> -root-test2
	projectName := strings.ReplaceAll(workDir, "/", "-")
	if !strings.HasPrefix(projectName, "-") {
		projectName = "-" + projectName
	}

	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude", "projects", projectName)
}

// GetLatestClaudeSession 获取最新的 Claude Code 会话ID
func GetLatestClaudeSession(workDir string) (string, error) {
	sessionDir := GetClaudeSessionDir(workDir)

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", err
	}

	// 过滤出会话文件（UUID格式的.jsonl文件）
	var sessions []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// UUID格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.jsonl
		if strings.HasSuffix(name, ".jsonl") && len(name) == 41 {
			sessions = append(sessions, entry)
		}
	}

	if len(sessions) == 0 {
		return "", nil
	}

	// 按修改时间排序，获取最新的
	sort.Slice(sessions, func(i, j int) bool {
		infoI, _ := sessions[i].Info()
		infoJ, _ := sessions[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// 返回最新会话的ID（去掉.jsonl后缀）
	return strings.TrimSuffix(sessions[0].Name(), ".jsonl"), nil
}

// ListClaudeSessions 列出所有 Claude Code 会话
func ListClaudeSessions(workDir string) ([]SessionInfo, error) {
	sessionDir := GetClaudeSessionDir(workDir)

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") && len(name) == 41 {
			info, _ := entry.Info()
			sessions = append(sessions, SessionInfo{
				ID:         strings.TrimSuffix(name, ".jsonl"),
				ModTime:    info.ModTime(),
				Size:       info.Size(),
				WorkDir:    workDir,
				SessionDir: sessionDir,
			})
		}
	}

	// 按修改时间排序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	return sessions, nil
}
