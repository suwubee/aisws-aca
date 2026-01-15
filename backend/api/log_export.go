package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
)

type LogExportController struct{}

func NewLogExportController() *LogExportController {
	return &LogExportController{}
}

// ExportLogs 导出日志（JSON/CSV）
func (ctrl *LogExportController) ExportLogs(c *fiber.Ctx) error {
	format := strings.ToLower(strings.TrimSpace(c.Query("format", "json")))
	if format != "json" && format != "csv" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid format, must be json or csv"})
	}

	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))
	if startDateStr == "" || endDateStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "start_date and end_date are required"})
	}

	startTime, endExclusive, err := parseLogExportDateRange(startDateStr, endDateStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	terminalID := strings.TrimSpace(c.Query("terminal_id"))

	query := model.DB.Model(&model.Log{}).
		Where("created_at >= ? AND created_at < ?", startTime, endExclusive).
		Order("created_at asc")

	if terminalID != "" {
		query = query.Where("terminal_id = ?", terminalID)
	}

	var logs []model.Log
	if err := query.Find(&logs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to export logs"})
	}

	endInclusive := endExclusive.Add(-time.Nanosecond)
	filename := buildLogExportFilename(format, terminalID, startTime, endInclusive)

	var data []byte
	var contentType string
	switch format {
	case "json":
		contentType = "application/json; charset=utf-8"
		data, err = json.Marshal(logs)
	case "csv":
		contentType = "text/csv; charset=utf-8"
		data, err = logsToCSV(logs)
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Invalid format"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to build export file"})
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Set("Cache-Control", "no-store")
	return c.Send(data)
}

// RegisterRoutes 注册路由
func (ctrl *LogExportController) RegisterRoutes(app fiber.Router) {
	logs := app.Group("/logs")
	logs.Get("/export", ctrl.ExportLogs)
}

func parseLogExportDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	startTime, startIsDateOnly, err := parseLogExportDate(startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("Invalid start_date")
	}

	endTime, endIsDateOnly, err := parseLogExportDate(endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("Invalid end_date")
	}

	var start time.Time
	if startIsDateOnly {
		start = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	} else {
		start = startTime
	}

	var endExclusive time.Time
	if endIsDateOnly {
		endExclusive = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 0, 0, 0, 0, endTime.Location()).AddDate(0, 0, 1)
	} else {
		endExclusive = endTime.Add(time.Nanosecond)
	}

	if !start.Before(endExclusive) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date must be before end_date")
	}

	return start, endExclusive, nil
}

func parseLogExportDate(value string) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, fmt.Errorf("empty date")
	}

	layoutsWithTZ := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layoutsWithTZ {
		if t, err := time.Parse(layout, value); err == nil {
			return t, false, nil
		}
	}

	layoutsLocal := []struct {
		layout   string
		dateOnly bool
	}{
		{layout: "2006-01-02 15:04:05", dateOnly: false},
		{layout: "2006-01-02T15:04:05", dateOnly: false},
		{layout: "2006-01-02", dateOnly: true},
	}
	for _, item := range layoutsLocal {
		if t, err := time.ParseInLocation(item.layout, value, time.Local); err == nil {
			return t, item.dateOnly, nil
		}
	}

	return time.Time{}, false, fmt.Errorf("invalid date")
}

func logsToCSV(logs []model.Log) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.UseCRLF = true

	if err := writer.Write([]string{"id", "terminal_id", "task_id", "log_type", "content", "created_at"}); err != nil {
		return nil, err
	}

	for _, log := range logs {
		terminalID := ""
		if log.TerminalID != nil {
			terminalID = *log.TerminalID
		}

		taskID := ""
		if log.TaskID != nil {
			taskID = *log.TaskID
		}

		if err := writer.Write([]string{
			log.ID,
			terminalID,
			taskID,
			log.LogType,
			log.Content,
			log.CreatedAt.Format(time.RFC3339Nano),
		}); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buildLogExportFilename(format, terminalID string, start, end time.Time) string {
	terminalPart := "all"
	if terminalID != "" {
		terminalPart = sanitizeFilenamePart(terminalID)
	}

	return fmt.Sprintf(
		"logs_%s_%s_%s.%s",
		terminalPart,
		start.Format("20060102"),
		end.Format("20060102"),
		format,
	)
}

func sanitizeFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	const maxLen = 64
	var b strings.Builder
	b.Grow(len(value))

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}

		if b.Len() >= maxLen {
			break
		}
	}

	out := b.String()
	if out == "" {
		return "unknown"
	}
	return out
}
