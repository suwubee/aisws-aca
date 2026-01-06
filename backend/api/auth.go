package api

import (
	"errors"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthController struct {
	config *config.AuthConfig
}

func NewAuthController(cfg *config.AuthConfig) *AuthController {
	return &AuthController{config: cfg}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

// Login 用户登录
func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 验证用户名（单用户模式）
	if req.Username != ctrl.config.Username {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid username or password"})
	}

	// 查找或创建用户（密码只存数据库）
	var user model.User
	result := model.DB.Where("username = ?", req.Username).First(&user)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to query user"})
		}

		// 首次启动/初始化：仅允许使用配置中的默认密码创建用户
		if req.Password != ctrl.config.Password {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid username or password"})
		}

		// 首次启动/初始化：写入配置中的默认密码到数据库（哈希）
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to hash password"})
		}

		user = model.User{
			ID:           uuid.New().String(),
			Username:     req.Username,
			PasswordHash: string(hashedPassword),
		}
		if err := model.DB.Create(&user).Error; err != nil {
			// 并发场景下可能已被创建，兜底重新读取
			if err := model.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
			}
		}
	}

	// 验证密码（从数据库读取password_hash进行bcrypt校验）
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid username or password"})
	}

	// 生成JWT Token
	expiresAt := time.Now().Add(ctrl.config.JWTExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"exp":      expiresAt.Unix(),
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(ctrl.config.JWTSecret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	return c.JSON(LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Unix(),
		User: struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		}{
			ID:       user.ID,
			Username: user.Username,
		},
	})
}

// Logout 用户登出
func (ctrl *AuthController) Logout(c *fiber.Ctx) error {
	// JWT是无状态的，客户端只需删除token即可
	return c.JSON(fiber.Map{"message": "Logged out successfully"})
}

// Me 获取当前用户
func (ctrl *AuthController) Me(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	username := c.Locals("username")

	if userID == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	return c.JSON(fiber.Map{
		"id":       userID,
		"username": username,
	})
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 修改密码
func (ctrl *AuthController) ChangePassword(c *fiber.Ctx) error {
	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Old password and new password are required"})
	}

	if len(req.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "New password must be at least 6 characters"})
	}

	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	// 验证旧密码（从数据库读取password_hash进行bcrypt校验）
	var user model.User
	result := model.DB.Where("id = ?", userID).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query user"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Old password is incorrect"})
	}

	// 更新数据库中的密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to hash password"})
	}
	if err := model.DB.Model(&model.User{}).Where("id = ?", userID).Update("password_hash", string(hashedPassword)).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update password"})
	}

	return c.JSON(fiber.Map{"message": "Password changed successfully"})
}

// ResetData 重置所有数据
func (ctrl *AuthController) ResetData(c *fiber.Ctx) error {
	// 按依赖顺序清理（子表 -> 父表），并兼容旧库/旧表缺失的情况
	tables := []string{
		"logs",
		"approval_records",
		"ai_sessions",
		"messages",
		"terminal_sessions",
		"tasks",
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, table := range tables {
			if !tx.Migrator().HasTable(table) {
				continue
			}
			if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to reset data"})
	}

	return c.JSON(fiber.Map{"message": "All data has been reset"})
}

// RegisterRoutes 注册路由
func (ctrl *AuthController) RegisterRoutes(app *fiber.App) {
	auth := app.Group("/api/auth")
	auth.Post("/login", ctrl.Login)
	auth.Post("/logout", ctrl.Logout)
	auth.Get("/me", ctrl.Me)
	auth.Post("/change-password", ctrl.ChangePassword)
	auth.Post("/reset-data", ctrl.ResetData)
}
