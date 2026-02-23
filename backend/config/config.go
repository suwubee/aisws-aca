package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
	fileEnv := loadDataEnv()

	return &Config{
		App: AppConfig{
			DemoMode: getEnvBool(fileEnv, "DEMO_MODE", false),
		},
		Server: ServerConfig{
			Host: getEnv(fileEnv, "SERVER_HOST", "0.0.0.0"),
			Port: getEnv(fileEnv, "SERVER_PORT", "34007"),
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    resolveDatabaseDSN(getEnv(fileEnv, "DATABASE_DSN", "")),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv(fileEnv, "JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiration: 24 * time.Hour,
			Username:      getEnv(fileEnv, "AUTH_USERNAME", "admin"),
			Password:      getEnv(fileEnv, "AUTH_PASSWORD", "admin123"),
		},
		Terminal: TerminalConfig{
			DefaultShell:    getEnv(fileEnv, "TERMINAL_SHELL", "/bin/bash"),
			DefaultLoginDir: getEnv(fileEnv, "TERMINAL_DEFAULT_LOGIN_DIR", "~/"),
			ScrollbackBytes: 256 * 1024, // 256KB
			IdleTimeout:     10 * time.Minute,
			MaxSessions:     100,
		},
		Log: LogConfig{
			Level: getEnv(fileEnv, "LOG_LEVEL", "info"),
			File:  getEnv(fileEnv, "LOG_FILE", ""),
		},
		CORS: CORSConfig{
			AllowOrigins: getEnv(fileEnv, "CORS_ALLOW_ORIGINS", "*"),
			AllowMethods: getEnv(fileEnv, "CORS_ALLOW_METHODS", "GET,POST,PUT,DELETE,OPTIONS"),
			AllowHeaders: getEnv(fileEnv, "CORS_ALLOW_HEADERS", "Origin,Content-Type,Accept,Authorization"),
		},
	}
}

func getEnv(fileEnv map[string]string, key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if fileEnv != nil {
		if value := strings.TrimSpace(fileEnv[key]); value != "" {
			return value
		}
	}
	return defaultValue
}

func getEnvBool(fileEnv map[string]string, key string, defaultValue bool) bool {
	value := strings.TrimSpace(getEnv(fileEnv, key, ""))
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

	baseDir := resolveRuntimeDir()
	if baseDir == "" {
		return filepath.Clean(dsn)
	}
	return filepath.Clean(filepath.Join(baseDir, dsn))
}

func resolveRuntimeDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func loadDataEnv() map[string]string {
	envPath := dataEnvPath()
	if envPath == "" {
		return parseDotEnv(defaultDataEnvContent)
	}

	dataDir := filepath.Dir(envPath)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return parseDotEnv(defaultDataEnvContent)
	}

	if _, err := os.Stat(envPath); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(envPath, []byte(defaultDataEnvContent), 0o600)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		return parseDotEnv(defaultDataEnvContent)
	}

	values := parseDotEnv(string(content))
	defaults := parseDotEnv(defaultDataEnvContent)
	for key, value := range defaults {
		if strings.TrimSpace(values[key]) == "" {
			values[key] = value
		}
	}
	return values
}

func dataEnvPath() string {
	runtimeDir := resolveRuntimeDir()
	if runtimeDir == "" {
		return ""
	}
	return filepath.Join(runtimeDir, "data", ".env")
}

func parseDotEnv(content string) map[string]string {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				if unquoted, err := strconv.Unquote(value); err == nil {
					value = unquoted
				} else {
					value = value[1 : len(value)-1]
				}
			}
		}
		values[key] = value
	}
	return values
}

const defaultDataEnvContent = `# ACA runtime config (auto-created)
SERVER_HOST=0.0.0.0
SERVER_PORT=34007
DATABASE_DSN=./data/aca.db
AUTH_USERNAME=admin
AUTH_PASSWORD=admin123
JWT_SECRET=your-secret-key-change-in-production
DEMO_MODE=false
TERMINAL_SHELL=/bin/bash
TERMINAL_DEFAULT_LOGIN_DIR=~/
LOG_LEVEL=info
LOG_FILE=
CORS_ALLOW_ORIGINS=*
CORS_ALLOW_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOW_HEADERS=Origin,Content-Type,Accept,Authorization
`
