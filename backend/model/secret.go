package model

import "time"

type Secret struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"` // ssh_password, ssh_key, api_key
	Ciphertext string    `json:"-"`    // AES-GCM加密后base64
	Meta       string    `json:"meta"` // JSON: 指纹/用途等
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
