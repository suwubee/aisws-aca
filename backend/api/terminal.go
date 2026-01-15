package api

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/ai-coding-assistant/config"
	"github.com/ai-coding-assistant/middleware"
	"github.com/ai-coding-assistant/model"
	"github.com/ai-coding-assistant/service/terminal"
	"github.com/ai-coding-assistant/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TerminalController struct {
	manager  *terminal.Manager
	demoMode bool
}

func NewTerminalController(manager *terminal.Manager) *TerminalController {
	return &TerminalController{manager: manager}
}

func (ctrl *TerminalController) SetDemoMode(enabled bool) {
	ctrl.demoMode = enabled
}

type CreateTerminalRequest struct {
	Title    string  `json:"title"`
	TaskID   *string `json:"task_id"`
	ServerID string  `json:"server_id"`
}

type TerminalResponse struct {
	ID        string                    `json:"id"`
	Title     string                    `json:"title"`
	TaskID    *string                   `json:"task_id"`
	Status    string                    `json:"status"`
	Hidden    bool                      `json:"hidden"`
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

	serverID := strings.TrimSpace(req.ServerID)
	if serverID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Server is required (local must be configured in Servers)"})
	}

	session, err := ctrl.manager.CreateSSHSession(serverID)
	if err != nil {
		utils.Error("Failed to create terminal", zap.Error(err))
		if strings.Contains(err.Error(), "server not found") {
			return c.Status(404).JSON(fiber.Map{"error": "Server not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create terminal"})
	}

	if req.TaskID != nil && strings.TrimSpace(*req.TaskID) != "" {
		_ = ctrl.manager.LinkTask(session.ID(), req.TaskID)
	}
	if strings.TrimSpace(req.Title) != "" {
		_ = ctrl.manager.RenameSession(session.ID(), strings.TrimSpace(req.Title))
	}

	return c.JSON(fiber.Map{
		"item": TerminalResponse{
			ID:        session.ID(),
			Title:     session.Title(),
			TaskID:    session.TaskID(),
			Status:    session.Status(),
			Hidden:    false,
			PID:       session.Metadata().PID,
			Metadata:  session.Metadata(),
			CreatedAt: session.CreatedAt().Unix(),
		},
	})
}

