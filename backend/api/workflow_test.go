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

func setupWorkflowTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:workflow_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		return c.Next()
	})

	ctrl := NewWorkflowController("", nil)
	ctrl.RegisterRoutes(apiGroup)

	return app
}

func TestWorkflowController_CRUD(t *testing.T) {
	app := setupWorkflowTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/workflows", bytes.NewBufferString(`{"name":"wf-1","nodes":"[]","edges":"[]"}`))
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
		Item model.Workflow `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if createBody.Item.ID == "" {
		t.Fatalf("expected non-empty workflow id")
	}
	if createBody.Item.Name != "wf-1" {
		t.Fatalf("expected name %q, got %q", "wf-1", createBody.Item.Name)
	}
	if createBody.Item.Status != "draft" {
		t.Fatalf("expected status %q, got %q", "draft", createBody.Item.Status)
	}

	listReq := httptest.NewRequest("GET", "/api/workflows", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.Workflow `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(listBody.Items))
	}

	getReq := httptest.NewRequest("GET", "/api/workflows/"+createBody.Item.ID, nil)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", getResp.StatusCode)
	}

	var getBody struct {
		Item model.Workflow `json:"item"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get response failed: %v", err)
	}
	if getBody.Item.ID != createBody.Item.ID {
		t.Fatalf("expected workflow id %q, got %q", createBody.Item.ID, getBody.Item.ID)
	}

	updateReq := httptest.NewRequest("PUT", "/api/workflows/"+createBody.Item.ID, bytes.NewBufferString(`{"name":"wf-1-updated","nodes":"[1]","edges":"[2]","status":"active"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("PUT request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", updateResp.StatusCode)
	}

	var updateBody struct {
		Item model.Workflow `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updateBody.Item.Name != "wf-1-updated" {
		t.Fatalf("expected updated name %q, got %q", "wf-1-updated", updateBody.Item.Name)
	}
	if updateBody.Item.Nodes != "[1]" {
		t.Fatalf("expected updated nodes %q, got %q", "[1]", updateBody.Item.Nodes)
	}
	if updateBody.Item.Edges != "[2]" {
		t.Fatalf("expected updated edges %q, got %q", "[2]", updateBody.Item.Edges)
	}
	if updateBody.Item.Status != "active" {
		t.Fatalf("expected updated status %q, got %q", "active", updateBody.Item.Status)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/workflows/"+createBody.Item.ID, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}

	getAfterDeleteReq := httptest.NewRequest("GET", "/api/workflows/"+createBody.Item.ID, nil)
	getAfterDeleteResp, err := app.Test(getAfterDeleteReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getAfterDeleteResp.Body.Close()

	if getAfterDeleteResp.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", getAfterDeleteResp.StatusCode)
	}
}

func TestWorkflowController_Runs(t *testing.T) {
	app := setupWorkflowTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/workflows", bytes.NewBufferString(`{"name":"wf-1","nodes":"[]","edges":"[]"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()

	var createBody struct {
		Item model.Workflow `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}

	runReq := httptest.NewRequest("POST", "/api/workflows/"+createBody.Item.ID+"/run", nil)
	runResp, err := app.Test(runReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer runResp.Body.Close()

	if runResp.StatusCode != 201 {
		t.Fatalf("expected status 201, got %d", runResp.StatusCode)
	}

	var runBody struct {
		Item model.WorkflowRun `json:"item"`
	}
	if err := json.NewDecoder(runResp.Body).Decode(&runBody); err != nil {
		t.Fatalf("decode run response failed: %v", err)
	}
	if runBody.Item.ID == "" {
		t.Fatalf("expected non-empty run id")
	}
	if runBody.Item.WorkflowID != createBody.Item.ID {
		t.Fatalf("expected workflow_id %q, got %q", createBody.Item.ID, runBody.Item.WorkflowID)
	}
	if runBody.Item.Status != "running" {
		t.Fatalf("expected status %q, got %q", "running", runBody.Item.Status)
	}
	if runBody.Item.StartedAt == nil {
		t.Fatalf("expected non-nil started_at")
	}

	historyReq := httptest.NewRequest("GET", "/api/workflows/"+createBody.Item.ID+"/runs", nil)
	historyResp, err := app.Test(historyReq)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer historyResp.Body.Close()

	if historyResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", historyResp.StatusCode)
	}

	var historyBody struct {
		Items []model.WorkflowRun `json:"items"`
	}
	if err := json.NewDecoder(historyResp.Body).Decode(&historyBody); err != nil {
		t.Fatalf("decode history response failed: %v", err)
	}
	if len(historyBody.Items) != 1 {
		t.Fatalf("expected 1 run, got %d", len(historyBody.Items))
	}
	if historyBody.Items[0].ID != runBody.Item.ID {
		t.Fatalf("expected run id %q, got %q", runBody.Item.ID, historyBody.Items[0].ID)
	}
}

func TestWorkflowController_CreateWorkflow_InvalidFields(t *testing.T) {
	app := setupWorkflowTestApp(t)

	missingNameReq := httptest.NewRequest("POST", "/api/workflows", bytes.NewBufferString(`{"nodes":"[]","edges":"[]"}`))
	missingNameReq.Header.Set("Content-Type", "application/json")
	missingNameResp, err := app.Test(missingNameReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer missingNameResp.Body.Close()
	if missingNameResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", missingNameResp.StatusCode)
	}

	invalidNodesReq := httptest.NewRequest("POST", "/api/workflows", bytes.NewBufferString(`{"name":"wf","nodes":"not-json","edges":"[]"}`))
	invalidNodesReq.Header.Set("Content-Type", "application/json")
	invalidNodesResp, err := app.Test(invalidNodesReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer invalidNodesResp.Body.Close()
	if invalidNodesResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidNodesResp.StatusCode)
	}

	invalidEdgesReq := httptest.NewRequest("POST", "/api/workflows", bytes.NewBufferString(`{"name":"wf","nodes":"[]","edges":"not-json"}`))
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

func TestWorkflowController_DeleteWorkflow_RemovesRunsAndNodes(t *testing.T) {
	app := setupWorkflowTestApp(t)

	createReq := httptest.NewRequest("POST", "/api/workflows", bytes.NewBufferString(`{"name":"wf-1","nodes":"[]","edges":"[]"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer createResp.Body.Close()

	var createBody struct {
		Item model.Workflow `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}

	node := model.WorkflowNode{
		ID:         "node-1",
		WorkflowID: createBody.Item.ID,
		Type:       model.WorkflowNodeTypeTask,
		Name:       "node",
		Config:     "{}",
		PositionX:  1,
		PositionY:  2,
	}
	if err := model.DB.Create(&node).Error; err != nil {
		t.Fatalf("create node failed: %v", err)
	}

	now := time.Now()
	run := model.WorkflowRun{
		ID:         "run-1",
		WorkflowID: createBody.Item.ID,
		Status:     "running",
		Logs:       "[]",
		StartedAt:  &now,
	}
	if err := model.DB.Create(&run).Error; err != nil {
		t.Fatalf("create run failed: %v", err)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/workflows/"+createBody.Item.ID, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", deleteResp.StatusCode)
	}

	var nodeCount int64
	if err := model.DB.Model(&model.WorkflowNode{}).Where("workflow_id = ?", createBody.Item.ID).Count(&nodeCount).Error; err != nil {
		t.Fatalf("count nodes failed: %v", err)
	}
	if nodeCount != 0 {
		t.Fatalf("expected node count 0, got %d", nodeCount)
	}

	var runCount int64
	if err := model.DB.Model(&model.WorkflowRun{}).Where("workflow_id = ?", createBody.Item.ID).Count(&runCount).Error; err != nil {
		t.Fatalf("count runs failed: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("expected run count 0, got %d", runCount)
	}
}
