package workflow

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/ai-coding-assistant/model"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
	"github.com/google/uuid"
)

const (
	taskAgentPollInterval = 2 * time.Second
	taskAgentMaxDuration  = 2 * time.Hour
)

var taskAgentMonitors sync.Map // sessionID -> struct{}

func (e *AIWorkflowEngine) StartTaskAgent(ctx context.Context, task *model.Task) (*AIWorkflowSession, error) {
	if e == nil {
		return nil, errors.New("ai workflow engine is nil")
	}
	if task == nil {
		return nil, errors.New("task is nil")
	}

	goal := taskAgentGoal(task)
	if goal == "" {
		return nil, errors.New("task goal is empty")
	}

	targetServerIDs := collectTaskTargetServerIDs(task)
	if len(targetServerIDs) == 0 {
		return nil, errors.New("target server is required")
	}
	targetServers := loadTargetServers(targetServerIDs)

	sysVars := map[string]any{
		"task_title":             strings.TrimSpace(task.Title),
		"task_description":       strings.TrimSpace(task.Description),
		"task_initial_prompt":    strings.TrimSpace(task.InitialPrompt),
		"task_ai_prompt":         strings.TrimSpace(task.AIPrompt),
		"task_ai_end_condition":  strings.TrimSpace(task.AIEndCondition),
		"task_ai_error_handling": strings.TrimSpace(task.AIErrorHandling),
		"work_dir":               strings.TrimSpace(task.WorkDir),
		"target_servers":         targetServers,
	}

	userVars := map[string]any{
		"user_goal": goal,
	}
	for k, v := range sysVars {
		userVars[k] = v
	}

	sessionCtx := map[string]any{
		"task_id": task.ID,
	}
	if workDir := strings.TrimSpace(task.WorkDir); workDir != "" {
		sessionCtx["work_dir"] = workDir
	}
	if len(targetServerIDs) > 0 {
		sessionCtx["target_server_ids"] = append([]string(nil), targetServerIDs...)
		sessionCtx["current_server_id"] = targetServerIDs[0]
	}

	session, err := e.StartWorkflowWithOptions(ctx, goal, StartWorkflowOptions{
		WorkflowID:              task.ID,
		Context:                 sessionCtx,
		SystemPromptTemplateKey: promptsvc.TemplateKeyTaskAgentSystemPrompt,
		SystemPromptVars:        sysVars,
		UserGoalTemplateKey:     promptsvc.TemplateKeyTaskAgentUserGoalPrompt,
		UserGoalVars:            userVars,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":           "in_progress",
		"agent_session_id": session.ID,
		"updated_at":       now,
	})

	go e.monitorTaskAgent(task.ID, session.ID)

	return session, nil
}

func taskAgentGoal(task *model.Task) string {
	if task == nil {
		return ""
	}
	if text := strings.TrimSpace(task.InitialPrompt); text != "" {
		return text
	}
	if text := strings.TrimSpace(task.Description); text != "" {
		return text
	}
	return strings.TrimSpace(task.Title)
}

func collectTaskTargetServerIDs(task *model.Task) []string {
	if task == nil {
		return nil
	}
	seen := map[string]struct{}{}
	ids := make([]string, 0)
	for _, raw := range task.TargetServerIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 && task.ServerID != nil {
		if id := strings.TrimSpace(*task.ServerID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func loadTargetServers(serverIDs []string) []map[string]any {
	if model.DB == nil {
		return nil
	}
	ids := make([]string, 0, len(serverIDs))
	for _, raw := range serverIDs {
		id := strings.TrimSpace(raw)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var servers []model.SSHServer
	_ = model.DB.Select("id", "name", "host").Where("id IN ?", ids).Find(&servers).Error

	byID := make(map[string]model.SSHServer, len(servers))
	for _, s := range servers {
		byID[s.ID] = s
	}

	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		s, ok := byID[id]
		if !ok {
			out = append(out, map[string]any{"id": id, "name": id, "host": ""})
			continue
		}
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = id
		}
		out = append(out, map[string]any{"id": s.ID, "name": name, "host": strings.TrimSpace(s.Host)})
	}
	return out
}

func (e *AIWorkflowEngine) monitorTaskAgent(taskID, sessionID string) {
	if e == nil || model.DB == nil {
		return
	}

	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return
	}
	if _, loaded := taskAgentMonitors.LoadOrStore(sid, struct{}{}); loaded {
		return
	}
	defer taskAgentMonitors.Delete(sid)

	tid := strings.TrimSpace(taskID)
	if tid == "" {
		return
	}

	start := time.Now()
	lastStatus := ""
	lastSummary := ""

	for {
		if time.Since(start) > taskAgentMaxDuration {
			updateTaskStatus(tid, "timeout")
			createTaskMessage(tid, "warning", "AI 托管超时", "达到最大执行时长，已停止监控")
			return
		}

		time.Sleep(taskAgentPollInterval)

		session, err := e.GetSession(sid)
		if err != nil || session == nil {
			return
		}

		status := strings.ToLower(strings.TrimSpace(session.Status))
		summary := strings.TrimSpace(session.Summary)

		if status == "" {
			status = "running"
		}

		if status == lastStatus && !(status == "paused" && summary != lastSummary) {
			continue
		}

		switch status {
		case "paused":
			updateTaskStatus(tid, "paused")
			if summary == "" {
				summary = "AI 需要用户补充信息/确认后继续"
			}
			createTaskMessage(tid, "approval_needed", "AI 托管需要确认", summary)

		case "completed":
			updateTaskStatus(tid, "done")
			if summary != "" {
				createTaskMessage(tid, "info", "AI 托管完成", summary)
			}
			return

		case "failed", "cancelled":
			updateTaskStatus(tid, "failed")
			if summary == "" {
				summary = "AI 托管执行失败"
			}
			createTaskMessage(tid, "error", "AI 托管失败", summary)
			return

		default:
			updateTaskStatus(tid, "in_progress")
		}

		lastStatus = status
		lastSummary = summary
	}
}

func updateTaskStatus(taskID, status string) {
	if model.DB == nil {
		return
	}
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == "done" {
		updates["completed_at"] = time.Now()
	}
	model.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(updates)
}

func createTaskMessage(taskID, msgType, title, content string) {
	if model.DB == nil {
		return
	}
	tid := strings.TrimSpace(taskID)
	if tid == "" {
		return
	}
	msg := model.Message{
		ID:        uuid.New().String(),
		TaskID:    &tid,
		Type:      strings.TrimSpace(msgType),
		Title:     strings.TrimSpace(title),
		Content:   strings.TrimSpace(content),
		Status:    "unread",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = model.DB.Create(&msg).Error
}
