# AI Coding Assistant - 产品需求文档 (PRD)

## 1. 产品概述

### 1.1 产品名称
**AI Coding Assistant** (简称 ACA)

### 1.2 产品定位
一个可视化多任务并行的AI编程助手管理平台，用于统一管理和监控多个AI编程代理（如Claude Code、Codex、Gemini CLI等）的执行状态，提供自动化审批、日志记录和任务进度可视化功能。

### 1.3 目标用户
- 独立开发者
- 小型开发团队
- 需要同时运行多个AI编程任务的用户

### 1.4 核心价值
1. **统一管理**：一个页面管理所有AI编程终端会话
2. **自动化审批**：减少手动确认yes/no和权限提醒的频率
3. **任务可视化**：Kanban看板展示任务状态和进度
4. **日志追踪**：完整记录每个AI代理的输出和操作历史
5. **智能检测**：自动识别AI代理状态（工作中/等待输入/等待审批）

---

## 2. 核心功能

### 2.1 用户认证模块
| 功能 | 描述 | 优先级 |
|------|------|--------|
| 单用户登录 | 简单的用户名密码认证 | P0 |
| Session管理 | JWT Token认证 | P0 |
| 权限验证 | 所有API需验证身份 | P0 |

### 2.2 终端托管模块
| 功能 | 描述 | 优先级 |
|------|------|--------|
| PTY会话管理 | 创建、管理伪终端会话 | P0 |
| 终端连接保持 | 会话持久化，断线重连 | P0 |
| Scrollback缓冲 | 保留终端历史输出（256KB） | P0 |
| Shell命令执行 | 支持执行服务器shell命令 | P0 |
| 多终端并行 | 支持同时运行多个终端 | P0 |
| 终端元数据 | 实时监测进程状态、前台命令 | P1 |

### 2.3 AI代理集成模块
| 功能 | 描述 | 优先级 |
|------|------|--------|
| Claude Code支持 | 检测和管理Claude Code会话 | P0 |
| Codex支持 | 检测和管理Codex会话 | P1 |
| Gemini CLI支持 | 检测和管理Gemini会话 | P1 |
| 代理状态检测 | 自动识别：工作中/等待输入/等待审批 | P0 |
| 日志监控 | 监控AI代理的日志文件 | P1 |

### 2.4 自动化审批模块
| 功能 | 描述 | 优先级 |
|------|------|--------|
| 输入检测 | 检测yes/no等确认提示 | P0 |
| 权限提醒检测 | 识别权限请求 | P0 |
| 自动响应接口 | 预留OpenAI API自动决策接口 | P1 |
| 手动审批 | 支持手动确认或拒绝 | P0 |
| 审批记录 | 记录所有审批操作 | P1 |

### 2.5 Kanban任务管理模块
| 功能 | 描述 | 优先级 |
|------|------|--------|
| 任务CRUD | 创建、读取、更新、删除任务 | P0 |
| 状态流转 | todo → in_progress → done | P0 |
| 拖拽排序 | 支持拖拽改变任务状态和顺序 | P0 |
| 任务-终端关联 | 将任务与终端会话关联 | P0 |
| 自动状态检测 | 检测任务开始和结束 | P1 |
| 任务评论 | 支持添加评论和备注 | P2 |

### 2.6 日志记录模块
| 功能 | 描述 | 优先级 |
|------|------|--------|
| 终端输出日志 | 记录所有终端输出 | P0 |
| 操作日志 | 记录用户操作 | P1 |
| 日志检索 | 支持搜索和过滤日志 | P2 |
| 日志导出 | 支持导出日志文件 | P2 |

---

## 3. 技术架构

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (Vue 3)                         │
│  Vue 3 + TypeScript + Pinia + Naive UI + xterm.js          │
└───────────────────┬─────────────────────────────────────────┘
                    │ REST API + WebSocket
                    ▼
┌─────────────────────────────────────────────────────────────┐
│                    Backend (Go)                             │
│  Fiber/Gin + GORM + SQLite + PTY + WebSocket               │
└───────────────────┬─────────────────────────────────────────┘
                    │
    ┌───────────────┼───────────────┬───────────────┐
    ▼               ▼               ▼               ▼
