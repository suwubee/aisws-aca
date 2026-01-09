package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/schedule"
)

type apiTestScheduleExecutor struct{}

func (apiTestScheduleExecutor) Execute(ctx context.Context, job model.ScheduledJob) (any, error) {
	return map[string]any{"ok": true, "job_id": job.ID}, nil
}

func TestScheduleEndpoints_CRUDAndRunNow(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := schedule.NewManager(apiTestScheduleExecutor{}, 0)
	manager.Start()
	defer manager.Stop()

	ctrl := NewScheduleController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	runAt := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	createBody := fmt.Sprintf(`{
		"name":"nightly workflow",
		"schedule_type":"cron",
		"cron_expr":"0 3 * * *",
		"timezone":"UTC",
		"target_type":"ai_workflow",
		"workflow_goal":"check status"
	}`)
	createReq := httptest.NewRequest("POST", "/api/schedules", bytes.NewBufferString(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 201 {
		t.Fatalf("expected create status 201, got %d", createResp.StatusCode)
	}

	var created struct {
		Item model.ScheduledJob `json:"item"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.Item.ID == "" {
		t.Fatalf("expected non-empty schedule id")
	}
	if created.Item.NextRunAt == nil {
		t.Fatalf("expected next_run_at to be computed")
	}

	listReq := httptest.NewRequest("GET", "/api/schedules", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}

	var listBody struct {
		Items []model.ScheduledJob `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list response failed: %v", err)
	}
	if len(listBody.Items) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(listBody.Items))
	}

	updateBody := fmt.Sprintf(`{
		"name":"nightly workflow updated",
		"cron_expr":"0 4 * * *",
		"timezone":"UTC",
		"workflow_goal":"check status updated",
		"description":"%s"
	}`, runAt)
	updateReq := httptest.NewRequest("PUT", "/api/schedules/"+created.Item.ID, bytes.NewBufferString(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != 200 {
		t.Fatalf("expected update status 200, got %d", updateResp.StatusCode)
	}

	var updated struct {
		Item model.ScheduledJob `json:"item"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response failed: %v", err)
	}
	if updated.Item.Name != "nightly workflow updated" {
		t.Fatalf("unexpected updated name: %q", updated.Item.Name)
	}

	runReq := httptest.NewRequest("POST", "/api/schedules/"+created.Item.ID+"/run", nil)
	runReq.Header.Set("Authorization", "Bearer "+token)
	runResp, err := app.Test(runReq)
	if err != nil {
		t.Fatalf("run request failed: %v", err)
	}
	defer runResp.Body.Close()
	if runResp.StatusCode != 200 {
		t.Fatalf("expected run status 200, got %d", runResp.StatusCode)
	}

	var runBody struct {
		Run model.ScheduledJobRun `json:"run"`
		Job model.ScheduledJob    `json:"job"`
	}
	if err := json.NewDecoder(runResp.Body).Decode(&runBody); err != nil {
		t.Fatalf("decode run response failed: %v", err)
	}
	if runBody.Run.ID == "" {
		t.Fatalf("expected non-empty run id")
	}
	if runBody.Run.Status != schedule.RunStatusSuccess {
		t.Fatalf("expected run status %q, got %q", schedule.RunStatusSuccess, runBody.Run.Status)
	}

	runsReq := httptest.NewRequest("GET", "/api/schedules/"+created.Item.ID+"/runs", nil)
	runsReq.Header.Set("Authorization", "Bearer "+token)
	runsResp, err := app.Test(runsReq)
	if err != nil {
		t.Fatalf("runs request failed: %v", err)
	}
	defer runsResp.Body.Close()
	if runsResp.StatusCode != 200 {
		t.Fatalf("expected runs status 200, got %d", runsResp.StatusCode)
	}

	var runsBody struct {
		Items []model.ScheduledJobRun `json:"items"`
		Total int64                   `json:"total"`
	}
	if err := json.NewDecoder(runsResp.Body).Decode(&runsBody); err != nil {
		t.Fatalf("decode runs response failed: %v", err)
	}
	if runsBody.Total != 1 || len(runsBody.Items) != 1 {
		t.Fatalf("expected 1 run, got total=%d len=%d", runsBody.Total, len(runsBody.Items))
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/schedules/"+created.Item.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 200 {
		t.Fatalf("expected delete status 200, got %d", deleteResp.StatusCode)
	}
}

func TestScheduleEndpoints_Validation(t *testing.T) {
	app, _, apiGroup := setupTestAppWithAuth(t)

	manager := schedule.NewManager(apiTestScheduleExecutor{}, 0)
	ctrl := NewScheduleController(manager)
	ctrl.RegisterRoutes(apiGroup)

	token := loginForToken(t, app, "admin", "admin123")

	invalidReq := httptest.NewRequest("POST", "/api/schedules", bytes.NewBufferString(`{"name":"x","schedule_type":"cron","cron_expr":"bad","target_type":"ai_workflow","workflow_goal":"g"}`))
	invalidReq.Header.Set("Authorization", "Bearer "+token)
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidResp, err := app.Test(invalidReq)
	if err != nil {
		t.Fatalf("invalid create request failed: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", invalidResp.StatusCode)
	}
}
