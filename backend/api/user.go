package api

import (
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UserController struct{}

func NewUserController() *UserController {
	return &UserController{}
}

type UpdateUserRequest struct {
	Role   *string `json:"role"`
	Status *string `json:"status"`
}

func isValidUserRole(role string) bool {
	switch role {
	case "admin", "user", "viewer":
		return true
	default:
		return false
	}
}

func isValidUserStatus(status string) bool {
	switch status {
	case "active", "disabled":
		return true
	default:
		return false
	}
}

// ListUsers 获取用户列表（管理员）
func (ctrl *UserController) ListUsers(c *fiber.Ctx) error {
	var users []model.User
	if err := model.DB.Order("created_at desc").Find(&users).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list users"})
	}
	return c.JSON(fiber.Map{"items": users})
}

// UpdateUser 更新用户信息（管理员）
func (ctrl *UserController) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")

	var user model.User
	if err := model.DB.First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query user"})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := make(map[string]interface{})

	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if role == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid role"})
		}
		if !isValidUserRole(role) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid role"})
		}
		updates["role"] = role
	}

	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if status == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
		}
		if !isValidUserStatus(status) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
		}
		updates["status"] = status
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := model.DB.Model(&user).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update user"})
		}
	}

	model.DB.First(&user, "id = ?", id)
	return c.JSON(fiber.Map{"item": user})
}

// RegisterRoutes 注册路由
func (ctrl *UserController) RegisterRoutes(app fiber.Router) {
	users := app.Group("/users")
	users.Get("/", ctrl.ListUsers)
	users.Put("/:id", ctrl.UpdateUser)
}
