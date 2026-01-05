package api

import (
	"encoding/base64"

	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"go.uber.org/zap"
)

type TerminalController struct {
	manager *terminal.Manager
}

func NewTerminalController(manager *terminal.Manager) *TerminalController {
	return &TerminalController{manager: manager}
}

type CreateTerminalRequest struct {
	Title  string  `json:"title"`
	TaskID *string `json:"task_id"`
}

type TerminalResponse struct {
	ID        string                    `json:"id"`
	Title     string                    `json:"title"`
	TaskID    *string                   `json:"task_id"`
	Status    string                    `json:"status"`
	PID       int                       `json:"pid"`
	Metadata  *terminal.SessionMetadata `json:"metadata"`
	CreatedAt int64                     `json:"created_at"`
}

// CreateTerminal 创建终端
func (ctrl *TerminalController) CreateTerminal(c *fiber.Ctx) error {
	var req CreateTerminalRequest
	if err := c.BodyParser(&req); err != nil {
		req = CreateTerminalRequest{}
	}

	session, err := ctrl.manager.CreateSession(req.Title, req.TaskID)
	if err != nil {
		utils.Error("Failed to create terminal", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create terminal"})
	}

	return c.JSON(fiber.Map{
		"item": TerminalResponse{
			ID:        session.ID(),
			Title:     session.Title(),
			TaskID:    session.TaskID(),
			Status:    session.Status(),
			PID:       session.Metadata().PID,
			Metadata:  session.Metadata(),
			CreatedAt: session.CreatedAt().Unix(),
		},
	})
}

// ListTerminals 获取终端列表
func (ctrl *TerminalController) ListTerminals(c *fiber.Ctx) error {
	taskID := c.Query("task_id")
	var taskIDPtr *string
	if taskID != "" {
		taskIDPtr = &taskID
	}

	sessions := ctrl.manager.ListSessions(taskIDPtr)
	items := make([]TerminalResponse, len(sessions))

	for i, s := range sessions {
		items[i] = TerminalResponse{
			ID:        s.ID(),
			Title:     s.Title(),
			TaskID:    s.TaskID(),
			Status:    s.Status(),
			PID:       s.Metadata().PID,
			Metadata:  s.Metadata(),
			CreatedAt: s.CreatedAt().Unix(),
		}
	}

	return c.JSON(fiber.Map{"items": items})
}

// GetTerminal 获取终端详情
func (ctrl *TerminalController) GetTerminal(c *fiber.Ctx) error {
	id := c.Params("id")
	session := ctrl.manager.GetSession(id)
	if session == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
	}

	return c.JSON(fiber.Map{
		"item": TerminalResponse{
			ID:        session.ID(),
			Title:     session.Title(),
			TaskID:    session.TaskID(),
			Status:    session.Status(),
			PID:       session.Metadata().PID,
			Metadata:  session.Metadata(),
			CreatedAt: session.CreatedAt().Unix(),
		},
	})
}

// CloseTerminal 关闭终端
func (ctrl *TerminalController) CloseTerminal(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := ctrl.manager.CloseSession(id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to close terminal"})
	}
	return c.JSON(fiber.Map{"message": "Terminal closed"})
}

// RenameTerminal 重命名终端
func (ctrl *TerminalController) RenameTerminal(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Title string `json:"title"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if err := ctrl.manager.RenameSession(id, req.Title); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to rename terminal"})
	}
	return c.JSON(fiber.Map{"message": "Terminal renamed"})
}

// LinkTask 关联任务
func (ctrl *TerminalController) LinkTask(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		TaskID *string `json:"task_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if err := ctrl.manager.LinkTask(id, req.TaskID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to link task"})
	}
	return c.JSON(fiber.Map{"message": "Task linked"})
}

