package model

import "time"

// KeyBinding defines a reusable terminal input preset (single key or short macro)
// shared by UI shortcuts and automation.
//
// PtyInput stores an escaped string (e.g. "\r", "y\r", "\x03") so it is safe to
// view/edit in JSON/UI.
type KeyBinding struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	PtyInput    string    `gorm:"type:text" json:"pty_input"` // escaped string, e.g. "\r"
	TmuxKeys    string    `gorm:"type:text" json:"tmux_keys"` // tmux send-keys key, e.g. "C-m"
	TmuxLiteral bool      `json:"tmux_literal"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
