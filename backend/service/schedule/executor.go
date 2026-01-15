package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/task"
	"github.com/ai-coding-assistant/service/workflow"
)

type DefaultExecutor struct {
	Automation *task.AutomationService
	AIWorkflow *workflow.AIWorkflowEngine
}

func (e DefaultExecutor) Execute(ctx context.Context, job model.ScheduledJob) (any, error) {
	switch job.TargetType {
	case TargetTypeTask:
		if job.TaskID == nil || strings.TrimSpace(*job.TaskID) == "" {
			return nil, errors.New("task_id is required")
		}

		var taskModel model.Task
		if err := model.DB.First(&taskModel, "id = ?", strings.TrimSpace(*job.TaskID)).Error; err != nil {
			return nil, err
		}

		if strings.EqualFold(strings.TrimSpace(taskModel.AutomationMode), "agent") {
			if e.AIWorkflow == nil {
				return nil, errors.New("AI workflow engine not configured")
			}

			// 幂等：任务进行中/暂停时不重复启动
			if taskModel.Status == "in_progress" || taskModel.Status == "paused" {
				return map[string]any{
					"task_id":           taskModel.ID,
					"agent_session_id":  strings.TrimSpace(taskModel.AgentSessionID),
					"needs_user_action": taskModel.Status == "paused",
				}, nil
			}

			session, err := e.AIWorkflow.StartTaskAgent(ctx, &taskModel)
			if session != nil {
				out := map[string]any{
					"task_id":          taskModel.ID,
					"agent_session_id": session.ID,
					"status":           session.Status,
				}
				if strings.TrimSpace(session.Summary) != "" {
					out["summary"] = strings.TrimSpace(session.Summary)
				}
				out["needs_user_action"] = strings.EqualFold(strings.TrimSpace(session.Status), "paused")
				return out, err
			}
			return map[string]any{"task_id": taskModel.ID}, err
		}

		if e.Automation == nil {
			return nil, errors.New("task automation service not configured")
		}

		result, err := e.Automation.StartTask(&taskModel)
		if result != nil {
			out := map[string]any{
				"task_id": taskModel.ID,
			}
			if result.Terminal != nil {
				out["terminal_id"] = result.Terminal.ID()
			}
			out["work_dir"] = result.WorkDir
			out["cli_started"] = result.CLIStarted
			out["needs_user_action"] = result.NeedsUserAction
			out["user_action_hint"] = result.UserActionHint
			if result.Error != "" {
				out["error"] = result.Error
			}
			return out, err
		}
		return map[string]any{"task_id": taskModel.ID}, err

	case TargetTypeAIWorkflow:
		if e.AIWorkflow == nil {
			return nil, errors.New("AI workflow engine not configured")
		}
		goal := strings.TrimSpace(job.WorkflowGoal)
		if goal == "" {
			return nil, errors.New("workflow_goal is required")
		}

		session, err := e.AIWorkflow.StartWorkflow(ctx, goal)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"session_id": session.ID,
			"status":     session.Status,
			"message":    "工作流已启动",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported target_type %q", job.TargetType)
	}
}
