package terminal

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ai-coding-assistant/model"
)

const (
	defaultHistorySnapshotBytes = 256 * 1024
	historySnapshotLogLimit     = 5000
)

func normalizeSnapshotMaxBytes(maxBytes int) int {
	if maxBytes <= 0 {
		return defaultHistorySnapshotBytes
	}
	// Keep a minimal buffer so we can at least show a few lines.
	if maxBytes < 8*1024 {
		return 8 * 1024
	}
	// Defensive upper bound for accidental huge requests.
	if maxBytes > 8*1024*1024 {
		return 8 * 1024 * 1024
	}
	return maxBytes
}

func truncateBytesFromLeftUTF8(data []byte, maxBytes int) []byte {
	if len(data) <= maxBytes {
		return data
	}
	start := len(data) - maxBytes
	// Move to a UTF-8 rune boundary (avoid starting on continuation bytes).
	for start < len(data) && (data[start]&0xC0) == 0x80 {
		start++
	}
	// As a last resort, ensure the slice is valid UTF-8.
	for start < len(data) && !utf8.Valid(data[start:]) {
		start++
	}
	return data[start:]
}

// CaptureTmuxSnapshot captures recent visible+history lines from a tmux pane and returns
// them as a plain-text snapshot. It is best-effort and intended for reconnect rehydration.
func CaptureTmuxSnapshot(tmuxSession string, maxBytes int) ([]byte, error) {
	tmuxSession = strings.TrimSpace(tmuxSession)
	if tmuxSession == "" {
		return nil, errors.New("tmux session is required")
	}

	maxBytes = normalizeSnapshotMaxBytes(maxBytes)

	// Heuristic: estimate ~120 bytes per line for a 120-col terminal.
	lines := maxBytes / 120
	if lines < 200 {
		lines = 200
	}
	if lines > 5000 {
		lines = 5000
	}

	target := tmuxSession
	if !strings.Contains(target, ":") {
		target = target + ":0.0"
	}

	startOpt := fmt.Sprintf("-%d", lines)
	out, err := execCommand("tmux", "capture-pane", "-p", "-S", startOpt, "-t", target).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux capture-pane failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	out = FilterInternalMarkers(out)
	out = truncateBytesFromLeftUTF8(out, maxBytes)
	return out, nil
}

type terminalLogRow struct {
	LogType   string
	Content   string
	CreatedAt time.Time
}

// BuildTranscriptSnapshotFromLogs rebuilds a reconnect snapshot from persisted terminal logs.
// It merges input/output logs in chronological order and returns up to maxBytes.
func BuildTranscriptSnapshotFromLogs(terminalID string, maxBytes int) ([]byte, error) {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return nil, errors.New("terminal id is required")
	}
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}

	maxBytes = normalizeSnapshotMaxBytes(maxBytes)

	var rows []terminalLogRow
	if err := model.DB.Model(&model.Log{}).
		Select("log_type", "content", "created_at").
		Where("terminal_id = ?", terminalID).
		Where("log_type IN ?", []string{"input", "output"}).
		Order("created_at desc").
		Limit(historySnapshotLogLimit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	selected := make([]terminalLogRow, 0, len(rows))
	total := 0
	for _, row := range rows {
		if strings.TrimSpace(row.Content) == "" {
			continue
		}
		b := []byte(row.Content)
		if len(b) > maxBytes {
			// Single giant row: keep the tail.
			b = truncateBytesFromLeftUTF8(b, maxBytes)
			return FilterInternalMarkers(b), nil
		}
		if total+len(b) > maxBytes {
			break
		}
		selected = append(selected, row)
		total += len(b)
	}
	if len(selected) == 0 {
		return nil, nil
	}

	// Reverse to chronological order.
	buf := make([]byte, 0, total)
	for i := len(selected) - 1; i >= 0; i-- {
		buf = append(buf, selected[i].Content...)
	}

	buf = FilterInternalMarkers(buf)
	return buf, nil
}
