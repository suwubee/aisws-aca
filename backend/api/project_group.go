package api

import (
	"errors"
	"strings"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectGroupController struct{}

func NewProjectGroupController() *ProjectGroupController {
	return &ProjectGroupController{}
}

type CreateProjectGroupRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Remark      string  `json:"remark"`
	ParentID    *string `json:"parent_id"`
}

type UpdateProjectGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Remark      *string `json:"remark"`
	ParentID    *string `json:"parent_id"`
}

// ListProjectGroups 项目集/项目组列表
func (ctrl *ProjectGroupController) ListProjectGroups(c *fiber.Ctx) error {
	var groups []model.ProjectGroup
	if err := model.DB.Order("name asc").Find(&groups).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list project groups"})
	}
	return c.JSON(fiber.Map{"items": groups})
}

// CreateProjectGroup 创建项目集/项目组
func (ctrl *ProjectGroupController) CreateProjectGroup(c *fiber.Ctx) error {
	var req CreateProjectGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	var parentID *string
	if req.ParentID != nil {
		trimmed := strings.TrimSpace(*req.ParentID)
		if trimmed != "" {
			var parent model.ProjectGroup
			if err := model.DB.First(&parent, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Parent group not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query parent group"})
			}
			parentID = &trimmed
		}
	}

	group := model.ProjectGroup{
		ID:          uuid.New().String(),
		UserID:      c.Locals("userID").(string),
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Remark:      strings.TrimSpace(req.Remark),
		ParentID:    parentID,
	}

	if err := model.DB.Create(&group).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create project group"})
	}

	return c.Status(201).JSON(fiber.Map{"item": group})
}

// UpdateProjectGroup 更新项目集/项目组
func (ctrl *ProjectGroupController) UpdateProjectGroup(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Group id is required"})
	}

	userID := c.Locals("userID").(string)
	isAdmin := c.Locals("role").(string) == "admin"

	var group model.ProjectGroup
	if err := model.DB.First(&group, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Project group not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query project group"})
	}

	if !isAdmin && group.UserID != userID {
		return c.Status(403).JSON(fiber.Map{"error": "Access denied"})
	}

	var req UpdateProjectGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.Remark != nil {
		updates["remark"] = strings.TrimSpace(*req.Remark)
	}
	if req.ParentID != nil {
		trimmed := strings.TrimSpace(*req.ParentID)
		if trimmed == "" {
			updates["parent_id"] = nil
		} else {
			if trimmed == id {
				return c.Status(400).JSON(fiber.Map{"error": "Parent group cannot be self"})
			}
			var parent model.ProjectGroup
			if err := model.DB.First(&parent, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Parent group not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query parent group"})
			}
			updates["parent_id"] = trimmed
		}
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&group).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update project group"})
		}
		model.DB.First(&group, "id = ?", id)
	}

	return c.JSON(fiber.Map{"item": group})
}

// DeleteProjectGroup 删除项目集/项目组
func (ctrl *ProjectGroupController) DeleteProjectGroup(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Group id is required"})
	}

	userID := c.Locals("userID").(string)
	isAdmin := c.Locals("role").(string) == "admin"

	var group model.ProjectGroup
	if err := model.DB.First(&group, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Project group not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query project group"})
	}

	if !isAdmin && group.UserID != userID {
		return c.Status(403).JSON(fiber.Map{"error": "Access denied"})
	}

	// 解绑关联项目
	if err := model.DB.Model(&model.Project{}).Where("group_id = ?", id).Update("group_id", nil).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to unbind projects"})
	}

	if err := model.DB.Delete(&group).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete project group"})
	}

	return c.JSON(fiber.Map{"message": "Project group deleted"})
}

// RegisterRoutes 注册路由
func (ctrl *ProjectGroupController) RegisterRoutes(app fiber.Router) {
	groups := app.Group("/project-groups")
	groups.Get("/", ctrl.ListProjectGroups)
	groups.Post("/", ctrl.CreateProjectGroup)
	groups.Put("/:id", ctrl.UpdateProjectGroup)
	groups.Delete("/:id", ctrl.DeleteProjectGroup)
}
