package model

import "time"

const (
	ProjectTypeLocal  = "local"
	ProjectTypeRemote = "remote"
	ProjectTypeGit    = "git"
)

type Project struct {
	ID          string `gorm:"primaryKey" json:"id"`
	UserID      string `gorm:"index" json:"user_id"` // 所属用户
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Remark      string `json:"remark"`

	Type string `gorm:"not null;default:local;index" json:"type"` // local, remote, git

	GroupID *string `gorm:"index" json:"group_id"`

	LocalPath  string  `json:"local_path"`
	ServerID   *string `gorm:"index" json:"server_id"`
	RemotePath string  `json:"remote_path"`

	GitRepo   string `json:"git_repo"`
	GitBranch string `json:"git_branch"`

	EnvVars StringMap `gorm:"type:text" json:"env_vars"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectGroup 项目集/项目组（Portfolio/Group）
type ProjectGroup struct {
	ID          string  `gorm:"primaryKey" json:"id"`
	UserID      string  `gorm:"index" json:"user_id"` // 所属用户
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Remark      string  `json:"remark"`
	ParentID    *string `gorm:"index" json:"parent_id"`
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
