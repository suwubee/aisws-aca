package schedule

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
)

type fakeExecutor struct {
	result any
	err    error
}

func (f fakeExecutor) Execute(ctx context.Context, job model.ScheduledJob) (any, error) {
	return f.result, f.err
}

func TestManager_runJob_DisablesOnceScheduleAfterSchedulerRun(t *testing.T) {
	dsn := fmt.Sprintf("file:schedule_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	runAt := now.Add(10 * time.Minute)
	nextRunAt := now.Add(-1 * time.Minute)

	taskID := uuid.NewString()
	job := model.ScheduledJob{
		ID:           uuid.NewString(),
		Name:         "once job",
		Enabled:      true,
		ScheduleType: ScheduleTypeOnce,
		RunAt:        &runAt,
		NextRunAt:    &nextRunAt,
		TargetType:   TargetTypeTask,
		TaskID:       &taskID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := model.DB.Create(&job).Error; err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	m := NewManager(fakeExecutor{result: map[string]any{"ok": true}}, 0)
	m.now = func() time.Time { return now }

	if ok := m.tryMarkRunning(job.ID, now, true); !ok {
		t.Fatalf("expected job to be marked running")
	}
	m.runJob(context.Background(), job, "scheduler", true)

	var got model.ScheduledJob
	if err := model.DB.First(&got, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("query job failed: %v", err)
	}
	if got.Enabled {
		t.Fatalf("expected job to be disabled after once run")
	}
	if got.NextRunAt != nil {
		t.Fatalf("expected next_run_at to be cleared after once run")
	}
	if got.Running {
		t.Fatalf("expected running=false after run")
	}
	if got.LastRunStatus != RunStatusSuccess {
		t.Fatalf("expected last_run_status=%q, got %q", RunStatusSuccess, got.LastRunStatus)
	}

	var runs []model.ScheduledJobRun
	if err := model.DB.Where("job_id = ?", job.ID).Find(&runs).Error; err != nil {
		t.Fatalf("query runs failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Trigger != "scheduler" {
		t.Fatalf("expected trigger scheduler, got %q", runs[0].Trigger)
	}
	if runs[0].Status != RunStatusSuccess {
		t.Fatalf("expected run status %q, got %q", RunStatusSuccess, runs[0].Status)
	}
}

func TestManager_runJob_AdvancesCronNextRunAt(t *testing.T) {
	dsn := fmt.Sprintf("file:schedule_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nextRunAt := now.Add(-1 * time.Minute)
	goal := "do something"
	job := model.ScheduledJob{
		ID:           uuid.NewString(),
		Name:         "cron job",
		Enabled:      true,
		ScheduleType: ScheduleTypeCron,
		CronExpr:     "*/5 * * * *",
		Timezone:     "UTC",
		NextRunAt:    &nextRunAt,
		TargetType:   TargetTypeAIWorkflow,
		WorkflowGoal: goal,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := model.DB.Create(&job).Error; err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	m := NewManager(fakeExecutor{result: map[string]any{"session_id": "s1"}}, 0)
	m.now = func() time.Time { return now }

	if ok := m.tryMarkRunning(job.ID, now, true); !ok {
		t.Fatalf("expected job to be marked running")
	}
	m.runJob(context.Background(), job, "scheduler", true)

	var got model.ScheduledJob
	if err := model.DB.First(&got, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("query job failed: %v", err)
	}
	if got.NextRunAt == nil {
		t.Fatalf("expected next_run_at to be set")
	}
	wantNext := time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC)
	if !got.NextRunAt.UTC().Equal(wantNext) {
		t.Fatalf("unexpected next_run_at.\nwant: %s\ngot:  %s", wantNext, got.NextRunAt.UTC())
	}
}
