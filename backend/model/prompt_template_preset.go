package model

import "time"

// PromptTemplatePreset stores named prompt template versions that can be applied to a PromptTemplate key.
// Presets are managed from Settings and can be applied as the active template without hardcoding prompts.
type PromptTemplatePreset struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"index;not null;uniqueIndex:idx_prompt_template_preset_key_name" json:"key"`
	Name        string    `gorm:"not null;uniqueIndex:idx_prompt_template_preset_key_name" json:"name"`
	Description string    `json:"description"`
	Template    string    `gorm:"type:text" json:"template"`
	IsBuiltin   bool      `gorm:"default:false" json:"is_builtin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
