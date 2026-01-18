package terminal

import "testing"

func TestSessionSetTaskIDCopiesValue(t *testing.T) {
	s := NewSession("sess-1", "bash", 1024)
	taskID := "task-1"
	s.SetTaskID(&taskID)

	// Mutate the original variable to simulate unsafe string reuse.
	taskID = "task-2"

	got := s.TaskID()
	if got == nil {
		t.Fatalf("expected task id to be set")
	}
	if *got != "task-1" {
		t.Fatalf("expected task id to remain %q, got %q", "task-1", *got)
	}

	meta := s.Metadata()
	if meta.TaskID == nil {
		t.Fatalf("expected metadata task id to be set")
	}
	if *meta.TaskID != "task-1" {
		t.Fatalf("expected metadata task id to remain %q, got %q", "task-1", *meta.TaskID)
	}
}

func TestSessionSetServerInfoCopiesValue(t *testing.T) {
	s := NewSession("sess-1", "bash", 1024)
	serverID := "server-1"
	s.SetServerInfo(&serverID, "name-1", "host-1")

	// Mutate the original variable to simulate unsafe string reuse.
	serverID = "server-2"

	meta := s.Metadata()
	if meta.ServerID == nil {
		t.Fatalf("expected metadata server id to be set")
	}
	if *meta.ServerID != "server-1" {
		t.Fatalf("expected metadata server id to remain %q, got %q", "server-1", *meta.ServerID)
	}
	if meta.ServerName != "name-1" {
		t.Fatalf("expected metadata server name %q, got %q", "name-1", meta.ServerName)
	}
	if meta.ServerHost != "host-1" {
		t.Fatalf("expected metadata server host %q, got %q", "host-1", meta.ServerHost)
	}
}

