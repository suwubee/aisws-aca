package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func setupLogExportTestApp(t *testing.T) (*fiber.App, string, []model.Log) {
	t.Helper()

	dsn := fmt.Sprintf("file:log_export_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	if err := model.InitDB(dsn); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	terminalID1 := uuid.New().String()
	terminalID2 := uuid.New().String()
	taskID := uuid.New().String()

	logs := []model.Log{
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID1,
			TaskID:     &taskID,
			LogType:    "input",
			Content:    "hello,world",
			CreatedAt:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local),
		},
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID1,
			LogType:    "output",
			Content:    "line1\nline2",
			CreatedAt:  time.Date(2024, 1, 2, 12, 0, 0, 0, time.Local),
		},
		{
			ID:         uuid.New().String(),
			TerminalID: &terminalID2,
			LogType:    "system",
			Content:    "other",
			CreatedAt:  time.Date(2024, 1, 3, 12, 0, 0, 0, time.Local),
		},
	}

	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("create logs failed: %v", err)
	}

	app := fiber.New()
	apiGroup := app.Group("/api")

	ctrl := NewLogExportController()
	ctrl.RegisterRoutes(apiGroup)

	return app, terminalID1, logs
}

func TestLogExportController_ExportJSON_ByDateRange(t *testing.T) {
	app, _, logs := setupLogExportTestApp(t)

	req := httptest.NewRequest("GET", "/api/logs/export?format=json&start_date=2024-01-01&end_date=2024-01-02", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected content-type application/json, got %q", got)
	}

	var exported []model.Log
	if err := json.NewDecoder(resp.Body).Decode(&exported); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(exported) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(exported))
	}
	if exported[0].ID != logs[0].ID {
		t.Fatalf("expected first log id %q, got %q", logs[0].ID, exported[0].ID)
	}
	if exported[1].ID != logs[1].ID {
		t.Fatalf("expected second log id %q, got %q", logs[1].ID, exported[1].ID)
	}
}

func TestLogExportController_ExportCSV_WithTerminalFilter(t *testing.T) {
	app, terminalID, logs := setupLogExportTestApp(t)

	req := httptest.NewRequest(
		"GET",
		fmt.Sprintf("/api/logs/export?format=csv&start_date=2024-01-01&end_date=2024-01-03&terminal_id=%s", terminalID),
		nil,
	)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Fatalf("expected content-type text/csv, got %q", got)
	}

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 csv rows (header + 2 logs), got %d", len(records))
	}

	if records[0][0] != "id" || records[0][1] != "terminal_id" {
		t.Fatalf("unexpected csv header: %v", records[0])
	}

	if records[1][0] != logs[0].ID {
		t.Fatalf("expected first csv log id %q, got %q", logs[0].ID, records[1][0])
	}
	if records[1][4] != logs[0].Content {
		t.Fatalf("expected first csv content %q, got %q", logs[0].Content, records[1][4])
	}

	if records[2][0] != logs[1].ID {
		t.Fatalf("expected second csv log id %q, got %q", logs[1].ID, records[2][0])
	}
	if records[2][4] != logs[1].Content {
		t.Fatalf("expected second csv content %q, got %q", logs[1].Content, records[2][4])
	}
}

func TestLogExportController_Export_Validation(t *testing.T) {
	app, _, _ := setupLogExportTestApp(t)

	cases := []struct {
		name       string
		url        string
		statusCode int
	}{
		{
			name:       "missing_dates",
			url:        "/api/logs/export?format=json",
			statusCode: 400,
		},
		{
			name:       "invalid_format",
			url:        "/api/logs/export?format=xml&start_date=2024-01-01&end_date=2024-01-02",
			statusCode: 400,
		},
		{
			name:       "invalid_start_date",
			url:        "/api/logs/export?format=json&start_date=bad&end_date=2024-01-02",
			statusCode: 400,
		},
		{
			name:       "start_after_end",
			url:        "/api/logs/export?format=json&start_date=2024-01-03&end_date=2024-01-02",
			statusCode: 400,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("GET request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.statusCode {
				t.Fatalf("expected status %d, got %d", tc.statusCode, resp.StatusCode)
			}
		})
	}
}
