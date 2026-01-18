package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type SchemaMigration struct {
	ID        string    `gorm:"primaryKey"`
	AppliedAt time.Time `gorm:"not null"`
}

type migration struct {
	id string
	up func(db *gorm.DB) error
}

func RunMigrations(db *gorm.DB) error {
	if db == nil {
		return errors.New("missing database connection")
	}

	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return err
	}

	migrations := []migration{
		{
			id: "20260107_add_tasks_server_id",
			up: migrateAddTaskServerID,
		},
		{
			id: "20260109_add_tasks_project_id",
			up: migrateAddTaskProjectID,
		},
		{
			id: "20260113_add_tasks_agent_session_id",
			up: migrateAddTaskAgentSessionID,
		},
		{
			id: "20260113_add_terminal_sessions_server_id",
			up: migrateAddTerminalSessionServerID,
		},
		{
			id: "20260107_add_approval_records_server_id",
			up: migrateAddApprovalRecordServerID,
		},
		{
			id: "20260107_add_messages_server_id",
			up: migrateAddMessageServerID,
		},
		{
			id: "20260109_add_projects_group_id",
			up: migrateAddProjectGroupID,
		},
		{
			id: "20260114_add_remark_fields",
			up: migrateAddRemarkFields,
		},
		{
			id: "20260116_add_task_ai_binding_fields",
			up: migrateAddTaskAIBindingFields,
		},
		{
			id: "20260116_fix_task_ai_column_names",
			up: migrateFixTaskAIColumnNames,
		},
		{
			id: "20260116_add_terminal_connection_fields",
			up: migrateAddTerminalConnectionFields,
		},
		{
			id: "20260116_add_user_server_shares_table",
			up: migrateAddUserServerSharesTable,
		},
		{
			id: "20260117_add_multi_user_isolation_fields",
			up: migrateAddMultiUserIsolationFields,
		},
	}

	for _, m := range migrations {
		var existing SchemaMigration
		tx := db.Limit(1).Find(&existing, "id = ?", m.id)
		if tx.Error != nil {
			return tx.Error
		}
		if tx.RowsAffected > 0 {
			continue
		}

		if err := m.up(db); err != nil {
			return err
		}

		if err := db.Create(&SchemaMigration{ID: m.id, AppliedAt: time.Now()}).Error; err != nil {
			return err
		}
	}

	return nil
}

func migrateAddTaskServerID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Task{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&Task{}, "ServerID") {
		if err := db.Migrator().AddColumn(&Task{}, "ServerID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_server_id ON tasks(server_id)").Error
}

func migrateAddTaskProjectID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Task{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&Task{}, "ProjectID") {
		if err := db.Migrator().AddColumn(&Task{}, "ProjectID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id)").Error
}

func migrateAddTaskAgentSessionID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Task{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&Task{}, "AgentSessionID") {
		if err := db.Migrator().AddColumn(&Task{}, "AgentSessionID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_agent_session_id ON tasks(agent_session_id)").Error
}

func migrateAddTerminalSessionServerID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&TerminalSession{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&TerminalSession{}, "ServerID") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "ServerID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_terminal_sessions_server_id ON terminal_sessions(server_id)").Error
}

func migrateAddApprovalRecordServerID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&ApprovalRecord{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&ApprovalRecord{}, "ServerID") {
		if err := db.Migrator().AddColumn(&ApprovalRecord{}, "ServerID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_approval_records_server_id ON approval_records(server_id)").Error
}

func migrateAddMessageServerID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Message{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&Message{}, "ServerID") {
		if err := db.Migrator().AddColumn(&Message{}, "ServerID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_server_id ON messages(server_id)").Error
}

func migrateAddProjectGroupID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Project{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&Project{}, "GroupID") {
		if err := db.Migrator().AddColumn(&Project{}, "GroupID"); err != nil {
			return err
		}
	}

	return db.Exec("CREATE INDEX IF NOT EXISTS idx_projects_group_id ON projects(group_id)").Error
}

func migrateAddRemarkFields(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	if db.Migrator().HasTable(&Task{}) && !db.Migrator().HasColumn(&Task{}, "Remark") {
		if err := db.Migrator().AddColumn(&Task{}, "Remark"); err != nil {
			return err
		}
	}

	if db.Migrator().HasTable(&Project{}) && !db.Migrator().HasColumn(&Project{}, "Remark") {
		if err := db.Migrator().AddColumn(&Project{}, "Remark"); err != nil {
			return err
		}
	}

	if db.Migrator().HasTable(&ProjectGroup{}) && !db.Migrator().HasColumn(&ProjectGroup{}, "Remark") {
		if err := db.Migrator().AddColumn(&ProjectGroup{}, "Remark"); err != nil {
			return err
		}
	}

	return nil
}

