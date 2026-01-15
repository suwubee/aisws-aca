package api

import (
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/appsetting"
	"github.com/ai-coding-assistant/service/keybinding"
	promptsvc "github.com/ai-coding-assistant/service/prompt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthController struct {
	config          *config.AuthConfig
	terminalManager terminalSessionCloser
	demoMode        bool
}

func NewAuthController(cfg *config.AuthConfig) *AuthController {
	return &AuthController{config: cfg}
}

type terminalSessionCloser interface {
	CloseAllSessions() error
}

func (ctrl *AuthController) SetTerminalManager(manager terminalSessionCloser) {
	ctrl.terminalManager = manager
}

func (ctrl *AuthController) SetDemoMode(enabled bool) {
	ctrl.demoMode = enabled
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`   // admin, user, viewer
	Status   string `json:"status"` // active, disabled
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
		DemoMode bool   `json:"demo_mode"`
	} `json:"user"`
}

// Login 用户登录
func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	identifier := strings.TrimSpace(req.Username)
	password := req.Password
	if identifier == "" || password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Username and password are required"})
	}

	recordLogin := func(success bool, user *model.User, reason string) {
		if identifier == "" {
			return
		}
		rec := model.LoginRecord{
			ID:         uuid.New().String(),
			Identifier: identifier,
			Success:    success,
			Error:      reason,
			IP:         c.IP(),
			UserAgent:  c.Get("User-Agent"),
			CreatedAt:  time.Now(),
		}
		if user != nil {
			rec.UserID = &user.ID
			rec.Username = user.Username
		}
		_ = model.DB.Create(&rec).Error
	}

	// 从数据库查询用户（按username或email）
	var user model.User
	result := model.DB.Where("username = ? OR email = ?", identifier, identifier).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// 启动初始化：当数据库中没有任何用户时，允许使用配置中的默认账号首次登录并创建管理员
			var userCount int64
			if err := model.DB.Model(&model.User{}).Count(&userCount).Error; err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query user"})
			}
			if userCount == 0 && identifier == ctrl.config.Username && password == ctrl.config.Password {
				hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
				if err != nil {
					recordLogin(false, nil, "internal_error")
					return c.Status(500).JSON(fiber.Map{"error": "Failed to hash password"})
				}

				user = model.User{
					ID:           uuid.New().String(),
					Username:     identifier,
					PasswordHash: string(hashedPassword),
					Role:         "admin",
					Status:       "active",
				}
				if err := model.DB.Create(&user).Error; err != nil {
					recordLogin(false, nil, "internal_error")
					return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
				}
			} else {
				recordLogin(false, nil, "invalid_credentials")
				return c.Status(401).JSON(fiber.Map{"error": "Invalid username or password"})
			}
		} else {
			recordLogin(false, nil, "internal_error")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to query user"})
		}
	}

	// 验证密码（从数据库读取password_hash进行bcrypt校验）
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		recordLogin(false, &user, "invalid_credentials")
		return c.Status(401).JSON(fiber.Map{"error": "Invalid username or password"})
	}

	// 检查用户状态(active/disabled)
	if user.Status != "" && user.Status != "active" {
		recordLogin(false, &user, "user_disabled")
		return c.Status(403).JSON(fiber.Map{"error": "User is disabled"})
	}

	// 兼容旧单用户模式：如果库里只有一个用户且它是配置默认账号，则提升为admin
	if user.Role != "admin" && user.Username == ctrl.config.Username {
		var userCount int64
		if err := model.DB.Model(&model.User{}).Count(&userCount).Error; err == nil && userCount == 1 {
			if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Update("role", "admin").Error; err == nil {
				user.Role = "admin"
			}
		}
	}

	// 更新LastLoginAt
	now := time.Now()
	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Update("last_login_at", now).Error; err != nil {
		recordLogin(false, &user, "internal_error")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update last login time"})
	}

	// 生成JWT Token
	expiresAt := now.Add(ctrl.config.JWTExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      expiresAt.Unix(),
		"iat":      now.Unix(),
	})

	tokenString, err := token.SignedString([]byte(ctrl.config.JWTSecret))
	if err != nil {
		recordLogin(false, &user, "internal_error")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	recordLogin(true, &user, "")
	return c.JSON(LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt.Unix(),
		User: struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
			DemoMode bool   `json:"demo_mode"`
		}{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
			DemoMode: ctrl.demoMode,
		},
	})
}

// Register 管理员创建用户
func (ctrl *AuthController) Register(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
	}

	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := req.Password
	if username == "" || email == "" || password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Username, email and password are required"})
	}
	if len(password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "Password must be at least 6 characters"})
	}

	userRole := strings.TrimSpace(req.Role)
	if userRole == "" {
		userRole = "user"
	}
	switch userRole {
	case "admin", "user", "viewer":
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Invalid role"})
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "disabled":
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
	}

	var existing model.User
	if err := model.DB.Where("username = ? OR email = ?", username, email).First(&existing).Error; err == nil {
		return c.Status(409).JSON(fiber.Map{"error": "User already exists"})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query user"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	user := model.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         userRole,
		Status:       status,
	}
	if err := model.DB.Create(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
	}

	return c.Status(201).JSON(fiber.Map{"item": user})
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
	role := c.Locals("role")

	if userID == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	return c.JSON(fiber.Map{
		"id":        userID,
		"username":  username,
		"role":      role,
		"demo_mode": ctrl.demoMode,
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
	closeErr := error(nil)
	if ctrl.terminalManager != nil {
		closeErr = ctrl.terminalManager.CloseAllSessions()
	}

	protectedTables := map[string]struct{}{
		"users":             {},
		"schema_migrations": {}, // 保留迁移记录，避免重复执行迁移
	}

	// 按依赖顺序清理（子表 -> 父表），并清理所有业务数据表（默认保留用户表与内置模板）
	ordered := []string{
		"logs",
		"approval_records",
		"ai_sessions",
		"messages",
		"app_settings",
		"terminal_sessions",
		"comments",
		"workflow_runs",
		"workflow_nodes",
		"workflows",
		"ai_workflow_sessions",
		"tasks",
		"projects",
		"cli_profiles",
		"secrets",
		"ssh_servers",
		"server_groups",
		"ai_provider_configs",
		"agent_configs",
		"rule_sets",
		"workflow_templates", // 保留内置模板，仅删除自定义模板
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		tables, err := tx.Migrator().GetTables()
		if err != nil {
			return err
		}

		present := make(map[string]struct{}, len(tables))
		for _, table := range tables {
			name := strings.TrimSpace(table)
			if name == "" {
				continue
			}
			present[name] = struct{}{}
		}

		handled := make(map[string]struct{}, len(present))

		deleteAll := func(table string) error {
			return tx.Exec("DELETE FROM " + table).Error
		}

		for _, table := range ordered {
			if _, ok := present[table]; !ok {
				continue
			}
			handled[table] = struct{}{}
			if _, ok := protectedTables[table]; ok {
				continue
			}
			if strings.HasPrefix(table, "sqlite_") {
				continue
			}
			if table == "workflow_templates" {
				if err := tx.Exec("DELETE FROM workflow_templates WHERE is_builtin = ?", false).Error; err != nil {
					return err
				}
				continue
			}
			if err := deleteAll(table); err != nil {
				return err
			}
		}

		// 清理未列入 ordered 的其他业务表，避免漏删（保留 protectedTables 与 sqlite 内置表）
		for table := range present {
			if _, ok := handled[table]; ok {
				continue
			}
			if _, ok := protectedTables[table]; ok {
				continue
			}
			if strings.HasPrefix(table, "sqlite_") {
				continue
			}
			if err := deleteAll(table); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to reset data"})
	}

	if err := promptsvc.EnsureDefaults(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to restore builtin prompt templates"})
	}
	if err := keybinding.EnsureDefaults(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to restore builtin key bindings"})
	}
	if err := appsetting.EnsureDefaults(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to restore builtin settings"})
	}

	resp := fiber.Map{"message": "All data has been reset"}
	if closeErr != nil {
		resp["warning"] = "Some terminal sessions could not be closed"
	}
	return c.JSON(resp)
}

// RegisterRoutes 注册路由
func (ctrl *AuthController) RegisterRoutes(app *fiber.App) {
	auth := app.Group("/api/auth")
	auth.Post("/login", ctrl.Login)
	auth.Post("/register", ctrl.Register)
	auth.Post("/logout", ctrl.Logout)
	auth.Get("/me", ctrl.Me)
	auth.Post("/change-password", ctrl.ChangePassword)
	auth.Post("/reset-data", ctrl.ResetData)
}
