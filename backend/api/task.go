package api

import (
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/task"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskController struct {
	automationService *task.AutomationService
}

func NewTaskController(tm *terminal.Manager) *TaskController {
	return &TaskController{
		automationService: task.NewAutomationService(tm),
	}
}

type CreateTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    int     `json:"priority"`
	Status      string  `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
	ServerID    *string `json:"server_id"`
	// 自动化配置
	WorkDir       string `json:"work_dir"`
	CLIType       string `json:"cli_type"`
	InitialPrompt string `json:"initial_prompt"`
	AutoStart     bool   `json:"auto_start"`
	AutoCreateDir *bool  `json:"auto_create_dir"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *int    `json:"priority"`
	Status      *string `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
	// 自动化配置
	WorkDir       *string `json:"work_dir"`
	CLIType       *string `json:"cli_type"`
	InitialPrompt *string `json:"initial_prompt"`
	AutoStart     *bool   `json:"auto_start"`
	AutoCreateDir *bool   `json:"auto_create_dir"`
}

type MoveTaskRequest struct {
	Status     string  `json:"status"`
	OrderIndex float64 `json:"order_index"`
}

type TaskServerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TaskListItem struct {
	model.Task
	Server *TaskServerInfo `json:"server,omitempty"`
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
		status = "pending"
	}

	cliType := req.CLIType
	if cliType == "" {
		cliType = "claude"
	}

	autoCreateDir := true
	if req.AutoCreateDir != nil {
		autoCreateDir = *req.AutoCreateDir
	}

	var serverID *string
	if req.ServerID != nil {
		trimmed := strings.TrimSpace(*req.ServerID)
		if trimmed != "" {
			var server model.SSHServer
			if err := model.DB.First(&server, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
			}
			serverID = &trimmed
		}
	}

	task := model.Task{
		ID:            uuid.New().String(),
		Title:         req.Title,
		Description:   req.Description,
		Status:        status,
		Priority:      req.Priority,
		RuleSetID:     req.RuleSetID,
		ServerID:      serverID,
		OrderIndex:    float64(time.Now().UnixNano()),
		WorkDir:       req.WorkDir,
		CLIType:       cliType,
		InitialPrompt: req.InitialPrompt,
		AutoStart:     req.AutoStart,
		AutoCreateDir: autoCreateDir,
	}

	if err := model.DB.Create(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create task"})
	}

	return c.Status(201).JSON(fiber.Map{"item": task})
}

func enrichTasksWithServerInfo(tasks []model.Task) ([]TaskListItem, error) {
	serverInfoByID := map[string]*TaskServerInfo{}
	uniqueServerIDs := map[string]struct{}{}
	serverIDs := make([]string, 0)

	for _, t := range tasks {
		if t.ServerID == nil {
			continue
		}
		serverID := strings.TrimSpace(*t.ServerID)
		if serverID == "" {
			continue
		}
		if _, ok := uniqueServerIDs[serverID]; ok {
			continue
		}
		uniqueServerIDs[serverID] = struct{}{}
		serverIDs = append(serverIDs, serverID)
	}

	if len(serverIDs) > 0 {
		var servers []model.SSHServer
		if err := model.DB.Select("id", "name").Where("id IN ?", serverIDs).Find(&servers).Error; err != nil {
			return nil, err
		}

		for _, s := range servers {
			serverInfoByID[s.ID] = &TaskServerInfo{ID: s.ID, Name: s.Name}
		}
	}

	items := make([]TaskListItem, len(tasks))
	for i, t := range tasks {
		item := TaskListItem{Task: t}
		if t.ServerID != nil {
			serverID := strings.TrimSpace(*t.ServerID)
			if serverID != "" {
				item.Server = serverInfoByID[serverID]
			}
		}
		items[i] = item
	}

	return items, nil
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
	if err := query.Order("status, order_index").Find(&tasks).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	items, err := enrichTasksWithServerInfo(tasks)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	return c.JSON(fiber.Map{"items": items})
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

// GetTaskDetail 获取任务完整详情（包含终端、日志、审批）
func (ctrl *TaskController) GetTaskDetail(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	// 获取关联的终端
	var terminals []model.TerminalSession
	model.DB.Where("task_id = ?", id).Order("created_at desc").Find(&terminals)

	// 获取终端ID列表
	terminalIDs := make([]string, len(terminals))
	for i, t := range terminals {
		terminalIDs[i] = t.ID
	}

	// 获取关联的日志（最近100条）
	var logs []model.Log
	if len(terminalIDs) > 0 {
		model.DB.Where("terminal_id IN ?", terminalIDs).
			Order("created_at desc").Limit(100).Find(&logs)
	}

	// 获取关联的审批记录
	var approvals []model.ApprovalRecord
	if len(terminalIDs) > 0 {
		model.DB.Where("terminal_id IN ?", terminalIDs).
			Order("created_at desc").Find(&approvals)
	}

	return c.JSON(fiber.Map{
		"task":      task,
		"terminals": terminals,
		"logs":      logs,
		"approvals": approvals,
	})
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
	// 自动化配置字段
	if req.WorkDir != nil {
		updates["work_dir"] = *req.WorkDir
	}
	if req.CLIType != nil {
		updates["cli_type"] = *req.CLIType
	}
	if req.InitialPrompt != nil {
		updates["initial_prompt"] = *req.InitialPrompt
	}
	if req.AutoStart != nil {
		updates["auto_start"] = *req.AutoStart
	}
	if req.AutoCreateDir != nil {
		updates["auto_create_dir"] = *req.AutoCreateDir
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
	if err := model.DB.Order("order_index").Find(&tasks).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	items, err := enrichTasksWithServerInfo(tasks)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	// 按状态分组
	grouped := map[string][]TaskListItem{
		"todo":        {},
		"in_progress": {},
		"done":        {},
		"archived":    {},
	}

	for _, item := range items {
		if _, ok := grouped[item.Status]; ok {
			grouped[item.Status] = append(grouped[item.Status], item)
		} else {
			grouped["todo"] = append(grouped["todo"], item)
		}
	}

	return c.JSON(fiber.Map{"items": grouped})
}

// StartTask 启动自动化任务
func (ctrl *TaskController) StartTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var taskModel model.Task
	if err := model.DB.First(&taskModel, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	// 如果任务已经在进行中，返回现有终端信息而不是错误
	if taskModel.Status == "in_progress" {
		var terminal model.TerminalSession
		if err := model.DB.Where("task_id = ?", id).Order("created_at desc").First(&terminal).Error; err == nil {
			return c.JSON(fiber.Map{
				"message":     "Task already running",
				"task":        taskModel,
				"terminal_id": terminal.ID,
				"work_dir":    taskModel.WorkDir,
				"cli_started": true,
			})
		}
	}

	result, err := ctrl.automationService.StartTask(&taskModel)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  err.Error(),
			"result": result,
		})
	}

	return c.JSON(fiber.Map{
		"message":     "Task started",
		"task":        result.Task,
		"terminal_id": result.Terminal.ID(),
		"work_dir":    result.WorkDir,
		"cli_started": result.CLIStarted,
	})
}

// GetTaskTerminals 获取任务关联的终端列表
func (ctrl *TaskController) GetTaskTerminals(c *fiber.Ctx) error {
	id := c.Params("id")

	var terminals []model.TerminalSession
	model.DB.Where("task_id = ?", id).Order("created_at desc").Find(&terminals)

	return c.JSON(fiber.Map{"items": terminals})
}

// RegisterRoutes 注册路由
func (ctrl *TaskController) RegisterRoutes(app fiber.Router) {
	tasks := app.Group("/tasks")
	tasks.Get("/", ctrl.ListTasks)
	tasks.Post("/", ctrl.CreateTask)
	tasks.Get("/by-status", ctrl.GetTasksByStatus)
	tasks.Get("/:id", ctrl.GetTask)
	tasks.Get("/:id/detail", ctrl.GetTaskDetail)
	tasks.Put("/:id", ctrl.UpdateTask)
	tasks.Delete("/:id", ctrl.DeleteTask)
	tasks.Post("/:id/move", ctrl.MoveTask)
	tasks.Post("/:id/start", ctrl.StartTask)
	tasks.Get("/:id/terminals", ctrl.GetTaskTerminals)
}
