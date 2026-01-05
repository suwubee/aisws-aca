package middleware

import (
	"strings"

	"github.com/ai-coding-assistant/config"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware 认证中间件
func AuthMiddleware(cfg *config.AuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 跳过登录接口
		if c.Path() == "/api/auth/login" {
			return c.Next()
		}

		var tokenString string

		// 先尝试从query参数获取token（WebSocket用）
		tokenString = c.Query("token")

		// 如果query没有，尝试从Authorization header获取
		if tokenString == "" {
			authHeader := c.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString = parts[1]
				}
			}
		}

		if tokenString == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "Missing authorization token",
			})
		}

		// 验证Token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// 提取claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Locals("userID", claims["sub"])
			c.Locals("username", claims["username"])
		}

		return c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件（WebSocket用）
func OptionalAuthMiddleware(cfg *config.AuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 尝试从query参数获取token（WebSocket用）
		tokenString := c.Query("token")
		if tokenString == "" {
			// 尝试从header获取
			authHeader := c.Get("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString = parts[1]
				}
			}
		}

		if tokenString != "" {
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.JWTSecret), nil
			})

			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					c.Locals("userID", claims["sub"])
					c.Locals("username", claims["username"])
				}
			}
		}

		return c.Next()
	}
}
