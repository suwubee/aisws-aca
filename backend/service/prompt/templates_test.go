package prompt

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
)

func TestEnsureDefaults_CreatesTemplates(t *testing.T) {
	dsn := fmt.Sprintf("file:prompt_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	items, err := ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	keys := SupportedTemplateKeys()
	if len(items) < len(keys) {
		t.Fatalf("expected at least %d templates, got %d", len(keys), len(items))
	}

	for _, key := range keys {
		tpl, err := GetTemplate(key)
		if err != nil {
			t.Fatalf("GetTemplate(%q) failed: %v", key, err)
		}
		if strings.TrimSpace(tpl.Template) == "" {
			t.Fatalf("template %q should not be empty", key)
		}
		if strings.TrimSpace(tpl.ActivePresetID) == "" {
			t.Fatalf("template %q should have default active_preset_id", key)
		}
	}
}

func TestRenderTemplate_InsertsVariables(t *testing.T) {
	dsn := fmt.Sprintf("file:prompt_render_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	out, err := RenderTemplate(TemplateKeyApprovalSystemPrompt, map[string]any{
		"extra_rules": "ALLOW_ALWAYS",
	})
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}
	if !strings.Contains(out, "ALLOW_ALWAYS") {
		t.Fatalf("expected rendered prompt to include extra_rules")
	}

	managed, err := RenderTemplate(TemplateKeyTaskManagedPrompt, map[string]any{
		"task_initial_prompt":   "do something",
		"task_ai_prompt":        "rules",
		"task_ai_end_condition": "done",
		"task_done_marker":      "ACA_TASK_DONE",
	})
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}
	if !strings.Contains(managed, "ACA_TASK_DONE") {
		t.Fatalf("expected managed prompt to include task_done_marker")
	}
}

func TestUpdateTemplate_ValidatesContent(t *testing.T) {
	dsn := fmt.Sprintf("file:prompt_update_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	if _, err := UpdateTemplate(TemplateKeyApprovalSystemPrompt, ""); err == nil {
		t.Fatalf("expected empty template to be rejected")
	}

	if _, err := UpdateTemplate(TemplateKeyApprovalSystemPrompt, "hello {{"); err == nil {
		t.Fatalf("expected invalid template syntax to be rejected")
	}
}

func TestResetTemplateToDefault_RestoresDefault(t *testing.T) {
	dsn := fmt.Sprintf("file:prompt_reset_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	if err := EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	updated, err := UpdateTemplate(TemplateKeyApprovalSystemPrompt, "hello {{.extra_rules}}")
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}
	if strings.TrimSpace(updated.Template) != "hello {{.extra_rules}}" {
		t.Fatalf("unexpected updated template content")
	}

	reset, err := ResetTemplateToDefault(TemplateKeyApprovalSystemPrompt)
	if err != nil {
		t.Fatalf("ResetTemplateToDefault failed: %v", err)
	}

	defaultText, err := readDefaultTemplateText(TemplateKeyApprovalSystemPrompt)
	if err != nil {
		t.Fatalf("readDefaultTemplateText failed: %v", err)
	}
	if strings.TrimSpace(reset.Template) != strings.TrimSpace(defaultText) {
		t.Fatalf("expected template to be reset to default")
	}
}
