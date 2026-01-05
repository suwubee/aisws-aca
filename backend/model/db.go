package model

import (
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(dsn string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	// 自动迁移
	return DB.AutoMigrate(
		&User{},
		&Task{},
		&TerminalSession{},
		&AISession{},
		&ApprovalRecord{},
		&Log{},
	)
}

// User 用户模型
type User struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Task 任务模型
type Task struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"not null" json:"title"`
	Description string     `json:"description"`
	Status      string     `gorm:"default:todo;index" json:"status"` // todo, in_progress, done, archived
	Priority    int        `gorm:"default:0;index" json:"priority"`  // 0-3
	OrderIndex  float64    `gorm:"index" json:"order_index"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// TerminalSession 终端会话模型
type TerminalSession struct {
	ID        string     `gorm:"primaryKey" json:"id"`
	Title     string     `json:"title"`
	TaskID    *string    `gorm:"index" json:"task_id"`
	Shell     string     `gorm:"default:bash" json:"shell"`
	Status    string     `gorm:"default:running;index" json:"status"` // running, exited
	PID       int        `json:"pid"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	Task      *Task      `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}

// AISession AI会话模型
type AISession struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	TerminalID  string    `gorm:"not null;index" json:"terminal_id"`
	TaskID      *string   `gorm:"index" json:"task_id"`
	AIType      string    `gorm:"not null" json:"ai_type"` // claude-code, codex, gemini
	State       string    `gorm:"default:unknown" json:"state"` // unknown, waiting_input, working, waiting_approval
	SessionFile string    `json:"session_file"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ApprovalRecord 审批记录模型
type ApprovalRecord struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	TerminalID    string    `gorm:"not null;index" json:"terminal_id"`
	AISessionID   *string   `gorm:"index" json:"ai_session_id"`
	PromptType    string    `json:"prompt_type"` // yes_no, permission, other
	PromptContent string    `json:"prompt_content"`
	Response      string    `json:"response"`
	AutoApproved  bool      `gorm:"default:false" json:"auto_approved"`
	CreatedAt     time.Time `json:"created_at"`
}

// Log 日志模型
type Log struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	TerminalID *string   `gorm:"index" json:"terminal_id"`
	TaskID     *string   `gorm:"index" json:"task_id"`
	LogType    string    `json:"log_type"` // terminal_output, operation, system
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}
