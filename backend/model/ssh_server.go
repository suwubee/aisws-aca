package model

import (
	"time"

	"gorm.io/gorm"
)

// SSHServer SSH服务器配置
type SSHServer struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	Name       string    `json:"name"`
	Host       string    `json:"host"`
	Port       int       `gorm:"default:22" json:"port"`
	Username   string    `json:"username"`
	AuthType   string    `json:"auth_type"` // password, key
	Password   string    `json:"-"`         // 加密存储
	PrivateKey string    `json:"-"`         // 加密存储
	Passphrase string    `json:"-"`         // 加密存储
	GroupID    *string   `gorm:"index" json:"group_id"`
	Tags       string    `json:"tags"` // JSON数组
	LastStatus string    `json:"last_status"`
	CreatedAt  time.Time `json:"created_at"`
}

// ServerGroup 服务器分组
type ServerGroup struct {
	ID          string  `gorm:"primaryKey" json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ParentID    *string `gorm:"index" json:"parent_id"` // 支持嵌套分组
}

// UserServerShare 用户服务器共享关系
type UserServerShare struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"index;not null" json:"user_id"`
	ServerID  string    `gorm:"index;not null" json:"server_id"`
	CreatedAt time.Time `json:"created_at"`
}

func AutoMigrateSSHServer(db *gorm.DB) error {
	return db.AutoMigrate(&SSHServer{}, &ServerGroup{}, &UserServerShare{})
}
