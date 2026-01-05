# AI Coding Assistant - 开发计划

## 1. 项目概述

**项目名称**: AI Coding Assistant (ACA)
**开发语言**: Go (后端) + Vue 3 (前端)
**目标**: 构建一个可视化多任务并行的AI编程助手管理平台

---

## 2. MVP开发计划

### Phase 1: 项目基础架构 (Sprint 1) ✅ 已完成

#### 1.1 后端基础架构
- [x] 初始化Go项目结构
- [x] 配置Fiber/Gin框架
- [x] 设置GORM + SQLite
- [x] 实现基础中间件（日志、CORS、错误处理）
- [x] 创建数据库迁移脚本

**关键文件**:
```
backend/
├── main.go
├── config/
│   └── config.go
├── model/
│   ├── db.go
│   └── migrate.go
└── middleware/
    ├── logger.go
    └── cors.go
```

#### 1.2 前端基础架构
- [x] 初始化Vue 3 + Vite项目
- [x] 配置TypeScript
- [x] 集成Naive UI组件库
- [x] 配置Pinia状态管理
- [x] 设置路由和布局

**关键文件**:
```
frontend/
├── src/
│   ├── main.ts
│   ├── App.vue
│   ├── router/index.ts
│   ├── stores/index.ts
│   └── layouts/MainLayout.vue
└── vite.config.ts
```

---

### Phase 2: 用户认证模块 (Sprint 1) ✅ 已完成

#### 2.1 后端认证
- [x] 用户模型和数据库表
- [x] 密码加密（bcrypt）
- [x] JWT Token生成和验证
- [x] 登录/登出API
- [x] 认证中间件

**API端点**:
```
POST /api/auth/login    - 用户登录
POST /api/auth/logout   - 用户登出
GET  /api/auth/me       - 获取当前用户
```

#### 2.2 前端认证
- [x] 登录页面组件
- [x] 认证状态管理（Pinia）
- [x] 路由守卫
- [x] Token存储和刷新

---

### Phase 3: 终端托管核心 (Sprint 2) ✅ 已完成

#### 3.1 终端管理器
- [x] PTY会话创建和管理
- [x] 会话生命周期管理
- [x] Scrollback缓冲区实现
- [x] 进程状态监控
- [x] 终端日志记录（输入/输出批量写入数据库）

**核心结构**:
```go
// service/terminal/manager.go
type Manager struct {
    sessions sync.Map  // sessionID -> *Session
    config   *TerminalConfig
}

// service/terminal/session.go
type Session struct {
    id          string
    pty         *os.File
    cmd         *exec.Cmd
    subscribers map[string]chan StreamEvent
    scrollback  *ScrollbackBuffer
}
```

#### 3.2 WebSocket通信
- [x] WebSocket升级处理
- [x] 双向消息传递
- [x] 会话订阅机制
- [x] 断线重连支持

**消息协议**:
```typescript
// 客户端 -> 服务端
{ type: "input", data: "base64..." }
{ type: "resize", cols: 120, rows: 40 }

// 服务端 -> 客户端
{ type: "ready", sessionId: "xxx" }
{ type: "data", data: "base64..." }
{ type: "metadata", metadata: {...} }
{ type: "exit", code: 0 }
```

#### 3.3 终端API
- [x] 创建终端会话
- [x] 获取终端列表
- [x] 关闭终端
- [x] 重命名终端
- [x] 获取终端日志 (GET /api/terminals/:id/logs)

---

### Phase 4: 前端终端组件 (Sprint 2) ✅ 已完成

#### 4.1 xterm.js集成
- [x] 安装和配置xterm.js
- [x] 终端组件封装
- [x] 主题配置
- [x] 快捷键绑定

**组件结构**:
```vue
<!-- components/Terminal.vue -->
<template>
  <div class="terminal-container">
    <div class="terminal-header">
      <span>{{ title }}</span>
      <div class="terminal-controls">...</div>
    </div>
    <div ref="terminalRef" class="terminal-body"></div>
  </div>
</template>
```

#### 4.2 多终端管理
- [x] 终端标签页
- [x] 标签拖拽排序
- [x] 终端状态指示
- [x] 终端快捷切换
- [x] 终端浮层/全屏显示
- [x] 终端日志面板

---

### Phase 5: Kanban任务管理 (Sprint 3) ✅ 已完成

#### 5.1 任务后端
- [x] 任务模型和数据库表
- [x] 任务CRUD API
- [x] 任务状态流转
- [x] 拖拽排序逻辑

**API端点**:
```
GET    /api/tasks          - 获取任务列表
POST   /api/tasks          - 创建任务
GET    /api/tasks/:id      - 获取任务详情
PUT    /api/tasks/:id      - 更新任务
DELETE /api/tasks/:id      - 删除任务
POST   /api/tasks/:id/move - 移动任务
```

