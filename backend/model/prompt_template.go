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
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
