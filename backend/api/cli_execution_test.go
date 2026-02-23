package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	clisvc "github.com/ai-coding-assistant/service/cli"
	"github.com/gofiber/fiber/v2"
)

func setupCLIExecutionTestApp(t *testing.T) *fiber.App {
	t.Helper()

	dsn := fmt.Sprintf("file:cli_execution_api_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("username", "tester")
		return c.Next()
	})

	ctrl := NewCLIExecutionController(nil)
	ctrl.RegisterRoutes(apiGroup)
	return app
}

func createExecutionFixture(t *testing.T, status string) (string, []model.CLIExecutionEvent) {
	t.Helper()

	tracker := clisvc.NewExecutionTracker(model.DB)
	if tracker == nil {
		t.Fatalf("expected tracker")
	}

	rec, err := tracker.Start(clisvc.StartExecutionInput{
		Tool:   "codex",
		Mode:   "execute",
		Source: "workflow",
		Prompt: "echo hi",
		Metadata: map[string]any{
			"server_id": "srv-1",
		},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := tracker.AppendEvent(rec.ID, clisvc.EventTypeStarted, map[string]any{
		"command": "echo hi",
	}); err != nil {
		t.Fatalf("AppendEvent(started) failed: %v", err)
	}
	if err := tracker.AppendEvent(rec.ID, clisvc.EventTypeOutput, map[string]any{
		"output": "hi",
	}); err != nil {
		t.Fatalf("AppendEvent(output) failed: %v", err)
	}

	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == clisvc.StatusCompleted || normalizedStatus == clisvc.StatusError || normalizedStatus == clisvc.StatusTimeout || normalizedStatus == clisvc.StatusCancelled {
		exitCode := 0
		if normalizedStatus == clisvc.StatusError {
			exitCode = 1
		}
		if err := tracker.Complete(rec.ID, normalizedStatus, &exitCode, ""); err != nil {
			t.Fatalf("Complete() failed: %v", err)
		}
	}

	events, err := clisvc.ListExecutionEvents(rec.ID, 0, 20)
	if err != nil {
		t.Fatalf("ListExecutionEvents() failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	return rec.ID, events
}

func createChildExecutionFixture(t *testing.T, parentID string, status string) string {
	t.Helper()

	tracker := clisvc.NewExecutionTracker(model.DB)
	if tracker == nil {
		t.Fatalf("expected tracker")
	}

	parentPtr := strings.TrimSpace(parentID)
	rec, err := tracker.Start(clisvc.StartExecutionInput{
		ParentExecutionID: &parentPtr,
		Role:              clisvc.ExecutionRoleReview,
		Tool:              "codex",
		Mode:              "review",
		Source:            "workflow-review",
		Prompt:            "review output",
		Metadata: map[string]any{
			"parent_execution": parentPtr,
		},
	})
	if err != nil {
		t.Fatalf("Start(child) failed: %v", err)
	}

	_ = tracker.AppendEvent(rec.ID, clisvc.EventTypeReview, map[string]any{
		"stage": "started",
	})

	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == clisvc.StatusCompleted || normalizedStatus == clisvc.StatusError || normalizedStatus == clisvc.StatusTimeout || normalizedStatus == clisvc.StatusCancelled {
		exitCode := 0
		if normalizedStatus == clisvc.StatusError {
			exitCode = 1
		}
		if err := tracker.Complete(rec.ID, normalizedStatus, &exitCode, ""); err != nil {
			t.Fatalf("Complete(child) failed: %v", err)
		}
	}

	return rec.ID
}

func TestCLIExecutionController_ListGetAndEvents(t *testing.T) {
	app := setupCLIExecutionTestApp(t)

	completedID, events := createExecutionFixture(t, clisvc.StatusCompleted)
	runningID, _ := createExecutionFixture(t, clisvc.StatusRunning)

	listReq := httptest.NewRequest("GET", "/api/cli-executions?status=running", nil)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.CLIExecution `json:"items"`
		Count int                  `json:"count"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if listBody.Count != 1 || len(listBody.Items) != 1 {
		t.Fatalf("expected one running execution, got count=%d len=%d", listBody.Count, len(listBody.Items))
	}
	if listBody.Items[0].ID != runningID {
		t.Fatalf("expected running id %q, got %q", runningID, listBody.Items[0].ID)
	}

	getReq := httptest.NewRequest("GET", "/api/cli-executions/"+completedID, nil)
	getResp, err := app.Test(getReq)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("expected get status 200, got %d", getResp.StatusCode)
	}

	var getBody struct {
		Item model.CLIExecution `json:"item"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get response failed: %v", err)
	}
	if getBody.Item.ID != completedID {
		t.Fatalf("expected execution id %q, got %q", completedID, getBody.Item.ID)
	}
	if getBody.Item.Status != clisvc.StatusCompleted {
		t.Fatalf("expected status %q, got %q", clisvc.StatusCompleted, getBody.Item.Status)
	}

	eventsReq := httptest.NewRequest("GET", "/api/cli-executions/"+completedID+"/events?limit=20", nil)
	eventsResp, err := app.Test(eventsReq)
	if err != nil {
		t.Fatalf("events request failed: %v", err)
	}
	defer eventsResp.Body.Close()
	if eventsResp.StatusCode != 200 {
		t.Fatalf("expected events status 200, got %d", eventsResp.StatusCode)
	}

	var eventsBody struct {
		Items []CLIExecutionEventResponse `json:"items"`
		Count int                         `json:"count"`
	}
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsBody); err != nil {
		t.Fatalf("decode events response failed: %v", err)
	}
	if eventsBody.Count != 2 || len(eventsBody.Items) != 2 {
		t.Fatalf("expected 2 events, got count=%d len=%d", eventsBody.Count, len(eventsBody.Items))
	}
	payloadMap, ok := eventsBody.Items[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected event payload map, got %T", eventsBody.Items[0].Payload)
	}
	if strings.TrimSpace(fmt.Sprintf("%v", payloadMap["command"])) != "echo hi" {
		t.Fatalf("expected command payload, got %#v", payloadMap)
	}

	afterReq := httptest.NewRequest("GET", fmt.Sprintf("/api/cli-executions/%s/events?after=%d", completedID, events[0].Seq), nil)
	afterResp, err := app.Test(afterReq)
	if err != nil {
		t.Fatalf("events-after request failed: %v", err)
	}
	defer afterResp.Body.Close()
	if afterResp.StatusCode != 200 {
		t.Fatalf("expected events-after status 200, got %d", afterResp.StatusCode)
	}

	var afterBody struct {
		Items []CLIExecutionEventResponse `json:"items"`
		Count int                         `json:"count"`
	}
	if err := json.NewDecoder(afterResp.Body).Decode(&afterBody); err != nil {
		t.Fatalf("decode events-after response failed: %v", err)
	}
	if afterBody.Count != 1 || len(afterBody.Items) != 1 {
		t.Fatalf("expected 1 event after seq, got count=%d len=%d", afterBody.Count, len(afterBody.Items))
	}
	if afterBody.Items[0].EventType != clisvc.EventTypeOutput {
		t.Fatalf("expected event type %q, got %q", clisvc.EventTypeOutput, afterBody.Items[0].EventType)
	}
}

func TestCLIExecutionController_StreamExecutionEvents(t *testing.T) {
	app := setupCLIExecutionTestApp(t)

	completedID, _ := createExecutionFixture(t, clisvc.StatusCompleted)

	req := httptest.NewRequest("GET", "/api/cli-executions/"+completedID+"/stream?after=0&poll_ms=100&timeout_sec=2", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected stream status 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", resp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream response body failed: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "event: ready") {
		t.Fatalf("expected ready event, got: %s", text)
	}
	if !strings.Contains(text, "event: message") {
		t.Fatalf("expected message events, got: %s", text)
	}
	if !strings.Contains(text, `"event_type":"started"`) {
		t.Fatalf("expected started payload in stream, got: %s", text)
	}
	if !strings.Contains(text, `"event_type":"output"`) {
		t.Fatalf("expected output payload in stream, got: %s", text)
	}
	if !strings.Contains(text, "event: done") {
		t.Fatalf("expected done event, got: %s", text)
	}
}

func TestCLIExecutionController_ListChildrenAndFilterByParentRole(t *testing.T) {
	app := setupCLIExecutionTestApp(t)

	parentID, _ := createExecutionFixture(t, clisvc.StatusCompleted)
	childID := createChildExecutionFixture(t, parentID, clisvc.StatusCompleted)

	filterReq := httptest.NewRequest("GET", "/api/cli-executions?parent_id="+parentID+"&role=review", nil)
	filterResp, err := app.Test(filterReq)
	if err != nil {
		t.Fatalf("filter request failed: %v", err)
	}
	defer filterResp.Body.Close()
	if filterResp.StatusCode != 200 {
		t.Fatalf("expected filter status 200, got %d", filterResp.StatusCode)
	}

	var filterBody struct {
		Items []model.CLIExecution `json:"items"`
		Count int                  `json:"count"`
	}
	if err := json.NewDecoder(filterResp.Body).Decode(&filterBody); err != nil {
		t.Fatalf("decode filter response failed: %v", err)
	}
	if filterBody.Count != 1 || len(filterBody.Items) != 1 {
		t.Fatalf("expected one child execution, got count=%d len=%d", filterBody.Count, len(filterBody.Items))
	}
	if filterBody.Items[0].ID != childID {
		t.Fatalf("expected child id %q, got %q", childID, filterBody.Items[0].ID)
	}
	if filterBody.Items[0].ParentExecutionID == nil || *filterBody.Items[0].ParentExecutionID != parentID {
		t.Fatalf("expected parent execution id %q, got %v", parentID, filterBody.Items[0].ParentExecutionID)
	}

	childrenReq := httptest.NewRequest("GET", "/api/cli-executions/"+parentID+"/children", nil)
	childrenResp, err := app.Test(childrenReq)
	if err != nil {
		t.Fatalf("children request failed: %v", err)
	}
	defer childrenResp.Body.Close()
	if childrenResp.StatusCode != 200 {
		t.Fatalf("expected children status 200, got %d", childrenResp.StatusCode)
	}

	var childrenBody struct {
		Items []model.CLIExecution `json:"items"`
		Count int                  `json:"count"`
	}
	if err := json.NewDecoder(childrenResp.Body).Decode(&childrenBody); err != nil {
		t.Fatalf("decode children response failed: %v", err)
	}
	if childrenBody.Count != 1 || len(childrenBody.Items) != 1 {
		t.Fatalf("expected one child in children list, got count=%d len=%d", childrenBody.Count, len(childrenBody.Items))
	}
	if childrenBody.Items[0].ID != childID {
		t.Fatalf("expected child id %q in children list, got %q", childID, childrenBody.Items[0].ID)
	}

	missingReq := httptest.NewRequest("GET", "/api/cli-executions/missing-parent/children", nil)
	missingResp, err := app.Test(missingReq)
	if err != nil {
		t.Fatalf("missing parent request failed: %v", err)
	}
	defer missingResp.Body.Close()
	if missingResp.StatusCode != 404 {
		t.Fatalf("expected missing parent status 404, got %d", missingResp.StatusCode)
	}
}

func TestCLIExecutionController_ListExecutions_FilterByModeSourceTool(t *testing.T) {
	app := setupCLIExecutionTestApp(t)

	tracker := clisvc.NewExecutionTracker(model.DB)
	if tracker == nil {
		t.Fatalf("expected tracker")
	}

	_, err := tracker.Start(clisvc.StartExecutionInput{
		Tool:   "codex",
		Mode:   "execute",
		Source: "workflow",
		Prompt: "execute command",
	})
	if err != nil {
		t.Fatalf("Start(execute) failed: %v", err)
	}

	target, err := tracker.Start(clisvc.StartExecutionInput{
		Tool:   "claude",
		Mode:   "review",
		Source: "workflow-review",
		Prompt: "review command",
	})
	if err != nil {
		t.Fatalf("Start(review) failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/cli-executions?mode=review&source=workflow-review&tool=claude", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("filter request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items  []model.CLIExecution `json:"items"`
		Count  int                  `json:"count"`
		Mode   string               `json:"mode"`
		Source string               `json:"source"`
		Tool   string               `json:"tool"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode filter response failed: %v", err)
	}
	if body.Count != 1 || len(body.Items) != 1 {
		t.Fatalf("expected one filtered execution, got count=%d len=%d", body.Count, len(body.Items))
	}
	if body.Items[0].ID != target.ID {
		t.Fatalf("expected filtered execution id %q, got %q", target.ID, body.Items[0].ID)
	}
	if body.Mode != "review" || body.Source != "workflow-review" || body.Tool != "claude" {
		t.Fatalf("expected echoed filters review/workflow-review/claude, got %q/%q/%q", body.Mode, body.Source, body.Tool)
	}
}

func TestCLIExecutionController_ResumeExecution_ManagerNotConfigured(t *testing.T) {
	app := setupCLIExecutionTestApp(t)

	parentID, _ := createExecutionFixture(t, clisvc.StatusCompleted)

	req := httptest.NewRequest("POST", "/api/cli-executions/"+parentID+"/resume", strings.NewReader(`{"strategy":"auto"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("resume request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected status 500 when terminal manager is nil, got %d", resp.StatusCode)
	}
}
