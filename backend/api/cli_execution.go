package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	clisvc "github.com/ai-coding-assistant/service/cli"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/gofiber/fiber/v2"
)

type CLIExecutionController struct {
	terminalManager *terminal.Manager
	taskLauncher    *clisvc.TaskLauncher
}

const (
	defaultExecutionEventsLimit = 200
	maxExecutionEventsLimit     = 1000

	defaultStreamPollMS       = 800
	minStreamPollMS           = 100
	maxStreamPollMS           = 5000
	defaultStreamTimeoutSec   = 60
	maxStreamTimeoutSec       = 300
	streamHeartbeatInterval   = 10 * time.Second
	iso8601MillisWithTZLayout = "2006-01-02T15:04:05.000Z07:00"
)

func NewCLIExecutionController(tm *terminal.Manager) *CLIExecutionController {
	ctrl := &CLIExecutionController{terminalManager: tm}
	if tm != nil {
		ctrl.taskLauncher = clisvc.NewTaskLauncher(tm)
	}
	return ctrl
}

type CLIExecutionEventResponse struct {
	Seq       uint64      `json:"seq"`
	EventType string      `json:"event_type"`
	Payload   interface{} `json:"payload"`
	CreatedAt string      `json:"created_at"`
}

func (ctrl *CLIExecutionController) ListExecutions(c *fiber.Ctx) error {
	status := strings.TrimSpace(c.Query("status"))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	taskID := strings.TrimSpace(c.Query("task_id"))
	workflowSessionID := strings.TrimSpace(c.Query("workflow_session_id"))
	parentID := strings.TrimSpace(c.Query("parent_id"))
	role := strings.TrimSpace(c.Query("role"))
	mode := strings.TrimSpace(c.Query("mode"))
	source := strings.TrimSpace(c.Query("source"))
	tool := strings.TrimSpace(c.Query("tool"))

	items, err := clisvc.ListExecutionsByFilter(clisvc.ListExecutionsInput{
		Status:            status,
		TaskID:            taskID,
		WorkflowSessionID: workflowSessionID,
		ParentExecutionID: parentID,
		Role:              role,
		Mode:              mode,
		Source:            source,
		Tool:              tool,
		Limit:             limit,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list CLI executions"})
	}

	return c.JSON(fiber.Map{
		"items":               items,
		"count":               len(items),
		"status":              status,
		"task_id":             taskID,
		"workflow_session_id": workflowSessionID,
		"parent_id":           parentID,
		"role":                role,
		"mode":                mode,
		"source":              source,
		"tool":                tool,
	})
}

func (ctrl *CLIExecutionController) GetExecution(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "execution id is required"})
	}

	var item model.CLIExecution
	if err := model.DB.First(&item, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "execution not found"})
	}

	return c.JSON(fiber.Map{"item": item})
}

func (ctrl *CLIExecutionController) ListExecutionEvents(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "execution id is required"})
	}

	afterRaw := strings.TrimSpace(c.Query("after"))
	limitRaw := strings.TrimSpace(c.Query("limit"))

	var after uint64
	var limit int
	if afterRaw != "" {
		parsed, err := strconv.ParseUint(afterRaw, 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid after"})
		}
		after = parsed
	}
	if limitRaw != "" {
		parsed, err := strconv.Atoi(limitRaw)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid limit"})
		}
		limit = parsed
	}

	rows, err := clisvc.ListExecutionEvents(id, after, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list execution events"})
	}

	resolvedLimit := limit
	if resolvedLimit <= 0 || resolvedLimit > maxExecutionEventsLimit {
		resolvedLimit = defaultExecutionEventsLimit
	}

	resp := toExecutionEventResponses(rows)

	return c.JSON(fiber.Map{
		"items":   resp,
		"count":   len(resp),
		"after":   after,
		"hasMore": len(resp) == resolvedLimit,
	})
}

