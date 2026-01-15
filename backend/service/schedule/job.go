package schedule

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
)

const (
	ScheduleTypeCron = "cron"
	ScheduleTypeOnce = "once"

	TargetTypeTask       = "task"
	TargetTypeAIWorkflow = "ai_workflow"
)

const (
	RunStatusRunning = "running"
	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"
	RunStatusSkipped = "skipped"
)

func NormalizeScheduleType(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", false
	case ScheduleTypeCron:
		return ScheduleTypeCron, true
	case ScheduleTypeOnce:
		return ScheduleTypeOnce, true
	default:
		return "", false
	}
}

func NormalizeTargetType(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", false
	case TargetTypeTask:
		return TargetTypeTask, true
	case TargetTypeAIWorkflow:
		return TargetTypeAIWorkflow, true
	default:
		return "", false
	}
}

func ValidateJobConfig(job *model.ScheduledJob) error {
	if job == nil {
		return errors.New("job is nil")
	}

	name := strings.TrimSpace(job.Name)
	if name == "" {
		return errors.New("name is required")
	}

	if _, ok := NormalizeScheduleType(job.ScheduleType); !ok {
		return errors.New("invalid schedule_type")
	}
	if _, ok := NormalizeTargetType(job.TargetType); !ok {
		return errors.New("invalid target_type")
	}

	switch job.ScheduleType {
	case ScheduleTypeCron:
		if strings.TrimSpace(job.CronExpr) == "" {
			return errors.New("cron_expr is required for cron schedule")
		}
		if _, err := ParseCronExpression(job.CronExpr); err != nil {
			return fmt.Errorf("invalid cron_expr: %w", err)
		}
		if _, err := loadLocation(job.Timezone); err != nil {
			return err
		}
	case ScheduleTypeOnce:
		if job.RunAt == nil {
			return errors.New("run_at is required for once schedule")
		}
	default:
		return errors.New("invalid schedule_type")
	}

	switch job.TargetType {
	case TargetTypeTask:
		if job.TaskID == nil || strings.TrimSpace(*job.TaskID) == "" {
			return errors.New("task_id is required for task target")
		}
	case TargetTypeAIWorkflow:
		if strings.TrimSpace(job.WorkflowGoal) == "" {
			return errors.New("workflow_goal is required for ai_workflow target")
		}
	default:
		return errors.New("invalid target_type")
	}

	return nil
}

func ComputeNextRunAt(job *model.ScheduledJob, from time.Time) (*time.Time, error) {
	if job == nil {
		return nil, errors.New("job is nil")
	}

	now := from.UTC()
	switch job.ScheduleType {
	case ScheduleTypeOnce:
		if job.RunAt == nil {
			return nil, errors.New("run_at is required")
		}
		runAt := job.RunAt.UTC()
		return &runAt, nil
	case ScheduleTypeCron:
		loc, err := loadLocation(job.Timezone)
		if err != nil {
			return nil, err
		}
		sched, err := ParseCronExpression(job.CronExpr)
		if err != nil {
			return nil, err
		}
		nextLocal, err := sched.Next(now, loc)
		if err != nil {
			return nil, err
		}
		nextUTC := nextLocal.UTC()
		return &nextUTC, nil
	default:
		return nil, errors.New("invalid schedule_type")
	}
}

func loadLocation(name string) (*time.Location, error) {
	tz := strings.TrimSpace(name)
	if tz == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q", tz)
	}
	return loc, nil
}
