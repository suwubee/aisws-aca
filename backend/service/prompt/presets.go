package prompt

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const defaultPresetName = "默认"

func ListPresets(key string) ([]model.PromptTemplatePreset, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}

	k := strings.TrimSpace(key)
	if k == "" {
		return nil, errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return nil, errors.New("unknown template key")
	}

	if err := EnsureDefaults(); err != nil {
		return nil, err
	}

	var items []model.PromptTemplatePreset
	if err := model.DB.Where("key = ?", k).Find(&items).Error; err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]
		if a.IsBuiltin != b.IsBuiltin {
			return a.IsBuiltin
		}
		return strings.ToLower(strings.TrimSpace(a.Name)) < strings.ToLower(strings.TrimSpace(b.Name))
	})

	return items, nil
}

type CreatePresetRequest struct {
	Name        string
	Description string
	Template    string
}

func CreatePreset(key string, req CreatePresetRequest) (*model.PromptTemplatePreset, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}

	k := strings.TrimSpace(key)
	if k == "" {
		return nil, errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return nil, errors.New("unknown template key")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("preset name is required")
	}
	if name == defaultPresetName {
		return nil, errors.New("preset name is reserved")
	}

	text := req.Template
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("template content is required")
	}
	if err := ValidateTemplate(k, text); err != nil {
		return nil, err
	}

	if err := EnsureDefaults(); err != nil {
		return nil, err
	}

	var exists model.PromptTemplatePreset
	if err := model.DB.First(&exists, "key = ? AND name = ?", k, name).Error; err == nil {
		return nil, errors.New("preset name already exists")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()
	item := &model.PromptTemplatePreset{
		ID:          uuid.NewString(),
		Key:         k,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Template:    text,
		IsBuiltin:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := model.DB.Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func DeletePreset(key string, presetID string) error {
	if model.DB == nil {
		return errors.New("database not initialized")
	}

	k := strings.TrimSpace(key)
	if k == "" {
		return errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return errors.New("unknown template key")
	}

	id := strings.TrimSpace(presetID)
	if id == "" {
		return errors.New("preset id is required")
	}

	if err := EnsureDefaults(); err != nil {
		return err
	}

	return model.DB.Transaction(func(tx *gorm.DB) error {
		var preset model.PromptTemplatePreset
		if err := tx.First(&preset, "id = ? AND key = ?", id, k).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("preset not found")
			}
			return err
		}
		if preset.IsBuiltin {
			return errors.New("builtin preset cannot be deleted")
		}

		if err := tx.Delete(&model.PromptTemplatePreset{}, "id = ?", preset.ID).Error; err != nil {
			return err
		}

		// If this preset is currently applied, clear the active preset id.
		if err := tx.Model(&model.PromptTemplate{}).
			Where("key = ? AND active_preset_id = ?", k, preset.ID).
			Updates(map[string]any{
				"active_preset_id": "",
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		return nil
	})
}

func ApplyPreset(key string, presetID string) (*model.PromptTemplate, *model.PromptTemplatePreset, error) {
	if model.DB == nil {
		return nil, nil, errors.New("database not initialized")
	}

	k := strings.TrimSpace(key)
	if k == "" {
		return nil, nil, errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return nil, nil, errors.New("unknown template key")
	}

	id := strings.TrimSpace(presetID)
	if id == "" {
		return nil, nil, errors.New("preset id is required")
	}

	if err := EnsureDefaults(); err != nil {
		return nil, nil, err
	}

	var preset model.PromptTemplatePreset
	if err := model.DB.First(&preset, "id = ? AND key = ?", id, k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("preset not found")
		}
		return nil, nil, err
	}

	if err := ValidateTemplate(k, preset.Template); err != nil {
		return nil, nil, err
	}

	item, err := GetTemplate(k)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	if err := model.DB.Model(&model.PromptTemplate{}).
		Where("key = ?", item.Key).
		Updates(map[string]any{
			"template":         preset.Template,
			"active_preset_id": preset.ID,
			"updated_at":       now,
		}).Error; err != nil {
		return nil, nil, err
	}

	item.Template = preset.Template
	item.ActivePresetID = preset.ID
	item.UpdatedAt = now
	return item, &preset, nil
}

func ensureDefaultPreset(db *gorm.DB, key string) (*model.PromptTemplatePreset, error) {
	if db == nil {
		return nil, errors.New("missing database connection")
	}

	k := strings.TrimSpace(key)
	if k == "" {
		return nil, errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return nil, errors.New("unknown template key")
	}

	defaultText, err := readDefaultTemplateText(k)
	if err != nil {
		return nil, err
	}
	if err := ValidateTemplate(k, defaultText); err != nil {
		return nil, err
	}

	var preset model.PromptTemplatePreset
	if err := db.First(&preset, "key = ? AND name = ?", k, defaultPresetName).Error; err == nil {
		if preset.IsBuiltin {
			updates := map[string]any{}
			if strings.TrimSpace(preset.Template) != strings.TrimSpace(defaultText) {
				updates["template"] = defaultText
			}
			if strings.TrimSpace(preset.Description) == "" {
				updates["description"] = "内置默认模板"
			}
			if len(updates) > 0 {
				updates["updated_at"] = time.Now()
				if err := db.Model(&model.PromptTemplatePreset{}).
					Where("id = ?", preset.ID).
					Updates(updates).Error; err != nil {
					return nil, err
				}
				_ = db.First(&preset, "id = ?", preset.ID).Error
			}
		}
		return &preset, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()
	preset = model.PromptTemplatePreset{
		ID:          uuid.NewString(),
		Key:         k,
		Name:        defaultPresetName,
		Description: "内置默认模板",
		Template:    defaultText,
		IsBuiltin:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&preset).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}