func (ctrl *CLIExecutionController) StreamExecutionEvents(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "execution id is required"})
	}

	var execution model.CLIExecution
	if err := model.DB.Select("id", "status").First(&execution, "id = ?", id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "execution not found"})
	}

	after, err := parseUintQueryValue(strings.TrimSpace(c.Query("after")))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid after"})
	}

	limit, err := parseIntQueryValue(strings.TrimSpace(c.Query("limit")), defaultExecutionEventsLimit, 1, maxExecutionEventsLimit)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid limit"})
	}

	pollMS, err := parseIntQueryValue(strings.TrimSpace(c.Query("poll_ms")), defaultStreamPollMS, minStreamPollMS, maxStreamPollMS)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid poll_ms"})
	}

	timeoutSec, err := parseIntQueryValue(strings.TrimSpace(c.Query("timeout_sec")), defaultStreamTimeoutSec, 1, maxStreamTimeoutSec)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid timeout_sec"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		currentAfter := after
		deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
		nextHeartbeatAt := time.Now().Add(streamHeartbeatInterval)
		pollInterval := time.Duration(pollMS) * time.Millisecond

		if err := writeSSEEvent(w, "ready", fiber.Map{
			"execution_id": id,
			"after":        currentAfter,
		}); err != nil {
			return
		}

		for {
			rows, listErr := clisvc.ListExecutionEvents(id, currentAfter, limit)
			if listErr != nil {
				_ = writeSSEEvent(w, "error", fiber.Map{"error": "failed to list execution events"})
				return
			}

			for _, row := range rows {
				payload := CLIExecutionEventResponse{
					Seq:       row.Seq,
					EventType: row.EventType,
					Payload:   parseExecutionEventPayload(row.Payload),
					CreatedAt: row.CreatedAt.Format(iso8601MillisWithTZLayout),
				}
				if err := writeSSEEvent(w, "message", payload); err != nil {
					return
				}
				currentAfter = row.Seq
			}

			status, finished, statusErr := loadExecutionStatus(id)
			if statusErr != nil {
				_ = writeSSEEvent(w, "error", fiber.Map{"error": "failed to load execution status"})
				return
			}

			if finished && len(rows) == 0 {
				_ = writeSSEEvent(w, "done", fiber.Map{
					"execution_id": id,
					"status":       status,
					"after":        currentAfter,
				})
				return
			}

			now := time.Now()
			if !now.Before(deadline) {
				_ = writeSSEEvent(w, "timeout", fiber.Map{
					"execution_id": id,
					"status":       status,
					"after":        currentAfter,
				})
				return
			}

			if !now.Before(nextHeartbeatAt) {
				if _, hbErr := fmt.Fprintf(w, ": keep-alive %d\n\n", now.Unix()); hbErr != nil {
					return
				}
				if hbErr := w.Flush(); hbErr != nil {
					return
				}
				nextHeartbeatAt = now.Add(streamHeartbeatInterval)
			}

			time.Sleep(pollInterval)
		}
	})

	return nil
}

func (ctrl *CLIExecutionController) ListChildExecutions(c *fiber.Ctx) error {
	parentID := strings.TrimSpace(c.Params("id"))
	if parentID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "execution id is required"})
	}

	var parent model.CLIExecution
	if err := model.DB.Select("id").First(&parent, "id = ?", parentID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "execution not found"})
	}

	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	items, err := clisvc.ListExecutionsByFilter(clisvc.ListExecutionsInput{
		ParentExecutionID: parentID,
		Limit:             limit,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list child executions"})
	}

	return c.JSON(fiber.Map{
		"items":     items,
		"count":     len(items),
		"parent_id": parentID,
	})
}

func toExecutionEventResponses(rows []model.CLIExecutionEvent) []CLIExecutionEventResponse {
	resp := make([]CLIExecutionEventResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, CLIExecutionEventResponse{
			Seq:       row.Seq,
			EventType: row.EventType,
			Payload:   parseExecutionEventPayload(row.Payload),
			CreatedAt: row.CreatedAt.Format(iso8601MillisWithTZLayout),
		})
	}
	return resp
}

func parseExecutionEventPayload(raw string) interface{} {
	payload := interface{}(raw)
	if strings.TrimSpace(raw) == "" {
		return payload
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return raw
	}
	return payload
}

func parseUintQueryValue(raw string) (uint64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
}

func parseIntQueryValue(raw string, defaultValue, minValue, maxValue int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	if value < minValue {
		return minValue, nil
	}
	if value > maxValue {
		return maxValue, nil
	}
	return value, nil
}

func loadExecutionStatus(id string) (string, bool, error) {
	var item model.CLIExecution
	if err := model.DB.Select("id", "status").First(&item, "id = ?", id).Error; err != nil {
		return "", false, err
	}
	status := strings.ToLower(strings.TrimSpace(item.Status))
	switch status {
	case clisvc.StatusCompleted, clisvc.StatusError, clisvc.StatusTimeout, clisvc.StatusCancelled:
		return status, true, nil
	default:
		return status, false, nil
	}
}

func writeSSEEvent(w *bufio.Writer, event string, payload any) error {
	if w == nil {
		return fmt.Errorf("writer is nil")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(event) != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", strings.TrimSpace(event)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
		return err
	}
	return w.Flush()
}

func (ctrl *CLIExecutionController) RegisterRoutes(app fiber.Router) {
	group := app.Group("/cli-executions")
	group.Get("/", ctrl.ListExecutions)
	group.Get("/:id/stream", ctrl.StreamExecutionEvents)
	group.Get("/:id/children", ctrl.ListChildExecutions)
	group.Get("/:id", ctrl.GetExecution)
	group.Get("/:id/events", ctrl.ListExecutionEvents)
	group.Post("/:id/resume", ctrl.ResumeExecution)
}
