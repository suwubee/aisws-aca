package api

import (
	"strings"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/service/prompt"
	"github.com/gofiber/fiber/v2"
)

type PromptTemplateController struct{}

func NewPromptTemplateController() *PromptTemplateController {
	return &PromptTemplateController{}
}

type UpdatePromptTemplateRequest struct {
	Template string `json:"template"`
}

type CreatePromptTemplatePresetRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

func (ctrl *PromptTemplateController) ListPromptTemplates(c *fiber.Ctx) error {
	items, err := prompt.ListTemplates()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list prompt templates"})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (ctrl *PromptTemplateController) GetPromptTemplate(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	item, err := prompt.GetTemplate(key)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"item": item})
}

func (ctrl *PromptTemplateController) UpdatePromptTemplate(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	var req UpdatePromptTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	item, err := prompt.UpdateTemplate(key, req.Template)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Prompt template updated", "item": item})
}

func (ctrl *PromptTemplateController) ResetPromptTemplate(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	item, err := prompt.ResetTemplateToDefault(key)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Prompt template reset", "item": item})
}

func (ctrl *PromptTemplateController) ListPromptTemplatePresets(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	items, err := prompt.ListPresets(key)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (ctrl *PromptTemplateController) CreatePromptTemplatePreset(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	var req CreatePromptTemplatePresetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	item, err := prompt.CreatePreset(key, prompt.CreatePresetRequest{
		Name:        req.Name,
		Description: req.Description,
		Template:    req.Template,
	})
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Prompt template preset created", "item": item})
}

func (ctrl *PromptTemplateController) DeletePromptTemplatePreset(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	id := strings.TrimSpace(c.Params("id"))
	if err := prompt.DeletePreset(key, id); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Prompt template preset deleted"})
}

func (ctrl *PromptTemplateController) ApplyPromptTemplatePreset(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	id := strings.TrimSpace(c.Params("id"))
	item, preset, err := prompt.ApplyPreset(key, id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"message": "Prompt template preset applied",
		"item":    item,
		"preset":  preset,
	})
}

func (ctrl *PromptTemplateController) RegisterRoutes(app fiber.Router) {
	prompts := app.Group("/prompt-templates")
	prompts.Get("/", ctrl.ListPromptTemplates)
	prompts.Get("/:key", ctrl.GetPromptTemplate)
	prompts.Get("/:key/presets", ctrl.ListPromptTemplatePresets)

	// Admin-only write operations.
	admin := prompts.Group("", middleware.RequireRole("admin"))
	admin.Put("/:key", ctrl.UpdatePromptTemplate)
	admin.Post("/:key/reset", ctrl.ResetPromptTemplate)
	admin.Post("/:key/presets", ctrl.CreatePromptTemplatePreset)
	admin.Delete("/:key/presets/:id", ctrl.DeletePromptTemplatePreset)
	admin.Post("/:key/presets/:id/apply", ctrl.ApplyPromptTemplatePreset)
}
