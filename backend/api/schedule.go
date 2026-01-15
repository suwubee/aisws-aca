package api

import (
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/schedule"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ScheduleController struct {
	manager *schedule.Manager
}

func NewScheduleController(manager *schedule.Manager) *ScheduleController {
	return &ScheduleController{manager: manager}
}

type CreateScheduledJobRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`

	ScheduleType string     `json:"schedule_type"` // cron, once
	CronExpr     string     `json:"cron_expr"`
	Timezone     string     `json:"timezone"`
	RunAt        *time.Time `json:"run_at"`

	TargetType   string  `json:"target_type"` // task, ai_workflow
	TaskID       *string `json:"task_id"`
	WorkflowGoal string  `json:"workflow_goal"`
}

type UpdateScheduledJobRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`

	ScheduleType *string    `json:"schedule_type"`
	CronExpr     *string    `json:"cron_expr"`
	Timezone     *string    `json:"timezone"`
	RunAt        *time.Time `json:"run_at"`

	TargetType   *string `json:"target_type"`
	TaskID       *string `json:"task_id"`
	WorkflowGoal *string `json:"workflow_goal"`
}

// ListScheduledJobs GET /api/schedules
func (ctrl *ScheduleController) ListScheduledJobs(c *fiber.Ctx) error {
	var jobs []model.ScheduledJob
	if err := model.DB.Order("created_at desc").Find(&jobs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list schedules"})
	}
	return c.JSON(fiber.Map{"items": jobs})
}

// GetScheduledJob GET /api/schedules/:id
func (ctrl *ScheduleController) GetScheduledJob(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing schedule id"})
	}

	var job model.ScheduledJob
	if err := model.DB.First(&job, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Schedule not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query schedule"})
	}

	return c.JSON(fiber.Map{"item": job})
}

// CreateScheduledJob POST /api/schedules
func (ctrl *ScheduleController) CreateScheduledJob(c *fiber.Ctx) error {
	var req CreateScheduledJobRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	scheduleType, ok := schedule.NormalizeScheduleType(req.ScheduleType)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid schedule_type"})
	}
	targetType, ok := schedule.NormalizeTargetType(req.TargetType)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target_type"})
	}

	now := time.Now().UTC()
	job := model.ScheduledJob{
		ID:           uuid.NewString(),
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Enabled:      enabled,
		ScheduleType: scheduleType,
		CronExpr:     strings.TrimSpace(req.CronExpr),
		Timezone:     strings.TrimSpace(req.Timezone),
		RunAt:        req.RunAt,
		TargetType:   targetType,
		TaskID:       req.TaskID,
		WorkflowGoal: strings.TrimSpace(req.WorkflowGoal),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if job.RunAt != nil {
		runAt := job.RunAt.UTC()
		job.RunAt = &runAt
	}
	if job.TaskID != nil {
		trimmed := strings.TrimSpace(*job.TaskID)
		if trimmed == "" {
			job.TaskID = nil
		} else {
			job.TaskID = &trimmed
		}
	}

	if err := schedule.ValidateJobConfig(&job); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	next, err := schedule.ComputeNextRunAt(&job, now)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	job.NextRunAt = next

	if err := model.DB.Create(&job).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create schedule"})
	}

	return c.Status(201).JSON(fiber.Map{"item": job})
}

