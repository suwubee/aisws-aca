package api

import (
	"errors"
	"sort"
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
	Remark      string  `json:"remark"`
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
	Remark      *string `json:"remark"`
	Priority    *int    `json:"priority"`
	Status      *string `json:"status"`
	RuleSetID   *string `json:"rule_set_id"`
	ServerID    *string `json:"server_id"`
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

type TaskHistoryWorkflowSession struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	WorkflowID  string     `json:"workflow_id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TaskHistoryItem struct {
	Task            TaskListItem                `json:"task"`
	WorkflowSession *TaskHistoryWorkflowSession `json:"workflow_session,omitempty"`
	LatestExecution *model.CLIExecution         `json:"latest_execution,omitempty"`
}

type TaskHistoryStatsOverview struct {
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
	ByMode   map[string]int64 `json:"by_mode"`
}

type TaskHistoryStatsGroup struct {
	GroupID   string           `json:"group_id"`
	GroupName string           `json:"group_name"`
	Total     int64            `json:"total"`
	ByStatus  map[string]int64 `json:"by_status"`
	ByMode    map[string]int64 `json:"by_mode"`
}

type TaskHistoryStatsProject struct {
	ProjectID   string           `json:"project_id"`
	ProjectName string           `json:"project_name"`
	GroupID     string           `json:"group_id"`
	GroupName   string           `json:"group_name"`
	Total       int64            `json:"total"`
	ByStatus    map[string]int64 `json:"by_status"`
	ByMode      map[string]int64 `json:"by_mode"`
}

type TaskHistoryStats struct {
	Overview  TaskHistoryStatsOverview  `json:"overview"`
	ByGroup   []TaskHistoryStatsGroup   `json:"by_group"`
	ByProject []TaskHistoryStatsProject `json:"by_project"`
}

type taskHistoryFilter struct {
	ProjectID      string
	GroupID        string
	Keyword        string
	Status         string
	AutomationMode string
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

func findLatestExecutionID(taskID, workflowSessionID string) string {
	if model.DB == nil {
		return ""
	}
	tid := strings.TrimSpace(taskID)
	sid := strings.TrimSpace(workflowSessionID)
	if tid == "" && sid == "" {
		return ""
	}

	query := model.DB.Model(&model.CLIExecution{})
	if sid != "" {
		query = query.Where("workflow_session_id = ?", sid)
	} else {
		query = query.Where("task_id = ?", tid)
	}

	var item model.CLIExecution
	if err := query.Order("updated_at desc").First(&item).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(item.ID)
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

	// 自动化任务不允许隐式本地执行：必须显式选择服务器（本地也需要在服务器列表中配置）
	switch automationMode {
	case "cli":
		if serverID == nil || strings.TrimSpace(*serverID) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Server is required (local must be configured in Servers)"})
		}
	case "script", "agent":
		if (serverID == nil || strings.TrimSpace(*serverID) == "") && len(targetServerIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Target server is required (local must be configured in Servers)"})
		}
		if len(targetServerIDs) == 0 && serverID != nil && strings.TrimSpace(*serverID) != "" {
			targetServerIDs = append(targetServerIDs, strings.TrimSpace(*serverID))
		}
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
		Remark:          strings.TrimSpace(req.Remark),
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
		nativeTypes := []string{"ai_input_native", "ai_output_native"}
		var nativeCount int64
		model.DB.Model(&model.Log{}).
			Where("terminal_id IN ?", terminalIDs).
			Where("log_type IN ?", nativeTypes).
			Count(&nativeCount)
		if nativeCount > 0 {
			model.DB.Where("terminal_id IN ?", terminalIDs).
				Where("log_type IN ?", []string{"ai_input_native", "ai_output_native", "system"}).
				Order("created_at desc").Limit(100).Find(&logs)
		} else {
			model.DB.Where("terminal_id IN ?", terminalIDs).
				Where("log_type NOT IN ?", []string{"input_raw", "output_raw"}).
				Order("created_at desc").Limit(100).Find(&logs)
		}
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
	if req.Remark != nil {
		updates["remark"] = strings.TrimSpace(*req.Remark)
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

	nextServerID := ""
	if task.ServerID != nil {
		nextServerID = strings.TrimSpace(*task.ServerID)
	}
	if req.ServerID != nil {
		trimmed := strings.TrimSpace(*req.ServerID)
		if trimmed == "" {
			nextServerID = ""
			updates["server_id"] = nil
		} else {
			var server model.SSHServer
			if err := model.DB.First(&server, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Server not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
			}
			nextServerID = trimmed
			updates["server_id"] = trimmed
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

	nextTargetServerIDs := make([]string, 0, len(task.TargetServerIDs))
	seenTargetIDs := map[string]struct{}{}
	for _, raw := range task.TargetServerIDs {
		sid := strings.TrimSpace(raw)
		if sid == "" {
			continue
		}
		if _, ok := seenTargetIDs[sid]; ok {
			continue
		}
		seenTargetIDs[sid] = struct{}{}
		nextTargetServerIDs = append(nextTargetServerIDs, sid)
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

		nextTargetServerIDs = targetServerIDs
		updates["target_server_ids"] = model.StringArray(targetServerIDs)
	}

	// 自动化任务不允许隐式本地执行：必须显式选择服务器（本地也需要在服务器列表中配置）
	switch nextAutomationMode {
	case "cli":
		if nextServerID == "" {
			if len(nextTargetServerIDs) > 0 {
				nextServerID = nextTargetServerIDs[0]
				updates["server_id"] = nextServerID
			} else {
				return c.Status(400).JSON(fiber.Map{"error": "Server is required (local must be configured in Servers)"})
			}
		}
	case "script", "agent":
		if nextServerID == "" && len(nextTargetServerIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Target server is required (local must be configured in Servers)"})
		}
		if len(nextTargetServerIDs) == 0 && nextServerID != "" {
			nextTargetServerIDs = []string{nextServerID}
			updates["target_server_ids"] = model.StringArray(nextTargetServerIDs)
		}
		if len(nextTargetServerIDs) > 0 {
			first := nextTargetServerIDs[0]
			if nextServerID == "" || nextServerID != first {
				nextServerID = first
				updates["server_id"] = first
			}
		}
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

func loadWorkflowSessionsByID(sessionIDs []string) (map[string]*TaskHistoryWorkflowSession, error) {
	result := map[string]*TaskHistoryWorkflowSession{}
	if len(sessionIDs) == 0 {
		return result, nil
	}

	var rows []model.AIWorkflowSession
	if err := model.DB.
		Select("id", "status", "workflow_id", "started_at", "completed_at").
		Where("id IN ?", sessionIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		result[id] = &TaskHistoryWorkflowSession{
			ID:          id,
			Status:      row.Status,
			WorkflowID:  row.WorkflowID,
			StartedAt:   row.StartedAt,
			CompletedAt: row.CompletedAt,
		}
	}
	return result, nil
}

func loadLatestExecutionByTaskID(taskIDs []string) (map[string]*model.CLIExecution, error) {
	result := map[string]*model.CLIExecution{}
	if len(taskIDs) == 0 {
		return result, nil
	}

	subQuery := model.DB.Model(&model.CLIExecution{}).
		Select("task_id, MAX(updated_at) AS max_updated_at").
		Where("task_id IN ?", taskIDs).
		Group("task_id")

	var rows []model.CLIExecution
	if err := model.DB.Model(&model.CLIExecution{}).
		Joins("JOIN (?) latest ON latest.task_id = cli_executions.task_id AND latest.max_updated_at = cli_executions.updated_at", subQuery).
		Order("cli_executions.updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for i := range rows {
		if rows[i].TaskID == nil {
			continue
		}
		taskID := strings.TrimSpace(*rows[i].TaskID)
		if taskID == "" {
			continue
		}
		if _, exists := result[taskID]; exists {
			continue
		}
		item := rows[i]
		result[taskID] = &item
	}

	return result, nil
}

func applyTaskHistoryFilter(query *gorm.DB, filter taskHistoryFilter, projectsJoined bool) *gorm.DB {
	if filter.ProjectID != "" {
		query = query.Where("tasks.project_id = ?", filter.ProjectID)
	}
	if filter.GroupID != "" {
		if !projectsJoined {
			query = query.Joins("JOIN projects ON projects.id = tasks.project_id")
		}
		query = query.Where("projects.group_id = ?", filter.GroupID)
	}
	if filter.Status != "" {
		query = query.Where("tasks.status = ?", filter.Status)
	}
	if filter.AutomationMode != "" {
		query = query.Where("tasks.automation_mode = ?", filter.AutomationMode)
	}
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		query = query.Where(
			"(tasks.id LIKE ? OR tasks.title LIKE ? OR tasks.description LIKE ? OR tasks.remark LIKE ?)",
			like, like, like, like,
		)
	}
	return query
}

func buildTaskHistoryStats(filter taskHistoryFilter) (*TaskHistoryStats, error) {
	stats := &TaskHistoryStats{
		Overview: TaskHistoryStatsOverview{
			ByStatus: map[string]int64{},
			ByMode:   map[string]int64{},
		},
		ByGroup:   []TaskHistoryStatsGroup{},
		ByProject: []TaskHistoryStatsProject{},
	}

	baseQuery := applyTaskHistoryFilter(model.DB.Model(&model.Task{}), filter, false)
	if err := baseQuery.Count(&stats.Overview.Total).Error; err != nil {
		return nil, err
	}

	var statusRows []struct {
		Status string `gorm:"column:status"`
		Count  int64  `gorm:"column:count"`
	}
	if err := applyTaskHistoryFilter(model.DB.Model(&model.Task{}), filter, false).
		Select("tasks.status AS status, COUNT(*) AS count").
		Group("tasks.status").
		Scan(&statusRows).Error; err != nil {
		return nil, err
	}
	for _, row := range statusRows {
		key := strings.TrimSpace(row.Status)
		if key == "" {
			key = "unknown"
		}
		stats.Overview.ByStatus[key] = row.Count
	}

	var modeRows []struct {
		Mode  string `gorm:"column:mode"`
		Count int64  `gorm:"column:count"`
	}
	if err := applyTaskHistoryFilter(model.DB.Model(&model.Task{}), filter, false).
		Select("tasks.automation_mode AS mode, COUNT(*) AS count").
		Group("tasks.automation_mode").
		Scan(&modeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range modeRows {
		mode := strings.TrimSpace(row.Mode)
		if mode == "" {
			mode = "cli"
		}
		stats.Overview.ByMode[mode] = row.Count
	}

	groupByKey := map[string]*TaskHistoryStatsGroup{}
	groupKey := func(groupID, groupName string) string { return groupID + "\x00" + groupName }

	var groupStatusRows []struct {
		GroupID   string `gorm:"column:group_id"`
		GroupName string `gorm:"column:group_name"`
		Status    string `gorm:"column:status"`
		Count     int64  `gorm:"column:count"`
	}
	groupStatusQuery := model.DB.Model(&model.Task{}).
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id").
		Joins("LEFT JOIN project_groups ON project_groups.id = projects.group_id")
	groupStatusQuery = applyTaskHistoryFilter(groupStatusQuery, filter, true)
	if err := groupStatusQuery.
		Select(
			"COALESCE(projects.group_id, '') AS group_id, " +
				"COALESCE(project_groups.name, '未分组') AS group_name, " +
				"tasks.status AS status, COUNT(*) AS count",
		).
		Group("projects.group_id, project_groups.name, tasks.status").
		Scan(&groupStatusRows).Error; err != nil {
		return nil, err
	}
	for _, row := range groupStatusRows {
		gid := strings.TrimSpace(row.GroupID)
		gname := strings.TrimSpace(row.GroupName)
		if gname == "" {
			gname = "未分组"
		}
		key := groupKey(gid, gname)
		entry, ok := groupByKey[key]
		if !ok {
			entry = &TaskHistoryStatsGroup{
				GroupID:   gid,
				GroupName: gname,
				ByStatus:  map[string]int64{},
				ByMode:    map[string]int64{},
			}
			groupByKey[key] = entry
		}
		status := strings.TrimSpace(row.Status)
		if status == "" {
			status = "unknown"
		}
		entry.ByStatus[status] += row.Count
		entry.Total += row.Count
	}

	var groupModeRows []struct {
		GroupID   string `gorm:"column:group_id"`
		GroupName string `gorm:"column:group_name"`
		Mode      string `gorm:"column:mode"`
		Count     int64  `gorm:"column:count"`
	}
	groupModeQuery := model.DB.Model(&model.Task{}).
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id").
		Joins("LEFT JOIN project_groups ON project_groups.id = projects.group_id")
	groupModeQuery = applyTaskHistoryFilter(groupModeQuery, filter, true)
	if err := groupModeQuery.
		Select(
			"COALESCE(projects.group_id, '') AS group_id, " +
				"COALESCE(project_groups.name, '未分组') AS group_name, " +
				"tasks.automation_mode AS mode, COUNT(*) AS count",
		).
		Group("projects.group_id, project_groups.name, tasks.automation_mode").
		Scan(&groupModeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range groupModeRows {
		gid := strings.TrimSpace(row.GroupID)
		gname := strings.TrimSpace(row.GroupName)
		if gname == "" {
			gname = "未分组"
		}
		key := groupKey(gid, gname)
		entry, ok := groupByKey[key]
		if !ok {
			entry = &TaskHistoryStatsGroup{
				GroupID:   gid,
				GroupName: gname,
				ByStatus:  map[string]int64{},
				ByMode:    map[string]int64{},
			}
			groupByKey[key] = entry
		}
		mode := strings.TrimSpace(row.Mode)
		if mode == "" {
			mode = "cli"
		}
		entry.ByMode[mode] += row.Count
	}

	for _, entry := range groupByKey {
		stats.ByGroup = append(stats.ByGroup, *entry)
	}
	sort.Slice(stats.ByGroup, func(i, j int) bool {
		if stats.ByGroup[i].Total == stats.ByGroup[j].Total {
			return stats.ByGroup[i].GroupName < stats.ByGroup[j].GroupName
		}
		return stats.ByGroup[i].Total > stats.ByGroup[j].Total
	})

	projectByKey := map[string]*TaskHistoryStatsProject{}
	projectKey := func(projectID, projectName, groupID, groupName string) string {
		return projectID + "\x00" + projectName + "\x00" + groupID + "\x00" + groupName
	}

	var projectStatusRows []struct {
		ProjectID   string `gorm:"column:project_id"`
		ProjectName string `gorm:"column:project_name"`
		GroupID     string `gorm:"column:group_id"`
		GroupName   string `gorm:"column:group_name"`
		Status      string `gorm:"column:status"`
		Count       int64  `gorm:"column:count"`
	}
	projectStatusQuery := model.DB.Model(&model.Task{}).
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id").
		Joins("LEFT JOIN project_groups ON project_groups.id = projects.group_id")
	projectStatusQuery = applyTaskHistoryFilter(projectStatusQuery, filter, true)
	if err := projectStatusQuery.
		Select(
			"COALESCE(tasks.project_id, '') AS project_id, " +
				"COALESCE(projects.name, '未绑定项目') AS project_name, " +
				"COALESCE(projects.group_id, '') AS group_id, " +
				"COALESCE(project_groups.name, '未分组') AS group_name, " +
				"tasks.status AS status, COUNT(*) AS count",
		).
		Group("tasks.project_id, projects.name, projects.group_id, project_groups.name, tasks.status").
		Scan(&projectStatusRows).Error; err != nil {
		return nil, err
	}
	for _, row := range projectStatusRows {
		pid := strings.TrimSpace(row.ProjectID)
		pname := strings.TrimSpace(row.ProjectName)
		gid := strings.TrimSpace(row.GroupID)
		gname := strings.TrimSpace(row.GroupName)
		if pname == "" {
			pname = "未绑定项目"
		}
		if gname == "" {
			gname = "未分组"
		}
		key := projectKey(pid, pname, gid, gname)
		entry, ok := projectByKey[key]
		if !ok {
			entry = &TaskHistoryStatsProject{
				ProjectID:   pid,
				ProjectName: pname,
				GroupID:     gid,
				GroupName:   gname,
				ByStatus:    map[string]int64{},
				ByMode:      map[string]int64{},
			}
			projectByKey[key] = entry
		}
		status := strings.TrimSpace(row.Status)
		if status == "" {
			status = "unknown"
		}
		entry.ByStatus[status] += row.Count
		entry.Total += row.Count
	}

	var projectModeRows []struct {
		ProjectID   string `gorm:"column:project_id"`
		ProjectName string `gorm:"column:project_name"`
		GroupID     string `gorm:"column:group_id"`
		GroupName   string `gorm:"column:group_name"`
		Mode        string `gorm:"column:mode"`
		Count       int64  `gorm:"column:count"`
	}
	projectModeQuery := model.DB.Model(&model.Task{}).
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id").
		Joins("LEFT JOIN project_groups ON project_groups.id = projects.group_id")
	projectModeQuery = applyTaskHistoryFilter(projectModeQuery, filter, true)
	if err := projectModeQuery.
		Select(
			"COALESCE(tasks.project_id, '') AS project_id, " +
				"COALESCE(projects.name, '未绑定项目') AS project_name, " +
				"COALESCE(projects.group_id, '') AS group_id, " +
				"COALESCE(project_groups.name, '未分组') AS group_name, " +
				"tasks.automation_mode AS mode, COUNT(*) AS count",
		).
		Group("tasks.project_id, projects.name, projects.group_id, project_groups.name, tasks.automation_mode").
		Scan(&projectModeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range projectModeRows {
		pid := strings.TrimSpace(row.ProjectID)
		pname := strings.TrimSpace(row.ProjectName)
		gid := strings.TrimSpace(row.GroupID)
		gname := strings.TrimSpace(row.GroupName)
		if pname == "" {
			pname = "未绑定项目"
		}
		if gname == "" {
			gname = "未分组"
		}
		key := projectKey(pid, pname, gid, gname)
		entry, ok := projectByKey[key]
		if !ok {
			entry = &TaskHistoryStatsProject{
				ProjectID:   pid,
				ProjectName: pname,
				GroupID:     gid,
				GroupName:   gname,
				ByStatus:    map[string]int64{},
				ByMode:      map[string]int64{},
			}
			projectByKey[key] = entry
		}
		mode := strings.TrimSpace(row.Mode)
		if mode == "" {
			mode = "cli"
		}
		entry.ByMode[mode] += row.Count
	}

	for _, entry := range projectByKey {
		stats.ByProject = append(stats.ByProject, *entry)
	}
	sort.Slice(stats.ByProject, func(i, j int) bool {
		if stats.ByProject[i].Total == stats.ByProject[j].Total {
			return stats.ByProject[i].ProjectName < stats.ByProject[j].ProjectName
		}
		return stats.ByProject[i].Total > stats.ByProject[j].Total
	})

	return stats, nil
}

// ListTaskHistory 获取任务历史清单（任务+工作流状态+最新CLI执行）
func (ctrl *TaskController) ListTaskHistory(c *fiber.Ctx) error {
	projectID := strings.TrimSpace(c.Query("project_id"))
	groupID := strings.TrimSpace(c.Query("project_group_id"))
	if groupID == "" {
		groupID = strings.TrimSpace(c.Query("group_id"))
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))
	automationMode := strings.TrimSpace(c.Query("automation_mode"))

	if status != "" {
		normalizedStatus, ok := normalizeTaskStatus(status)
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
		}
		status = normalizedStatus
	}
	if automationMode != "" {
		normalizedMode, ok := normalizeAutomationMode(automationMode)
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid automation_mode"})
		}
		automationMode = normalizedMode
	}

	limit := parseBoundedInt(c.Query("limit"), 100, 1, 500)
	offset := parseBoundedInt(c.Query("offset"), 0, 0, 1000000)

	filter := taskHistoryFilter{
		ProjectID:      projectID,
		GroupID:        groupID,
		Keyword:        keyword,
		Status:         status,
		AutomationMode: automationMode,
	}

	query := applyTaskHistoryFilter(model.DB.Model(&model.Task{}), filter, false)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query tasks"})
	}

	var tasks []model.Task
	if err := query.Order("tasks.updated_at DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	items, err := enrichTasks(tasks)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	taskIDs := make([]string, 0, len(items))
	sessionIDSet := map[string]struct{}{}
	for _, item := range items {
		taskID := strings.TrimSpace(item.ID)
		if taskID != "" {
			taskIDs = append(taskIDs, taskID)
		}
		sid := strings.TrimSpace(item.AgentSessionID)
		if sid != "" {
			sessionIDSet[sid] = struct{}{}
		}
	}

	sessionIDs := make([]string, 0, len(sessionIDSet))
	for sid := range sessionIDSet {
		sessionIDs = append(sessionIDs, sid)
	}

	sessionByID, err := loadWorkflowSessionsByID(sessionIDs)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load workflow sessions"})
	}

	latestExecByTaskID, err := loadLatestExecutionByTaskID(taskIDs)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load CLI executions"})
	}

	stats, err := buildTaskHistoryStats(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to build task history stats"})
	}

	rows := make([]TaskHistoryItem, 0, len(items))
	for _, item := range items {
		row := TaskHistoryItem{
			Task: item,
		}
		if sid := strings.TrimSpace(item.AgentSessionID); sid != "" {
			row.WorkflowSession = sessionByID[sid]
		}
		if exec, ok := latestExecByTaskID[item.ID]; ok {
			row.LatestExecution = exec
		}
		rows = append(rows, row)
	}

	return c.JSON(fiber.Map{
		"items":            rows,
		"count":            len(rows),
		"total":            total,
		"limit":            limit,
		"offset":           offset,
		"project_id":       projectID,
		"project_group_id": groupID,
		"status":           status,
		"automation_mode":  automationMode,
		"keyword":          keyword,
		"stats":            stats,
	})
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

		if taskModel.ServerID == nil && len(taskModel.TargetServerIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Target server is required (local must be configured in Servers)"})
		}

		// 幂等：任务进行中/暂停时直接返回已有会话
		if taskModel.Status == "in_progress" || taskModel.Status == "paused" {
			sessionID := strings.TrimSpace(taskModel.AgentSessionID)
			needsUserAction := taskModel.Status == "paused"
			userHint := ""
			terminalID := ""
			terminalIDs := []string{}

			if sessionID != "" {
				if session, err := ctrl.aiWorkflowEngine.GetSession(sessionID); err == nil && session != nil {
					if strings.EqualFold(strings.TrimSpace(session.Status), "paused") {
						needsUserAction = true
						userHint = strings.TrimSpace(session.Summary)
					}
					if session.Context != nil {
						if v, ok := session.Context["terminal_id"].(string); ok {
							terminalID = strings.TrimSpace(v)
						}
					}
				}
			}

			if terminalID == "" {
				var terminals []model.TerminalSession
				_ = model.DB.Where("task_id = ? AND status = ? AND hidden = ?", id, "running", false).Order("created_at desc").Find(&terminals).Error
				if len(terminals) > 0 {
					terminalID = strings.TrimSpace(terminals[0].ID)
				}
			}
			if terminalID != "" {
				terminalIDs = []string{terminalID}
			}

			return c.JSON(fiber.Map{
				"message":           "Task already running",
				"task":              taskModel,
				"agent_session_id":  sessionID,
				"execution_id":      findLatestExecutionID(id, sessionID),
				"terminal_id":       terminalID,
				"terminal_ids":      terminalIDs,
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
		terminalID := ""
		terminalIDs := []string{}
		if session != nil && session.Context != nil {
			if v, ok := session.Context["terminal_id"].(string); ok {
				terminalID = strings.TrimSpace(v)
			}
		}
		if terminalID == "" {
			var terminals []model.TerminalSession
			_ = model.DB.Where("task_id = ? AND status = ? AND hidden = ?", id, "running", false).Order("created_at desc").Find(&terminals).Error
			if len(terminals) > 0 {
				terminalID = strings.TrimSpace(terminals[0].ID)
			}
		}
		if terminalID != "" {
			terminalIDs = []string{terminalID}
		}

		return c.JSON(fiber.Map{
			"message":           "Task started",
			"task":              taskModel,
			"agent_session_id":  session.ID,
			"execution_id":      findLatestExecutionID(id, session.ID),
			"terminal_id":       terminalID,
			"terminal_ids":      terminalIDs,
			"work_dir":          taskModel.WorkDir,
			"cli_started":       false,
			"needs_user_action": strings.EqualFold(strings.TrimSpace(session.Status), "paused"),
			"user_action_hint":  strings.TrimSpace(session.Summary),
		})
	}

	if mode == "" {
		mode = "cli"
	}
	if mode == "cli" {
		if taskModel.ServerID == nil || strings.TrimSpace(*taskModel.ServerID) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Server is required (local must be configured in Servers)"})
		}
	}
	if mode == "script" {
		hasServer := false
		if taskModel.ServerID != nil && strings.TrimSpace(*taskModel.ServerID) != "" {
			hasServer = true
		}
		if len(taskModel.TargetServerIDs) > 0 {
			hasServer = true
		}
		if !hasServer {
			return c.Status(400).JSON(fiber.Map{"error": "Target server is required (local must be configured in Servers)"})
		}
	}

	// 如果任务已经在进行中/暂停，优先返回匹配服务器的运行中终端，避免误返回到本地终端
	if taskModel.Status == "in_progress" || taskModel.Status == "paused" {
		var terminals []model.TerminalSession
		_ = model.DB.Where("task_id = ? AND status = ?", id, "running").Order("created_at desc").Find(&terminals).Error
		if len(terminals) > 0 {
			desiredServers := map[string]struct{}{}
			if taskModel.ServerID != nil {
				if sid := strings.TrimSpace(*taskModel.ServerID); sid != "" {
					desiredServers[sid] = struct{}{}
				}
			}
			for _, raw := range taskModel.TargetServerIDs {
				if sid := strings.TrimSpace(raw); sid != "" {
					desiredServers[sid] = struct{}{}
				}
			}

			ordered := make([]string, 0, len(terminals))
			seen := map[string]struct{}{}
			appendUnique := func(id string) {
				id = strings.TrimSpace(id)
				if id == "" {
					return
				}
				if _, ok := seen[id]; ok {
					return
				}
				seen[id] = struct{}{}
				ordered = append(ordered, id)
			}

			// 1) 优先选择 server_id 匹配的终端
			if len(desiredServers) > 0 {
				for _, t := range terminals {
					if t.ServerID == nil {
						continue
					}
					sid := strings.TrimSpace(*t.ServerID)
					if sid == "" {
						continue
					}
					if _, ok := desiredServers[sid]; ok {
						appendUnique(t.ID)
					}
				}
			}

			// 2) 若任务有期望服务器但终端缺少 server_id（历史数据），退化为优先返回 SSH 终端
			if len(ordered) == 0 && len(desiredServers) > 0 {
				for _, t := range terminals {
					if strings.EqualFold(strings.TrimSpace(t.Shell), "ssh") {
						appendUnique(t.ID)
					}
				}
			}

			// 3) 补齐剩余终端
			for _, t := range terminals {
				appendUnique(t.ID)
			}

			if len(ordered) > 0 {
				cliStarted := strings.ToLower(strings.TrimSpace(taskModel.AutomationMode)) != "script"
				return c.JSON(fiber.Map{
					"message":           "Task already running",
					"task":              taskModel,
					"execution_id":      findLatestExecutionID(id, strings.TrimSpace(taskModel.AgentSessionID)),
					"terminal_id":       ordered[0],
					"terminal_ids":      ordered,
					"work_dir":          taskModel.WorkDir,
					"cli_started":       cliStarted,
					"needs_user_action": taskModel.Status == "paused",
					"user_action_hint":  "",
				})
			}
		}
		// 任务标记为进行中但不存在运行中终端：允许重新启动（避免卡死）
		taskModel.Status = "todo"
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
	executionID := strings.TrimSpace(result.ExecutionID)
	if executionID == "" {
		executionID = findLatestExecutionID(taskModel.ID, strings.TrimSpace(taskModel.AgentSessionID))
	}

	return c.JSON(fiber.Map{
		"message":           "Task started",
		"task":              result.Task,
		"agent_session_id":  strings.TrimSpace(result.Task.AgentSessionID),
		"execution_id":      executionID,
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
	tasks.Get("/history", ctrl.ListTaskHistory)
	tasks.Get("/:id", ctrl.GetTask)
	tasks.Get("/:id/detail", ctrl.GetTaskDetail)
	tasks.Put("/:id", ctrl.UpdateTask)
	tasks.Delete("/:id", ctrl.DeleteTask)
	tasks.Post("/:id/move", ctrl.MoveTask)
	tasks.Post("/:id/start", ctrl.StartTask)
	tasks.Get("/:id/terminals", ctrl.GetTaskTerminals)
}
