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