// WebSocket消息结构
type WSMessage struct {
	Type     string                    `json:"type"`
	Data     string                    `json:"data,omitempty"`
	Cols     uint16                    `json:"cols,omitempty"`
	Rows     uint16                    `json:"rows,omitempty"`
	Metadata *terminal.SessionMetadata `json:"metadata,omitempty"`
	ExitCode int                       `json:"exit_code,omitempty"`
	Message  string                    `json:"message,omitempty"`
}

// HandleWebSocket WebSocket处理
func (ctrl *TerminalController) HandleWebSocket(c *websocket.Conn) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.WriteJSON(WSMessage{Type: "error", Message: "Missing sessionId"})
		return
	}

	session := ctrl.manager.GetSession(sessionID)
	if session == nil {
		c.WriteJSON(WSMessage{Type: "error", Message: "Session not found"})
		return
	}

	// 发送ready消息
	c.WriteJSON(WSMessage{
		Type:     "ready",
		Metadata: session.Metadata(),
	})

	// 发送历史输出
	scrollback := session.Scrollback()
	if len(scrollback) > 0 {
		c.WriteJSON(WSMessage{
			Type: "data",
			Data: base64.StdEncoding.EncodeToString(scrollback),
		})
	}

	// 订阅会话
	subID, eventCh := session.Subscribe()
	defer session.Unsubscribe(subID)

	// 处理客户端消息
	go func() {
		for {
			var msg WSMessage
			if err := c.ReadJSON(&msg); err != nil {
				utils.Debug("WebSocket read error", zap.Error(err))
				return
			}

			switch msg.Type {
			case "input":
				data, err := base64.StdEncoding.DecodeString(msg.Data)
				if err == nil {
					session.Write(data)
				}
			case "resize":
				session.Resize(msg.Cols, msg.Rows)
			case "close":
				return
			}
		}
	}()

	// 转发会话事件
	for event := range eventCh {
		wsMsg := WSMessage{
			Type:     string(event.Type),
			Data:     event.Data,
			Metadata: event.Metadata,
			ExitCode: event.ExitCode,
			Message:  event.Message,
		}
		if err := c.WriteJSON(wsMsg); err != nil {
			utils.Debug("WebSocket write error", zap.Error(err))
			return
		}
	}
}

// GetTerminalStats 获取终端统计
func (ctrl *TerminalController) GetTerminalStats(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"total": ctrl.manager.GetSessionCount(),
	})
}

// RegisterRoutes 注册路由
func (ctrl *TerminalController) RegisterRoutes(app *fiber.App) {
	terminals := app.Group("/api/terminals")
	terminals.Get("/", ctrl.ListTerminals)
	terminals.Post("/", ctrl.CreateTerminal)
	terminals.Get("/stats", ctrl.GetTerminalStats)
	terminals.Get("/:id", ctrl.GetTerminal)
	terminals.Post("/:id/close", ctrl.CloseTerminal)
	terminals.Post("/:id/rename", ctrl.RenameTerminal)
	terminals.Post("/:id/link-task", ctrl.LinkTask)
	terminals.Get("/:id/logs", ctrl.GetLogs)

	// WebSocket路由
	app.Get("/api/terminal/ws", websocket.New(ctrl.HandleWebSocket))
}

// GetLogs 获取终端日志
func (ctrl *TerminalController) GetLogs(c *fiber.Ctx) error {
	id := c.Params("id")
	limit := c.QueryInt("limit", 1000)
	offset := c.QueryInt("offset", 0)
	logType := c.Query("type") // input, output, 或空表示全部

	query := model.DB.Where("terminal_id = ?", id).Order("created_at asc")

	if logType != "" {
		query = query.Where("log_type = ?", logType)
	}

	var total int64
	query.Model(&model.Log{}).Count(&total)

	var logs []model.Log
	query.Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"items": logs,
		"total": total,
	})
}

// GetDBSessions 获取数据库中的会话列表
func GetDBSessions(c *fiber.Ctx) error {
	var sessions []model.TerminalSession
	model.DB.Order("created_at desc").Find(&sessions)
	return c.JSON(fiber.Map{"items": sessions})
}
