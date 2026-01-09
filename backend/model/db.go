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
	if err := DB.AutoMigrate(
		&User{},
		&Task{},
		&Project{},
		&CLIProfile{},
		&Workflow{},
		&WorkflowTemplate{},
		&WorkflowNode{},
		&WorkflowRun{},
		&AIWorkflowSession{},
		&PromptTemplate{},
		&Comment{},
		&Secret{},
		&SSHServer{},
		&ServerGroup{},
		&TerminalSession{},
		&AISession{},
		&ApprovalRecord{},
		&Log{},
		&AIProviderConfig{},
		&AgentConfig{},
		&RuleSet{},
		&Message{},
	); err != nil {
		return err
	}

	if err := RunMigrations(DB); err != nil {
		return err
	}

	if err := ensureBuiltinWorkflowTemplates(DB); err != nil {
		return err
	}

	return nil
}

// User 用户模型
type User struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"uniqueIndex" json:"username"`
	Email        string     `gorm:"uniqueIndex" json:"email"`
	PasswordHash string     `json:"-"`
	Role         string     `gorm:"default:user" json:"role"`     // admin, user, viewer
	Status       string     `gorm:"default:active" json:"status"` // active, disabled
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

// Task 任务模型
type Task struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	UserID      string     `gorm:"index" json:"user_id"`
	ServerID    *string    `gorm:"index" json:"server_id"`
	Title       string     `gorm:"not null" json:"title"`
	Description string     `json:"description"`
	Status      string     `gorm:"default:todo;index" json:"status"` // todo, in_progress, done, archived
	Priority    int        `gorm:"default:0;index" json:"priority"`  // 0-3
	OrderIndex  float64    `gorm:"index" json:"order_index"`
	RuleSetID   *string    `gorm:"index" json:"rule_set_id"` // 任务关联的规则集
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 自动化任务配置
	WorkDir       string `json:"work_dir"`                            // 工作目录
	CLIType       string `gorm:"default:claude" json:"cli_type"`      // CLI类型: claude, codex, gemini
	InitialPrompt string `json:"initial_prompt"`                      // 初始提示/需求描述
	AutoStart     bool   `gorm:"default:false" json:"auto_start"`     // 是否自动启动
	AutoCreateDir bool   `gorm:"default:true" json:"auto_create_dir"` // 是否自动创建目录

	// AI托管配置
	AIManaged       bool   `gorm:"default:false" json:"ai_managed"` // 是否AI全程托管
	AIPrompt        string `json:"ai_prompt"`                       // AI托管提示词
	AIEndCondition  string `json:"ai_end_condition"`                // AI结束条件
	AIErrorHandling string `json:"ai_error_handling"`               // AI错误处理策略
}

// TerminalSession 终端会话模型
type TerminalSession struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	UserID      string     `gorm:"index" json:"user_id"`
	Title       string     `json:"title"`
	TaskID      *string    `gorm:"index" json:"task_id"`
	Shell       string     `gorm:"default:bash" json:"shell"`
	Status      string     `gorm:"default:running;index" json:"status"` // running, exited, detached
	Hidden      bool       `gorm:"default:false" json:"hidden"`         // 是否在工作台隐藏
	PID         int        `json:"pid"`
	TmuxSession string     `json:"tmux_session"`                    // tmux 会话名称
	RuleMode    string     `gorm:"default:system" json:"rule_mode"` // none, system, task, custom
	RuleSetID   *string    `gorm:"index" json:"rule_set_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	Task        *Task      `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}

// AISession AI会话模型
type AISession struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	TerminalID  string    `gorm:"not null;index" json:"terminal_id"`
	TaskID      *string   `gorm:"index" json:"task_id"`
	AIType      string    `gorm:"not null" json:"ai_type"`      // claude-code, codex, gemini
	State       string    `gorm:"default:unknown" json:"state"` // unknown, waiting_input, working, waiting_approval
	SessionID   string    `json:"session_id"`                   // AI CLI 工具的会话ID（用于 --resume）
	SessionFile string    `json:"session_file"`                 // 会话文件路径
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ApprovalRecord 审批记录模型
type ApprovalRecord struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	TerminalID    string    `gorm:"not null;index" json:"terminal_id"`
	ServerID      *string   `gorm:"index" json:"server_id"`
	AISessionID   *string   `gorm:"index" json:"ai_session_id"`
	PromptType    string    `json:"prompt_type"` // yes_no, permission, other
	PromptContent string    `json:"prompt_content"`
	Response      string    `json:"response"`
	AutoApproved  bool      `gorm:"default:false" json:"auto_approved"`
	RuleMatched   string    `json:"rule_matched"` // 匹配的规则
	AIDecision    string    `json:"ai_decision"`  // AI决策说明
	CreatedAt     time.Time `json:"created_at"`
}

