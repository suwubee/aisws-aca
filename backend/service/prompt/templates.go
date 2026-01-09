package prompt

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/ai-coding-assistant/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TemplateKeyApprovalSystemPrompt     = "approval.system_prompt"
	TemplateKeyTaskMonitorSystemPrompt  = "task_monitor.system_prompt"
	TemplateKeyTaskManagedPrompt        = "task.managed_prompt"
	TemplateKeyAIWorkflowSystemPrompt   = "ai_workflow.system_prompt"
	TemplateKeyAIWorkflowUserGoalPrompt = "ai_workflow.user_goal_prompt"
)

type builtinTemplate struct {
	Key         string
	Name        string
	Description string
	Variables   []string
	File        string
}

var builtinTemplates = []builtinTemplate{
	{
		Key:         TemplateKeyApprovalSystemPrompt,
		Name:        "AI 审核：系统提示词",
		Description: "用于 AI 辅助审批（AI 审核终端输出/提示是否需要输入以及输入什么）的系统提示词模板；可通过变量 extra_rules 注入规则集的 ai_prompt。",
		Variables:   []string{"extra_rules"},
		File:        "defaults/approval_system_prompt.tmpl",
	},
	{
		Key:         TemplateKeyTaskMonitorSystemPrompt,
		Name:        "任务监控：系统提示词",
		Description: "用于任务监控（终端日志分析/状态判断）的系统提示词模板。",
		Variables:   []string{"log_limit", "max_log_chars"},
		File:        "defaults/task_monitor_system_prompt.tmpl",
	},
	{
		Key:         TemplateKeyTaskManagedPrompt,
		Name:        "AI 任务：托管提示词模板",
		Description: "用于 AI 托管模式下发送给 CLI 的提示词模板（由任务字段与完成标记渲染）。",
		Variables:   []string{"task_initial_prompt", "task_ai_prompt", "task_ai_end_condition", "task_done_marker"},
		File:        "defaults/task_managed_prompt.tmpl",
	},
	{
		Key:         TemplateKeyAIWorkflowSystemPrompt,
		Name:        "AI 工作流：系统提示词",
		Description: "用于 ACA 内置 AI 工作流（ReAct + 工具调用）的系统提示词模板。",
		Variables:   []string{"tools"},
		File:        "defaults/ai_workflow_system_prompt.tmpl",
	},
	{
		Key:         TemplateKeyAIWorkflowUserGoalPrompt,
		Name:        "AI 工作流：用户目标包装模板",
		Description: "用于 AI 工作流启动时包装用户目标的提示词模板。",
		Variables:   []string{"user_goal"},
		File:        "defaults/ai_workflow_user_goal_prompt.tmpl",
	},
}

var builtinTemplateByKey = func() map[string]builtinTemplate {
	m := make(map[string]builtinTemplate, len(builtinTemplates))
	for _, t := range builtinTemplates {
		m[t.Key] = t
	}
	return m
}()

//go:embed defaults/*.tmpl
var defaultTemplatesFS embed.FS

var templateFuncs = template.FuncMap{
	"join": strings.Join,
}

func EnsureDefaults() error {
	if model.DB == nil {
		return errors.New("database not initialized")
	}
	for _, t := range builtinTemplates {
		if _, err := ensureDefaultPreset(model.DB, t.Key); err != nil {
			return err
		}
		if err := ensureDefaultTemplate(model.DB, t.Key); err != nil {
			return err
		}
	}
	return nil
}

