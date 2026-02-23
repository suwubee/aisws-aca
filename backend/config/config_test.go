package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_CreatesDataEnvInCurrentWorkingDir(t *testing.T) {
	tempDir := t.TempDir()
	setWorkingDir(t, tempDir)
	clearConfigEnv(t)

	cfg := Load()

	envPath := filepath.Join(tempDir, "data", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("expected %s to be created, got: %v", envPath, err)
	}
	if !strings.Contains(string(content), "SERVER_PORT=34007") {
		t.Fatalf("expected default SERVER_PORT in %s", envPath)
	}

	wantDSN := filepath.Join(tempDir, "data", "aca.db")
	if cfg.Database.DSN != wantDSN {
		t.Fatalf("expected dsn %q, got %q", wantDSN, cfg.Database.DSN)
	}
}

func TestLoad_ReadsValuesFromDataEnv(t *testing.T) {
	tempDir := t.TempDir()
	setWorkingDir(t, tempDir)
	clearConfigEnv(t)

	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data failed: %v", err)
	}

	dotenv := strings.Join([]string{
		"SERVER_PORT=35555",
		"DATABASE_DSN=./data/custom.db",
		"AUTH_USERNAME=alice",
		"AUTH_PASSWORD=secret123",
		"DEMO_MODE=true",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatalf("write data/.env failed: %v", err)
	}

	cfg := Load()
	if cfg.Server.Port != "35555" {
		t.Fatalf("expected server port 35555, got %q", cfg.Server.Port)
	}
	wantDSN := filepath.Join(tempDir, "data", "custom.db")
	if cfg.Database.DSN != wantDSN {
		t.Fatalf("expected dsn %q, got %q", wantDSN, cfg.Database.DSN)
	}
	if cfg.Auth.Username != "alice" {
		t.Fatalf("expected auth username alice, got %q", cfg.Auth.Username)
	}
	if cfg.Auth.Password != "secret123" {
		t.Fatalf("expected auth password secret123, got %q", cfg.Auth.Password)
	}
	if !cfg.App.DemoMode {
		t.Fatalf("expected demo mode true")
	}
}

func TestLoad_EnvCanOverrideDataEnv(t *testing.T) {
	tempDir := t.TempDir()
	setWorkingDir(t, tempDir)
	clearConfigEnv(t)

	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("SERVER_PORT=35555\nJWT_SECRET=file-secret\n"), 0o600); err != nil {
		t.Fatalf("write data/.env failed: %v", err)
	}

	t.Setenv("SERVER_PORT", "36666")
	t.Setenv("JWT_SECRET", "env-secret")

	cfg := Load()
	if cfg.Server.Port != "36666" {
		t.Fatalf("expected server port from env 36666, got %q", cfg.Server.Port)
	}
	if cfg.Auth.JWTSecret != "env-secret" {
		t.Fatalf("expected jwt secret from env, got %q", cfg.Auth.JWTSecret)
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

func setWorkingDir(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s failed: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"DEMO_MODE",
		"SERVER_HOST",
		"SERVER_PORT",
		"DATABASE_DSN",
		"JWT_SECRET",
		"AUTH_USERNAME",
		"AUTH_PASSWORD",
		"TERMINAL_SHELL",
		"TERMINAL_DEFAULT_LOGIN_DIR",
		"LOG_LEVEL",
		"LOG_FILE",
		"CORS_ALLOW_ORIGINS",
		"CORS_ALLOW_METHODS",
		"CORS_ALLOW_HEADERS",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
