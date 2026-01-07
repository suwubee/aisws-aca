package api

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/ai-coding-assistant/model"
	secretservice "github.com/ai-coding-assistant/service/secret"
	sshservice "github.com/ai-coding-assistant/service/ssh"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	cryptossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type sshTester interface {
	TestConnection(server *model.SSHServer) error
	ExecuteCommand(serverID, command string) (string, error)
	BatchExecute(serverIDs []string, command string) map[string]sshservice.ExecuteResult
}

type SSHServerController struct {
	encryptionKey []byte
	sshManager    sshTester
	terminalMgr   sshTerminalSessionCreator
}

type sshTerminalSessionCreator interface {
	CreateSSHSession(serverID string) (*terminal.Session, error)
}

func NewSSHServerController(masterKey string, terminalMgr sshTerminalSessionCreator) *SSHServerController {
	key := secretservice.DeriveKey(masterKey)
	if len(key) == 0 {
		return &SSHServerController{}
	}

	return &SSHServerController{
		encryptionKey: key,
		sshManager:    sshservice.NewSSHManagerWithKey(key),
		terminalMgr:   terminalMgr,
	}
}

type CreateSSHServerRequest struct {
	Name       string  `json:"name"`
	Host       string  `json:"host"`
	Port       int     `json:"port"`
	Username   string  `json:"username"`
	AuthType   string  `json:"auth_type"`
	Password   string  `json:"password"`
	PrivateKey string  `json:"private_key"`
	Passphrase string  `json:"passphrase"`
	GroupID    *string `json:"group_id"`
	Tags       string  `json:"tags"`
}

type UpdateSSHServerRequest struct {
	Name       *string `json:"name"`
	Host       *string `json:"host"`
	Port       *int    `json:"port"`
	Username   *string `json:"username"`
	AuthType   *string `json:"auth_type"`
	Password   *string `json:"password"`
	PrivateKey *string `json:"private_key"`
	Passphrase *string `json:"passphrase"`
	GroupID    *string `json:"group_id"`
	Tags       *string `json:"tags"`
	LastStatus *string `json:"last_status"`
}

type CreateServerGroupRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ParentID    *string `json:"parent_id"`
}

type BatchExecuteRequest struct {
	ServerIDs []string `json:"server_ids"`
	Command   string   `json:"command"`
}

func isValidSSHAuthType(value string) bool {
	switch value {
	case "password", "key":
		return true
	default:
		return false
	}
}

func normalizeTags(tags string) (string, error) {
	trimmed := strings.TrimSpace(tags)
	if trimmed == "" {
		return "", nil
	}

	var parsed []string
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", err
	}

	return trimmed, nil
}

func encryptIfPresent(key []byte, plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", nil
	}
	return secretservice.EncryptAESGCMBase64(key, plaintext)
}

// CreateServer 添加服务器
func (ctrl *SSHServerController) CreateServer(c *fiber.Ctx) error {
	var req CreateSSHServerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	host := strings.TrimSpace(req.Host)
	if host == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Host is required"})
	}

	port := req.Port
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid port"})
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Username is required"})
	}

	authType := strings.ToLower(strings.TrimSpace(req.AuthType))
	if authType == "" {
		authType = "password"
	}
	if !isValidSSHAuthType(authType) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid auth_type"})
	}

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid tags JSON"})
	}

	var groupID *string
	if req.GroupID != nil {
		trimmed := strings.TrimSpace(*req.GroupID)
		if trimmed != "" {
			var group model.ServerGroup
			if err := model.DB.First(&group, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Group not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query group"})
			}
			groupID = &trimmed
		}
	}

	var encryptedPassword string
	var encryptedKey string
	var encryptedPassphrase string

	switch authType {
	case "password":
		if strings.TrimSpace(req.Password) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Password is required"})
		}
		encryptedPassword, err = encryptIfPresent(ctrl.encryptionKey, req.Password)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt password"})
		}
	case "key":
		if strings.TrimSpace(req.PrivateKey) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Private key is required"})
		}
		encryptedKey, err = encryptIfPresent(ctrl.encryptionKey, req.PrivateKey)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt private key"})
		}
		encryptedPassphrase, err = encryptIfPresent(ctrl.encryptionKey, req.Passphrase)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt passphrase"})
		}
	}

	server := model.SSHServer{
		ID:         uuid.New().String(),
		Name:       name,
		Host:       host,
		Port:       port,
		Username:   username,
		AuthType:   authType,
		Password:   encryptedPassword,
		PrivateKey: encryptedKey,
		Passphrase: encryptedPassphrase,
		GroupID:    groupID,
		Tags:       tags,
		LastStatus: "unknown",
	}

	if err := model.DB.Create(&server).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create server"})
	}

	return c.Status(201).JSON(fiber.Map{"item": server})
}

