package api

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/model"
	secretservice "github.com/ai-coding-assistant/service/secret"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SecretController struct {
	encryptionKey []byte
}

func NewSecretController(masterKey string) *SecretController {
	key := secretservice.DeriveKey(masterKey)
	if len(key) == 0 {
		return &SecretController{}
	}

	return &SecretController{
		encryptionKey: key,
	}
}

type CreateSecretRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Plaintext string `json:"plaintext"`
	Meta      string `json:"meta"`
}

type UpdateSecretRequest struct {
	Name      *string `json:"name"`
	Type      *string `json:"type"`
	Plaintext *string `json:"plaintext"`
	Meta      *string `json:"meta"`
}

func isValidSecretType(value string) bool {
	switch value {
	case "ssh_password", "ssh_key", "api_key":
		return true
	default:
		return false
	}
}

func validateMeta(meta string) bool {
	trimmed := strings.TrimSpace(meta)
	if trimmed == "" {
		return true
	}
	return json.Valid([]byte(trimmed))
}

// ListSecrets 获取凭据列表（不返回明文）
func (ctrl *SecretController) ListSecrets(c *fiber.Ctx) error {
	var secrets []model.Secret
	if err := model.DB.Order("created_at desc").Find(&secrets).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list secrets"})
	}

	return c.JSON(fiber.Map{"items": secrets})
}

// CreateSecret 创建凭据
func (ctrl *SecretController) CreateSecret(c *fiber.Ctx) error {
	var req CreateSecretRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	secretType := strings.TrimSpace(req.Type)
	if secretType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Type is required"})
	}
	if !isValidSecretType(secretType) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid secret type"})
	}

	if req.Plaintext == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Plaintext is required"})
	}

	meta := strings.TrimSpace(req.Meta)
	if !validateMeta(meta) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid meta JSON"})
	}

	ciphertext, err := secretservice.EncryptAESGCMBase64(ctrl.encryptionKey, req.Plaintext)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt secret"})
	}

	secret := model.Secret{
		ID:         uuid.New().String(),
		Name:       name,
		Type:       secretType,
		Ciphertext: ciphertext,
		Meta:       meta,
	}

	if err := model.DB.Create(&secret).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create secret"})
	}

	return c.Status(201).JSON(fiber.Map{"item": secret})
}

// UpdateSecret 更新凭据
func (ctrl *SecretController) UpdateSecret(c *fiber.Ctx) error {
	id := c.Params("id")

	var secret model.Secret
	if err := model.DB.First(&secret, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Secret not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query secret"})
	}

	var req UpdateSecretRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := make(map[string]interface{})

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
		}
		updates["name"] = name
	}

	if req.Type != nil {
		secretType := strings.TrimSpace(*req.Type)
		if secretType == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Type is required"})
		}
		if !isValidSecretType(secretType) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid secret type"})
		}
		updates["type"] = secretType
	}

	if req.Meta != nil {
		meta := strings.TrimSpace(*req.Meta)
		if !validateMeta(meta) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid meta JSON"})
		}
		updates["meta"] = meta
	}

	if req.Plaintext != nil {
		if *req.Plaintext == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Plaintext is required"})
		}

		ciphertext, err := secretservice.EncryptAESGCMBase64(ctrl.encryptionKey, *req.Plaintext)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt secret"})
		}
		updates["ciphertext"] = ciphertext
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := model.DB.Model(&secret).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update secret"})
		}
	}

	model.DB.First(&secret, "id = ?", id)
	return c.JSON(fiber.Map{"item": secret})
}

// DeleteSecret 删除凭据
func (ctrl *SecretController) DeleteSecret(c *fiber.Ctx) error {
	id := c.Params("id")

	result := model.DB.Delete(&model.Secret{}, "id = ?", id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete secret"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Secret not found"})
	}

	return c.JSON(fiber.Map{"message": "Secret deleted"})
}

// RegisterRoutes 注册路由
func (ctrl *SecretController) RegisterRoutes(app fiber.Router) {
	secrets := app.Group("/secrets")
	secrets.Get("/", ctrl.ListSecrets)
	secrets.Post("/", ctrl.CreateSecret)
	secrets.Put("/:id", ctrl.UpdateSecret)
	secrets.Delete("/:id", ctrl.DeleteSecret)
}