┌────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐
│ SQLite │   │ Terminal │   │ AI Agent │   │ Auto Approve │
│   DB   │   │ Manager  │   │ Detector │   │   Engine     │
└────────┘   └──────────┘   └──────────┘   └──────────────┘
```

### 3.2 技术栈选型

| 层级 | 技术 | 理由 |
|------|------|------|
| 前端框架 | Vue 3 + TypeScript | 响应式、类型安全 |
| UI组件库 | Naive UI | 企业级、功能完整 |
| 终端组件 | xterm.js | VSCode同款、功能强大 |
| 状态管理 | Pinia | Vue 3官方推荐 |
| 后端框架 | Go + Fiber | 高性能、单文件部署 |
| ORM | GORM | Go生态成熟ORM |
| 数据库 | SQLite | 轻量、无需额外部署 |
| PTY管理 | creack/pty | Go PTY库 |
| 实时通信 | WebSocket | 双向实时通信 |

### 3.3 目录结构

```
ai-coding-assistant/
├── backend/                    # Go后端
│   ├── main.go                # 入口文件
│   ├── api/                   # API路由
│   │   ├── auth.go           # 认证API
│   │   ├── terminal.go       # 终端API
│   │   ├── task.go           # 任务API
│   │   └── websocket.go      # WebSocket处理
│   ├── service/              # 业务逻辑
│   │   ├── terminal/         # 终端管理
│   │   │   ├── manager.go    # 终端管理器
│   │   │   └── session.go    # 会话管理
│   │   └── ai_detector/      # AI代理检测
│   │       ├── detector.go   # 检测器
│   │       └── tracker.go    # 状态追踪
│   ├── model/                # 数据模型
│   │   ├── user.go
│   │   ├── task.go
│   │   └── terminal.go
│   ├── utils/                # 工具函数
│   └── config/               # 配置管理
├── frontend/                  # Vue前端
│   ├── src/
│   │   ├── main.ts
│   │   ├── App.vue
│   │   ├── api/              # API调用
│   │   ├── stores/           # Pinia状态
│   │   ├── components/       # 组件
│   │   │   ├── Terminal.vue
│   │   │   ├── Kanban.vue
│   │   │   └── TaskCard.vue
│   │   ├── views/            # 页面
│   │   └── utils/            # 工具函数
│   ├── package.json
│   └── vite.config.ts
├── docs/                      # 文档
│   ├── PRD.md
│   └── DEVELOPMENT_PLAN.md
└── README.md
```

---

## 4. 数据模型设计

### 4.1 用户表 (users)
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 4.2 任务表 (tasks)
```sql
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT DEFAULT 'todo',  -- todo, in_progress, done, archived
    priority INTEGER DEFAULT 0,   -- 0-3
    order_index REAL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
```

### 4.3 终端会话表 (terminal_sessions)
```sql
CREATE TABLE terminal_sessions (
    id TEXT PRIMARY KEY,
    title TEXT,
    task_id TEXT,
    shell TEXT DEFAULT 'bash',
    status TEXT DEFAULT 'running',  -- running, exited
    pid INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    closed_at DATETIME,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);