// ListServers 服务器列表
func (ctrl *SSHServerController) ListServers(c *fiber.Ctx) error {
	var servers []model.SSHServer
	if err := model.DB.Order("created_at desc").Find(&servers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list servers"})
	}

	return c.JSON(fiber.Map{"items": servers})
}

// GetServer 服务器详情
func (ctrl *SSHServerController) GetServer(c *fiber.Ctx) error {
	id := c.Params("id")

	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
	}

	return c.JSON(fiber.Map{"item": server})
}

// UpdateServer 更新服务器
func (ctrl *SSHServerController) UpdateServer(c *fiber.Ctx) error {
	id := c.Params("id")

	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
	}

	var req UpdateSSHServerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := make(map[string]interface{})
	candidate := server

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
		}
		updates["name"] = name
		candidate.Name = name
	}

	if req.Host != nil {
		host := strings.TrimSpace(*req.Host)
		if host == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Host is required"})
		}
		updates["host"] = host
		candidate.Host = host
	}

	if req.Port != nil {
		port := *req.Port
		if port < 1 || port > 65535 {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid port"})
		}
		updates["port"] = port
		candidate.Port = port
	}

	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if username == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Username is required"})
		}
		updates["username"] = username
		candidate.Username = username
	}

	if req.GroupID != nil {
		groupValue := strings.TrimSpace(*req.GroupID)
		if groupValue == "" {
			updates["group_id"] = nil
			candidate.GroupID = nil
		} else {
			var group model.ServerGroup
			if err := model.DB.First(&group, "id = ?", groupValue).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Group not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query group"})
			}
			updates["group_id"] = groupValue
			candidate.GroupID = &groupValue
		}
	}

	if req.Tags != nil {
		tags, err := normalizeTags(*req.Tags)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid tags JSON"})
		}
		updates["tags"] = tags
		candidate.Tags = tags
	}

	if req.LastStatus != nil {
		status := strings.TrimSpace(*req.LastStatus)
		updates["last_status"] = status
		candidate.LastStatus = status
	}

	if req.AuthType != nil {
		authType := strings.ToLower(strings.TrimSpace(*req.AuthType))
		if authType == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Auth type is required"})
		}
		if !isValidSSHAuthType(authType) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid auth_type"})
		}
		updates["auth_type"] = authType
		candidate.AuthType = authType
	}

	if req.Password != nil {
		if strings.TrimSpace(*req.Password) == "" {
			updates["password"] = ""
			candidate.Password = ""
		} else {
			encryptedPassword, err := encryptIfPresent(ctrl.encryptionKey, *req.Password)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt password"})
			}
			updates["password"] = encryptedPassword
			candidate.Password = encryptedPassword
		}
	}

	if req.PrivateKey != nil {
		if strings.TrimSpace(*req.PrivateKey) == "" {
			updates["private_key"] = ""
			candidate.PrivateKey = ""
		} else {
			encryptedKey, err := encryptIfPresent(ctrl.encryptionKey, *req.PrivateKey)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt private key"})
			}
			updates["private_key"] = encryptedKey
			candidate.PrivateKey = encryptedKey
		}
	}

	if req.Passphrase != nil {
		if strings.TrimSpace(*req.Passphrase) == "" {
			updates["passphrase"] = ""
			candidate.Passphrase = ""
		} else {
			encryptedPassphrase, err := encryptIfPresent(ctrl.encryptionKey, *req.Passphrase)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt passphrase"})
			}
			updates["passphrase"] = encryptedPassphrase
			candidate.Passphrase = encryptedPassphrase
		}
	}

	if server.AuthType != candidate.AuthType {
		switch candidate.AuthType {
		case "password":
			updates["private_key"] = ""
			updates["passphrase"] = ""
			candidate.PrivateKey = ""
			candidate.Passphrase = ""
		case "key":
			updates["password"] = ""
			candidate.Password = ""
		}
	}

	if candidate.AuthType == "password" && candidate.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Password is required"})
	}
	if candidate.AuthType == "key" && candidate.PrivateKey == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Private key is required"})
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&server).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update server"})
		}
	}

	model.DB.First(&server, "id = ?", id)
	return c.JSON(fiber.Map{"item": server})
}

