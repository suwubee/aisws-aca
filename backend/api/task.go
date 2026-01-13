package api

import (
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/task"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/service/workflow"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskController struct {
	automationService *task.AutomationService
	aiWorkflowEngine  *workflow.AIWorkflowEngine
}

func NewTaskController(tm *terminal.Manager) *TaskController {
	return &TaskController{
		automationService: task.NewAutomationService(tm),
	}
}

func (ctrl *TaskController) SetAIWorkflowEngine(engine *workflow.AIWorkflowEngine) {
	ctrl.aiWorkflowEngine = engine
}

type CreateTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    int     `json:"priority"`
	Status      string  `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
	ServerID    *string `json:"server_id"`
	ProjectID   *string `json:"project_id"`
	// 自动化配置
	AutomationMode  string   `json:"automation_mode"`
	TargetServerIDs []string `json:"target_server_ids"`
	Script          string   `json:"script"`
	WorkDir         string   `json:"work_dir"`
	CLIType         string   `json:"cli_type"`
	InitialPrompt   string   `json:"initial_prompt"`
	AutoStart       bool     `json:"auto_start"`
	AutoCreateDir   *bool    `json:"auto_create_dir"`
	// AI托管配置
	AIManaged       bool   `json:"ai_managed"`
	AIPrompt        string `json:"ai_prompt"`
	AIEndCondition  string `json:"ai_end_condition"`
	AIErrorHandling string `json:"ai_error_handling"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *int    `json:"priority"`
	Status      *string `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
	ProjectID   *string `json:"project_id"`
	// 自动化配置
	AutomationMode  *string   `json:"automation_mode"`
	TargetServerIDs *[]string `json:"target_server_ids"`
	Script          *string   `json:"script"`
	WorkDir         *string   `json:"work_dir"`
	CLIType         *string   `json:"cli_type"`
	InitialPrompt   *string   `json:"initial_prompt"`
	AutoStart       *bool     `json:"auto_start"`
	AutoCreateDir   *bool     `json:"auto_create_dir"`
	// AI托管配置
	AIManaged       *bool   `json:"ai_managed"`
	AIPrompt        *string `json:"ai_prompt"`
	AIEndCondition  *string `json:"ai_end_condition"`
	AIErrorHandling *string `json:"ai_error_handling"`
}

type MoveTaskRequest struct {
	Status     string  `json:"status"`
	OrderIndex float64 `json:"order_index"`
}

type TaskServerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TaskProjectGroupInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TaskProjectInfo struct {
	ID    string                `json:"id"`
	Name  string                `json:"name"`
	Group *TaskProjectGroupInfo `json:"group,omitempty"`
}

type TaskListItem struct {
	model.Task
	Server  *TaskServerInfo  `json:"server,omitempty"`
	Project *TaskProjectInfo `json:"project,omitempty"`
}

var allowedTaskStatuses = map[string]struct{}{
	"todo":        {},
	"in_progress": {},
	"paused":      {},
	"done":        {},
	"failed":      {},
	"timeout":     {},
	"archived":    {},
}

func normalizeTaskStatus(status string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return "todo", true
	}
	_, ok := allowedTaskStatuses[s]
	return s, ok
}

var allowedAutomationModes = map[string]struct{}{
	"none":   {},
	"cli":    {},
	"script": {},
	"agent":  {},
}

func normalizeAutomationMode(value string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return "cli", true
	}
	_, ok := allowedAutomationModes[s]
	return s, ok
}

var allowedCLITypes = map[string]struct{}{
	"claude": {},
	"codex":  {},
	"gemini": {},
}

func normalizeCLIType(value string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "" {
		return "claude", true
	}
	_, ok := allowedCLITypes[s]
	return s, ok
}

func taskStatusGroup(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "todo":
		return "todo"
	case "in_progress", "paused":
		return "in_progress"
	case "done", "failed", "timeout":
		return "done"
	case "archived":
		return "archived"
	default:
		return "todo"
	}
}

// CreateTask 创建任务
func (ctrl *TaskController) CreateTask(c *fiber.Ctx) error {
	var req CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required"})
	}

	status, ok := normalizeTaskStatus(req.Status)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
	}

	automationMode, ok := normalizeAutomationMode(req.AutomationMode)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid automation_mode"})
	}

	cliType := "claude"
	if automationMode == "cli" {
		if normalized, ok := normalizeCLIType(req.CLIType); ok {
			cliType = normalized
		} else {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid cli_type"})
		}
	} else if strings.TrimSpace(req.CLIType) != "" {
		if normalized, ok := normalizeCLIType(req.CLIType); ok {
			cliType = normalized
		}
	}

	autoCreateDir := true
	if req.AutoCreateDir != nil {
		autoCreateDir = *req.AutoCreateDir
	}

	targetServerIDs := make([]string, 0, len(req.TargetServerIDs))
	if len(req.TargetServerIDs) > 0 {
		seen := map[string]struct{}{}
		for _, raw := range req.TargetServerIDs {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			targetServerIDs = append(targetServerIDs, id)
		}

		if len(targetServerIDs) > 0 {
			var servers []model.SSHServer
			if err := model.DB.Select("id").Where("id IN ?", targetServerIDs).Find(&servers).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query servers"})
			}

			found := map[string]struct{}{}
			for _, s := range servers {
				found[s.ID] = struct{}{}
			}
			for _, sid := range targetServerIDs {
				if _, ok := found[sid]; !ok {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
			}
		}
	}

	var serverID *string
	if req.ServerID != nil {
		trimmed := strings.TrimSpace(*req.ServerID)
		if trimmed != "" {
			var server model.SSHServer
			if err := model.DB.First(&server, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
			}
			serverID = &trimmed
		}
	}

	if serverID == nil && len(targetServerIDs) > 0 {
		first := targetServerIDs[0]
		serverID = &first
	}

	var projectID *string
	if req.ProjectID != nil {
		trimmed := strings.TrimSpace(*req.ProjectID)
		if trimmed != "" {
			var project model.Project
			if err := model.DB.First(&project, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Project not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query project"})
			}
			projectID = &trimmed
		}
	}

	task := model.Task{
		ID:              uuid.New().String(),
		Title:           req.Title,
		Description:     req.Description,
		Status:          status,
		Priority:        req.Priority,
		RuleSetID:       req.RuleSetID,
		ServerID:        serverID,
		ProjectID:       projectID,
		OrderIndex:      float64(time.Now().UnixNano()),
		AutomationMode:  automationMode,
		TargetServerIDs: model.StringArray(targetServerIDs),
		Script:          req.Script,
		WorkDir:         req.WorkDir,
		CLIType:         cliType,
		InitialPrompt:   req.InitialPrompt,
		AutoStart:       req.AutoStart,
		AutoCreateDir:   autoCreateDir,
		// AI托管配置
		AIManaged:       req.AIManaged,
		AIPrompt:        req.AIPrompt,
		AIEndCondition:  req.AIEndCondition,
		AIErrorHandling: req.AIErrorHandling,
	}

	if err := model.DB.Create(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create task"})
	}

	return c.Status(201).JSON(fiber.Map{"item": task})
}

func enrichTasks(tasks []model.Task) ([]TaskListItem, error) {
	serverInfoByID := map[string]*TaskServerInfo{}
	uniqueServerIDs := map[string]struct{}{}
	serverIDs := make([]string, 0)

	uniqueProjectIDs := map[string]struct{}{}
	projectIDs := make([]string, 0)

	for _, t := range tasks {
		if t.ServerID == nil {
			// noop
		} else {
			serverID := strings.TrimSpace(*t.ServerID)
			if serverID != "" {
				if _, ok := uniqueServerIDs[serverID]; !ok {
					uniqueServerIDs[serverID] = struct{}{}
					serverIDs = append(serverIDs, serverID)
				}
			}
		}

		if t.ProjectID == nil {
			continue
		}
		pid := strings.TrimSpace(*t.ProjectID)
		if pid == "" {
			continue
		}
		if _, ok := uniqueProjectIDs[pid]; ok {
			continue
		}
		uniqueProjectIDs[pid] = struct{}{}
		projectIDs = append(projectIDs, pid)
	}

	if len(serverIDs) > 0 {
		var servers []model.SSHServer
		if err := model.DB.Select("id", "name").Where("id IN ?", serverIDs).Find(&servers).Error; err != nil {
			return nil, err
		}

		for _, s := range servers {
			serverInfoByID[s.ID] = &TaskServerInfo{ID: s.ID, Name: s.Name}
		}
	}

	projectInfoByID := map[string]*TaskProjectInfo{}
	groupInfoByID := map[string]*TaskProjectGroupInfo{}

	if len(projectIDs) > 0 {
		var projects []model.Project
		if err := model.DB.Select("id", "name", "group_id").Where("id IN ?", projectIDs).Find(&projects).Error; err != nil {
			return nil, err
		}

		groupIDs := make([]string, 0)
		seenGroupIDs := map[string]struct{}{}
		for _, p := range projects {
			if p.GroupID == nil {
				continue
			}
			gid := strings.TrimSpace(*p.GroupID)
			if gid == "" {
				continue
			}
			if _, ok := seenGroupIDs[gid]; ok {
				continue
			}
			seenGroupIDs[gid] = struct{}{}
			groupIDs = append(groupIDs, gid)
		}

		if len(groupIDs) > 0 {
			var groups []model.ProjectGroup
			if err := model.DB.Select("id", "name").Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
				return nil, err
			}
			for _, g := range groups {
				groupInfoByID[g.ID] = &TaskProjectGroupInfo{ID: g.ID, Name: g.Name}
			}
		}

		for _, p := range projects {
			info := &TaskProjectInfo{ID: p.ID, Name: p.Name}
			if p.GroupID != nil {
				gid := strings.TrimSpace(*p.GroupID)
				if gid != "" {
					info.Group = groupInfoByID[gid]
				}
			}
			projectInfoByID[p.ID] = info
		}
	}

	items := make([]TaskListItem, len(tasks))
	for i, t := range tasks {
		item := TaskListItem{Task: t}
		if t.ServerID != nil {
			serverID := strings.TrimSpace(*t.ServerID)
			if serverID != "" {
				item.Server = serverInfoByID[serverID]
			}
		}
		if t.ProjectID != nil {
			pid := strings.TrimSpace(*t.ProjectID)
			if pid != "" {
				item.Project = projectInfoByID[pid]
			}
		}
		items[i] = item
	}

	return items, nil
}

// ListTasks 获取任务列表
func (ctrl *TaskController) ListTasks(c *fiber.Ctx) error {
	status := c.Query("status")
	priority := c.Query("priority")
	keyword := c.Query("keyword")
	projectID := strings.TrimSpace(c.Query("project_id"))
	groupID := strings.TrimSpace(c.Query("project_group_id"))
	if groupID == "" {
		groupID = strings.TrimSpace(c.Query("group_id"))
	}

	query := model.DB.Model(&model.Task{})

	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if groupID != "" {
		query = query.Joins("JOIN projects ON projects.id = tasks.project_id").Where("projects.group_id = ?", groupID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var tasks []model.Task
	if err := query.Order("status, order_index").Find(&tasks).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	items, err := enrichTasks(tasks)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	return c.JSON(fiber.Map{"items": items})
}

// GetTask 获取任务详情
func (ctrl *TaskController) GetTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	return c.JSON(fiber.Map{"item": task})
}

// GetTaskDetail 获取任务完整详情（包含终端、日志、审批）
func (ctrl *TaskController) GetTaskDetail(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	// 获取关联的终端
	var terminals []model.TerminalSession
	model.DB.Where("task_id = ?", id).Order("created_at desc").Find(&terminals)

	// 获取终端ID列表
	terminalIDs := make([]string, len(terminals))
	for i, t := range terminals {
		terminalIDs[i] = t.ID
	}

	// 获取关联的日志（最近100条）
	var logs []model.Log
	if len(terminalIDs) > 0 {
		model.DB.Where("terminal_id IN ?", terminalIDs).
			Order("created_at desc").Limit(100).Find(&logs)
	}

	// 获取关联的审批记录
	var approvals []model.ApprovalRecord
	if len(terminalIDs) > 0 {
		model.DB.Where("terminal_id IN ?", terminalIDs).
			Order("created_at desc").Find(&approvals)
	}

	return c.JSON(fiber.Map{
		"task":      task,
		"terminals": terminals,
		"logs":      logs,
		"approvals": approvals,
	})
}

// UpdateTask 更新任务
func (ctrl *TaskController) UpdateTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	var req UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Status != nil {
		normalized, ok := normalizeTaskStatus(*req.Status)
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
		}
		updates["status"] = normalized
		if normalized == "done" {
			now := time.Now()
			updates["completed_at"] = now
		}
	}
	if req.RuleSetID != nil {
		if *req.RuleSetID == "" {
			updates["rule_set_id"] = nil
		} else {
			updates["rule_set_id"] = *req.RuleSetID
		}
	}
	if req.ProjectID != nil {
		trimmed := strings.TrimSpace(*req.ProjectID)
		if trimmed == "" {
			updates["project_id"] = nil
		} else {
			var project model.Project
			if err := model.DB.First(&project, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Project not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query project"})
			}
			updates["project_id"] = trimmed
		}
	}

	nextAutomationMode := strings.TrimSpace(task.AutomationMode)
	if nextAutomationMode == "" {
		nextAutomationMode = "cli"
	}
	if req.AutomationMode != nil {
		normalized, ok := normalizeAutomationMode(*req.AutomationMode)
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid automation_mode"})
		}
		nextAutomationMode = normalized
		updates["automation_mode"] = normalized
	}
	// 自动化配置字段
	if req.TargetServerIDs != nil {
		targetServerIDs := make([]string, 0, len(*req.TargetServerIDs))
		seen := map[string]struct{}{}
		for _, raw := range *req.TargetServerIDs {
			sid := strings.TrimSpace(raw)
			if sid == "" {
				continue
			}
			if _, ok := seen[sid]; ok {
				continue
			}
			seen[sid] = struct{}{}
			targetServerIDs = append(targetServerIDs, sid)
		}

		if len(targetServerIDs) > 0 {
			var servers []model.SSHServer
			if err := model.DB.Select("id").Where("id IN ?", targetServerIDs).Find(&servers).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query servers"})
			}
			found := map[string]struct{}{}
			for _, s := range servers {
				found[s.ID] = struct{}{}
			}
			for _, sid := range targetServerIDs {
				if _, ok := found[sid]; !ok {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
			}
		}

		updates["target_server_ids"] = model.StringArray(targetServerIDs)
	}
	if req.Script != nil {
		updates["script"] = *req.Script
	}
	if req.WorkDir != nil {
		updates["work_dir"] = *req.WorkDir
	}
	if req.CLIType != nil {
		if nextAutomationMode == "cli" {
			if normalized, ok := normalizeCLIType(*req.CLIType); ok {
				updates["cli_type"] = normalized
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid cli_type"})
			}
		}
	}
	if req.InitialPrompt != nil {
		updates["initial_prompt"] = *req.InitialPrompt
	}
	if req.AutoStart != nil {
		updates["auto_start"] = *req.AutoStart
	}
	if req.AutoCreateDir != nil {
		updates["auto_create_dir"] = *req.AutoCreateDir
	}
	// AI托管配置字段
	if req.AIManaged != nil {
		updates["ai_managed"] = *req.AIManaged
	}
	if req.AIPrompt != nil {
		updates["ai_prompt"] = *req.AIPrompt
	}
	if req.AIEndCondition != nil {
		updates["ai_end_condition"] = *req.AIEndCondition
	}
	if req.AIErrorHandling != nil {
		updates["ai_error_handling"] = *req.AIErrorHandling
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		model.DB.Model(&task).Updates(updates)
	}

	model.DB.First(&task, "id = ?", id)
	return c.JSON(fiber.Map{"item": task})
}

// DeleteTask 删除任务
func (ctrl *TaskController) DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")

	result := model.DB.Delete(&model.Task{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	return c.JSON(fiber.Map{"message": "Task deleted"})
}

// MoveTask 移动任务（拖拽）
func (ctrl *TaskController) MoveTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var task model.Task
	if err := model.DB.First(&task, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	var req MoveTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := map[string]interface{}{
		"status":      "",
		"order_index": req.OrderIndex,
		"updated_at":  time.Now(),
	}

	normalized, ok := normalizeTaskStatus(req.Status)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
	}
	updates["status"] = normalized

	if normalized == "done" {
		now := time.Now()
		updates["completed_at"] = now
	}

	model.DB.Model(&task).Updates(updates)
	model.DB.First(&task, "id = ?", id)

	return c.JSON(fiber.Map{"item": task})
}

// GetTasksByStatus 按状态获取任务（用于Kanban）
func (ctrl *TaskController) GetTasksByStatus(c *fiber.Ctx) error {
	projectID := strings.TrimSpace(c.Query("project_id"))
	groupID := strings.TrimSpace(c.Query("project_group_id"))
	if groupID == "" {
		groupID = strings.TrimSpace(c.Query("group_id"))
	}

	query := model.DB.Model(&model.Task{})
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if groupID != "" {
		query = query.Joins("JOIN projects ON projects.id = tasks.project_id").Where("projects.group_id = ?", groupID)
	}

	var tasks []model.Task
	if err := query.Order("order_index").Find(&tasks).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	items, err := enrichTasks(tasks)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	// 按状态分组
	grouped := map[string][]TaskListItem{
		"todo":        {},
		"in_progress": {},
		"done":        {},
		"archived":    {},
	}

	for _, item := range items {
		group := taskStatusGroup(item.Status)
		grouped[group] = append(grouped[group], item)
	}

	return c.JSON(fiber.Map{"items": grouped})
}

// StartTask 启动自动化任务
func (ctrl *TaskController) StartTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var taskModel model.Task
	if err := model.DB.First(&taskModel, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	mode := strings.ToLower(strings.TrimSpace(taskModel.AutomationMode))
	if mode == "agent" {
		if ctrl.aiWorkflowEngine == nil {
			return c.Status(500).JSON(fiber.Map{"error": "AI workflow engine not initialized"})
		}

		// 幂等：任务进行中/暂停时直接返回已有会话
		if taskModel.Status == "in_progress" || taskModel.Status == "paused" {
			sessionID := strings.TrimSpace(taskModel.AgentSessionID)
			needsUserAction := taskModel.Status == "paused"
			userHint := ""

			if sessionID != "" {
				if session, err := ctrl.aiWorkflowEngine.GetSession(sessionID); err == nil && session != nil {
					if strings.EqualFold(strings.TrimSpace(session.Status), "paused") {
						needsUserAction = true
						userHint = strings.TrimSpace(session.Summary)
					}
				}
			}

			return c.JSON(fiber.Map{
				"message":           "Task already running",
				"task":              taskModel,
				"agent_session_id":  sessionID,
				"terminal_id":       "",
				"terminal_ids":      []string{},
				"work_dir":          taskModel.WorkDir,
				"cli_started":       false,
				"needs_user_action": needsUserAction,
				"user_action_hint":  userHint,
			})
		}

		session, err := ctrl.aiWorkflowEngine.StartTaskAgent(c.Context(), &taskModel)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		model.DB.First(&taskModel, "id = ?", id)

		return c.JSON(fiber.Map{
			"message":           "Task started",
			"task":              taskModel,
			"agent_session_id":  session.ID,
			"terminal_id":       "",
			"terminal_ids":      []string{},
			"work_dir":          taskModel.WorkDir,
			"cli_started":       false,
			"needs_user_action": strings.EqualFold(strings.TrimSpace(session.Status), "paused"),
			"user_action_hint":  strings.TrimSpace(session.Summary),
		})
	}

	// 如果任务已经在进行中，返回现有终端信息而不是错误
	if taskModel.Status == "in_progress" {
		var terminals []model.TerminalSession
		_ = model.DB.Where("task_id = ?", id).Order("created_at desc").Find(&terminals).Error
		if len(terminals) > 0 {
			terminalIDs := make([]string, 0, len(terminals))
			for _, t := range terminals {
				terminalIDs = append(terminalIDs, t.ID)
			}
			cliStarted := strings.ToLower(strings.TrimSpace(taskModel.AutomationMode)) != "script"
			return c.JSON(fiber.Map{
				"message":           "Task already running",
				"task":              taskModel,
				"terminal_id":       terminals[0].ID,
				"terminal_ids":      terminalIDs,
				"work_dir":          taskModel.WorkDir,
				"cli_started":       cliStarted,
				"needs_user_action": false,
				"user_action_hint":  "",
			})
		}
	}

	result, err := ctrl.automationService.StartTask(&taskModel)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  err.Error(),
			"result": result,
		})
	}

	terminalIDs := result.TerminalIDs
	terminalID := ""
	if len(terminalIDs) > 0 {
		terminalID = terminalIDs[0]
	} else if result.Terminal != nil {
		terminalID = result.Terminal.ID()
		terminalIDs = []string{terminalID}
	}

	return c.JSON(fiber.Map{
		"message":           "Task started",
		"task":              result.Task,
		"agent_session_id":  strings.TrimSpace(result.Task.AgentSessionID),
		"terminal_id":       terminalID,
		"terminal_ids":      terminalIDs,
		"work_dir":          result.WorkDir,
		"cli_started":       result.CLIStarted,
		"needs_user_action": result.NeedsUserAction,
		"user_action_hint":  result.UserActionHint,
	})
}

// GetTaskTerminals 获取任务关联的终端列表
func (ctrl *TaskController) GetTaskTerminals(c *fiber.Ctx) error {
	id := c.Params("id")

	var terminals []model.TerminalSession
	model.DB.Where("task_id = ?", id).Order("created_at desc").Find(&terminals)

	return c.JSON(fiber.Map{"items": terminals})
}

// RegisterRoutes 注册路由
func (ctrl *TaskController) RegisterRoutes(app fiber.Router) {
	tasks := app.Group("/tasks")
	tasks.Get("/", ctrl.ListTasks)
	tasks.Post("/", ctrl.CreateTask)
	tasks.Get("/by-status", ctrl.GetTasksByStatus)
	tasks.Get("/:id", ctrl.GetTask)
	tasks.Get("/:id/detail", ctrl.GetTaskDetail)
	tasks.Put("/:id", ctrl.UpdateTask)
	tasks.Delete("/:id", ctrl.DeleteTask)
	tasks.Post("/:id/move", ctrl.MoveTask)
	tasks.Post("/:id/start", ctrl.StartTask)
	tasks.Get("/:id/terminals", ctrl.GetTaskTerminals)
}
