package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ResolvesDatabaseDSNRelativeToBackendDir(t *testing.T) {
	tempDir := t.TempDir()

	backendDir := filepath.Join(tempDir, "backend")
	frontendDir := filepath.Join(tempDir, "frontend")

	if err := os.MkdirAll(filepath.Join(backendDir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir backend/data failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(frontendDir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir frontend/data failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(backendDir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("write backend/go.mod failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backendDir, "data", "aca.db"), []byte(""), 0o644); err != nil {
		t.Fatalf("write backend/data/aca.db failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "data", "aca.db"), []byte(""), 0o644); err != nil {
		t.Fatalf("write frontend/data/aca.db failed: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.Chdir(frontendDir); err != nil {
		t.Fatalf("chdir frontend failed: %v", err)
	}

	prev, had := os.LookupEnv("DATABASE_DSN")
	if err := os.Unsetenv("DATABASE_DSN"); err != nil {
		t.Fatalf("unset DATABASE_DSN failed: %v", err)
	}
	t.Cleanup(func() {
		if !had {
			_ = os.Unsetenv("DATABASE_DSN")
			return
		}
		_ = os.Setenv("DATABASE_DSN", prev)
	})

	cfg := Load()
	want := filepath.Join(tempDir, "backend", "data", "aca.db")
	if cfg.Database.DSN != want {
		t.Fatalf("expected resolved DSN %q, got %q", want, cfg.Database.DSN)
	}
}

func TestResolveDatabaseDSN_PreservesSpecialForms(t *testing.T) {
	if got := resolveDatabaseDSN(":memory:"); got != ":memory:" {
		t.Fatalf("expected :memory:, got %q", got)
	}

	if got := resolveDatabaseDSN("file:test.db?mode=memory&cache=shared"); got != "file:test.db?mode=memory&cache=shared" {
		t.Fatalf("expected file: DSN unchanged, got %q", got)
	}

	if got := resolveDatabaseDSN("postgres://example"); got != "postgres://example" {
		t.Fatalf("expected url DSN unchanged, got %q", got)
	}
}
