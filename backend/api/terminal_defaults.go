package api

import (
	"strings"

	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/service/appsetting"
	"github.com/gofiber/fiber/v2"
)

type TerminalDefaultsController struct{}

func NewTerminalDefaultsController() *TerminalDefaultsController {
	return &TerminalDefaultsController{}
}

type UpdateTerminalDefaultsRequest struct {
	DefaultLoginDir string `json:"default_login_dir"`
}

func (ctrl *TerminalDefaultsController) GetDefaults(c *fiber.Ctx) error {
	dir, err := appsetting.GetTerminalDefaultLoginDir()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load terminal defaults"})
	}
	return c.JSON(fiber.Map{
		"item": fiber.Map{
			"default_login_dir": dir,
		},
	})
}

func (ctrl *TerminalDefaultsController) UpdateDefaults(c *fiber.Ctx) error {
	var req UpdateTerminalDefaultsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	dir := strings.TrimSpace(req.DefaultLoginDir)
	updated, err := appsetting.UpdateTerminalDefaultLoginDir(dir)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Terminal defaults updated",
		"item": fiber.Map{
			"default_login_dir": updated,
		},
	})
}

func (ctrl *TerminalDefaultsController) RegisterRoutes(app fiber.Router) {
	group := app.Group("/terminal-defaults")
	group.Get("/", ctrl.GetDefaults)
	group.Put("/", middleware.RequireRole("admin"), ctrl.UpdateDefaults)
}

