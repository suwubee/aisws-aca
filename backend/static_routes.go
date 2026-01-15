package main

import (
	"net/http"
	"path"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

func registerStaticRoutes(app *fiber.App, staticRoot http.FileSystem) {
	if app == nil || staticRoot == nil {
		return
	}

	// Serve real files; do NOT fallback to index.html for missing assets (avoid MIME errors for module scripts).
	app.Use("/", filesystem.New(filesystem.Config{
		Root:  staticRoot,
		Index: "index.html",
		Next: func(c *fiber.Ctx) bool {
			requestPath := c.Path()
			if requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") {
				return true
			}
			if requestPath == "/assets" || strings.HasPrefix(requestPath, "/assets/") {
				return false
			}
			if requestPath == "/" {
				return false
			}
			// Let the filesystem middleware serve file-like paths.
			if strings.Contains(path.Base(requestPath), ".") {
				return false
			}
			// SPA route: skip filesystem and let the fallback serve index.html.
			return true
		},
	}))

	// SPA fallback (routes like /tasks, /settings). Assets and file-like paths should return 404.
	spaFallback := func(c *fiber.Ctx) error {
		requestPath := c.Path()
		if requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") {
			return fiber.ErrNotFound
		}
		if requestPath == "/assets" || strings.HasPrefix(requestPath, "/assets/") {
			return fiber.ErrNotFound
		}
		if strings.Contains(path.Base(requestPath), ".") {
			return fiber.ErrNotFound
		}
		return filesystem.SendFile(c, staticRoot, "/index.html")
	}
	app.Get("/*", spaFallback)
	app.Head("/*", spaFallback)
}