// ListTerminals 获取终端列表
func (ctrl *TerminalController) ListTerminals(c *fiber.Ctx) error {
	taskID := strings.TrimSpace(c.Query("task_id"))
	showHidden := c.Query("show_hidden") == "true"
	includeHistory := c.Query("include_history") == "true"
	var taskIDPtr *string
	if taskID != "" {
		taskIDPtr = &taskID
	}

	sessions := ctrl.manager.ListSessions(taskIDPtr)

	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID())
	}

	type terminalHiddenRow struct {
		ID        string    `gorm:"column:id"`
		Hidden    bool      `gorm:"column:hidden"`
		Status    string    `gorm:"column:status"`
		TaskID    *string   `gorm:"column:task_id"`
		ServerID  *string   `gorm:"column:server_id"`
		Title     string    `gorm:"column:title"`
		PID       int       `gorm:"column:p_id"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	hiddenByID := map[string]bool{}
	dbByID := map[string]terminalHiddenRow{}
	if len(ids) > 0 {
		var rows []terminalHiddenRow
		if err := model.DB.Model(&model.TerminalSession{}).Select("id", "hidden", "status", "task_id", "server_id", "title", "p_id", "created_at").Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to query terminal visibility"})
		}
		for _, r := range rows {
			hiddenByID[r.ID] = r.Hidden
			dbByID[r.ID] = r
		}
	}

	items := make([]TerminalResponse, 0, len(sessions))
	for _, s := range sessions {
		hidden := hiddenByID[s.ID()]
		if hidden && !showHidden {
			continue
		}
		meta := s.Metadata()
		if row, ok := dbByID[s.ID()]; ok {
			if meta != nil {
				if meta.TaskID == nil && row.TaskID != nil {
					meta.TaskID = row.TaskID
				}
				if meta.ServerID == nil && row.ServerID != nil {
					meta.ServerID = row.ServerID
				}
				if strings.TrimSpace(meta.Title) == "" && strings.TrimSpace(row.Title) != "" {
					meta.Title = row.Title
				}
				if meta.PID == 0 && row.PID != 0 {
					meta.PID = row.PID
				}
				if strings.TrimSpace(meta.Status) == "" && strings.TrimSpace(row.Status) != "" {
					meta.Status = row.Status
				}
			}
		}

		taskID := s.TaskID()
		if taskID == nil && meta != nil && meta.TaskID != nil {
			taskID = meta.TaskID
		}
		items = append(items, TerminalResponse{
			ID:        s.ID(),
			Title:     s.Title(),
			TaskID:    taskID,
			Status:    s.Status(),
			Hidden:    hidden,
			PID:       meta.PID,
			Metadata:  meta,
			CreatedAt: s.CreatedAt().Unix(),
		})
	}

	if !includeHistory {
		return c.JSON(fiber.Map{"items": items})
	}

	// include_history=true: merge DB terminal sessions (including exited/hidden) with in-memory sessions for live metadata.
	query := model.DB.Model(&model.TerminalSession{})
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if !showHidden {
		query = query.Where("hidden = ?", false)
	}
	query = query.Order("created_at desc")

	var dbSessions []model.TerminalSession
	if err := query.Find(&dbSessions).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list terminal sessions"})
	}

	type memTerminal struct {
		resp TerminalResponse
	}
	memByID := map[string]memTerminal{}
	for _, it := range items {
		memByID[it.ID] = memTerminal{resp: it}
	}

	merged := make([]TerminalResponse, 0, len(dbSessions)+len(items))
	seen := map[string]struct{}{}

	for _, dbs := range dbSessions {
		id := strings.TrimSpace(dbs.ID)
		if id == "" {
			continue
		}
		if mem, ok := memByID[id]; ok {
			merged = append(merged, mem.resp)
		} else {
			meta := &terminal.SessionMetadata{
				Title:    dbs.Title,
				PID:      dbs.PID,
				Status:   dbs.Status,
				TaskID:   dbs.TaskID,
				ServerID: dbs.ServerID,
			}
			merged = append(merged, TerminalResponse{
				ID:        id,
				Title:     dbs.Title,
				TaskID:    dbs.TaskID,
				Status:    dbs.Status,
				Hidden:    dbs.Hidden,
				PID:       dbs.PID,
				Metadata:  meta,
				CreatedAt: dbs.CreatedAt.Unix(),
			})
		}
		seen[id] = struct{}{}
	}

	// Include any in-memory sessions that are missing from DB (legacy), keeping existing order.
	for _, it := range items {
		if _, ok := seen[it.ID]; ok {
			continue
		}
		merged = append(merged, it)
	}

	return c.JSON(fiber.Map{"items": merged})
}

// GetTerminal 获取终端详情
func (ctrl *TerminalController) GetTerminal(c *fiber.Ctx) error {
	id := c.Params("id")
	id = strings.TrimSpace(id)
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing terminal id"})
	}

	var dbSession model.TerminalSession
	err := model.DB.First(&dbSession, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query terminal"})
	}

	session := ctrl.manager.GetSession(id)
	if session == nil {
		// Best-effort resume for tmux-based sessions.
		if resumed, resumeErr := ctrl.manager.GetOrResumeSession(id); resumeErr == nil && resumed != nil {
			session = resumed
		}
	}

	if session != nil {
		return c.JSON(fiber.Map{
			"item": TerminalResponse{
				ID:        session.ID(),
				Title:     session.Title(),
				TaskID:    session.TaskID(),
				Status:    session.Status(),
				Hidden:    dbSession.Hidden,
				PID:       session.Metadata().PID,
				Metadata:  session.Metadata(),
				CreatedAt: session.CreatedAt().Unix(),
			},
		})
	}

	// Session is not active (e.g., SSH session after restart). Still return DB snapshot for logs/UX.
	meta := &terminal.SessionMetadata{
		Title:    dbSession.Title,
		PID:      dbSession.PID,
		Status:   dbSession.Status,
		TaskID:   dbSession.TaskID,
		ServerID: dbSession.ServerID,
	}
	return c.JSON(fiber.Map{
		"item": TerminalResponse{
			ID:        dbSession.ID,
			Title:     dbSession.Title,
			TaskID:    dbSession.TaskID,
			Status:    dbSession.Status,
			Hidden:    dbSession.Hidden,
			PID:       dbSession.PID,
			Metadata:  meta,
			CreatedAt: dbSession.CreatedAt.Unix(),
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

// RestartTerminal 重启终端（关闭旧会话并创建新会话，保留任务绑定）
func (ctrl *TerminalController) RestartTerminal(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing terminal id"})
	}
	if ctrl.manager == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Terminal manager not configured"})
	}

	session, err := ctrl.manager.RestartSession(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to restart terminal"})
	}

	return c.JSON(fiber.Map{
		"message":           "Terminal restarted",
		"old_terminal_id":   id,
		"new_terminal_id":   session.ID(),
		"terminal_id":       session.ID(),
		"terminal_metadata": session.Metadata(),
	})
}

// HideTerminal 隐藏/显示终端
func (ctrl *TerminalController) HideTerminal(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Hidden bool `json:"hidden"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var existing model.TerminalSession
	err := model.DB.Select("id").First(&existing, "id = ?", id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query terminal"})
	}

	// 终端记录可能因历史原因缺失；若运行中的会话存在，则补齐记录并写入隐藏状态
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if ctrl.manager == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
		}
		session := ctrl.manager.GetSession(id)
		if session == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
		}

		dbSession := session.ToDBModel()
		dbSession.Hidden = req.Hidden
		if err := model.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"hidden": req.Hidden}),
		}).Create(dbSession).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update terminal"})
		}

		return c.JSON(fiber.Map{"message": "Terminal updated", "hidden": req.Hidden})
	}

	if err := model.DB.Model(&model.TerminalSession{}).Where("id = ?", id).Update("hidden", req.Hidden).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update terminal"})
	}
	return c.JSON(fiber.Map{"message": "Terminal updated", "hidden": req.Hidden})
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
	Type           string                    `json:"type"`
	Data           string                    `json:"data,omitempty"`
	Action         string                    `json:"action,omitempty"`
	Cols           uint16                    `json:"cols,omitempty"`
	Rows           uint16                    `json:"rows,omitempty"`
	Metadata       *terminal.SessionMetadata `json:"metadata,omitempty"`
	ExitCode       int                       `json:"exit_code,omitempty"`
	Message        string                    `json:"message,omitempty"`
	ApprovalResult *terminal.ApprovalEvent   `json:"approval_result,omitempty"`
	AILog          *terminal.AILogEvent      `json:"ai_log,omitempty"`
}

