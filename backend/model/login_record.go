package model

import "time"

// LoginRecord 登录记录（用于审计）
type LoginRecord struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	UserID     *string   `gorm:"index" json:"user_id"`
	Identifier string    `gorm:"index" json:"identifier"` // 用户输入：username/email
	Username   string    `gorm:"index" json:"username"`   // 归一化后的用户名（成功时）
	Success    bool      `gorm:"index" json:"success"`
	Error      string    `json:"error"` // 失败原因（不包含敏感信息）
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
}
