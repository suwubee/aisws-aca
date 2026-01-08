package api

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	sshservice "github.com/ai-coding-assistant/service/ssh"
	taskservice "github.com/ai-coding-assistant/service/task"
	"github.com/ai-coding-assistant/service/terminal"
	workflowservice "github.com/ai-coding-assistant/service/workflow"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkflowController struct {
	engine *workflowservice.WorkflowEngine
}

func NewWorkflowController(jwtSecret string, terminalMgr *terminal.Manager) *WorkflowController {
	sshManager := sshservice.NewSSHManager(jwtSecret)

	var automation *taskservice.AutomationService
	if terminalMgr != nil {
		automation = taskservice.NewAutomationService(terminalMgr)
	}

	return &WorkflowController{
		engine: workflowservice.NewWorkflowEngine(sshManager, automation, workflowservice.NewTerminalManagerAdapter(terminalMgr)),
	}
}

type CreateWorkflowRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ProjectID   *string `json:"project_id"`
	Nodes       string  `json:"nodes"` // JSON string
	Edges       string  `json:"edges"` // JSON string
	Status      string  `json:"status"`
}

type UpdateWorkflowRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ProjectID   *string `json:"project_id"`
	Nodes       *string `json:"nodes"` // JSON string
	Edges       *string `json:"edges"` // JSON string
	Status      *string `json:"status"`
}

func normalizeJSONString(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "[]", nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", err
	}

	return trimmed, nil
}

// ListWorkflows 获取工作流列表
func (ctrl *WorkflowController) ListWorkflows(c *fiber.Ctx) error {
	var workflows []model.Workflow
	if err := model.DB.Order("created_at desc").Find(&workflows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list workflows"})
	}

	return c.JSON(fiber.Map{"items": workflows})
}

// CreateWorkflow 创建工作流
func (ctrl *WorkflowController) CreateWorkflow(c *fiber.Ctx) error {
	var req CreateWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	nodes, err := normalizeJSONString(req.Nodes)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid nodes JSON"})
	}

	edges, err := normalizeJSONString(req.Edges)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid edges JSON"})
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "draft"
	}

	var projectID *string
	if req.ProjectID != nil {
		trimmed := strings.TrimSpace(*req.ProjectID)
		if trimmed != "" {
			var project model.Project
			if err := model.DB.Select("id").First(&project, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Project not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query project"})
			}
			projectID = &trimmed
		}
	}

	workflow := model.Workflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: req.Description,
		ProjectID:   projectID,
		Nodes:       nodes,
		Edges:       edges,
		Status:      status,
	}

	if err := model.DB.Create(&workflow).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create workflow"})
	}

	return c.Status(201).JSON(fiber.Map{"item": workflow})
}

// GetWorkflow 获取工作流详情
func (ctrl *WorkflowController) GetWorkflow(c *fiber.Ctx) error {
	id := c.Params("id")

	var workflow model.Workflow
	if err := model.DB.First(&workflow, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Workflow not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query workflow"})
	}

	return c.JSON(fiber.Map{"item": workflow})
}

// UpdateWorkflow 更新工作流
func (ctrl *WorkflowController) UpdateWorkflow(c *fiber.Ctx) error {
	id := c.Params("id")

	var workflow model.Workflow
	if err := model.DB.First(&workflow, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Workflow not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query workflow"})
	}

	var req UpdateWorkflowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := make(map[string]interface{})

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ProjectID != nil {
		projectID := strings.TrimSpace(*req.ProjectID)
		if projectID == "" {
			updates["project_id"] = nil
		} else {
			var project model.Project
			if err := model.DB.Select("id").First(&project, "id = ?", projectID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Project not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query project"})
			}
			updates["project_id"] = projectID
		}
	}
	if req.Nodes != nil {
		nodes, err := normalizeJSONString(*req.Nodes)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid nodes JSON"})
		}
		updates["nodes"] = nodes
	}
	if req.Edges != nil {
		edges, err := normalizeJSONString(*req.Edges)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid edges JSON"})
		}
		updates["edges"] = edges
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status == "" {
			status = "draft"
		}
		updates["status"] = status
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := model.DB.Model(&workflow).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update workflow"})
		}
		model.DB.First(&workflow, "id = ?", id)
	}

	return c.JSON(fiber.Map{"item": workflow})
}

// DeleteWorkflow 删除工作流
func (ctrl *WorkflowController) DeleteWorkflow(c *fiber.Ctx) error {
	id := c.Params("id")

	tx := model.DB.Begin()
	if tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete workflow"})
	}

	result := tx.Delete(&model.Workflow{}, "id = ?", id)
	if result.Error != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete workflow"})
	}
	if result.RowsAffected == 0 {
		_ = tx.Rollback()
		return c.Status(404).JSON(fiber.Map{"error": "Workflow not found"})
	}

	if err := tx.Where("workflow_id = ?", id).Delete(&model.WorkflowNode{}).Error; err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete workflow"})
	}
	if err := tx.Where("workflow_id = ?", id).Delete(&model.WorkflowRun{}).Error; err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete workflow"})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete workflow"})
	}

	return c.JSON(fiber.Map{"message": "Workflow deleted"})
}

// StartWorkflowRun 启动工作流执行
func (ctrl *WorkflowController) StartWorkflowRun(c *fiber.Ctx) error {
	id := c.Params("id")

	if ctrl.engine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Workflow engine not configured"})
	}

	run, err := ctrl.engine.RunWorkflow(id)
	if err != nil {
		if errors.Is(err, workflowservice.ErrWorkflowNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Workflow not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to start workflow"})
	}

	return c.Status(201).JSON(fiber.Map{"item": run})
}

// ListWorkflowRuns 获取工作流执行历史
func (ctrl *WorkflowController) ListWorkflowRuns(c *fiber.Ctx) error {
	id := c.Params("id")

	var workflow model.Workflow
	if err := model.DB.Select("id").First(&workflow, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Workflow not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query workflow"})
	}

	var runs []model.WorkflowRun
	if err := model.DB.Where("workflow_id = ?", id).Order("started_at desc").Find(&runs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list workflow runs"})
	}

	return c.JSON(fiber.Map{"items": runs})
}

// RegisterRoutes 注册路由
func (ctrl *WorkflowController) RegisterRoutes(app fiber.Router) {
	workflows := app.Group("/workflows")
	workflows.Get("/", ctrl.ListWorkflows)
	workflows.Post("/", ctrl.CreateWorkflow)
	workflows.Post("/:id/run", ctrl.StartWorkflowRun)
	workflows.Get("/:id/runs", ctrl.ListWorkflowRuns)
	workflows.Get("/:id", ctrl.GetWorkflow)
	workflows.Put("/:id", ctrl.UpdateWorkflow)
	workflows.Delete("/:id", ctrl.DeleteWorkflow)
}
