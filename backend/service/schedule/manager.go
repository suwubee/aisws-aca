package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Executor interface {
	Execute(ctx context.Context, job model.ScheduledJob) (result any, err error)
}

type Manager struct {
	executor     Executor
	pollInterval time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup

	now func() time.Time
}

func NewManager(executor Executor, pollInterval time.Duration) *Manager {
	interval := pollInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Manager{
		executor:     executor,
		pollInterval: interval,
		stopCh:       make(chan struct{}),
		now:          time.Now,
	}
}

func (m *Manager) Start() {
	if m == nil {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		ticker := time.NewTicker(m.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				_, _ = m.TickOnce(context.Background())
			}
		}
	}()
}

func (m *Manager) Stop() {
	if m == nil {
		return
	}
	select {
	case <-m.stopCh:
		// already stopped
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
}

func (m *Manager) TickOnce(ctx context.Context) (int, error) {
	if m == nil {
		return 0, errors.New("manager is nil")
	}
	if model.DB == nil {
		return 0, errors.New("database not initialized")
	}
	if m.executor == nil {
		return 0, errors.New("executor not configured")
	}

	now := m.now().UTC()

	var jobs []model.ScheduledJob
	if err := model.DB.
		Where("enabled = ? AND running = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, false, now).
		Order("next_run_at asc").
		Limit(32).
		Find(&jobs).Error; err != nil {
		return 0, err
	}

	executed := 0
	for _, job := range jobs {
		if ok := m.tryMarkRunning(job.ID, now, true); !ok {
			continue
		}
		executed++
		go m.runJob(context.Background(), job, "scheduler", true)
	}

	return executed, nil
}

func (m *Manager) RunJobNow(ctx context.Context, jobID string) (*model.ScheduledJobRun, error) {
	if m == nil {
		return nil, errors.New("manager is nil")
	}
	if model.DB == nil {
		return nil, errors.New("database not initialized")
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return nil, errors.New("job id is required")
	}

	var job model.ScheduledJob
	if err := model.DB.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}

	now := m.now().UTC()
	if !m.tryMarkRunning(job.ID, now, false) {
		return nil, errors.New("job is already running")
	}

	run := m.runJob(ctx, job, "manual", false)
	return run, nil
}

func (m *Manager) tryMarkRunning(jobID string, now time.Time, requireEnabled bool) bool {
	if m == nil || model.DB == nil {
		return false
	}
	query := model.DB.Model(&model.ScheduledJob{}).Where("id = ? AND running = ?", jobID, false)
	if requireEnabled {
		query = query.Where("enabled = ?", true)
	}

	result := query.Updates(map[string]any{
		"running":       true,
		"running_since": now,
		"updated_at":    now,
	})
	return result.Error == nil && result.RowsAffected == 1
}

func (m *Manager) runJob(ctx context.Context, job model.ScheduledJob, trigger string, advanceSchedule bool) *model.ScheduledJobRun {
	startedAt := m.now().UTC()

	run := &model.ScheduledJobRun{
		ID:        uuid.NewString(),
		JobID:     job.ID,
		Trigger:   trigger,
		StartedAt: startedAt,
		Status:    RunStatusRunning,
		CreatedAt: startedAt,
		UpdatedAt: startedAt,
	}
	_ = model.DB.Create(run).Error

	status := RunStatusSuccess
	var errText string
	var resultText string

	defer func() {
		finishedAt := m.now().UTC()
		run.FinishedAt = &finishedAt
		run.Status = status
		run.Error = errText
		run.Result = resultText
		run.UpdatedAt = finishedAt

		_ = model.DB.Model(&model.ScheduledJobRun{}).Where("id = ?", run.ID).Updates(map[string]any{
			"finished_at": finishedAt,
			"status":      status,
			"error":       errText,
			"result":      resultText,
			"updated_at":  finishedAt,
		}).Error

		updates := map[string]any{
			"running":         false,
			"running_since":   nil,
			"last_run_at":     finishedAt,
			"last_run_status": status,
			"last_run_error":  errText,
			"last_run_result": resultText,
			"updated_at":      finishedAt,
		}

		if advanceSchedule {
			next, err := ComputeNextRunAt(&job, finishedAt)
			if err != nil {
				updates["last_run_error"] = strings.TrimSpace(fmt.Sprintf("%s; next_run_at: %v", errText, err))
				updates["next_run_at"] = nil
				updates["enabled"] = false
			} else if job.ScheduleType == ScheduleTypeOnce {
				updates["next_run_at"] = nil
				updates["enabled"] = false
			} else {
				updates["next_run_at"] = next
			}
		}

		if err := model.DB.Model(&model.ScheduledJob{}).Where("id = ?", job.ID).Updates(updates).Error; err != nil {
			zap.L().Warn("Failed to update scheduled job after run", zap.String("job_id", job.ID), zap.Error(err))
		}
	}()

	if err := ValidateJobConfig(&job); err != nil {
		status = RunStatusFailed
		errText = err.Error()
		return run
	}

	result, err := m.executor.Execute(ctx, job)
	if err != nil {
		status, errText = classifyExecutionError(err)
	}
	if result != nil {
		if encoded, e := json.Marshal(result); e == nil {
			resultText = string(encoded)
		} else {
			resultText = fmt.Sprintf(`{"error":"failed_to_encode_result","detail":%q}`, e.Error())
		}
	}

	return run
}

func classifyExecutionError(err error) (string, string) {
	if err == nil {
		return RunStatusSuccess, ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = "unknown error"
	}
	low := strings.ToLower(msg)
	if strings.Contains(low, "already in progress") || strings.Contains(low, "already running") {
		return RunStatusSkipped, msg
	}
	return RunStatusFailed, msg
}
