package setupwizard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func startProcessDetached(command string, dir string, envMap map[string]string, logPath string, pidPath string, extraEnv map[string]string, args ...string) error {
	cmdName := strings.TrimSpace(command)
	if cmdName == "" {
		return errors.New("command is required")
	}

	workingDir := strings.TrimSpace(dir)
	if workingDir == "" {
		return errors.New("dir is required")
	}

	if strings.TrimSpace(logPath) == "" {
		return errors.New("logPath is required")
	}
	if strings.TrimSpace(pidPath) == "" {
		return errors.New("pidPath is required")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return err
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = workingDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = mergeEnviron(os.Environ(), mergeMaps(envMap, extraEnv))

	applyDetach(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	pid := cmd.Process.Pid
	if pid <= 0 {
		return errors.New("failed to start process")
	}

	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return err
	}
	return nil
}

func mergeMaps(a map[string]string, b map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// applyDetach ensures child processes won't receive the parent's Ctrl+C / terminal signals.
// On platforms where detaching is not supported, this is a no-op.
func applyDetach(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	applyDetachPlatform(cmd)
}