#### 5.2 Kanban前端
- [x] Kanban看板组件
- [x] 任务卡片组件
- [x] 拖拽功能（@vueuse/drag）
- [x] 任务创建/编辑模态框
- [x] 任务筛选和搜索

---

### Phase 6: 任务-终端集成 (Sprint 3) ✅ 已完成

#### 6.1 关联功能
- [x] 任务关联终端API
- [x] 任务卡片显示关联终端状态
- [x] 从任务打开终端
- [x] 终端关联任务选择器

#### 6.2 界面整合
- [x] 主界面布局（上下分割）
- [x] Kanban在上方
- [x] 终端面板在下方
- [x] 面板大小调整

---

### Phase 7: AI代理检测 (Sprint 4) ✅ 已完成

#### 7.1 基础检测器
- [x] AI代理命令检测
- [x] Claude Code检测器
- [x] 状态追踪器（状态机）
- [x] 元数据定期更新

**状态机**:
```
Unknown -> WaitingInput -> Working -> WaitingApproval
    ^                         |
    |                         v
    +--------- (循环) --------+
```

#### 7.2 前端状态显示
- [x] AI状态指示器组件
- [x] 终端标签状态显示
- [x] 任务卡片状态同步

---

### Phase 8: 审批界面 (Sprint 4) ✅ 已完成（后端）

#### 8.1 权限/确认检测
- [x] 输出模式匹配
- [x] yes/no提示检测
- [x] 权限请求检测
- [x] 审批事件通知（WebSocket推送）

#### 8.2 审批后端
- [x] 审批引擎（规则匹配 + AI辅助）
- [x] 审批记录存储
- [x] 消息/通知系统
- [x] 审批API接口

#### 8.3 审批UI（待开发）
- [ ] 审批提示弹窗
- [ ] 快捷操作按钮（允许/拒绝）
- [ ] 审批历史记录
- [ ] 手动输入框

---

### Phase 9: 自动化配置系统 (Sprint 4) ✅ 已完成（后端）

#### 9.1 AI Provider配置
- [x] OpenAI兼容API服务
- [x] 多Provider支持（OpenAI/DeepSeek/Ollama等）
- [x] API配置CRUD接口
- [x] API Key安全存储

#### 9.2 终端自动化配置
- [x] 三种审批模式（manual/auto_yes/smart）
- [x] 白名单/黑名单规则
- [x] AI辅助决策
- [x] 通知设置

#### 9.3 AI代理检测增强
- [x] Claude Code检测
- [x] Codex检测
- [x] Gemini CLI检测
- [x] 状态机实现

#### 9.4 配置UI（待开发）
- [ ] AI Provider配置页面
- [ ] 终端自动化配置面板
- [ ] 默认模式设置

---

## 3. 技术实现细节

### 3.1 终端托管实现

```go
// 创建PTY会话
func (m *Manager) CreateSession(shell string) (*Session, error) {
    // 创建命令
    cmd := exec.Command(shell)

    // 创建PTY
    pty, err := pty.Start(cmd)
    if err != nil {
        return nil, err
    }

    session := &Session{
        id:          uuid.New().String(),
        pty:         pty,
        cmd:         cmd,
        subscribers: make(map[string]chan StreamEvent),
        scrollback:  NewScrollbackBuffer(256 * 1024),
    }

    // 启动输出读取goroutine
    go session.readPTY()

    m.sessions.Store(session.id, session)
    return session, nil
}

// PTY输出读取
func (s *Session) readPTY() {
    buf := make([]byte, 4096)
    for {
        n, err := s.pty.Read(buf)
        if err != nil {
            s.broadcast(StreamEvent{Type: "exit"})
            return
        }

        data := buf[:n]
        s.scrollback.Write(data)
        s.broadcast(StreamEvent{
            Type: "data",
            Data: base64.StdEncoding.EncodeToString(data),
        })
    }
}
```

### 3.2 WebSocket处理

```go
// WebSocket处理器
func (c *TerminalController) HandleWebSocket(ctx *fiber.Ctx) error {
    sessionID := ctx.Query("sessionId")

    return websocket.New(func(conn *websocket.Conn) {
        session := c.manager.GetSession(sessionID)
        if session == nil {
            return
        }

        // 发送历史输出
        for _, chunk := range session.Scrollback() {
            conn.WriteJSON(WSMessage{Type: "data", Data: chunk})
        }

        // 订阅会话
        ch := session.Subscribe()
        defer session.Unsubscribe(ch)

        // 接收客户端消息
        go func() {
            for {
                var msg WSMessage
                if err := conn.ReadJSON(&msg); err != nil {
                    return
                }

                switch msg.Type {
                case "input":
                    data, _ := base64.StdEncoding.DecodeString(msg.Data)
                    session.Write(data)
                case "resize":
                    session.Resize(msg.Cols, msg.Rows)
                }
            }
        }()

        // 转发会话事件
        for event := range ch {
            conn.WriteJSON(event.ToWSMessage())
        }
    })(ctx)
}
```

### 3.3 AI状态检测

