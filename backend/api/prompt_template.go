package api

import (
	"strings"

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

func (ctrl *PromptTemplateController) RegisterRoutes(app fiber.Router) {
	prompts := app.Group("/prompt-templates")
	prompts.Get("/", ctrl.ListPromptTemplates)
	prompts.Get("/:key", ctrl.GetPromptTemplate)
	prompts.Put("/:key", ctrl.UpdatePromptTemplate)
	prompts.Post("/:key/reset", ctrl.ResetPromptTemplate)
}
