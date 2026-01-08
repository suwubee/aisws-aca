package model

import "time"

const (
	ProjectTypeLocal  = "local"
	ProjectTypeRemote = "remote"
	ProjectTypeGit    = "git"
)

type Project struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`

	Type string `gorm:"not null;default:local;index" json:"type"` // local, remote, git

	LocalPath  string  `json:"local_path"`
	ServerID   *string `gorm:"index" json:"server_id"`
	RemotePath string  `json:"remote_path"`

	GitRepo   string `json:"git_repo"`
	GitBranch string `json:"git_branch"`

	EnvVars StringMap `gorm:"type:text" json:"env_vars"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	CLIProfileTypeClaude = "claude"
	CLIProfileTypeCodex  = "codex"
	CLIProfileTypeGemini = "gemini"
)

type CLIProfile struct {
	ID   string `gorm:"primaryKey" json:"id"`
	Name string `gorm:"not null" json:"name"`
	Type string `gorm:"not null;default:claude;index" json:"type"` // claude, codex, gemini

	Command     string      `json:"command"`
	DefaultArgs StringArray `gorm:"type:text" json:"default_args"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CLIProfile) TableName() string {
	return "cli_profiles"
}