// HandleWebSocket WebSocket处理
func (ctrl *TerminalController) HandleWebSocket(c *websocket.Conn) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.WriteJSON(WSMessage{Type: "error", Message: "Missing sessionId"})
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	session := ctrl.manager.GetSession(sessionID)
	if session == nil {
		// Allow reconnect after backend restart for tmux-backed sessions.
		resumed, err := ctrl.manager.GetOrResumeSession(sessionID)
		if err != nil {
			c.WriteJSON(WSMessage{Type: "error", Message: "Failed to resume session"})
			return
		}
		session = resumed
	}
	if session == nil {
		c.WriteJSON(WSMessage{Type: "error", Message: "Session not found"})
		return
	}

	// 更新连接状态为 connected
	ctrl.updateConnectionStatus(sessionID, "connected", "")

	// 确保断开时更新状态
	defer ctrl.updateConnectionStatus(sessionID, "disconnected", "websocket_closed")

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
				if ctrl.demoMode {
					utils.Debug("Demo mode: ignoring terminal input", zap.String("terminal", session.ID()))
					continue
				}
				data, err := base64.StdEncoding.DecodeString(msg.Data)
				if err == nil {
					session.Write(data)
				}
			case "key_action":
				if ctrl.demoMode {
					utils.Debug("Demo mode: ignoring terminal key action",
						zap.String("terminal", session.ID()),
						zap.String("action", msg.Action))
					continue
				}
				if err := session.SendKeyAction(msg.Action); err != nil {
					utils.Warn("Failed to send key action",
						zap.String("terminal", session.ID()),
						zap.String("action", msg.Action),
						zap.Error(err))
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
			Type:           string(event.Type),
			Data:           event.Data,
			Metadata:       event.Metadata,
			ExitCode:       event.ExitCode,
			Message:        event.Message,
			ApprovalResult: event.ApprovalResult,
			AILog:          event.AILog,
		}
		if err := c.WriteJSON(wsMsg); err != nil {
			utils.Debug("WebSocket write error", zap.Error(err))
			return
		}
	}
}

// SendInput 通过HTTP向终端发送输入（用于审批中心等非WS场景）
func (ctrl *TerminalController) SendInput(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing terminal id"})
	}
	if ctrl.manager == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Terminal manager not configured"})
	}

	session := ctrl.manager.GetSession(id)
	if session == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
	}

	var req struct {
		Data  string `json:"data"`  // base64 encoded bytes (preferred)
		Input string `json:"input"` // raw string input (fallback)
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var payload []byte
	if strings.TrimSpace(req.Data) != "" {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Data))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid base64 data"})
		}
		payload = decoded
	} else {
		payload = []byte(req.Input)
	}

	if len(payload) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Empty input"})
	}

	if err := session.Write(payload); err != nil {
		utils.Error("Failed to write terminal input", zap.String("terminal", id), zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "Failed to send input"})
	}

	return c.JSON(fiber.Map{"message": "Input sent"})
}

