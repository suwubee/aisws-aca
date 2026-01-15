package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v2"
)

func TestRegisterStaticRoutes(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><html><body>INDEX</body></html>")},
		"assets/app.js": {
			Data: []byte("console.log('ok')"),
		},
	}

	app := fiber.New()
	registerStaticRoutes(app, http.FS(staticFS))

	type respInfo struct {
		status int
		ct     string
		body   string
	}

	doReq := func(path string) respInfo {
		req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("request %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return respInfo{
			status: resp.StatusCode,
			ct:     resp.Header.Get("Content-Type"),
			body:   string(bodyBytes),
		}
	}

	t.Run("serves_index_for_root", func(t *testing.T) {
		res := doReq("/")
		if res.status != http.StatusOK {
			t.Fatalf("expected status 200, got %d", res.status)
		}
		if !strings.Contains(res.body, "INDEX") {
			t.Fatalf("expected index.html body")
		}
		if !strings.Contains(res.ct, "text/html") {
			t.Fatalf("expected text/html, got %q", res.ct)
		}
	})

	t.Run("spa_fallback_for_routes", func(t *testing.T) {
		res := doReq("/tasks")
		if res.status != http.StatusOK {
			t.Fatalf("expected status 200, got %d", res.status)
		}
		if !strings.Contains(res.body, "INDEX") {
			t.Fatalf("expected index.html body")
		}
		if !strings.Contains(res.ct, "text/html") {
			t.Fatalf("expected text/html, got %q", res.ct)
		}
	})

	t.Run("serves_assets_without_html_fallback", func(t *testing.T) {
		res := doReq("/assets/app.js")
		if res.status != http.StatusOK {
			t.Fatalf("expected status 200, got %d", res.status)
		}
		if strings.Contains(res.ct, "text/html") {
			t.Fatalf("expected non-html content-type, got %q", res.ct)
		}
		if !strings.Contains(res.body, "console.log") {
			t.Fatalf("expected js body")
		}
	})

	t.Run("missing_assets_return_404", func(t *testing.T) {
		res := doReq("/assets/missing.js")
		if res.status != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", res.status)
		}
	})

	t.Run("file_like_paths_return_404", func(t *testing.T) {
		res := doReq("/foo.bar")
		if res.status != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", res.status)
		}
	})

	t.Run("api_prefix_not_handled_by_spa", func(t *testing.T) {
		res := doReq("/api/health")
		if res.status != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", res.status)
		}
	})
}
