package api

import (
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type TaskController struct{}

func NewTaskController() *TaskController {
	return &TaskController{}
}

type CreateTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    int     `json:"priority"`
	Status      string  `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *int    `json:"priority"`
	Status      *string `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
}

type MoveTaskRequest struct {
	Status     string  `json:"status"`
	OrderIndex float64 `json:"order_index"`
}

// CreateTask 创建任务
func (ctrl *TaskController) CreateTask(c *fiber.Ctx) error {
	var req CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required"})
	}

	status := req.Status
	if status == "" {
		status = "todo"
	}

	task := model.Task{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		Priority:    req.Priority,
		RuleSetID:   req.RuleSetID,
		OrderIndex:  float64(time.Now().UnixNano()),
	}

	if err := model.DB.Create(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create task"})
	}

	return c.Status(201).JSON(fiber.Map{"item": task})
}

// ListTasks 获取任务列表
func (ctrl *TaskController) ListTasks(c *fiber.Ctx) error {
	status := c.Query("status")
	priority := c.Query("priority")
	keyword := c.Query("keyword")

	query := model.DB.Model(&model.Task{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var tasks []model.Task
	query.Order("status, order_index").Find(&tasks)

	return c.JSON(fiber.Map{"items": tasks})
}

// GetTask 获取任务详情
func (ctrl *TaskController) GetTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	return c.JSON(fiber.Map{"item": task})
}

// UpdateTask 更新任务
func (ctrl *TaskController) UpdateTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	var req UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Status != nil {
		updates["status"] = *req.Status
		if *req.Status == "done" {
			now := time.Now()
			updates["completed_at"] = now
		}
	}
	if req.RuleSetID != nil {
		if *req.RuleSetID == "" {
			updates["rule_set_id"] = nil
		} else {
			updates["rule_set_id"] = *req.RuleSetID
		}
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		model.DB.Model(&task).Updates(updates)
	}

	model.DB.First(&task, "id = ?", id)
	return c.JSON(fiber.Map{"item": task})
}

// DeleteTask 删除任务
func (ctrl *TaskController) DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")

	result := model.DB.Delete(&model.Task{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	return c.JSON(fiber.Map{"message": "Task deleted"})
}

// MoveTask 移动任务（拖拽）
func (ctrl *TaskController) MoveTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	var req MoveTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := map[string]interface{}{
		"status":      req.Status,
		"order_index": req.OrderIndex,
		"updated_at":  time.Now(),
	}

	if req.Status == "done" {
		now := time.Now()
		updates["completed_at"] = now
	}

	model.DB.Model(&task).Updates(updates)
	model.DB.First(&task, "id = ?", id)

	return c.JSON(fiber.Map{"item": task})
}

// GetTasksByStatus 按状态获取任务（用于Kanban）
func (ctrl *TaskController) GetTasksByStatus(c *fiber.Ctx) error {
	var tasks []model.Task
	model.DB.Order("order_index").Find(&tasks)

	// 按状态分组
	grouped := map[string][]model.Task{
		"todo":        {},
		"in_progress": {},
		"done":        {},
		"archived":    {},
	}

	for _, task := range tasks {
		if _, ok := grouped[task.Status]; ok {
			grouped[task.Status] = append(grouped[task.Status], task)
		} else {
			grouped["todo"] = append(grouped["todo"], task)
		}
	}

	return c.JSON(fiber.Map{"items": grouped})
}

// RegisterRoutes 注册路由
func (ctrl *TaskController) RegisterRoutes(app fiber.Router) {
	tasks := app.Group("/tasks")
	tasks.Get("/", ctrl.ListTasks)
	tasks.Post("/", ctrl.CreateTask)
	tasks.Get("/by-status", ctrl.GetTasksByStatus)
	tasks.Get("/:id", ctrl.GetTask)
	tasks.Put("/:id", ctrl.UpdateTask)
	tasks.Delete("/:id", ctrl.DeleteTask)
	tasks.Post("/:id/move", ctrl.MoveTask)
}
