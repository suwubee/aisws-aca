package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ai-coding-assistant/middleware"
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

// ExportServers 导出服务器列表为CSV（不含密码）
func (ctrl *SSHServerController) ExportServers(c *fiber.Ctx) error {
	var servers []model.SSHServer
	if err := model.DB.Order("created_at desc").Find(&servers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list servers"})
	}

	// 获取所有分组用于映射
	var groups []model.ServerGroup
	model.DB.Find(&groups)
	groupMap := make(map[string]string)
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

	// 构建CSV
	var buf strings.Builder
	buf.WriteString("name,host,port,username,auth_type,group_name,tags\n")
	for _, s := range servers {
		groupName := ""
		if s.GroupID != nil {
			groupName = groupMap[*s.GroupID]
		}
		// CSV转义：双引号内的双引号需要转义
		name := strings.ReplaceAll(s.Name, "\"", "\"\"")
		tags := strings.ReplaceAll(s.Tags, "\"", "\"\"")
		groupName = strings.ReplaceAll(groupName, "\"", "\"\"")
		line := fmt.Sprintf("\"%s\",\"%s\",%d,\"%s\",\"%s\",\"%s\",\"%s\"\n",
			name, s.Host, s.Port, s.Username, s.AuthType, groupName, tags)
		buf.WriteString(line)
	}

	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=servers.csv")
	return c.SendString(buf.String())
}

// ImportServers 从CSV导入服务器
func (ctrl *SSHServerController) ImportServers(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "CSV file is required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Failed to open file"})
	}
	defer file.Close()

	// 读取CSV
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid CSV format"})
	}

	if len(records) < 2 {
		return c.Status(400).JSON(fiber.Map{"error": "CSV file is empty"})
	}

	// 获取现有分组
	var groups []model.ServerGroup
	model.DB.Find(&groups)
	groupNameMap := make(map[string]string) // name -> id
	for _, g := range groups {
		groupNameMap[g.Name] = g.ID
	}

	imported := 0
	skipped := 0
	errors := []string{}

	for i, record := range records[1:] { // 跳过标题行
		if len(record) < 5 {
			errors = append(errors, fmt.Sprintf("Row %d: insufficient columns", i+2))
			continue
		}

		name := strings.TrimSpace(record[0])
		host := strings.TrimSpace(record[1])
		portStr := strings.TrimSpace(record[2])
		username := strings.TrimSpace(record[3])
		authType := strings.TrimSpace(record[4])

		if host == "" || username == "" {
			errors = append(errors, fmt.Sprintf("Row %d: host and username required", i+2))
			continue
		}

		port := 22
		if portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p < 65536 {
				port = p
			}
		}

		if authType == "" {
			authType = "password"
		}

		// 检查是否已存在
		var existing model.SSHServer
		if err := model.DB.Where("host = ? AND port = ? AND username = ?", host, port, username).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		// 处理分组
		var groupID *string
		if len(record) > 5 {
			groupName := strings.TrimSpace(record[5])
			if groupName != "" {
				if gid, ok := groupNameMap[groupName]; ok {
					groupID = &gid
				} else {
					// 创建新分组
					newGroup := model.ServerGroup{
						ID:   uuid.New().String(),
						Name: groupName,
					}
					if err := model.DB.Create(&newGroup).Error; err == nil {
						groupNameMap[groupName] = newGroup.ID
						groupID = &newGroup.ID
					}
				}
			}
		}

		tags := ""
		if len(record) > 6 {
			tags = strings.TrimSpace(record[6])
		}

		server := model.SSHServer{
			ID:         uuid.New().String(),
			Name:       name,
			Host:       host,
			Port:       port,
			Username:   username,
			AuthType:   authType,
			GroupID:    groupID,
			Tags:       tags,
			LastStatus: "unknown",
		}

		if err := model.DB.Create(&server).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Row %d: %v", i+2, err))
			continue
		}
		imported++
	}

	return c.JSON(fiber.Map{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
	})
}

// ShareServerRequest 共享服务器请求
type ShareServerRequest struct {
	UserIDs []string `json:"user_ids"`
}

// ListServerShares 获取服务器的共享用户列表
func (ctrl *SSHServerController) ListServerShares(c *fiber.Ctx) error {
	serverID := strings.TrimSpace(c.Params("id"))
	if serverID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Server id is required"})
	}

	// 验证服务器存在
	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
	}

	// 获取共享记录
	var shares []model.UserServerShare
	if err := model.DB.Where("server_id = ?", serverID).Find(&shares).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list shares"})
	}

	// 获取用户信息
	userIDs := make([]string, len(shares))
	for i, s := range shares {
		userIDs[i] = s.UserID
	}

	type UserInfo struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	var users []model.User
	if len(userIDs) > 0 {
		model.DB.Select("id", "username", "email").Where("id IN ?", userIDs).Find(&users)
	}

	userInfos := make([]UserInfo, len(users))
	for i, u := range users {
		userInfos[i] = UserInfo{ID: u.ID, Username: u.Username, Email: u.Email}
	}

	return c.JSON(fiber.Map{"items": userInfos})
}

