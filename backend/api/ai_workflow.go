package api

import (
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/workflow"
	"github.com/gofiber/fiber/v2"
)

var aiWorkflowEngine *workflow.AIWorkflowEngine

// InitAIWorkflowEngine initializes the AI workflow engine
func InitAIWorkflowEngine(toolExecutor *workflow.ToolExecutor) {
	aiWorkflowEngine = workflow.NewAIWorkflowEngine(toolExecutor)
}

// StartAIWorkflow starts an AI-driven workflow
// POST /api/ai-workflow/start
func StartAIWorkflow(c *fiber.Ctx) error {
	var req struct {
		Goal string `json:"goal"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Goal == "" {
		return c.Status(400).JSON(fiber.Map{"error": "goal is required"})
	}

	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	session, err := aiWorkflowEngine.StartWorkflow(c.Context(), req.Goal)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"session_id": session.ID,
		"status":     session.Status,
		"message":    "工作流已启动",
	})
}

// GetAIWorkflowSession gets workflow session status
// GET /api/ai-workflow/session/:id
func GetAIWorkflowSession(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session id required"})
	}

	if aiWorkflowEngine == nil {
		return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
	}

	session, err := aiWorkflowEngine.GetSession(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"session": session})
}

// ListAIWorkflowSessions lists all workflow sessions
// GET /api/ai-workflow/sessions
func ListAIWorkflowSessions(c *fiber.Ctx) error {
	var sessions []model.AIWorkflowSession
	if err := model.DB.Order("started_at DESC").Limit(50).Find(&sessions).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"items": sessions})
}