// migrateAddTaskAIBindingFields 添加任务AI绑定相关字段
func migrateAddTaskAIBindingFields(db *gorm.DB) error {
	if !db.Migrator().HasTable(&Task{}) {
		return nil
	}

	// ActiveTerminalID
	if !db.Migrator().HasColumn(&Task{}, "ActiveTerminalID") {
		if err := db.Migrator().AddColumn(&Task{}, "ActiveTerminalID"); err != nil {
			return err
		}
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_active_terminal_id ON tasks(active_terminal_id)").Error; err != nil {
			return err
		}
	}

	// AIStatus
	if !db.Migrator().HasColumn(&Task{}, "AIStatus") {
		if err := db.Migrator().AddColumn(&Task{}, "AIStatus"); err != nil {
			return err
		}
	}

	// AIPauseReason
	if !db.Migrator().HasColumn(&Task{}, "AIPauseReason") {
		if err := db.Migrator().AddColumn(&Task{}, "AIPauseReason"); err != nil {
			return err
		}
	}

	// ExpectDisconnect
	if !db.Migrator().HasColumn(&Task{}, "ExpectDisconnect") {
		if err := db.Migrator().AddColumn(&Task{}, "ExpectDisconnect"); err != nil {
			return err
		}
	}

	// ReconnectAttempts
	if !db.Migrator().HasColumn(&Task{}, "ReconnectAttempts") {
		if err := db.Migrator().AddColumn(&Task{}, "ReconnectAttempts"); err != nil {
			return err
		}
	}

	// LastReconnectAt
	if !db.Migrator().HasColumn(&Task{}, "LastReconnectAt") {
		if err := db.Migrator().AddColumn(&Task{}, "LastReconnectAt"); err != nil {
			return err
		}
	}

	return nil
}

// migrateFixTaskAIColumnNames 修复早期版本中 AI* 字段的列名（a_iprompt/a_ipause_reason）
// 备注：AutoMigrate 已会创建新列，这里负责把旧列数据拷贝到新列，避免丢失。
func migrateFixTaskAIColumnNames(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&Task{}) {
		return nil
	}

	// a_iprompt -> ai_prompt
	if db.Migrator().HasColumn("tasks", "a_iprompt") && db.Migrator().HasColumn("tasks", "ai_prompt") {
		if err := db.Exec("UPDATE tasks SET ai_prompt = a_iprompt WHERE (ai_prompt IS NULL OR ai_prompt = '') AND a_iprompt IS NOT NULL AND a_iprompt != ''").Error; err != nil {
			return err
		}
	}

	// a_ipause_reason -> ai_pause_reason
	if db.Migrator().HasColumn("tasks", "a_ipause_reason") && db.Migrator().HasColumn("tasks", "ai_pause_reason") {
		if err := db.Exec("UPDATE tasks SET ai_pause_reason = a_ipause_reason WHERE (ai_pause_reason IS NULL OR ai_pause_reason = '') AND a_ipause_reason IS NOT NULL AND a_ipause_reason != ''").Error; err != nil {
			return err
		}
	}

	return nil
}

// migrateAddTerminalConnectionFields 添加终端连接状态相关字段
func migrateAddTerminalConnectionFields(db *gorm.DB) error {
	if !db.Migrator().HasTable(&TerminalSession{}) {
		return nil
	}

	// ConnectionStatus
	if !db.Migrator().HasColumn(&TerminalSession{}, "ConnectionStatus") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "ConnectionStatus"); err != nil {
			return err
		}
	}

	// AutoReconnect
	if !db.Migrator().HasColumn(&TerminalSession{}, "AutoReconnect") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "AutoReconnect"); err != nil {
			return err
		}
	}

	// LastDisconnectAt
	if !db.Migrator().HasColumn(&TerminalSession{}, "LastDisconnectAt") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "LastDisconnectAt"); err != nil {
			return err
		}
	}

	// CloseReason
	if !db.Migrator().HasColumn(&TerminalSession{}, "CloseReason") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "CloseReason"); err != nil {
			return err
		}
	}

	// ReplacedByTerminalID
	if !db.Migrator().HasColumn(&TerminalSession{}, "ReplacedByTerminalID") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "ReplacedByTerminalID"); err != nil {
			return err
		}
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_terminal_sessions_replaced_by ON terminal_sessions(replaced_by_terminal_id)").Error; err != nil {
			return err
		}
	}

	// LastWorkDir
	if !db.Migrator().HasColumn(&TerminalSession{}, "LastWorkDir") {
		if err := db.Migrator().AddColumn(&TerminalSession{}, "LastWorkDir"); err != nil {
			return err
		}
	}

	return nil
}

