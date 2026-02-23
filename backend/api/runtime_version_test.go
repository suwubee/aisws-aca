package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRuntimeVersionControllerGetVersion(t *testing.T) {
	startedAt := time.Date(2026, 2, 20, 14, 29, 49, 0, time.UTC)
	ctrl := NewRuntimeVersionController(RuntimeVersionOptions{
		AppName:    "AISWS-ACA",
		ServerHost: "",
		ServerPort: "34007",
		BinaryPath: "/tmp/aca-new",
		PID:        12345,
		StartedAt:  startedAt,
	})
	ctrl.SetStaticDetails("disk:/opt/aca/backend/static", []string{
		"index-b.js",
		"index-a.css",
		"index-b.js",
	})

	app := fiber.New()
	app.Get("/api/runtime/version", ctrl.GetVersion)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/version", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Item RuntimeVersionInfo `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body failed: %v", err)
	}

	if body.Item.AppName != "AISWS-ACA" {
		t.Fatalf("unexpected app name: %q", body.Item.AppName)
	}
	if body.Item.ServerHost != "0.0.0.0" {
		t.Fatalf("expected default server host 0.0.0.0, got %q", body.Item.ServerHost)
	}
	if body.Item.ServerAddr != "0.0.0.0:34007" {
		t.Fatalf("unexpected server addr: %q", body.Item.ServerAddr)
	}
	if body.Item.StaticSource != "disk:/opt/aca/backend/static" {
		t.Fatalf("unexpected static source: %q", body.Item.StaticSource)
	}
	if len(body.Item.StaticIndexAssets) != 2 {
		t.Fatalf("expected 2 static assets, got %d", len(body.Item.StaticIndexAssets))
	}
	if body.Item.StaticIndexAssets[0] != "index-a.css" || body.Item.StaticIndexAssets[1] != "index-b.js" {
		t.Fatalf("unexpected static assets order: %#v", body.Item.StaticIndexAssets)
	}
	if !body.Item.StartedAt.Equal(startedAt) {
		t.Fatalf("unexpected started_at: %s", body.Item.StartedAt.Format(time.RFC3339))
	}
}

func TestRuntimeVersionControllerHealth(t *testing.T) {
	ctrl := NewRuntimeVersionController(RuntimeVersionOptions{
		AppName:    "AISWS-ACA",
		ServerHost: "127.0.0.1",
		ServerPort: "34007",
	})
	ctrl.SetStaticDetails("embedded", []string{"index-x.js"})

	app := fiber.New()
	app.Get("/api/health", ctrl.Health)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body failed: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected status: %#v", body["status"])
	}
	if body["version"] == "" {
		t.Fatalf("expected non-empty version")
	}
	if body["static_source"] != "embedded" {
		t.Fatalf("unexpected static_source: %#v", body["static_source"])
	}
}
