package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
)

func setupRuleSetImportExportTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:ruleset_import_export_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("role", "admin")
		c.Locals("username", "tester")
		c.Locals("userID", "u-1")
		return c.Next()
	})

	ctrl := NewAutomationController(nil)
	ctrl.RegisterRoutes(apiGroup)

	return app
}

func TestAutomationController_ExportRuleSets(t *testing.T) {
	app := setupRuleSetImportExportTestApp(t)

	ruleSets := []model.RuleSet{
		{
			ID:              "rs-1",
			Name:            "Rule Set 1",
			Type:            "terminal",
			ApprovalMode:    "manual",
			AutoInputType:   "yes",
			NotifyOnBlock:   true,
			NotifyOnApprove: false,
			CreatedAt:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local),
			UpdatedAt:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local),
		},
		{
			ID:              "rs-2",
			Name:            "Rule Set 2",
			Type:            "task",
			ApprovalMode:    "smart",
			AutoInputType:   "enter",
			NotifyOnBlock:   false,
			NotifyOnApprove: true,
			CreatedAt:       time.Date(2024, 1, 2, 12, 0, 0, 0, time.Local),
			UpdatedAt:       time.Date(2024, 1, 2, 12, 0, 0, 0, time.Local),
		},
	}
	if err := model.DB.Create(&ruleSets).Error; err != nil {
		t.Fatalf("create rule sets failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/rule-sets/export", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected content-type application/json, got %q", got)
	}

	if got := resp.Header.Get("Content-Disposition"); !strings.Contains(got, "rule_sets_") || !strings.Contains(got, ".json") {
		t.Fatalf("expected content-disposition to contain filename, got %q", got)
	}

	var exported []model.RuleSet
	if err := json.NewDecoder(resp.Body).Decode(&exported); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(exported) != 2 {
		t.Fatalf("expected 2 rule sets, got %d", len(exported))
	}
	if exported[0].ID != "rs-1" {
		t.Fatalf("expected first rule set id %q, got %q", "rs-1", exported[0].ID)
	}
	if exported[1].ID != "rs-2" {
		t.Fatalf("expected second rule set id %q, got %q", "rs-2", exported[1].ID)
	}
}

func TestAutomationController_ImportRuleSets_CreateAndUpdate(t *testing.T) {
	app := setupRuleSetImportExportTestApp(t)

	existing := model.RuleSet{
		ID:              "rs-existing",
		Name:            "Old Name",
		Type:            "terminal",
		ApprovalMode:    "manual",
		AutoInputType:   "yes",
		NotifyOnBlock:   true,
		NotifyOnApprove: false,
		CreatedAt:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local),
		UpdatedAt:       time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local),
	}
	if err := model.DB.Create(&existing).Error; err != nil {
		t.Fatalf("create existing rule set failed: %v", err)
	}

	payload := ruleSetImportEnvelope{
		RuleSets: []model.RuleSet{
			{
				ID:              "rs-existing",
				Name:            "New Name",
				Type:            "terminal",
				ApprovalMode:    "smart",
				AutoInputType:   "enter",
				NotifyOnBlock:   false,
				NotifyOnApprove: true,
			},
			{
				ID:              "rs-new",
				Name:            "Brand New",
				Type:            "task",
				ApprovalMode:    "manual",
				AutoInputType:   "yes",
				NotifyOnBlock:   true,
				NotifyOnApprove: false,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/rule-sets/import", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Total   int `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if result.Created != 1 || result.Updated != 1 || result.Total != 2 {
		t.Fatalf("unexpected import result: %+v", result)
	}

	var updated model.RuleSet
	if err := model.DB.First(&updated, "id = ?", "rs-existing").Error; err != nil {
		t.Fatalf("load updated rule set failed: %v", err)
	}
	if updated.Name != "New Name" {
		t.Fatalf("expected updated name %q, got %q", "New Name", updated.Name)
	}
	if updated.NotifyOnBlock != false {
		t.Fatalf("expected updated notify_on_block false, got %v", updated.NotifyOnBlock)
	}

	var created model.RuleSet
	if err := model.DB.First(&created, "id = ?", "rs-new").Error; err != nil {
		t.Fatalf("load created rule set failed: %v", err)
	}
	if created.Type != "task" {
		t.Fatalf("expected created type %q, got %q", "task", created.Type)
	}
}

func TestAutomationController_ImportRuleSets_Validation(t *testing.T) {
	app := setupRuleSetImportExportTestApp(t)

	invalidJSON := httptest.NewRequest("POST", "/api/rule-sets/import", bytes.NewBufferString(`{"bad":`))
	invalidJSON.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(invalidJSON)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	missingIDBody, err := json.Marshal([]model.RuleSet{{Name: "no id", Type: "terminal"}})
	if err != nil {
		t.Fatalf("marshal missing-id body failed: %v", err)
	}
	missingID := httptest.NewRequest("POST", "/api/rule-sets/import", bytes.NewBuffer(missingIDBody))
	missingID.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(missingID)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp2.StatusCode)
	}

	invalidTypeBody, err := json.Marshal([]model.RuleSet{{ID: "x", Name: "bad type", Type: "unknown"}})
	if err != nil {
		t.Fatalf("marshal invalid-type body failed: %v", err)
	}
	invalidType := httptest.NewRequest("POST", "/api/rule-sets/import", bytes.NewBuffer(invalidTypeBody))
	invalidType.Header.Set("Content-Type", "application/json")
	resp3, err := app.Test(invalidType)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp3.StatusCode)
	}
}
