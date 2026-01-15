package prompt

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

func TestPresets_CreateApplyAndDelete(t *testing.T) {
	dsn := fmt.Sprintf("file:prompt_presets_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	presets, err := ListPresets(TemplateKeyApprovalSystemPrompt)
	if err != nil {
		t.Fatalf("ListPresets failed: %v", err)
	}
	if len(presets) == 0 {
		t.Fatalf("expected at least one preset")
	}

	custom, err := CreatePreset(TemplateKeyApprovalSystemPrompt, CreatePresetRequest{
		Name:     "自定义-1",
		Template: "hello {{.extra_rules}}",
	})
	if err != nil {
		t.Fatalf("CreatePreset failed: %v", err)
	}
	if strings.TrimSpace(custom.ID) == "" {
		t.Fatalf("expected created preset to have id")
	}

	applied, preset, err := ApplyPreset(TemplateKeyApprovalSystemPrompt, custom.ID)
	if err != nil {
		t.Fatalf("ApplyPreset failed: %v", err)
	}
	if preset.ID != custom.ID {
		t.Fatalf("unexpected preset returned from apply")
	}
	if strings.TrimSpace(applied.ActivePresetID) != custom.ID {
		t.Fatalf("expected active_preset_id to be set")
	}
	if strings.TrimSpace(applied.Template) != "hello {{.extra_rules}}" {
		t.Fatalf("unexpected applied template content")
	}

	if err := DeletePreset(TemplateKeyApprovalSystemPrompt, custom.ID); err != nil {
		t.Fatalf("DeletePreset failed: %v", err)
	}

	after, err := GetTemplate(TemplateKeyApprovalSystemPrompt)
	if err != nil {
		t.Fatalf("GetTemplate after delete failed: %v", err)
	}
	if strings.TrimSpace(after.ActivePresetID) != "" {
		t.Fatalf("expected active_preset_id to be cleared after deleting active preset")
	}
}

func TestCreatePreset_RejectsReservedName(t *testing.T) {
	dsn := fmt.Sprintf("file:prompt_preset_reserved_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	if _, err := CreatePreset(TemplateKeyApprovalSystemPrompt, CreatePresetRequest{
		Name:     "默认",
		Template: "x",
	}); err == nil {
		t.Fatalf("expected reserved name to be rejected")
	}
}
