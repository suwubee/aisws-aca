package model

import "time"

type ScheduledJob struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Enabled      bool   `gorm:"default:true;index" json:"enabled"`
	ScheduleType string `gorm:"not null;index" json:"schedule_type"` // cron, once

	// cron schedule
	CronExpr string `gorm:"type:text" json:"cron_expr,omitempty"`
	Timezone string `json:"timezone,omitempty"` // IANA tz name, empty=Local

	// one-off schedule
	RunAt *time.Time `json:"run_at,omitempty"`

	// computed fields
	NextRunAt *time.Time `gorm:"index" json:"next_run_at,omitempty"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`

	LastRunStatus string `json:"last_run_status,omitempty"` // success, failed, skipped
	LastRunError  string `gorm:"type:text" json:"last_run_error,omitempty"`
	LastRunResult string `gorm:"type:text" json:"last_run_result,omitempty"` // JSON string

	Running      bool       `gorm:"default:false;index" json:"running"`
	RunningSince *time.Time `json:"running_since,omitempty"`

	TargetType string  `gorm:"not null;index" json:"target_type"` // task, ai_workflow
	TaskID     *string `gorm:"index" json:"task_id,omitempty"`

	WorkflowGoal string `gorm:"type:text" json:"workflow_goal,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScheduledJobRun struct {
	ID      string `gorm:"primaryKey" json:"id"`
	JobID   string `gorm:"not null;index" json:"job_id"`
	Trigger string `json:"trigger"` // scheduler, manual

	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	Status string `json:"status"` // success, failed, skipped, running
	Error  string `gorm:"type:text" json:"error,omitempty"`
	Result string `gorm:"type:text" json:"result,omitempty"` // JSON string

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
