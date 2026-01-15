package model

import "time"

// PromptTemplate stores system-level prompt templates used by ACA.
// Templates are rendered by backend/service/prompt with dynamic variables.
type PromptTemplate struct {
	Key         string      `gorm:"primaryKey" json:"key"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Template    string      `gorm:"type:text" json:"template"`
	Variables   StringArray `gorm:"type:text" json:"variables"`
	// ActivePresetID indicates which preset is currently applied to this template.
	// Empty means the template is manually edited or reset without tracking.
	ActivePresetID string    `gorm:"index" json:"active_preset_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