// UpdateScheduledJob PUT /api/schedules/:id
func (ctrl *ScheduleController) UpdateScheduledJob(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing schedule id"})
	}

	var job model.ScheduledJob
	if err := model.DB.First(&job, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Schedule not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query schedule"})
	}

	var req UpdateScheduledJobRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := map[string]any{}
	now := time.Now().UTC()

	if req.Name != nil {
		job.Name = strings.TrimSpace(*req.Name)
		updates["name"] = job.Name
	}
	if req.Description != nil {
		job.Description = strings.TrimSpace(*req.Description)
		updates["description"] = job.Description
	}
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
		updates["enabled"] = job.Enabled
	}

	if req.ScheduleType != nil {
		if normalized, ok := schedule.NormalizeScheduleType(*req.ScheduleType); ok {
			job.ScheduleType = normalized
			updates["schedule_type"] = normalized
		} else {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid schedule_type"})
		}
	}
	if req.CronExpr != nil {
		job.CronExpr = strings.TrimSpace(*req.CronExpr)
		updates["cron_expr"] = job.CronExpr
	}
	if req.Timezone != nil {
		job.Timezone = strings.TrimSpace(*req.Timezone)
		updates["timezone"] = job.Timezone
	}
	if req.RunAt != nil {
		runAt := req.RunAt.UTC()
		job.RunAt = &runAt
		updates["run_at"] = job.RunAt
	}

	if req.TargetType != nil {
		if normalized, ok := schedule.NormalizeTargetType(*req.TargetType); ok {
			job.TargetType = normalized
			updates["target_type"] = normalized
		} else {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid target_type"})
		}
	}
	if req.TaskID != nil {
		trimmed := strings.TrimSpace(*req.TaskID)
		if trimmed == "" {
			job.TaskID = nil
		} else {
			job.TaskID = &trimmed
		}
		updates["task_id"] = job.TaskID
	}
	if req.WorkflowGoal != nil {
		job.WorkflowGoal = strings.TrimSpace(*req.WorkflowGoal)
		updates["workflow_goal"] = job.WorkflowGoal
	}

	if err := schedule.ValidateJobConfig(&job); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	next, err := schedule.ComputeNextRunAt(&job, now)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	job.NextRunAt = next
	updates["next_run_at"] = next

	updates["updated_at"] = now
	if err := model.DB.Model(&model.ScheduledJob{}).Where("id = ?", job.ID).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update schedule"})
	}

	model.DB.First(&job, "id = ?", job.ID)
	return c.JSON(fiber.Map{"item": job})
}

// DeleteScheduledJob DELETE /api/schedules/:id
func (ctrl *ScheduleController) DeleteScheduledJob(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing schedule id"})
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", id).Delete(&model.ScheduledJobRun{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.ScheduledJob{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Schedule not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete schedule"})
	}

	return c.JSON(fiber.Map{"message": "Schedule deleted"})
}

// RunScheduledJobNow POST /api/schedules/:id/run
func (ctrl *ScheduleController) RunScheduledJobNow(c *fiber.Ctx) error {
	if ctrl.manager == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Scheduler not configured"})
	}
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing schedule id"})
	}

	run, err := ctrl.manager.RunJobNow(c.Context(), id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	var job model.ScheduledJob
	_ = model.DB.First(&job, "id = ?", id).Error

	return c.JSON(fiber.Map{"message": "Job executed", "run": run, "job": job})
}

// ListScheduledJobRuns GET /api/schedules/:id/runs
func (ctrl *ScheduleController) ListScheduledJobRuns(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing schedule id"})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	query := model.DB.Model(&model.ScheduledJobRun{}).Where("job_id = ?", id).Order("started_at desc")
	var total int64
	query.Count(&total)

	var items []model.ScheduledJobRun
	if err := query.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list schedule runs"})
	}

	return c.JSON(fiber.Map{"items": items, "total": total})
}

func (ctrl *ScheduleController) RegisterRoutes(app fiber.Router) {
	group := app.Group("/schedules")
	group.Get("/", ctrl.ListScheduledJobs)
	group.Post("/", ctrl.CreateScheduledJob)
	group.Get("/:id", ctrl.GetScheduledJob)
	group.Put("/:id", ctrl.UpdateScheduledJob)
	group.Delete("/:id", ctrl.DeleteScheduledJob)
	group.Post("/:id/run", ctrl.RunScheduledJobNow)
	group.Get("/:id/runs", ctrl.ListScheduledJobRuns)
}