// UploadKey 上传SSH私钥（multipart/form-data）
func (ctrl *SSHServerController) UploadKey(c *fiber.Ctx) error {
	id := c.Params("id")

	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
	}

	fileHeader, err := c.FormFile("key")
	if err != nil {
		fileHeader, err = c.FormFile("file")
	}
	if err != nil || fileHeader == nil {
		return c.Status(400).JSON(fiber.Map{"error": "Key file is required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid key file"})
	}
	defer file.Close()

	const maxKeySize = 128 * 1024
	data, err := io.ReadAll(io.LimitReader(file, maxKeySize+1))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid key file"})
	}
	if len(data) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Key file is required"})
	}
	if len(data) > maxKeySize {
		return c.Status(400).JSON(fiber.Map{"error": "Key file is too large"})
	}

	passphrase := c.FormValue("passphrase")

	if strings.TrimSpace(passphrase) != "" {
		if _, err := cryptossh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase)); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid private key"})
		}
	} else {
		if _, err := cryptossh.ParsePrivateKey(data); err != nil {
			var passphraseMissing *cryptossh.PassphraseMissingError
			if errors.As(err, &passphraseMissing) {
				return c.Status(400).JSON(fiber.Map{"error": "Passphrase is required"})
			}
			return c.Status(400).JSON(fiber.Map{"error": "Invalid private key"})
		}
	}

	encryptedKey, err := secretservice.EncryptAESGCMBase64(ctrl.encryptionKey, string(data))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt private key"})
	}

	var encryptedPassphrase string
	if passphrase != "" {
		encryptedPassphrase, err = secretservice.EncryptAESGCMBase64(ctrl.encryptionKey, passphrase)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt passphrase"})
		}
	}

	updates := map[string]interface{}{
		"auth_type":   "key",
		"private_key": encryptedKey,
		"passphrase":  encryptedPassphrase,
		"password":    "",
	}

	if err := model.DB.Model(&server).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update server"})
	}

	model.DB.First(&server, "id = ?", id)
	return c.JSON(fiber.Map{"item": server})
}

// DeleteServer 删除服务器
func (ctrl *SSHServerController) DeleteServer(c *fiber.Ctx) error {
	id := c.Params("id")

	result := model.DB.Delete(&model.SSHServer{}, "id = ?", id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete server"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
	}

	return c.JSON(fiber.Map{"message": "Server deleted"})
}

// ListServerGroups 分组列表
func (ctrl *SSHServerController) ListServerGroups(c *fiber.Ctx) error {
	var groups []model.ServerGroup
	if err := model.DB.Order("name asc").Find(&groups).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list server groups"})
	}

	return c.JSON(fiber.Map{"items": groups})
}