// SendKeyAction 通过HTTP向终端发送按键动作（用于审批中心等非WS场景）
func (ctrl *TerminalController) SendKeyAction(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing terminal id"})
	}
	if ctrl.manager == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Terminal manager not configured"})
	}

	session := ctrl.manager.GetSession(id)
	if session == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Terminal not found"})
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing action"})
	}

	if err := session.SendKeyAction(action); err != nil {
		utils.Warn("Failed to send terminal key action",
			zap.String("terminal", id),
			zap.String("action", action),
			zap.Error(err))
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Key action sent"})
}

// GetTerminalStats 获取终端统计
func (ctrl *TerminalController) GetTerminalStats(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"total": ctrl.manager.GetSessionCount(),
	})
}

// RegisterRoutes 注册路由
func (ctrl *TerminalController) RegisterRoutes(app fiber.Router) {
	terminals := app.Group("/terminals")
	terminals.Get("/", ctrl.ListTerminals)
	terminals.Post("/", ctrl.CreateTerminal)
	terminals.Get("/stats", ctrl.GetTerminalStats)
	terminals.Get("/:id", ctrl.GetTerminal)
	terminals.Post("/:id/close", ctrl.CloseTerminal)
	terminals.Post("/:id/restart", ctrl.RestartTerminal)
	terminals.Post("/:id/hide", ctrl.HideTerminal)
	terminals.Post("/:id/rename", ctrl.RenameTerminal)
	terminals.Post("/:id/link-task", ctrl.LinkTask)
	terminals.Post("/:id/input", ctrl.SendInput)
	terminals.Post("/:id/key-action", ctrl.SendKeyAction)
	terminals.Get("/:id/logs", ctrl.GetLogs)
	terminals.Delete("/:id/logs", ctrl.ClearLogs)
	terminals.Delete("/:id/logs/:logId", ctrl.DeleteLog)

	// 日志管理路由
	logs := app.Group("/logs")
	logs.Get("/", ctrl.ListAllLogs)
	logs.Get("/sessions", ctrl.ListLogSessions)
	logs.Delete("/:id", ctrl.DeleteLogById)
}

// RegisterWebSocket 注册WebSocket路由（需要单独处理）
func (ctrl *TerminalController) RegisterWebSocket(app *fiber.App) {
	cfg := config.Load()
	app.Get("/api/terminal/ws", middleware.AuthMiddleware(&cfg.Auth), websocket.New(ctrl.HandleWebSocket))
}

// GetLogs 获取终端日志
func (ctrl *TerminalController) GetLogs(c *fiber.Ctx) error {
	id := c.Params("id")
	limit := c.QueryInt("limit", 1000)
	offset := c.QueryInt("offset", 0)
	logType := c.Query("type")                         // input, output, 或空表示全部
	order := strings.ToLower(c.Query("order", "desc")) // asc/desc，默认返回最新日志
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	query := model.DB.Model(&model.Log{}).Where("terminal_id = ?", id)

	if logType != "" {
		query = query.Where("log_type = ?", logType)
	}

	var total int64
	query.Count(&total)

	if order == "asc" {
		query = query.Order("created_at asc")
	} else {
		query = query.Order("created_at desc")
	}

	var logs []model.Log
	query.Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"items": logs,
		"total": total,
		"order": order,
	})
}

// ClearLogs 清空终端日志
func (ctrl *TerminalController) ClearLogs(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := model.DB.Where("terminal_id = ?", id).Delete(&model.Log{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to clear logs"})
	}
	return c.JSON(fiber.Map{"message": "Logs cleared"})
}

// DeleteLog 删除单条日志
func (ctrl *TerminalController) DeleteLog(c *fiber.Ctx) error {
	logId := c.Params("logId")
	if err := model.DB.Where("id = ?", logId).Delete(&model.Log{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete log"})
	}
	return c.JSON(fiber.Map{"message": "Log deleted"})
}

// ListAllLogs 获取所有日志（分页）
func (ctrl *TerminalController) ListAllLogs(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	terminalID := c.Query("terminal_id")
	logType := c.Query("type")
	keyword := c.Query("keyword")

	query := model.DB.Model(&model.Log{}).Order("created_at desc")

	if terminalID != "" {
		query = query.Where("terminal_id = ?", terminalID)
	}
	if logType != "" {
		query = query.Where("log_type = ?", logType)
	}
	if keyword != "" {
		query = query.Where("content LIKE ?", "%"+keyword+"%")
	}

	var total int64
	query.Count(&total)

	var logs []model.Log
	query.Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"items": logs,
		"total": total,
	})
}

