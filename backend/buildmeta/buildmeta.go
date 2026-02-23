package buildmeta

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

// These fields are designed to be injected with go build -ldflags.
var (
	Version   = "dev"
	GitCommit = ""
	GitBranch = ""
	BuildTime = ""
)

type Info struct {
	Version   string
	GitCommit string
	GitBranch string
	BuildTime string
	GitDirty  bool
	GoVersion string
}

func Resolve() Info {
	info := Info{
		Version:   normalizeVersion(Version),
		GitCommit: strings.TrimSpace(GitCommit),
		GitBranch: strings.TrimSpace(GitBranch),
		BuildTime: strings.TrimSpace(BuildTime),
		GoVersion: runtime.Version(),
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = strings.TrimSpace(bi.Main.Version)
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.GitCommit == "" {
					info.GitCommit = strings.TrimSpace(s.Value)
				}
			case "vcs.time":
				if info.BuildTime == "" {
					info.BuildTime = strings.TrimSpace(s.Value)
				}
			case "vcs.modified":
				if strings.EqualFold(strings.TrimSpace(s.Value), "true") {
					info.GitDirty = true
				}
			}
		}
	}

	if info.GitBranch == "" {
		info.GitBranch = detectGitBranch()
	}

	if info.GitCommit == "" {
		info.GitCommit = "unknown"
	}
	if info.GitBranch == "" {
		info.GitBranch = "unknown"
	}
	if info.BuildTime == "" {
		info.BuildTime = "unknown"
	}

	return info
}

func normalizeVersion(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return "dev"
	}
	return trimmed
}

func detectGitBranch() string {
	if fromEnv := strings.TrimSpace(os.Getenv("ACA_GIT_BRANCH")); fromEnv != "" {
		return fromEnv
	}
	if wd, err := os.Getwd(); err == nil {
		if branch := readGitBranchFromTree(wd); branch != "" {
			return branch
		}
	}
	if exePath, err := os.Executable(); err == nil {
		if branch := readGitBranchFromTree(filepath.Dir(exePath)); branch != "" {
			return branch
		}
	}
	return ""
}

func readGitBranchFromTree(startDir string) string {
	dir := filepath.Clean(startDir)
	for {
		gitDir, ok := resolveGitDir(dir)
		if ok {
			headPath := filepath.Join(gitDir, "HEAD")
			if data, err := os.ReadFile(headPath); err == nil {
				branch := parseHeadBranch(string(data))
				if branch != "" {
					return branch
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func resolveGitDir(repoDir string) (string, bool) {
	gitPath := filepath.Join(repoDir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return gitPath, true
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	content := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(strings.ToLower(content), prefix) {
		return "", false
	}
	gitDir := strings.TrimSpace(content[len(prefix):])
	if gitDir == "" {
		return "", false
	}
	if filepath.IsAbs(gitDir) {
		return gitDir, true
	}
	return filepath.Clean(filepath.Join(repoDir, gitDir)), true
}

func parseHeadBranch(head string) string {
	line := strings.TrimSpace(head)
	if line == "" {
		return ""
	}
	const refPrefix = "ref:"
	if !strings.HasPrefix(strings.ToLower(line), refPrefix) {
		return ""
	}
	ref := strings.TrimSpace(line[len(refPrefix):])
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return strings.TrimSpace(ref)
}
