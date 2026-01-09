package keybinding

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"gorm.io/gorm"
)

const (
	IDEnter   = "enter"   // CR, confirm selection
	IDNewline = "newline" // LF, insert newline
	IDEsc     = "esc"
	IDCtrlC   = "ctrl_c"
	IDCtrlD   = "ctrl_d"
	IDTab     = "tab"

	// Common macros used by automation and UI.
	IDYes     = "yes"
	IDY       = "y"
	IDNo      = "no"
	IDN       = "n"
	IDOption1 = "option1"
	IDOption2 = "option2"
)

type builtinKeyBinding struct {
	ID          string
	Label       string
	Description string
	PtyInput    string
	TmuxKeys    string
	TmuxLiteral bool
}

var builtinKeyBindings = []builtinKeyBinding{
	{
		ID:          IDEnter,
		Label:       "Enter",
		Description: "回车确认（CR，用于确认/选择）",
		PtyInput:    "\\r",
		TmuxKeys:    "C-m",
	},
	{
		ID:          IDNewline,
		Label:       "Newline",
		Description: "换行（LF，用于文本输入换行）",
		PtyInput:    "\\n",
		TmuxKeys:    "C-j",
	},
	{
		ID:          IDEsc,
		Label:       "Esc",
		Description: "取消/退出/返回",
		PtyInput:    "\\x1b",
		TmuxKeys:    "Escape",
	},
	{
		ID:          IDCtrlC,
		Label:       "Ctrl+C",
		Description: "中断当前操作（SIGINT）",
		PtyInput:    "\\x03",
		TmuxKeys:    "C-c",
	},
	{
		ID:          IDCtrlD,
		Label:       "Ctrl+D",
		Description: "EOF/退出",
		PtyInput:    "\\x04",
		TmuxKeys:    "C-d",
	},
	{
		ID:          IDTab,
		Label:       "Tab",
		Description: "自动补全",
		PtyInput:    "\\t",
		TmuxKeys:    "Tab",
	},
	{
		ID:          IDYes,
		Label:       "yes",
		Description: "输入 yes 并确认",
		PtyInput:    "yes\\r",
	},
	{
		ID:          IDY,
		Label:       "y",
		Description: "输入 y 并确认",
		PtyInput:    "y\\r",
	},
	{
		ID:          IDNo,
		Label:       "no",
		Description: "输入 no 并确认",
		PtyInput:    "no\\r",
	},
	{
		ID:          IDN,
		Label:       "n",
		Description: "输入 n 并确认",
		PtyInput:    "n\\r",
	},
	{
		ID:          IDOption1,
		Label:       "1",
		Description: "选择选项 1 并确认",
		PtyInput:    "1\\r",
	},
	{
		ID:          IDOption2,
		Label:       "2",
		Description: "选择选项 2 并确认",
		PtyInput:    "2\\r",
	},
}

var builtinByID = func() map[string]builtinKeyBinding {
	m := make(map[string]builtinKeyBinding, len(builtinKeyBindings))
	for _, b := range builtinKeyBindings {
		m[b.ID] = b
	}
	return m
}()

func SupportedIDs() []string {
	ids := make([]string, 0, len(builtinKeyBindings))
	for _, b := range builtinKeyBindings {
		ids = append(ids, b.ID)
	}
	return ids
}