// ListLogSessions 获取有日志的终端会话列表
func (ctrl *TerminalController) ListLogSessions(c *fiber.Ctx) error {
	type SessionLogInfo struct {
		TerminalID string `json:"terminal_id"`
		Title      string `json:"title"`
		LogCount   int64  `json:"log_count"`
		FirstLog   string `json:"first_log"`
		LastLog    string `json:"last_log"`
	}

	var results []SessionLogInfo

	// 获取每个终端的日志统计
	rows, err := model.DB.Raw(`
		SELECT
			l.terminal_id,
			COALESCE(t.title, 'Unknown') as title,
			COUNT(l.id) as log_count,
			MIN(l.created_at) as first_log,
			MAX(l.created_at) as last_log
		FROM logs l
		LEFT JOIN terminal_sessions t ON l.terminal_id = t.id
		WHERE l.terminal_id IS NOT NULL
		GROUP BY l.terminal_id, t.title
		ORDER BY last_log DESC
	`).Rows()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get log sessions"})
	}
	defer rows.Close()

	for rows.Next() {
		var info SessionLogInfo
		if err := rows.Scan(&info.TerminalID, &info.Title, &info.LogCount, &info.FirstLog, &info.LastLog); err != nil {
			continue
		}
		results = append(results, info)
	}

	return c.JSON(fiber.Map{"items": results})
}

// DeleteLogById 通过ID删除日志
func (ctrl *TerminalController) DeleteLogById(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := model.DB.Where("id = ?", id).Delete(&model.Log{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete log"})
	}
	return c.JSON(fiber.Map{"message": "Log deleted"})
}

// GetDBSessions 获取数据库中的会话列表
func GetDBSessions(c *fiber.Ctx) error {
	var sessions []model.TerminalSession
	model.DB.Order("created_at desc").Find(&sessions)
	return c.JSON(fiber.Map{"items": sessions})
}

// updateConnectionStatus 更新终端连接状态，并处理关联任务的AI状态
func (ctrl *TerminalController) updateConnectionStatus(terminalID, status, reason string) {
	if model.DB == nil {
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"connection_status": status,
	}

	if status == "disconnected" {
		updates["last_disconnect_at"] = now
		if reason != "" {
			updates["close_reason"] = reason
		}
	}

	if err := model.DB.Model(&model.TerminalSession{}).
		Where("id = ?", terminalID).
		Updates(updates).Error; err != nil {
		utils.Warn("Failed to update terminal connection status",
			zap.String("terminal_id", terminalID),
			zap.String("status", status),
			zap.Error(err))
		return
	}

	// 查找关联的任务，更新AI状态
	var terminal model.TerminalSession
	if err := model.DB.Select("task_id").First(&terminal, "id = ?", terminalID).Error; err != nil {
		return
	}

	if terminal.TaskID == nil {
		return
	}

	// 更新任务的AI状态
	ctrl.updateTaskAIStatus(terminalID, *terminal.TaskID, status)
}

// updateTaskAIStatus 根据终端连接状态更新任务的AI状态
func (ctrl *TerminalController) updateTaskAIStatus(terminalID, taskID, connectionStatus string) {
	if model.DB == nil {
		return
	}

	var task model.Task
	if err := model.DB.Select("id", "active_terminal_id", "ai_status", "ai_pause_reason").
		First(&task, "id = ?", taskID).Error; err != nil {
		return
	}

	// 只处理当前活跃终端的状态变化
	if task.ActiveTerminalID == nil || *task.ActiveTerminalID != terminalID {
		return
	}

	now := time.Now()
	updates := map[string]interface{}{}

	if connectionStatus == "disconnected" {
		// 终端断开，暂停AI
		if task.AIStatus == "running" {
			updates["ai_status"] = "paused"
			updates["ai_pause_reason"] = "terminal_disconnected"
			updates["updated_at"] = now
		}
	} else if connectionStatus == "connected" {
		// 终端重连，恢复AI（如果之前是因为终端断开而暂停）
		if task.AIStatus == "paused" && task.AIPauseReason == "terminal_disconnected" {
			updates["ai_status"] = "running"
			updates["ai_pause_reason"] = ""
			updates["updated_at"] = now
		}
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&model.Task{}).
			Where("id = ?", taskID).
			Updates(updates).Error; err != nil {
			utils.Warn("Failed to update task AI status",
				zap.String("task_id", taskID),
				zap.String("connection_status", connectionStatus),
				zap.Error(err))
		} else {
			utils.Info("Task AI status updated",
				zap.String("task_id", taskID),
				zap.String("terminal_id", terminalID),
				zap.String("connection_status", connectionStatus),
				zap.Any("updates", updates))
		}
	}
}
