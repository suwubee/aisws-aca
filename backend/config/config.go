package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Terminal TerminalConfig
	Log      LogConfig
	CORS     CORSConfig
}

type AppConfig struct {
	DemoMode bool
}

type ServerConfig struct {
	Host string
	Port string
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type AuthConfig struct {
	JWTSecret     string
	JWTExpiration time.Duration
	Username      string
	Password      string
}

type TerminalConfig struct {
	DefaultShell    string
	DefaultLoginDir string
	ScrollbackBytes int
	IdleTimeout     time.Duration
	MaxSessions     int
}

type LogConfig struct {
	Level string
	File  string
}

type CORSConfig struct {
	AllowOrigins string
	AllowMethods string
	AllowHeaders string
}

func Load() *Config {
	return &Config{
		App: AppConfig{
			DemoMode: getEnvBool("DEMO_MODE", false),
		},
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "34007"),
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    resolveDatabaseDSN(getEnv("DATABASE_DSN", "")),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiration: 24 * time.Hour,
			Username:      getEnv("AUTH_USERNAME", "admin"),
			Password:      getEnv("AUTH_PASSWORD", "admin123"),
		},
		Terminal: TerminalConfig{
			DefaultShell:    getEnv("TERMINAL_SHELL", "/bin/bash"),
			DefaultLoginDir: getEnv("TERMINAL_DEFAULT_LOGIN_DIR", "~/"),
			ScrollbackBytes: 256 * 1024, // 256KB
			IdleTimeout:     10 * time.Minute,
			MaxSessions:     100,
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
			File:  getEnv("LOG_FILE", ""),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv("CORS_ALLOW_ORIGINS", "*"),
			AllowMethods: getEnv("CORS_ALLOW_METHODS", "GET,POST,PUT,DELETE,OPTIONS"),
			AllowHeaders: getEnv("CORS_ALLOW_HEADERS", "Origin,Content-Type,Accept,Authorization"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

func resolveDatabaseDSN(raw string) string {
	dsn := strings.TrimSpace(raw)
	if dsn == "" {
		dsn = "./data/aca.db"
	}

	// Keep DSNs with schemes or sqlite URI forms as-is.
	if dsn == ":memory:" || strings.HasPrefix(dsn, "file:") || strings.Contains(dsn, "://") {
		return dsn
	}

	if filepath.IsAbs(dsn) {
		return dsn
	}

	baseDir := resolveProjectBackendDir()
	if baseDir == "" {
		return filepath.Clean(dsn)
	}
	return filepath.Clean(filepath.Join(baseDir, dsn))
}

func resolveProjectBackendDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	if root, ok := findProjectRoot(cwd); ok {
		backendDir := filepath.Join(root, "backend")
		if info, err := os.Stat(backendDir); err == nil && info.IsDir() {
			return backendDir
		}
	}

	return cwd
}

func findProjectRoot(startDir string) (string, bool) {
	dir := startDir
	for {
		backendDir := filepath.Join(dir, "backend")
		if info, err := os.Stat(backendDir); err == nil && info.IsDir() {
			if fileExists(filepath.Join(backendDir, "go.mod")) || fileExists(filepath.Join(backendDir, "main.go")) {
				return dir, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