func EnsureDefaults() error {
	if model.DB == nil {
		return errors.New("database not initialized")
	}

	now := time.Now()
	for _, b := range builtinKeyBindings {
		var existing model.KeyBinding
		if err := model.DB.First(&existing, "id = ?", b.ID).Error; err == nil {
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		item := model.KeyBinding{
			ID:          b.ID,
			Label:       b.Label,
			Description: b.Description,
			PtyInput:    b.PtyInput,
			TmuxKeys:    b.TmuxKeys,
			TmuxLiteral: b.TmuxLiteral,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := model.DB.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func List() ([]model.KeyBinding, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	if err := EnsureDefaults(); err != nil {
		return nil, err
	}

	var items []model.KeyBinding
	if err := model.DB.Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func Get(id string) (*model.KeyBinding, error) {
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	key := strings.TrimSpace(id)
	if key == "" {
		return nil, errors.New("key binding id is required")
	}
	if _, ok := builtinByID[key]; !ok {
		return nil, errors.New("unknown key binding id")
	}

	if err := EnsureDefaults(); err != nil {
		return nil, err
	}

	var item model.KeyBinding
	if err := model.DB.First(&item, "id = ?", key).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

type UpdateRequest struct {
	Label       *string `json:"label"`
	Description *string `json:"description"`
	PtyInput    *string `json:"pty_input"`
	TmuxKeys    *string `json:"tmux_keys"`
	TmuxLiteral *bool   `json:"tmux_literal"`
}

func Update(id string, req UpdateRequest) (*model.KeyBinding, error) {
	item, err := Get(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}

	if req.Label != nil {
		label := strings.TrimSpace(*req.Label)
		if label == "" {
			return nil, errors.New("label is required")
		}
		updates["label"] = label
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.PtyInput != nil {
		pty := strings.TrimSpace(*req.PtyInput)
		if pty == "" {
			return nil, errors.New("pty_input is required")
		}
		if _, err := DecodePtyInput(pty); err != nil {
			return nil, fmt.Errorf("invalid pty_input: %w", err)
		}
		updates["pty_input"] = pty
	}
	if req.TmuxKeys != nil {
		updates["tmux_keys"] = strings.TrimSpace(*req.TmuxKeys)
	}
	if req.TmuxLiteral != nil {
		updates["tmux_literal"] = *req.TmuxLiteral
	}

	if len(updates) == 0 {
		return item, nil
	}

	now := time.Now()
	updates["updated_at"] = now

	if err := model.DB.Model(&model.KeyBinding{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
		return nil, err
	}

	updated := *item
	if v, ok := updates["label"].(string); ok {
		updated.Label = v
	}
	if v, ok := updates["description"].(string); ok {
		updated.Description = v
	}
	if v, ok := updates["pty_input"].(string); ok {
		updated.PtyInput = v
	}
	if v, ok := updates["tmux_keys"].(string); ok {
		updated.TmuxKeys = v
	}
	if v, ok := updates["tmux_literal"].(bool); ok {
		updated.TmuxLiteral = v
	}
	updated.UpdatedAt = now

	return &updated, nil
}

func ResetToDefault(id string) (*model.KeyBinding, error) {
	key := strings.TrimSpace(id)
	if key == "" {
		return nil, errors.New("key binding id is required")
	}
	b, ok := builtinByID[key]
	if !ok {
		return nil, errors.New("unknown key binding id")
	}

	if err := EnsureDefaults(); err != nil {
		return nil, err
	}

	now := time.Now()
	if err := model.DB.Model(&model.KeyBinding{}).Where("id = ?", key).Updates(map[string]any{
		"label":       b.Label,
		"description": b.Description,
		"pty_input":   b.PtyInput,
		"tmux_keys":   b.TmuxKeys,
		"tmux_literal": func() bool {
			return b.TmuxLiteral
		}(),
		"updated_at": now,
	}).Error; err != nil {
		return nil, err
	}

	return Get(key)
}

func DecodePtyInput(escaped string) (string, error) {
	raw := strings.TrimSpace(escaped)
	// Treat raw as the body of a double-quoted Go string literal.
	quoted := `"` + strings.ReplaceAll(raw, `"`, `\"`) + `"`
	out, err := strconv.Unquote(quoted)
	if err != nil {
		return "", err
	}
	return out, nil
}

func ResolvePty(id string) (string, error) {
	item, err := Get(id)
	if err != nil {
		return "", err
	}
	return DecodePtyInput(item.PtyInput)
}

// Alias maps common variants to built-in IDs.
func Alias(input string) string {
	v := strings.TrimSpace(strings.ToLower(input))
	switch v {
	case "", "none", "null":
		return ""
	case "enter", "回车", "confirm":
		return IDEnter
	case "newline", "换行", "lf":
		return IDNewline
	case "esc", "escape":
		return IDEsc
	case "ctrl+c", "ctrl_c", "c-c", "sigint":
		return IDCtrlC
	case "ctrl+d", "ctrl_d", "c-d", "eof":
		return IDCtrlD
	case "tab":
		return IDTab
	case "yes":
		return IDYes
	case "y":
		return IDY
	case "no":
		return IDNo
	case "n":
		return IDN
	case "1", "option1":
		return IDOption1
	case "2", "option2":
		return IDOption2
	default:
		return v
	}
}
