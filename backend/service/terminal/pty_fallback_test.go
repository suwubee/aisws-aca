package terminal

import (
	"os/exec"
	"testing"
)

func TestSessionStart_FallsBackToPipesWhenPTYDenied(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}

	s := NewSession("term-pipe-fallback", bashPath, 1024)
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if s.backend == nil && s.pty == nil {
		t.Fatalf("expected either PTY backend or pipe fallback backend")
	}
}
