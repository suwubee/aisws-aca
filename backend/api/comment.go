package api

import (
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CommentController struct{}

func NewCommentController() *CommentController {
	return &CommentController{}
}

type CreateCommentRequest struct {
	Content string `json:"content"`
	Author  string `json:"author"`
}

// ListTaskComments 获取任务评论列表
func (ctrl *CommentController) ListTaskComments(c *fiber.Ctx) error {
	taskID := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query task"})
	}

	order := strings.ToLower(c.Query("order", "asc"))
	if order != "asc" && order != "desc" {
		order = "asc"
	}

	query := model.DB.Model(&model.Comment{}).Where("task_id = ?", taskID)
	if order == "desc" {
		query = query.Order("created_at desc")
	} else {
		query = query.Order("created_at asc")
	}

	var comments []model.Comment
	if err := query.Find(&comments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list comments"})
	}

	return c.JSON(fiber.Map{"items": comments})
}

// CreateTaskComment 添加评论
func (ctrl *CommentController) CreateTaskComment(c *fiber.Ctx) error {
	taskID := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query task"})
	}

	var req CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Content is required"})
	}

	author, _ := c.Locals("username").(string)
	if author == "" {
		author = strings.TrimSpace(req.Author)
	}
	if author == "" {
		author = "unknown"
	}

	comment := model.Comment{
		ID:      uuid.New().String(),
		TaskID:  taskID,
		Content: content,
		Author:  author,
	}

	if err := model.DB.Create(&comment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create comment"})
	}

	return c.Status(201).JSON(fiber.Map{"item": comment})
}

// UpdateComment 更新评论
func (ctrl *CommentController) UpdateComment(c *fiber.Ctx) error {
	id := c.Params("id")

	var comment model.Comment
	if err := model.DB.First(&comment, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Comment not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query comment"})
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Content is required"})
	}

	if err := model.DB.Model(&comment).Updates(map[string]interface{}{
		"content":    content,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update comment"})
	}

	model.DB.First(&comment, "id = ?", id)
	return c.JSON(fiber.Map{"item": comment})
}

// DeleteComment 删除评论
func (ctrl *CommentController) DeleteComment(c *fiber.Ctx) error {
	id := c.Params("id")

	result := model.DB.Delete(&model.Comment{}, "id = ?", id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete comment"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Comment not found"})
	}

	return c.JSON(fiber.Map{"message": "Comment deleted"})
}

// RegisterRoutes 注册路由
func (ctrl *CommentController) RegisterRoutes(app fiber.Router) {
	tasks := app.Group("/tasks")
	tasks.Get("/:id/comments", ctrl.ListTaskComments)
	tasks.Post("/:id/comments", ctrl.CreateTaskComment)

	comments := app.Group("/comments")
	comments.Put("/:id", ctrl.UpdateComment)
	comments.Delete("/:id", ctrl.DeleteComment)
}
