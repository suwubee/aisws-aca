package api

import (
	"errors"
	"strings"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkflowTemplateController struct{}

func NewWorkflowTemplateController() *WorkflowTemplateController {
	return &WorkflowTemplateController{}
}

type CreateWorkflowTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Nodes       string `json:"nodes"` // JSON string
	Edges       string `json:"edges"` // JSON string
}

type ApplyWorkflowTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func isValidWorkflowTemplateCategory(category string) bool {
	switch category {
	case model.WorkflowTemplateCategoryDevelopment,
		model.WorkflowTemplateCategoryDevOps,
		model.WorkflowTemplateCategoryDocumentation,
		model.WorkflowTemplateCategoryTesting:
		return true
	default:
		return false
	}
}

// ListWorkflowTemplates 获取模板列表
func (ctrl *WorkflowTemplateController) ListWorkflowTemplates(c *fiber.Ctx) error {
	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	if category != "" && !isValidWorkflowTemplateCategory(category) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid category"})
	}

	query := model.DB.Model(&model.WorkflowTemplate{})
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var templates []model.WorkflowTemplate
	if err := query.Order("is_builtin desc").Order("created_at desc").Find(&templates).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list workflow templates"})
	}

	return c.JSON(fiber.Map{"items": templates})
}

// CreateWorkflowTemplate 创建自定义模板
func (ctrl *WorkflowTemplateController) CreateWorkflowTemplate(c *fiber.Ctx) error {
	var req CreateWorkflowTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	category := strings.ToLower(strings.TrimSpace(req.Category))
	if category == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Category is required"})
	}
	if !isValidWorkflowTemplateCategory(category) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid category"})
	}

	nodes, err := normalizeJSONString(req.Nodes)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid nodes JSON"})
	}

	edges, err := normalizeJSONString(req.Edges)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid edges JSON"})
	}

	template := model.WorkflowTemplate{
		ID:          uuid.New().String(),
		Name:        name,
		Description: req.Description,
		Category:    category,
		Nodes:       nodes,
		Edges:       edges,
		IsBuiltin:   false,
	}

	if err := model.DB.Create(&template).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create workflow template"})
	}

	return c.Status(201).JSON(fiber.Map{"item": template})
}

// ApplyWorkflowTemplate 由模板创建工作流
func (ctrl *WorkflowTemplateController) ApplyWorkflowTemplate(c *fiber.Ctx) error {
	id := c.Params("id")

	var template model.WorkflowTemplate
	if err := model.DB.First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Workflow template not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query workflow template"})
	}

	var req ApplyWorkflowTemplateRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = template.Name
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = template.Description
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "draft"
	}

	workflow := model.Workflow{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Nodes:       template.Nodes,
		Edges:       template.Edges,
		Status:      status,
	}

	if err := model.DB.Create(&workflow).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create workflow"})
	}

	return c.Status(201).JSON(fiber.Map{"item": workflow})
}

// RegisterRoutes 注册路由
func (ctrl *WorkflowTemplateController) RegisterRoutes(app fiber.Router) {
	templates := app.Group("/workflow-templates")
	templates.Get("/", ctrl.ListWorkflowTemplates)
	templates.Post("/", ctrl.CreateWorkflowTemplate)
	templates.Post("/:id/apply", ctrl.ApplyWorkflowTemplate)
}
