package model

import "time"

type Comment struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	TaskID    string    `gorm:"index" json:"task_id"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