// Log 日志模型
type Log struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	TerminalID *string   `gorm:"index" json:"terminal_id"`
	TaskID     *string   `gorm:"index" json:"task_id"`
	LogType    string    `json:"log_type"` // input, output, system
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// AIProviderConfig AI提供商配置 (OpenAI兼容格式)
type AIProviderConfig struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null" json:"name"` // 配置名称，如 "default", "gpt4", "deepseek"
	Provider    string    `gorm:"not null" json:"provider"`         // openai, anthropic, deepseek, ollama
	BaseURL     string    `json:"base_url"`                         // API基础URL
	APIKey      string    `json:"-"`                                // API密钥，不返回给前端
	Model       string    `gorm:"not null" json:"model"`            // 模型名称
	Temperature float64   `gorm:"default:0.7" json:"temperature"`
	MaxTokens   int       `gorm:"default:2048" json:"max_tokens"`
	IsDefault   bool      `gorm:"default:false" json:"is_default"` // 是否为默认配置
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RuleSet 规则集模型 - 可被系统、任务、终端复用
type RuleSet struct {
	ID   string `gorm:"primaryKey" json:"id"`
	Name string `gorm:"not null" json:"name"`       // 规则集名称
	Type string `gorm:"not null;index" json:"type"` // system, task, terminal

	// 审批模式: manual(手动), auto_yes(全自动yes), smart(AI辅助)
	ApprovalMode string `gorm:"default:manual" json:"approval_mode"`

	// 自动输入设置（auto_yes模式）
	AutoInputType string `gorm:"default:yes" json:"auto_input_type"` // yes, y, enter, option1

	// 规则设置
	WhitelistPatterns string `json:"whitelist_patterns"` // JSON数组，允许通过的模式
	BlacklistPatterns string `json:"blacklist_patterns"` // JSON数组，需要阻止的模式

	// AI辅助设置（smart模式）
	AIProviderID *string `json:"ai_provider_id"`                  // 关联的AI配置
	AIPrompt     string  `json:"ai_prompt"`                       // AI判断提示词
	ContextLines int     `gorm:"default:50" json:"context_lines"` // 发送给AI的上下文行数

	// 检测设置
	DetectClaudeCode bool `gorm:"default:true" json:"detect_claude_code"`
	DetectCodex      bool `gorm:"default:true" json:"detect_codex"`
	DetectGemini     bool `gorm:"default:true" json:"detect_gemini"`

	// 通知设置
	NotifyOnBlock   bool `gorm:"default:true" json:"notify_on_block"`
	NotifyOnApprove bool `gorm:"default:false" json:"notify_on_approve"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message 消息/通知模型
type Message struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	TerminalID  *string    `gorm:"index" json:"terminal_id"`
	TaskID      *string    `gorm:"index" json:"task_id"`
	ServerID    *string    `gorm:"index" json:"server_id"`
	Type        string     `gorm:"not null;index" json:"type"` // approval_needed, blocked, info, warning, error
	Title       string     `gorm:"not null" json:"title"`
	Content     string     `json:"content"`
	Context     string     `json:"context"`                            // 相关上下文（终端输出等）
	Status      string     `gorm:"default:unread;index" json:"status"` // unread, read, handled, dismissed
	ActionTaken string     `json:"action_taken"`                       // 用户采取的操作
	Priority    int        `gorm:"default:0" json:"priority"`          // 0=normal, 1=high, 2=urgent
	ExpiresAt   *time.Time `json:"expires_at"`                         // 过期时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ReadAt      *time.Time `json:"read_at"`
	HandledAt   *time.Time `json:"handled_at"`
}

// AIWorkflowSession AI驱动的工作流会话
type AIWorkflowSession struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	WorkflowID  string     `gorm:"index" json:"workflow_id"`
	UserGoal    string     `gorm:"type:text" json:"user_goal"`
	Status      string     `gorm:"default:running;index" json:"status"` // running, completed, failed, paused
	Messages    string     `gorm:"type:text" json:"messages"`           // JSON array of chat messages
	Steps       string     `gorm:"type:text" json:"steps"`              // JSON array of workflow steps
	Context     string     `gorm:"type:text" json:"context"`            // JSON object of context
	Summary     string     `json:"summary"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}
