package config

import (
	"os"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Terminal TerminalConfig
	Log      LogConfig
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
	ScrollbackBytes int
	IdleTimeout     time.Duration
	MaxSessions     int
}

type LogConfig struct {
	Level string
	File  string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "3007"),
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    getEnv("DATABASE_DSN", "./data/aca.db"),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiration: 24 * time.Hour,
			Username:      getEnv("AUTH_USERNAME", "admin"),
			Password:      getEnv("AUTH_PASSWORD", "admin123"),
		},
		Terminal: TerminalConfig{
			DefaultShell:    getEnv("TERMINAL_SHELL", "/bin/bash"),
			ScrollbackBytes: 256 * 1024, // 256KB
			IdleTimeout:     10 * time.Minute,
			MaxSessions:     100,
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
			File:  getEnv("LOG_FILE", ""),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