```go
// AI代理检测器
type AIAgentDetector struct {
    detectors []AgentDetector
}

func (d *AIAgentDetector) DetectFromCommand(cmd string) *AIAgentInfo {
    for _, detector := range d.detectors {
        if info := detector.Match(cmd); info != nil {
            return info
        }
    }
    return nil
}

// Claude Code检测器
type ClaudeCodeDetector struct{}

func (d *ClaudeCodeDetector) Match(cmd string) *AIAgentInfo {
    // 匹配 claude, npx claude-code 等命令
    patterns := []string{
        `^claude\s`,
        `npx.*claude-code`,
        `@anthropic-ai/claude-code`,
    }

    for _, p := range patterns {
        if matched, _ := regexp.MatchString(p, cmd); matched {
            return &AIAgentInfo{
                Type: "claude-code",
                Name: "Claude Code",
            }
        }
    }
    return nil
}
```

### 3.4 自动化审批接口（预留）

```go
// 自动化审批引擎接口
type ApprovalEngine interface {
    // 分析并决策
    Analyze(ctx context.Context, req *ApprovalRequest) (*ApprovalDecision, error)

    // 执行决策
    Execute(ctx context.Context, sessionID string, decision *ApprovalDecision) error
}

// 默认实现（手动审批）
type ManualApprovalEngine struct {
    pendingApprovals sync.Map
}

// OpenAI集成实现（预留）
type OpenAIApprovalEngine struct {
    client     *openai.Client
    config     *OpenAIConfig
    promptTmpl string
}

func (e *OpenAIApprovalEngine) Analyze(ctx context.Context, req *ApprovalRequest) (*ApprovalDecision, error) {
    // 构建提示词
    prompt := e.buildPrompt(req)

    // 调用OpenAI API
    resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4",
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: e.config.SystemPrompt},
            {Role: "user", Content: prompt},
        },
        ResponseFormat: &openai.ResponseFormat{
            Type: "json_object",
        },
    })

    // 解析响应
    var decision ApprovalDecision
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &decision)

    return &decision, nil
}
```

---

## 4. MVP验收标准

### 4.1 功能验收

| 功能 | 验收标准 |
|------|---------|
| 用户登录 | 能够使用用户名密码登录，登录后获取JWT Token |
| 终端创建 | 能够创建新终端会话，显示shell提示符 |
| 终端输入输出 | 能够输入命令并看到输出，支持中文 |
| 终端持久化 | 刷新页面后能够重新连接到现有会话 |
| 多终端管理 | 能够同时打开多个终端，通过标签切换 |
| Kanban看板 | 能够创建任务，拖拽改变状态 |
| 任务-终端关联 | 能够将任务与终端关联，从任务打开终端 |
| AI检测 | 能够检测到Claude Code运行状态 |

### 4.2 性能验收

| 指标 | 目标值 |
|------|--------|
| 终端输入延迟 | < 50ms |
| 页面加载时间 | < 2s |
| 并发终端数 | >= 10 |
| WebSocket重连 | < 3s |

---

## 5. 迭代计划

### v1.0.0 - MVP Release
- 完成Phase 1-8所有功能
- 基础文档
- Docker部署支持

### v1.1.0 - 自动化增强
- AI状态检测增强
- 审批记录和历史
- 日志检索功能
- 配置管理界面

### v1.2.0 - AI集成
- OpenAI API集成
- 自动审批引擎
- 任务状态自动检测
- 智能建议

### v2.0.0 - 多代理支持
- Codex集成
- Gemini CLI集成
- 代理配置管理
- 多代理并行监控

---

## 6. 开发规范

### 6.1 代码规范
- Go: 遵循官方style guide
- Vue: 使用Composition API
- TypeScript: 严格模式
- 提交信息: Conventional Commits

### 6.2 Git工作流
```
main        - 稳定发布分支
develop     - 开发主分支
feature/*   - 功能分支
fix/*       - 修复分支
release/*   - 发布分支
```

### 6.3 目录命名
- 小写字母
- 使用下划线分隔
- 组件文件使用PascalCase

---

## 7. 风险评估

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|---------|
| PTY跨平台兼容 | 中 | 高 | 优先支持Linux，后续适配 |
| WebSocket稳定性 | 中 | 高 | 实现完善的重连机制 |
| AI检测准确性 | 高 | 中 | 支持多种检测策略，可配置 |
| 性能瓶颈 | 低 | 高 | 限制并发会话数，优化缓冲区 |

---

## 8. 立即开始

### 第一步：初始化项目

```bash
# 创建项目目录
mkdir -p ai-coding-assistant/{backend,frontend,docs}

# 初始化Go模块
cd ai-coding-assistant/backend
go mod init github.com/user/ai-coding-assistant

# 初始化Vue项目
cd ../frontend
npm create vite@latest . -- --template vue-ts

# 初始化Git
cd ..
git init
git add .
git commit -m "feat: 初始化项目结构"
```

开始构建吧！