// CreateServerGroup 创建分组
func (ctrl *SSHServerController) CreateServerGroup(c *fiber.Ctx) error {
	var req CreateServerGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	var parentID *string
	if req.ParentID != nil {
		trimmed := strings.TrimSpace(*req.ParentID)
		if trimmed != "" {
			var group model.ServerGroup
			if err := model.DB.First(&group, "id = ?", trimmed).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(400).JSON(fiber.Map{"error": "Parent group not found"})
				}
				return c.Status(500).JSON(fiber.Map{"error": "Failed to query parent group"})
			}
			parentID = &trimmed
		}
	}

	group := model.ServerGroup{
		ID:          uuid.New().String(),
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		ParentID:    parentID,
	}

	if err := model.DB.Create(&group).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create server group"})
	}

	return c.Status(201).JSON(fiber.Map{"item": group})
}

// TestServerConnection 测试服务器连接
func (ctrl *SSHServerController) TestServerConnection(c *fiber.Ctx) error {
	id := c.Params("id")

	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
	}

	if ctrl.sshManager == nil {
		ctrl.sshManager = sshservice.NewSSHManagerWithKey(ctrl.encryptionKey)
	}

	if err := ctrl.sshManager.TestConnection(&server); err != nil {
		_ = model.DB.Model(&model.SSHServer{}).Where("id = ?", server.ID).Update("last_status", "offline").Error
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	_ = model.DB.Model(&model.SSHServer{}).Where("id = ?", server.ID).Update("last_status", "online").Error
	return c.JSON(fiber.Map{"message": "Connection successful"})
}

// BatchExecute 批量执行命令
func (ctrl *SSHServerController) BatchExecute(c *fiber.Ctx) error {
	var req BatchExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	command := strings.TrimSpace(req.Command)
	if command == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Command is required"})
	}

	if len(req.ServerIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "server_ids is required"})
	}

	serverIDs := make([]string, 0, len(req.ServerIDs))
	seen := make(map[string]struct{}, len(req.ServerIDs))
	for _, raw := range req.ServerIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid server id"})
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		serverIDs = append(serverIDs, id)
	}
	if len(serverIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "server_ids is required"})
	}

	if ctrl.sshManager == nil {
		ctrl.sshManager = sshservice.NewSSHManagerWithKey(ctrl.encryptionKey)
	}

	results := ctrl.sshManager.BatchExecute(serverIDs, command)
	return c.JSON(fiber.Map{"results": results})
}

// CreateServerTerminal 创建SSH终端会话
func (ctrl *SSHServerController) CreateServerTerminal(c *fiber.Ctx) error {
	serverID := strings.TrimSpace(c.Params("id"))
	if serverID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Server id is required"})
	}
	if ctrl.terminalMgr == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Terminal manager not configured"})
	}

	session, err := ctrl.terminalMgr.CreateSSHSession(serverID)
	if err != nil {
		if strings.Contains(err.Error(), "server not found") {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create SSH terminal session"})
	}

	return c.Status(201).JSON(fiber.Map{
		"session_id": session.ID(),
	})
}

// RegisterRoutes 注册路由
func (ctrl *SSHServerController) RegisterRoutes(app fiber.Router) {
	servers := app.Group("/servers")
	servers.Post("/", ctrl.CreateServer)
	servers.Get("/", ctrl.ListServers)
	servers.Post("/batch-execute", ctrl.BatchExecute)
	servers.Get("/:id", ctrl.GetServer)
	servers.Put("/:id", ctrl.UpdateServer)
	servers.Delete("/:id", ctrl.DeleteServer)
	servers.Post("/:id/terminal", ctrl.CreateServerTerminal)
	servers.Post("/:id/test", ctrl.TestServerConnection)
	servers.Post("/:id/upload-key", ctrl.UploadKey)

	groups := app.Group("/server-groups")
	groups.Get("/", ctrl.ListServerGroups)
	groups.Post("/", ctrl.CreateServerGroup)
}