// ShareServer 共享服务器给用户
func (ctrl *SSHServerController) ShareServer(c *fiber.Ctx) error {
	serverID := strings.TrimSpace(c.Params("id"))
	if serverID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Server id is required"})
	}

	var req ShareServerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// 验证服务器存在
	var server model.SSHServer
	if err := model.DB.First(&server, "id = ?", serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query server"})
	}

	// 验证用户存在
	userIDs := make([]string, 0, len(req.UserIDs))
	seen := make(map[string]struct{})
	for _, uid := range req.UserIDs {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		userIDs = append(userIDs, uid)
	}

	if len(userIDs) > 0 {
		var users []model.User
		if err := model.DB.Select("id").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to query users"})
		}
		foundIDs := make(map[string]struct{})
		for _, u := range users {
			foundIDs[u.ID] = struct{}{}
		}
		for _, uid := range userIDs {
			if _, ok := foundIDs[uid]; !ok {
				return c.Status(400).JSON(fiber.Map{"error": "User not found: " + uid})
			}
		}
	}

	// 删除现有共享，重新创建
	if err := model.DB.Where("server_id = ?", serverID).Delete(&model.UserServerShare{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update shares"})
	}

	// 创建新共享
	for _, uid := range userIDs {
		share := model.UserServerShare{
			ID:       uuid.New().String(),
			UserID:   uid,
			ServerID: serverID,
		}
		if err := model.DB.Create(&share).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create share"})
		}
	}

	return c.JSON(fiber.Map{"message": "Server shared successfully", "user_count": len(userIDs)})
}

// UnshareServer 取消服务器共享
func (ctrl *SSHServerController) UnshareServer(c *fiber.Ctx) error {
	serverID := strings.TrimSpace(c.Params("id"))
	userID := strings.TrimSpace(c.Params("userId"))

	if serverID == "" || userID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Server id and user id are required"})
	}

	result := model.DB.Where("server_id = ? AND user_id = ?", serverID, userID).Delete(&model.UserServerShare{})
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to remove share"})
	}

	return c.JSON(fiber.Map{"message": "Share removed"})
}

// ListSharedServers 获取用户可访问的共享服务器列表
func (ctrl *SSHServerController) ListSharedServers(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	// 获取用户共享的服务器ID
	var shares []model.UserServerShare
	if err := model.DB.Where("user_id = ?", userID).Find(&shares).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list shares"})
	}

	if len(shares) == 0 {
		return c.JSON(fiber.Map{"items": []model.SSHServer{}})
	}

	serverIDs := make([]string, len(shares))
	for i, s := range shares {
		serverIDs[i] = s.ServerID
	}

	var servers []model.SSHServer
	if err := model.DB.Where("id IN ?", serverIDs).Order("created_at desc").Find(&servers).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list servers"})
	}

	return c.JSON(fiber.Map{"items": servers})
}

// RegisterRoutes 注册路由
func (ctrl *SSHServerController) RegisterRoutes(app fiber.Router) {
	servers := app.Group("/servers")
	servers.Get("/", ctrl.ListServers)
	servers.Get("/:id", ctrl.GetServer)
	servers.Post("/:id/terminal", ctrl.CreateServerTerminal)
	servers.Post("/:id/test", ctrl.TestServerConnection)
	servers.Post("/batch-execute", ctrl.BatchExecute)

	admin := servers.Group("", middleware.RequireRole("admin"))
	admin.Post("/", ctrl.CreateServer)
	admin.Get("/export", ctrl.ExportServers)
	admin.Post("/import", ctrl.ImportServers)
	admin.Put("/:id", ctrl.UpdateServer)
	admin.Delete("/:id", ctrl.DeleteServer)
	admin.Post("/:id/upload-key", ctrl.UploadKey)
	admin.Get("/:id/shares", ctrl.ListServerShares)
	admin.Post("/:id/shares", ctrl.ShareServer)
	admin.Delete("/:id/shares/:userId", ctrl.UnshareServer)

	// 用户可访问的共享服务器
	servers.Get("/shared/list", ctrl.ListSharedServers)

	groups := app.Group("/server-groups")
	groups.Get("/", ctrl.ListServerGroups)
	adminGroups := groups.Group("", middleware.RequireRole("admin"))
	adminGroups.Post("/", ctrl.CreateServerGroup)
}
