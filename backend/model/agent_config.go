package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// StringArray stores a string slice as JSON in the database.
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	data, err := json.Marshal([]string(a))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (a *StringArray) Scan(value interface{}) error {
	if a == nil {
		return fmt.Errorf("StringArray: Scan on nil pointer")
	}

	if value == nil {
		*a = StringArray{}
		return nil
	}

	var raw string
	switch v := value.(type) {
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		return fmt.Errorf("StringArray: unsupported Scan type %T", value)
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		*a = StringArray{}
		return nil
	}

	var decoded []string
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		*a = StringArray(decoded)
		return nil
	}

	// Backward/invalid data compatibility: fall back to comma-separated list.
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	*a = StringArray(result)
	return nil
}

// AgentConfig AI代理检测配置
type AgentConfig struct {
	AgentType   string      `gorm:"primaryKey" json:"agent_type"`
	DisplayName string      `json:"display_name"`
	Enabled     bool        `gorm:"default:true" json:"enabled"`
	Priority    int         `gorm:"default:0" json:"priority"`
	DetectModes StringArray `gorm:"type:text" json:"detect_modes"`
}
