package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
)

func setupWorkflowTemplateTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:workflow_template_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		return c.Next()
	})

	ctrl := NewWorkflowTemplateController()
	ctrl.RegisterRoutes(apiGroup)

	return app
}

func TestWorkflowTemplateController_ListIncludesBuiltins(t *testing.T) {
	app := setupWorkflowTemplateTestApp(t)

	req := httptest.NewRequest("GET", "/api/workflow-templates", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items []model.WorkflowTemplate `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	names := make(map[string]model.WorkflowTemplate, len(body.Items))
	for _, item := range body.Items {
		names[item.Name] = item
	}

	required := []string{"PRD编写流程", "DevOps全流程", "Bug修复流程"}
	for _, name := range required {
		item, ok := names[name]
		if !ok {
			t.Fatalf("expected builtin template %q in list", name)
		}
		if !item.IsBuiltin {
			t.Fatalf("expected template %q to be builtin", name)
		}
	}
}

func TestWorkflowTemplateController_CreateAndApply(t *testing.T) {
	app := setupWorkflowTemplateTestApp(t)

	createReq := httptest.NewRequest(
		"POST",
		"/api/workflow-templates",
		bytes.NewBufferString(`{"name":"custom","category":"testing","nodes":"[]","edges":"[]"}`),
	)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", createResp.StatusCode)
	}

	var createBody struct {
		Item model.WorkflowTemplate `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty template id")
	}
	if createBody.Item.IsBuiltin {
		t.Fatalf("expected custom template to not be builtin")
	}
	if createBody.Item.Category != model.WorkflowTemplateCategoryTesting {
		t.Fatalf("expected category %q, got %q", model.WorkflowTemplateCategoryTesting, createBody.Item.Category)
	}

	applyReq := httptest.NewRequest(
		"POST",
		"/api/workflow-templates/"+createBody.Item.ID+"/apply",
		bytes.NewBufferString(`{"name":"wf-from-template"}`),
	)
	applyReq.Header.Set("Content-Type", "application/json")
	applyResp, err := app.Test(applyReq)
	if err != nil {
		t.Fatalf("POST apply request failed: %v", err)
	}
	defer applyResp.Body.Close()

	if applyResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", applyResp.StatusCode)
	}

	var applyBody struct {
		Item model.Workflow `json:"item"`
	}
	if err := json.NewDecoder(applyResp.Body).Decode(&applyBody); err != nil {
		t.Fatalf("decode apply response failed: %v", err)
	}
	if applyBody.Item.ID == "" {
		t.Fatalf("expected non-empty workflow id")
	}
	if applyBody.Item.Name != "wf-from-template" {
		t.Fatalf("expected workflow name %q, got %q", "wf-from-template", applyBody.Item.Name)
	}
	if applyBody.Item.Status != "draft" {
		t.Fatalf("expected status %q, got %q", "draft", applyBody.Item.Status)
	}
	if applyBody.Item.Nodes != "[]" {
		t.Fatalf("expected nodes %q, got %q", "[]", applyBody.Item.Nodes)
	}
	if applyBody.Item.Edges != "[]" {
		t.Fatalf("expected edges %q, got %q", "[]", applyBody.Item.Edges)
	}
}

func TestWorkflowTemplateController_CreateWorkflowTemplate_InvalidFields(t *testing.T) {
	app := setupWorkflowTemplateTestApp(t)

	missingNameReq := httptest.NewRequest(
		"POST",
		"/api/workflow-templates",
		bytes.NewBufferString(`{"category":"testing","nodes":"[]","edges":"[]"}`),
	)
	missingNameReq.Header.Set("Content-Type", "application/json")
	missingNameResp, err := app.Test(missingNameReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer missingNameResp.Body.Close()
	if missingNameResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", missingNameResp.StatusCode)
	}

	invalidCategoryReq := httptest.NewRequest(
		"POST",
		"/api/workflow-templates",
		bytes.NewBufferString(`{"name":"t1","category":"unknown","nodes":"[]","edges":"[]"}`),
	)
	invalidCategoryReq.Header.Set("Content-Type", "application/json")
	invalidCategoryResp, err := app.Test(invalidCategoryReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidCategoryResp.Body.Close()
	if invalidCategoryResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidCategoryResp.StatusCode)
	}

	invalidNodesReq := httptest.NewRequest(
		"POST",
		"/api/workflow-templates",
		bytes.NewBufferString(`{"name":"t1","category":"testing","nodes":"not-json","edges":"[]"}`),
	)
	invalidNodesReq.Header.Set("Content-Type", "application/json")
	invalidNodesResp, err := app.Test(invalidNodesReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidNodesResp.Body.Close()
	if invalidNodesResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidNodesResp.StatusCode)
	}

	invalidEdgesReq := httptest.NewRequest(
		"POST",
		"/api/workflow-templates",
		bytes.NewBufferString(`{"name":"t1","category":"testing","nodes":"[]","edges":"not-json"}`),
	)
	invalidEdgesReq.Header.Set("Content-Type", "application/json")
	invalidEdgesResp, err := app.Test(invalidEdgesReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidEdgesResp.Body.Close()
	if invalidEdgesResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidEdgesResp.StatusCode)
	}
}

func TestWorkflowTemplateController_ApplyWorkflowTemplate_NotFound(t *testing.T) {
	app := setupWorkflowTemplateTestApp(t)

	req := httptest.NewRequest("POST", "/api/workflow-templates/does-not-exist/apply", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}