func ListTemplates() ([]model.PromptTemplate, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	if err := EnsureDefaults(); err != nil {
		return nil, err
	}

	var items []model.PromptTemplate
	if err := model.DB.Order("key asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func GetTemplate(key string) (*model.PromptTemplate, error) {
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

	var item model.PromptTemplate
	if err := model.DB.First(&item, "key = ?", k).Error; err == nil {
		return &item, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := ensureDefaultTemplate(model.DB, k); err != nil {
		return nil, err
	}
	if err := model.DB.First(&item, "key = ?", k).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func UpdateTemplate(key string, templateText string) (*model.PromptTemplate, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return nil, errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return nil, errors.New("unknown template key")
	}

	text := templateText
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("template content is required")
	}

	if err := ValidateTemplate(k, text); err != nil {
		return nil, err
	}

	item, err := GetTemplate(k)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := model.DB.Model(&model.PromptTemplate{}).
		Where("key = ?", item.Key).
		Updates(map[string]any{
			"template":         text,
			"active_preset_id": "",
			"updated_at":       now,
		}).Error; err != nil {
		return nil, err
	}

	item.Template = text
	item.ActivePresetID = ""
	item.UpdatedAt = now
	return item, nil
}

func ResetTemplateToDefault(key string) (*model.PromptTemplate, error) {
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

	item, err := GetTemplate(k)
	if err != nil {
		return nil, err
	}

	defaultPreset, err := ensureDefaultPreset(model.DB, k)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := model.DB.Model(&model.PromptTemplate{}).
		Where("key = ?", item.Key).
		Updates(map[string]any{
			"template":         defaultText,
			"active_preset_id": defaultPreset.ID,
			"updated_at":       now,
		}).Error; err != nil {
		return nil, err
	}

	item.Template = defaultText
	item.ActivePresetID = defaultPreset.ID
	item.UpdatedAt = now
	return item, nil
}

func RenderTemplate(key string, data any) (string, error) {
	item, err := GetTemplate(key)
	if err != nil {
		return "", err
	}

	tpl, err := template.New(item.Key).
		Option("missingkey=zero").
		Funcs(templateFuncs).
		Parse(item.Template)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ValidateTemplate(key string, templateText string) error {
	k := strings.TrimSpace(key)
	if k == "" {
		return errors.New("template key is required")
	}
	if _, ok := builtinTemplateByKey[k]; !ok {
		return errors.New("unknown template key")
	}

	text := templateText
	if strings.TrimSpace(text) == "" {
		return errors.New("template content is required")
	}

	_, err := template.New(k).
		Option("missingkey=zero").
		Funcs(templateFuncs).
		Parse(text)
	return err
}

func ensureDefaultTemplate(db *gorm.DB, key string) error {
	if db == nil {
		return errors.New("missing database connection")
	}

	k := strings.TrimSpace(key)
	def, ok := builtinTemplateByKey[k]
	if !ok {
		return errors.New("unknown template key")
	}

	defaultText, err := readDefaultTemplateText(k)
	if err != nil {
		return err
	}

	defaultPreset, err := ensureDefaultPreset(db, k)
	if err != nil {
		return err
	}

	now := time.Now()
	record := &model.PromptTemplate{
		Key:            def.Key,
		Name:           def.Name,
		Description:    def.Description,
		Template:       defaultText,
		Variables:      model.StringArray(append([]string(nil), def.Variables...)),
		ActivePresetID: defaultPreset.ID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error; err != nil {
		return err
	}

	// Keep metadata in sync (name/description/variables), without overriding template content.
	if err := db.Model(&model.PromptTemplate{}).
		Where("key = ?", def.Key).
		Updates(map[string]any{
			"name":        def.Name,
			"description": def.Description,
			"variables":   model.StringArray(append([]string(nil), def.Variables...)),
		}).Error; err != nil {
		return err
	}

	// Backfill active preset id only when template matches the builtin default content.
	var existing model.PromptTemplate
	if err := db.Select("key", "template", "active_preset_id").First(&existing, "key = ?", def.Key).Error; err != nil {
		return err
	}
	if strings.TrimSpace(existing.ActivePresetID) == "" && strings.TrimSpace(existing.Template) == strings.TrimSpace(defaultText) {
		if err := db.Model(&model.PromptTemplate{}).
			Where("key = ?", def.Key).
			Update("active_preset_id", defaultPreset.ID).Error; err != nil {
			return err
		}
	}

	return nil
}

func readDefaultTemplateText(key string) (string, error) {
	def, ok := builtinTemplateByKey[strings.TrimSpace(key)]
	if !ok {
		return "", errors.New("unknown template key")
	}

	data, err := fs.ReadFile(defaultTemplatesFS, def.File)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func SupportedTemplateKeys() []string {
	keys := make([]string, 0, len(builtinTemplates))
	for _, tpl := range builtinTemplates {
		keys = append(keys, tpl.Key)
	}
	sort.Strings(keys)
	return keys
}
