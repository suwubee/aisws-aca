package model

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB    *gorm.DB // 主数据库 (SQLite 或 PostgreSQL)
	LogDB *gorm.DB // 日志数据库 (可选，PostgreSQL)
)

type DBConfig struct {
	Type string
	DSN  string
}

func bootstrapMainDatabase(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("missing database connection")
	}

	// 自动迁移
	if err := db.AutoMigrate(
		&User{},
		&LoginRecord{},
		&Task{},
		&Project{},
		&ProjectGroup{},
		&CLIProfile{},
		&Workflow{},
		&WorkflowTemplate{},
		&WorkflowNode{},
		&WorkflowRun{},
		&AIWorkflowSession{},
		&PromptTemplate{},
		&PromptTemplatePreset{},
		&KeyBinding{},
		&ScheduledJob{},
		&ScheduledJobRun{},
		&Comment{},
		&AppSetting{},
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

	if err := RunMigrations(db); err != nil {
		return err
	}

	if err := ensureBuiltinWorkflowTemplates(db); err != nil {
		return err
	}

	return nil
}

// InitDatabase 初始化主数据库
func InitDatabase(cfg DBConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Type {
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.DSN)
	default: // sqlite
		dialector = sqlite.Open(cfg.DSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := bootstrapMainDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return db, nil
}

// InitLogDatabase 初始化日志数据库（PostgreSQL）
func InitLogDatabase(cfg DBConfig) (*gorm.DB, error) {
	if cfg.Type != "postgres" && cfg.Type != "postgresql" {
		return nil, fmt.Errorf("log database only supports PostgreSQL")
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect log database: %w", err)
	}

	if err := db.AutoMigrate(&Log{}); err != nil {
		return nil, fmt.Errorf("failed to initialize log database schema: %w", err)
	}

	return db, nil
}