// migrateAddUserServerSharesTable 创建用户服务器共享表
func migrateAddUserServerSharesTable(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	// 使用 AutoMigrate 创建表
	if err := db.AutoMigrate(&UserServerShare{}); err != nil {
		return err
	}

	// 创建复合唯一索引防止重复共享
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_server_shares_unique ON user_server_shares(user_id, server_id)").Error
}

// migrateAddMultiUserIsolationFields 为多用户隔离添加 user_id 字段
func migrateAddMultiUserIsolationFields(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	// 获取第一个管理员用户作为默认用户
	var adminUser User
	if err := db.Where("role = ?", "admin").First(&adminUser).Error; err != nil {
		// 如果没有管理员，尝试获取任意用户
		if err := db.First(&adminUser).Error; err != nil {
			// 没有用户则跳过迁移
			return nil
		}
	}
	defaultUserID := adminUser.ID

	// SSHServer 添加 user_id
	if db.Migrator().HasTable("ssh_servers") {
		if !db.Migrator().HasColumn(&SSHServer{}, "UserID") {
			if err := db.Exec("ALTER TABLE ssh_servers ADD COLUMN user_id TEXT").Error; err != nil {
				return err
			}
		}
		// 更新现有记录
		db.Exec("UPDATE ssh_servers SET user_id = ? WHERE user_id IS NULL OR user_id = ''", defaultUserID)
		db.Exec("CREATE INDEX IF NOT EXISTS idx_ssh_servers_user_id ON ssh_servers(user_id)")
	}

	// ServerGroup 添加 user_id
	if db.Migrator().HasTable("server_groups") {
		if !db.Migrator().HasColumn(&ServerGroup{}, "UserID") {
			if err := db.Exec("ALTER TABLE server_groups ADD COLUMN user_id TEXT").Error; err != nil {
				return err
			}
		}
		db.Exec("UPDATE server_groups SET user_id = ? WHERE user_id IS NULL OR user_id = ''", defaultUserID)
		db.Exec("CREATE INDEX IF NOT EXISTS idx_server_groups_user_id ON server_groups(user_id)")
	}

	// Project 添加 user_id
	if db.Migrator().HasTable("projects") {
		if !db.Migrator().HasColumn(&Project{}, "UserID") {
			if err := db.Exec("ALTER TABLE projects ADD COLUMN user_id TEXT").Error; err != nil {
				return err
			}
		}
		db.Exec("UPDATE projects SET user_id = ? WHERE user_id IS NULL OR user_id = ''", defaultUserID)
		db.Exec("CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id)")
	}

	// ProjectGroup 添加 user_id
	if db.Migrator().HasTable("project_groups") {
		if !db.Migrator().HasColumn(&ProjectGroup{}, "UserID") {
			if err := db.Exec("ALTER TABLE project_groups ADD COLUMN user_id TEXT").Error; err != nil {
				return err
			}
		}
		db.Exec("UPDATE project_groups SET user_id = ? WHERE user_id IS NULL OR user_id = ''", defaultUserID)
		db.Exec("CREATE INDEX IF NOT EXISTS idx_project_groups_user_id ON project_groups(user_id)")
	}

	// AIProviderConfig 添加 user_id
	if db.Migrator().HasTable("ai_provider_configs") {
		if !db.Migrator().HasColumn(&AIProviderConfig{}, "UserID") {
			if err := db.Exec("ALTER TABLE ai_provider_configs ADD COLUMN user_id TEXT").Error; err != nil {
				return err
			}
		}
		db.Exec("UPDATE ai_provider_configs SET user_id = ? WHERE user_id IS NULL OR user_id = ''", defaultUserID)
		db.Exec("CREATE INDEX IF NOT EXISTS idx_ai_provider_configs_user_id ON ai_provider_configs(user_id)")
	}

	// RuleSet 添加 user_id
	if db.Migrator().HasTable("rule_sets") {
		if !db.Migrator().HasColumn(&RuleSet{}, "UserID") {
			if err := db.Exec("ALTER TABLE rule_sets ADD COLUMN user_id TEXT").Error; err != nil {
				return err
			}
		}
		db.Exec("UPDATE rule_sets SET user_id = ? WHERE user_id IS NULL OR user_id = ''", defaultUserID)
		db.Exec("CREATE INDEX IF NOT EXISTS idx_rule_sets_user_id ON rule_sets(user_id)")
	}

	return nil
}
