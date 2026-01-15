package appsetting

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	KeyTerminalDefaultLoginDir = "terminal.default_login_dir"
)

func defaultTerminalDefaultLoginDir() string {
	value := strings.TrimSpace(os.Getenv("TERMINAL_DEFAULT_LOGIN_DIR"))
	if value == "" {
		return "~/"
	}
	return value
}

func expandHomeDir(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}

	expanded := os.ExpandEnv(raw)
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			if expanded == "~" {
				expanded = home
			} else {
				expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
			}
		}
	}

	return filepath.Clean(expanded)
}

func SupportedKeys() []string {
	return []string{
		KeyTerminalDefaultLoginDir,
	}
}

func EnsureDefaults() error {
	if model.DB == nil {
		return errors.New("database not initialized")
	}

	now := time.Now()
	defaultLoginDir := defaultTerminalDefaultLoginDir()
	if resolved := expandHomeDir(defaultLoginDir); strings.TrimSpace(resolved) != "" {
		if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
			defaultLoginDir = "~/"
		}
	}
	defaults := []model.AppSetting{
		{
			Key:       KeyTerminalDefaultLoginDir,
			Value:     defaultLoginDir,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, d := range defaults {
		var existing model.AppSetting
		err := model.DB.First(&existing, "key = ?", d.Key).Error
		if err == nil {
			if strings.TrimSpace(existing.Value) != "" {
				continue
			}
			if err := model.DB.Model(&model.AppSetting{}).Where("key = ?", d.Key).Updates(map[string]any{
				"value":      d.Value,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := model.DB.Create(&d).Error; err != nil {
			return err
		}
	}

	return nil
}

func Get(key string) (string, error) {
	if model.DB == nil {
		return "", errors.New("database not initialized")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return "", errors.New("setting key is required")
	}
	if err := EnsureDefaults(); err != nil {
		return "", err
	}

	var item model.AppSetting
	if err := model.DB.First(&item, "key = ?", k).Error; err != nil {
		return "", err
	}
	return strings.TrimSpace(item.Value), nil
}

func Upsert(key string, value string) (string, error) {
	if model.DB == nil {
		return "", errors.New("database not initialized")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return "", errors.New("setting key is required")
	}

	v := strings.TrimSpace(value)
	now := time.Now()
	item := model.AppSetting{
		Key:       k,
		Value:     v,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := model.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&item).Error; err != nil {
		return "", err
	}
	return v, nil
}

func GetTerminalDefaultLoginDir() (string, error) {
	return Get(KeyTerminalDefaultLoginDir)
}

func UpdateTerminalDefaultLoginDir(value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		v = defaultTerminalDefaultLoginDir()
	}

	resolved := expandHomeDir(v)
	if strings.TrimSpace(resolved) != "" {
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return "", errors.New("directory does not exist")
		}
	}

	return Upsert(KeyTerminalDefaultLoginDir, v)
}