```

### 4.4 AI会话表 (ai_sessions)
```sql
CREATE TABLE ai_sessions (
    id TEXT PRIMARY KEY,
    terminal_id TEXT NOT NULL,
    task_id TEXT,
    ai_type TEXT NOT NULL,  -- claude-code, codex, gemini
    state TEXT DEFAULT 'unknown',  -- unknown, waiting_input, working, waiting_approval
    session_file TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (terminal_id) REFERENCES terminal_sessions(id),
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);
```

### 4.5 审批记录表 (approval_records)
```sql
CREATE TABLE approval_records (
    id TEXT PRIMARY KEY,
    terminal_id TEXT NOT NULL,
    ai_session_id TEXT,
    prompt_type TEXT,  -- yes_no, permission, other
    prompt_content TEXT,
    response TEXT,
    auto_approved BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (terminal_id) REFERENCES terminal_sessions(id)
);
```

### 4.6 日志表 (logs)
```sql
CREATE TABLE logs (
    id TEXT PRIMARY KEY,
    terminal_id TEXT,
    task_id TEXT,
    log_type TEXT,  -- terminal_output, operation, system
    content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 5. API设计

### 5.1 认证API

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/auth/login | 用户登录 |
| POST | /api/auth/logout | 用户登出 |
| GET | /api/auth/me | 获取当前用户 |

### 5.2 任务API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | /api/tasks | 获取任务列表 |
| POST | /api/tasks | 创建任务 |
| GET | /api/tasks/:id | 获取任务详情 |
| PUT | /api/tasks/:id | 更新任务 |
| DELETE | /api/tasks/:id | 删除任务 |
| POST | /api/tasks/:id/move | 移动任务（拖拽） |

### 5.3 终端API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | /api/terminals | 获取终端列表 |
| POST | /api/terminals | 创建终端会话 |
| GET | /api/terminals/:id | 获取终端详情 |
| POST | /api/terminals/:id/close | 关闭终端 |
| POST | /api/terminals/:id/rename | 重命名终端 |
| POST | /api/terminals/:id/link-task | 关联任务 |
| WebSocket | /api/terminal/ws | 终端WebSocket连接 |

### 5.4 自动化审批API（预留接口）

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/automation/analyze | 分析最新日志并获取AI建议 |
| POST | /api/automation/execute | 执行自动化命令 |
| GET | /api/automation/config | 获取自动化配置 |
| PUT | /api/automation/config | 更新自动化配置 |

**请求/响应格式示例**：

```json
// POST /api/automation/analyze
// Request
{
    "terminal_id": "term-123",
    "recent_logs": "最近的终端输出...",
    "context": {
        "task_id": "task-456",
        "ai_type": "claude-code",
        "current_state": "waiting_approval"
    }
}

// Response (预留OpenAI集成)
{
    "action": "approve",  // approve, reject, input, wait
    "input_command": "yes",
    "confidence": 0.95,
    "reasoning": "检测到权限确认请求，建议批准"
}
```

---

## 6. WebSocket消息协议

### 6.1 客户端发送

```typescript
// 输入数据
{ type: "input", data: "base64编码的输入" }

// 调整终端大小
{ type: "resize", cols: 120, rows: 40 }

// 关闭终端
{ type: "close" }
```

### 6.2 服务端发送

```typescript
// 连接就绪
{ type: "ready", sessionId: "xxx", status: {...} }

// 终端输出
{ type: "data", data: "base64编码的输出" }

// 元数据更新
{
    type: "metadata",
    metadata: {
        title: "终端标题",
        pid: 12345,
        status: "running",
        runningCommand: "claude",
        aiAssistant: {
            type: "claude-code",
            state: "working",  // waiting_input, working, waiting_approval
            detected: true
        }
    }
}

// 会话退出
{ type: "exit", code: 0, message: "进程已退出" }

// 错误
{ type: "error", message: "错误信息" }
```

---

## 7. 自动化功能接口设计

### 7.1 AI代理状态检测

```go
// 状态机
type AIAgentState string
const (
    StateUnknown         AIAgentState = "unknown"
    StateWaitingInput    AIAgentState = "waiting_input"
    StateWorking         AIAgentState = "working"
    StateWaitingApproval AIAgentState = "waiting_approval"
)

// 检测接口
type AIAgentDetector interface {
    DetectFromCommand(cmd string) *AIAgentInfo
    DetectFromOutput(output []byte) *AIAgentInfo
    GetCurrentState() AIAgentState
}
```

### 7.2 自动审批引擎接口

```go
// 审批决策接口
type ApprovalEngine interface {
    // 分析提示并返回建议动作
    Analyze(ctx context.Context, prompt ApprovalPrompt) (*ApprovalDecision, error)

    // 执行动作
    Execute(ctx context.Context, sessionID string, decision *ApprovalDecision) error

    // 配置管理
    GetConfig() *ApprovalConfig
    UpdateConfig(config *ApprovalConfig) error
}

// 审批提示
type ApprovalPrompt struct {
    TerminalID   string
    AIType       string
    PromptType   string  // yes_no, permission, input_required
    Content      string
    Context      map[string]interface{}
}

// 审批决策
type ApprovalDecision struct {
    Action      string  // approve, reject, input, wait
    Input       string  // 输入内容（如果需要）
    Confidence  float64
    Reasoning   string
}
```

### 7.3 任务状态自动检测接口

```go
// 任务状态检测器
type TaskStateDetector interface {
    // 检测任务是否开始
    DetectTaskStart(terminalID string, output []byte) bool

    // 检测任务是否完成
    DetectTaskEnd(terminalID string, output []byte) bool

    // 自动更新任务状态
    AutoUpdateTaskStatus(taskID string, newStatus TaskStatus) error
}
```

### 7.4 外部AI集成接口（预留）

```go
// 外部AI服务接口
type ExternalAIService interface {
    // 发送日志获取分析结果
    AnalyzeLogs(ctx context.Context, logs string, systemPrompt string) (*AIResponse, error)
}

// AI响应
type AIResponse struct {
    Action     string          `json:"action"`
    Command    string          `json:"command,omitempty"`
    Reasoning  string          `json:"reasoning"`
    Confidence float64         `json:"confidence"`
    Metadata   json.RawMessage `json:"metadata,omitempty"`
}
```

---

## 8. 界面设计

### 8.1 主界面布局

```
┌──────────────────────────────────────────────────────────────────┐
│  Logo   [任务看板]  [终端管理]  [日志]  [设置]      [用户名] [退出] │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │    待办     │  │   进行中    │  │    完成     │             │
│  │  (TODO)     │  │(IN PROGRESS)│  │   (DONE)    │             │
│  ├─────────────┤  ├─────────────┤  ├─────────────┤             │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │             │
│  │ │ Task 1  │ │  │ │ Task 3  │ │  │ │ Task 5  │ │             │
│  │ │         │ │  │ │ ●运行中 │ │  │ │         │ │             │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │             │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │             │             │
│  │ │ Task 2  │ │  │ │ Task 4  │ │  │             │             │
│  │ │         │ │  │ │ ○等待   │ │  │             │             │
│  │ └─────────┘ │  │ └─────────┘ │  │             │             │
│  │             │  │             │  │             │             │
│  │   [+ 添加]  │  │             │  │             │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│  终端标签: [Terminal 1 ●] [Terminal 2 ○] [Terminal 3] [+]        │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ $ claude                                                    │ │
│  │ Claude Code v2.0.75                                         │ │
│  │ > Working on task...                                        │ │
│  │ > [需要确认] 是否允许写入文件 src/main.ts? (yes/no)         │ │
│  │                                                              │ │
│  │ [自动审批: 开启]  [手动输入: ____________] [发送]           │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 8.2 任务卡片设计

```
┌─────────────────────────────┐
│ ○ 实现用户登录功能          │  ← 任务标题
├─────────────────────────────┤
│ 优先级: ★★★               │  ← 优先级
│ 代理: Claude Code           │  ← 关联的AI代理
│ 状态: ● 工作中              │  ← AI状态指示
├─────────────────────────────┤
│ [打开终端] [编辑] [删除]    │  ← 操作按钮
└─────────────────────────────┘
```

### 8.3 终端面板设计

```
┌─ Terminal 1 - 实现用户登录 ──────────────────────── [_][□][×] ─┐
│ AI状态: ● Claude Code - 等待审批                              │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ $ claude "实现用户登录功能"                                   │
│ Claude Code v2.0.75                                           │
│ > Analyzing codebase...                                       │
│ > Creating src/auth/login.ts                                  │
│                                                               │
│ ┌─────────────────────────────────────────────────────────┐  │
│ │ [!] 需要权限确认                                        │  │
│ │ 是否允许创建文件 src/auth/login.ts?                     │  │
│ │                                                          │  │
│ │ [允许 (yes)]  [拒绝 (no)]  [始终允许]                   │  │
│ └─────────────────────────────────────────────────────────┘  │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│ > ______________________________________________ [发送]       │
└───────────────────────────────────────────────────────────────┘
```

---

## 9. 非功能性需求

### 9.1 性能要求
- 终端响应延迟 < 50ms
- 支持同时运行 10+ 个终端会话
- WebSocket连接稳定，支持断线重连

### 9.2 安全要求
- 所有API需要认证
- 密码需要加密存储
- WebSocket连接需要Token验证

### 9.3 可用性要求
- 支持主流浏览器（Chrome、Firefox、Safari、Edge）
- 响应式设计，支持不同屏幕尺寸
- 终端会话持久化，页面刷新不丢失

### 9.4 部署要求
- 单文件部署（前端静态文件嵌入后端二进制）
- 支持Docker部署
- 支持配置文件和环境变量

---

## 10. 版本规划

### v1.0 MVP（最小可行产品）
- [x] 单用户登录认证
- [x] 基础终端托管（创建、管理、输入输出）
- [x] Kanban任务看板（基础CRUD和拖拽）
- [x] 任务-终端关联
- [x] AI代理基础检测（Claude Code）

### v1.1 自动化增强
- [ ] AI代理状态自动检测
- [ ] 权限/确认提示检测
- [ ] 手动审批界面
- [ ] 审批记录

### v1.2 智能自动化
- [ ] OpenAI API集成接口
- [ ] 自动化审批引擎
- [ ] 任务状态自动检测

### v2.0 多代理支持
- [ ] Codex集成
- [ ] Gemini CLI集成
- [ ] 其他AI代理扩展

---

## 附录

### A. 参考资料
- [CodeKanban](https://github.com/fy0/CodeKanban) - 终端托管和AI检测参考
- [Claude-Code-Workflow](https://github.com/catlog22/Claude-Code-Workflow) - 多代理工作流参考
- [vibe-kanban](https://github.com/BloopAI/vibe-kanban) - Kanban UI参考

### B. 术语表
| 术语 | 定义 |
|------|------|
| PTY | Pseudo Terminal，伪终端 |
| AI Agent | AI编程代理，如Claude Code |
| Scrollback | 终端历史输出缓冲区 |
| Kanban | 看板，任务可视化管理方法 |
| WebSocket | 全双工实时通信协议 |
