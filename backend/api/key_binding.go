package api

import (
	"strings"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/service/keybinding"
	"github.com/gofiber/fiber/v2"
)

type KeyBindingController struct{}

func NewKeyBindingController() *KeyBindingController {
	return &KeyBindingController{}
}

func (ctrl *KeyBindingController) ListKeyBindings(c *fiber.Ctx) error {
	items, err := keybinding.List()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list key bindings"})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (ctrl *KeyBindingController) GetKeyBinding(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	item, err := keybinding.Get(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"item": item})
}

func (ctrl *KeyBindingController) UpdateKeyBinding(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	var req keybinding.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	item, err := keybinding.Update(id, req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Key binding updated", "item": item})
}

func (ctrl *KeyBindingController) ResetKeyBinding(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	item, err := keybinding.ResetToDefault(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Key binding reset", "item": item})
}

func (ctrl *KeyBindingController) RegisterRoutes(app fiber.Router) {
	group := app.Group("/key-bindings")
	group.Get("/", ctrl.ListKeyBindings)
	group.Get("/:id", ctrl.GetKeyBinding)

	admin := group.Group("", middleware.RequireRole("admin"))
	admin.Put("/:id", ctrl.UpdateKeyBinding)
	admin.Post("/:id/reset", ctrl.ResetKeyBinding)
}
