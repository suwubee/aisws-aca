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

type ProjectController struct{}

func NewProjectController() *ProjectController {
	return &ProjectController{}
}

type CreateProjectRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"` // local, remote, git
	LocalPath   string            `json:"local_path"`
	ServerID    *string           `json:"server_id"`
	RemotePath  string            `json:"remote_path"`
	GitRepo     string            `json:"git_repo"`
	GitBranch   string            `json:"git_branch"`
	EnvVars     map[string]string `json:"env_vars"`
}

type UpdateProjectRequest struct {
	Name        *string            `json:"name"`
	Description *string            `json:"description"`
	Type        *string            `json:"type"` // local, remote, git
	LocalPath   *string            `json:"local_path"`
	ServerID    *string            `json:"server_id"`
	RemotePath  *string            `json:"remote_path"`
	GitRepo     *string            `json:"git_repo"`
	GitBranch   *string            `json:"git_branch"`
	EnvVars     *map[string]string `json:"env_vars"`
}

var (
	errProjectServerNotFound = errors.New("server not found")
	errProjectServerQuery    = errors.New("failed to query server")
)

func normalizeProjectType(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return model.ProjectTypeLocal, nil
	}

	switch trimmed {
	case model.ProjectTypeLocal, model.ProjectTypeRemote, model.ProjectTypeGit:
		return trimmed, nil
	default:
		return "", errors.New("invalid project type")
	}
}

func normalizeEnvVars(vars map[string]string) map[string]string {
	if vars == nil {
		return map[string]string{}
	}
	return vars
}

func validateSSHServerID(serverID string) error {
	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errProjectServerNotFound
		}
		return errProjectServerQuery
	}
	return nil
}

// ListProjects 获取项目列表
func (ctrl *ProjectController) ListProjects(c *fiber.Ctx) error {
	var projects []model.Project
	if err := model.DB.Order("created_at desc").Find(&projects).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list projects"})
	}

	return c.JSON(fiber.Map{"items": projects})
}

// CreateProject 创建项目
func (ctrl *ProjectController) CreateProject(c *fiber.Ctx) error {
	var req CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	projectType, err := normalizeProjectType(req.Type)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid project type"})
	}

	var serverID *string
	if req.ServerID != nil {
		trimmed := strings.TrimSpace(*req.ServerID)
		if trimmed != "" {
			if err := validateSSHServerID(trimmed); err != nil {
				if errors.Is(err, errProjectServerNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
			}
			serverID = &trimmed
		}
	}

	project := model.Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: req.Description,
		Type:        projectType,
		LocalPath:   strings.TrimSpace(req.LocalPath),
		ServerID:    serverID,
		RemotePath:  strings.TrimSpace(req.RemotePath),
		GitRepo:     strings.TrimSpace(req.GitRepo),
		GitBranch:   strings.TrimSpace(req.GitBranch),
		EnvVars:     model.StringMap(normalizeEnvVars(req.EnvVars)),
	}

	if err := model.DB.Create(&project).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create project"})
	}

	return c.Status(201).JSON(fiber.Map{"item": project})
}

// GetProject 获取项目详情
func (ctrl *ProjectController) GetProject(c *fiber.Ctx) error {
	id := c.Params("id")

	var project model.Project
	if err := model.DB.First(&project, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query project"})
	}

	return c.JSON(fiber.Map{"item": project})
}

// UpdateProject 更新项目
func (ctrl *ProjectController) UpdateProject(c *fiber.Ctx) error {
	id := c.Params("id")

	var project model.Project
	if err := model.DB.First(&project, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query project"})
	}

	var req UpdateProjectRequest
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
	if req.Type != nil {
		projectType, err := normalizeProjectType(*req.Type)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid project type"})
		}
		updates["type"] = projectType
	}
	if req.LocalPath != nil {
		updates["local_path"] = strings.TrimSpace(*req.LocalPath)
	}
	if req.ServerID != nil {
		trimmed := strings.TrimSpace(*req.ServerID)
		if trimmed == "" {
			updates["server_id"] = nil
		} else {
			if err := validateSSHServerID(trimmed); err != nil {
				if errors.Is(err, errProjectServerNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
			}
			updates["server_id"] = trimmed
		}
	}
	if req.RemotePath != nil {
		updates["remote_path"] = strings.TrimSpace(*req.RemotePath)
	}
	if req.GitRepo != nil {
		updates["git_repo"] = strings.TrimSpace(*req.GitRepo)
	}
	if req.GitBranch != nil {
		updates["git_branch"] = strings.TrimSpace(*req.GitBranch)
	}
	if req.EnvVars != nil {
		updates["env_vars"] = model.StringMap(normalizeEnvVars(*req.EnvVars))
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := model.DB.Model(&project).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update project"})
		}
		model.DB.First(&project, "id = ?", id)
	}

	return c.JSON(fiber.Map{"item": project})
}

// DeleteProject 删除项目
func (ctrl *ProjectController) DeleteProject(c *fiber.Ctx) error {
	id := c.Params("id")

	result := model.DB.Delete(&model.Project{}, "id = ?", id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete project"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
	}

	return c.JSON(fiber.Map{"message": "Project deleted"})
}

// RegisterRoutes 注册路由
func (ctrl *ProjectController) RegisterRoutes(app fiber.Router) {
	projects := app.Group("/projects")
	projects.Get("/", ctrl.ListProjects)
	projects.Post("/", ctrl.CreateProject)
	projects.Get("/:id", ctrl.GetProject)
	projects.Put("/:id", ctrl.UpdateProject)
	projects.Delete("/:id", ctrl.DeleteProject)
}
