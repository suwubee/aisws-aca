package model

import "time"

// AppSetting stores system-level key/value configurations.
// It is intentionally generic so the UI can evolve without recompiling config files.
type AppSetting struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AppSetting) TableName() string {
	return "app_settings"
}

