package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func DemoModeMiddleware(enabled bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !enabled {
			return c.Next()
		}

		method := strings.ToUpper(c.Method())
		switch method {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}

		// Allow logout even in demo mode.
		if method == fiber.MethodPost && c.Path() == "/api/auth/logout" {
			return c.Next()
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Demo mode: read-only",
		})
	}
}

