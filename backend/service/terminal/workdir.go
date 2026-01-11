package terminal

import (
	"os"
	"path/filepath"
	"strings"
)

func normalizeWorkDir(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}

	expanded := os.ExpandEnv(raw)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			if expanded == "~" {
				expanded = home
			} else {
				expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
			}
		}
	}

	return filepath.Clean(expanded)
}

func resolveExistingWorkDir(candidates ...string) string {
	for _, candidate := range candidates {
		dir := normalizeWorkDir(candidate)
		if dir == "" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if info, err := os.Stat(home); err == nil && info.IsDir() {
		return home
	}
	return ""
}

