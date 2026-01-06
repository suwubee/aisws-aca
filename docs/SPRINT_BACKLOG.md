# AI Coding Assistant - Sprint Backlog

## 当前版本: v1.3.x (任务自动化已完成)

---

## Sprint 1: Bug修复 & 基础完善 (优先级: P0)

### 目标
修复已知Bug，完善核心功能缺失部分

### 任务列表

| ID | 任务 | 类型 | 模块 | 可并行 | 依赖 |
|----|------|------|------|--------|------|
| S1-1 | 修复 ResetData 引用不存在的表 | Bug | 后端/Auth | ✅ | - |
| S1-2 | 实现任务编辑模态框 | Feature | 前端/Kanban | ✅ | - |
| S1-3 | 实现任务卡片AI状态显示 | Feature | 前端/TaskCard | ✅ | - |
| S1-4 | 修复密码修改持久化问题 | Bug | 后端/Auth | ✅ | - |

### 并行开发建议
- **开发者A**: S1-1, S1-4 (后端Bug修复)
- **开发者B**: S1-2, S1-3 (前端功能完善)

---

## Sprint 2: 审批UI完善 (优先级: P0)

### 目标
完成审批交互界面，实现用户手动审批流程

### 任务列表

| ID | 任务 | 类型 | 模块 | 可并行 | 依赖 |
|----|------|------|------|--------|------|
| S2-1 | 审批提示弹窗组件 | Feature | 前端/Terminal | ✅ | - |
| S2-2 | 快捷操作按钮(允许/拒绝) | Feature | 前端/Terminal | ❌ | S2-1 |
| S2-3 | 手动输入框组件 | Feature | 前端/Terminal | ❌ | S2-1 |
| S2-4 | WebSocket审批事件监听 | Feature | 前端/Store | ✅ | - |
| S2-5 | 审批消息中心组件 | Feature | 前端/Layout | ❌ | S2-4 |

### 并行开发建议
- **开发者A**: S2-1 → S2-2 → S2-3 (审批弹窗链)
- **开发者B**: S2-4 → S2-5 (消息系统链)

---

## Sprint 3: AI集成增强 (优先级: P1)

### 目标
将AI审批决策集成到前端，提供智能建议

### 任务列表

| ID | 任务 | 类型 | 模块 | 可并行 | 依赖 |
|----|------|------|------|--------|------|
| S3-1 | AI决策结果展示组件 | Feature | 前端/Terminal | ✅ | - |
| S3-2 | 审批置信度可视化 | Feature | 前端/Terminal | ❌ | S3-1 |
| S3-3 | AI决策日志面板 | Feature | 前端/Settings | ✅ | - |
| S3-4 | 智能建议展示 | Feature | 前端/Terminal | ❌ | S3-1 |

### 并行开发建议
- **开发者A**: S3-1 → S3-2 → S3-4
- **开发者B**: S3-3

---

## Sprint 4: 多代理支持 (优先级: P1)

### 目标
完成Codex/Gemini集成测试，构建多代理监控面板

### 任务列表

| ID | 任务 | 类型 | 模块 | 可并行 | 依赖 |
|----|------|------|------|--------|------|
| S4-1 | Codex完整集成测试 | Test | 后端/Detector | ✅ | - |
| S4-2 | Gemini CLI完整集成测试 | Test | 后端/Detector | ✅ | - |
| S4-3 | 多代理状态监控面板 | Feature | 前端/Dashboard | ❌ | S4-1, S4-2 |
| S4-4 | 代理配置管理界面 | Feature | 前端/Settings | ✅ | - |
| S4-5 | 代理性能统计 | Feature | 后端/Service | ❌ | S4-3 |

### 并行开发建议
- **开发者A**: S4-1, S4-2 (测试)
- **开发者B**: S4-4 (配置界面)
- **开发者C**: S4-3 → S4-5 (监控面板，等待测试完成)

---

## Sprint 5: 高级功能 (优先级: P2)

### 目标
实现任务评论、日志导出等增强功能

### 任务列表

| ID | 任务 | 类型 | 模块 | 可并行 | 依赖 |
|----|------|------|------|--------|------|
| S5-1 | 任务评论后端API | Feature | 后端/Task | ✅ | - |
| S5-2 | 任务评论前端组件 | Feature | 前端/TaskDetail | ❌ | S5-1 |
| S5-3 | 日志导出后端API | Feature | 后端/Log | ✅ | - |
| S5-4 | 日志导出前端功能 | Feature | 前端/LogManagement | ❌ | S5-3 |
| S5-5 | 规则导入导出 | Feature | 全栈 | ✅ | - |

### 并行开发建议
- **开发者A**: S5-1 → S5-2 (评论功能)
- **开发者B**: S5-3 → S5-4 (日志导出)
- **开发者C**: S5-5 (规则导入导出)

---

## Sprint 6: 部署优化 (优先级: P2)

### 目标
优化Docker部署，准备多用户支持

### 任务列表

| ID | 任务 | 类型 | 模块 | 可并行 | 依赖 |
|----|------|------|------|--------|------|
| S6-1 | Docker镜像优化 | DevOps | 部署 | ✅ | - |
| S6-2 | docker-compose配置 | DevOps | 部署 | ✅ | - |
| S6-3 | 多用户数据模型设计 | Design | 后端/Model | ✅ | - |
| S6-4 | 多用户认证改造 | Feature | 后端/Auth | ❌ | S6-3 |
| S6-5 | 用户管理界面 | Feature | 前端/Settings | ❌ | S6-4 |

### 并行开发建议
- **开发者A**: S6-1, S6-2 (DevOps)
- **开发者B**: S6-3 → S6-4 → S6-5 (多用户)

---

## Bug修复详情

### BUG-1: ResetData 引用不存在的表

**文件**: `backend/api/auth.go:164`

**问题**:
```go
model.DB.Exec("DELETE FROM terminal_automations")
```
表 `terminal_automations` 不存在于数据库模型中。

**修复方案**: 删除该行或替换为正确的表名。

---

### BUG-2: 任务卡片AI状态未实现

**文件**: `frontend/src/components/TaskCard.vue:115-116`

**问题**:
```typescript
const aiStatus = computed(() => {
  // TODO: 从关联的终端获取AI状态
  return null
})
```

**修复方案**:
从 `terminalStore` 获取关联终端的AI状态并显示。

---

### BUG-3: 任务编辑功能未实现

**文件**: `frontend/src/components/Kanban.vue:96-99`

**问题**:
```typescript
function handleEditTask(task: Task) {
  // TODO: 打开编辑模态框
  console.log('Edit task:', task)
}
```

**修复方案**:
创建任务编辑模态框组件，复用创建任务的表单。

---

### BUG-4: 密码修改未持久化

**文件**: `backend/api/auth.go:136-137`

**问题**:
```go
// 更新配置文件中的密码（这里简化处理，实际应该更新配置文件）
ctrl.config.Password = req.NewPassword
```
仅更新内存中的配置，重启后失效。

**修复方案**:
实现配置文件写入功能，或改用数据库存储密码。

---

## 版本规划

| 版本 | Sprint | 预计功能 |
|------|--------|----------|
| v1.4.0 | Sprint 1 | Bug修复 + 基础完善 |
| v1.5.0 | Sprint 2 | 审批UI完善 |
| v1.6.0 | Sprint 3 | AI集成增强 |
| v2.0.0 | Sprint 4 | 多代理支持 |
| v2.1.0 | Sprint 5 | 高级功能 |
| v2.2.0 | Sprint 6 | 部署优化 + 多用户 |

---

## 任务依赖图

```
Sprint 1 (并行)
├── S1-1 (Bug: ResetData)
├── S1-2 (任务编辑)
├── S1-3 (AI状态显示)
└── S1-4 (Bug: 密码持久化)

Sprint 2
├── S2-1 (审批弹窗) ──┬── S2-2 (快捷按钮)
│                     └── S2-3 (手动输入)
└── S2-4 (WS监听) ────── S2-5 (消息中心)

Sprint 3
├── S3-1 (AI决策展示) ──┬── S3-2 (置信度)
│                       └── S3-4 (智能建议)
└── S3-3 (AI日志面板)

Sprint 4
├── S4-1 (Codex测试) ──┐
├── S4-2 (Gemini测试) ─┼── S4-3 (监控面板) ── S4-5 (性能统计)
└── S4-4 (配置界面)    │
                       └──────────────────────┘

Sprint 5 (并行)
├── S5-1 → S5-2 (评论功能)
├── S5-3 → S5-4 (日志导出)
└── S5-5 (规则导入导出)

Sprint 6
├── S6-1, S6-2 (Docker优化)
└── S6-3 → S6-4 → S6-5 (多用户)
```

---

## 快速开始

### 立即可执行的任务 (无依赖)

1. **S1-1**: 修复 `auth.go` 中的 `terminal_automations` 表引用
2. **S1-2**: 实现任务编辑模态框
3. **S1-3**: 实现任务卡片AI状态显示
4. **S1-4**: 修复密码修改持久化
5. **S2-1**: 开始审批弹窗组件开发
6. **S2-4**: 开始WebSocket审批事件监听

### 建议优先级

```
高优先级 (P0): Sprint 1 全部 + Sprint 2 全部
中优先级 (P1): Sprint 3 + Sprint 4
低优先级 (P2): Sprint 5 + Sprint 6
```

---

# 第二部分：功能扩展路线图

## 共享基础设施 (Foundation Layer)

> **重要**: 以下基础设施必须先完成，才能并行开发扩展功能模块

### 架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                      共享基础设施层 (Sprint 7)                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐         │
│  │  统一认证系统   │  │  统一终端服务   │  │  统一事件总线   │         │
│  │  (Auth v2)     │  │  (Terminal v2) │  │  (EventBus)    │         │
│  │                │  │                │  │                │         │
│  │ • 多用户       │  │ • 本地PTY      │  │ • WebSocket    │         │
│  │ • RBAC权限     │  │ • SSH远程      │  │ • Redis PubSub │         │
│  │ • API Key      │  │ • 统一接口     │  │ • 事件订阅     │         │
│  └───────┬────────┘  └───────┬────────┘  └───────┬────────┘         │
│          │                   │                   │                   │
│          └───────────────────┼───────────────────┘                   │
│                              ▼                                       │
│                    ┌────────────────┐                                │
│                    │  统一API网关    │                                │
│                    │  (Gateway)     │                                │
│                    │                │                                │
│                    │ • 路由分发     │                                │
│                    │ • 限流熔断     │                                │
│                    │ • 日志追踪     │                                │
│                    └────────────────┘                                │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         ▼                       ▼                       ▼
┌────────────────┐     ┌────────────────┐     ┌────────────────┐
│  SSH管理模块    │     │  工作流引擎     │     │  Agent编排     │
│  (Sprint 8)    │     │  (Sprint 9)    │     │  (Sprint 10)   │
│  [可并行开发]   │     │  [可并行开发]   │     │  [可并行开发]   │
└────────────────┘     └────────────────┘     └────────────────┘
```

---

## Sprint 7: 共享基础设施 (优先级: P0 - 阻塞后续开发)

### 目标
构建统一的基础设施层，为后续扩展功能提供公共服务

> 参考: codex-sprint.md Sprint 0 平台加固

### 7.0 平台加固 (Sprint 0 前置条件)

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S7-0-1 | Auth持久化 | bcrypt + GORM | 密码存DB，不再依赖内存配置 |
| S7-0-2 | WebSocket鉴权 | JWT | WS连接必须验证token |
| S7-0-3 | API契约定义 | OpenAPI/TypeScript | 固化REST/WS schema |
| S7-0-4 | 开发环境锁定 | package.json | 固定Node/TS版本 |

**实施路径**:
- **S7-0-1**: 登录从DB读取password_hash校验，改密仅更新DB
- **S7-0-2**: `/api/terminal/ws` 纳入token校验，避免绕过认证
- **S7-0-3**: 前端 `src/api/types.ts` 集中定义请求/响应类型

### 7.1a 统一凭据管理 (Secrets)

> 来源: codex-sprint.md - 统一 Secrets 模块

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S7-1a-1 | Secrets数据模型 | GORM | 统一存储SSH密钥/密码/API Key |
| S7-1a-2 | AES-GCM加密 | crypto/aes | 主密钥环境变量，加密存储 |
| S7-1a-3 | Secrets API | REST | CRUD (不返回明文) |
| S7-1a-4 | 密钥轮换 | Go | 支持主密钥更新 |

**数据模型**:
```go
// 统一凭据
type Secret struct {
    ID         string    `gorm:"primaryKey" json:"id"`
    Name       string    `json:"name"`
    Type       string    `json:"type"` // ssh_password, ssh_key, api_key
    Ciphertext string    `json:"-"`    // AES-GCM加密后base64
    Meta       string    `json:"meta"` // JSON: 指纹/用途等
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

**API设计**:
```
GET    /api/secrets          # 列表(仅meta，不返回明文)
POST   /api/secrets          # 创建
PUT    /api/secrets/:id      # 更新
DELETE /api/secrets/:id      # 删除
```

### 7.1 统一认证系统 v2 (Auth v2)

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S7-1-1 | 多用户数据模型 | GORM | 扩展User表，添加角色、权限字段 |
| S7-1-2 | RBAC权限系统 | Casbin | 集成Casbin实现角色权限控制 |
| S7-1-3 | API Key认证 | JWT+UUID | 支持长期有效的API Key用于自动化 |
| S7-1-4 | OAuth2集成 | golang.org/x/oauth2 | 支持GitHub/Google登录(可选) |

**数据模型设计**:
```go
// 用户角色
type Role struct {
    ID          string `gorm:"primaryKey"`
    Name        string `gorm:"uniqueIndex"` // admin, user, viewer
    Permissions string // JSON数组
}

// API Key
type APIKey struct {
    ID        string `gorm:"primaryKey"`
    UserID    string `gorm:"index"`
    Name      string
    KeyHash   string // bcrypt哈希
    Scopes    string // JSON数组: ["terminal:read", "task:write"]
    ExpiresAt *time.Time
    LastUsed  *time.Time
}
```

**技术选型理由**:
- **Casbin**: Go生态最成熟的RBAC库，支持多种权限模型
- **API Key**: 便于CI/CD和自动化脚本调用

---

### 7.2 统一终端服务 v2 (Terminal v2)

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S7-2-1 | 终端接口抽象 | Go Interface | 定义统一的Terminal接口 |
| S7-2-2 | 本地PTY适配器 | creack/pty | 封装现有PTY实现 |
| S7-2-3 | SSH远程适配器 | golang.org/x/crypto/ssh | 实现SSH终端适配器 |
| S7-2-4 | 终端工厂模式 | Factory Pattern | 根据类型创建不同终端 |

**接口设计**:
```go
// 统一终端接口
type Terminal interface {
    ID() string
    Type() TerminalType  // local, ssh, docker

    // 生命周期
    Start() error
    Close() error

    // IO操作
    Read() ([]byte, error)
    Write(data []byte) error
    Resize(cols, rows uint16) error

    // 状态
    Status() TerminalStatus
    Metadata() map[string]interface{}
}

// 终端类型
type TerminalType string
const (
    TerminalTypeLocal  TerminalType = "local"
    TerminalTypeSSH    TerminalType = "ssh"
    TerminalTypeDocker TerminalType = "docker"
)
```

---

### 7.3 统一事件总线 (EventBus)

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S7-3-1 | 事件定义 | Go Struct | 定义标准事件结构 |
| S7-3-2 | 内存事件总线 | Go Channel | 单机模式事件分发 |
| S7-3-3 | Redis PubSub | go-redis | 分布式事件分发(可选) |
| S7-3-4 | WebSocket广播 | gorilla/websocket | 前端实时推送 |

**事件结构**:
```go
type Event struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Source    string                 `json:"source"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}

// 事件类型常量
const (
    EventTerminalOutput   = "terminal.output"
    EventTerminalStatus   = "terminal.status"
    EventApprovalRequired = "approval.required"
    EventTaskUpdated      = "task.updated"
    EventWorkflowStep     = "workflow.step"
)
```

### 7.4 Sprint 7 任务汇总

| ID | 任务 | 可并行 | 依赖 |
|----|------|--------|------|
| S7-1-1 | 多用户数据模型 | ✅ | - |
| S7-1-2 | RBAC权限系统 | ❌ | S7-1-1 |
| S7-2-1 | 终端接口抽象 | ✅ | - |
| S7-2-2 | 本地PTY适配器 | ❌ | S7-2-1 |
| S7-3-1 | 事件定义 | ✅ | - |
| S7-3-2 | 内存事件总线 | ❌ | S7-3-1 |

**并行开发建议**:
- **开发者A**: S7-1-1 → S7-1-2 (认证系统)
- **开发者B**: S7-2-1 → S7-2-2 (终端服务)
- **开发者C**: S7-3-1 → S7-3-2 (事件总线)

---

## Sprint 8: SSH多服务器管理 (依赖: Sprint 7)

### 目标
实现SSH远程服务器连接和管理功能

### 8.1 后端实现

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S8-1-1 | SSH服务器模型 | GORM | 服务器配置存储 |
| S8-1-2 | SSH连接管理器 | golang.org/x/crypto/ssh | 连接池、会话管理 |
| S8-1-3 | 密钥管理服务 | AES-256加密 | 安全存储SSH密钥 |
| S8-1-4 | SSH终端适配器 | Terminal接口 | 实现统一终端接口 |
| S8-1-5 | 服务器分组API | REST | 服务器分组CRUD |

**数据模型**:
```go
// SSH服务器
type SSHServer struct {
    ID          string     `gorm:"primaryKey" json:"id"`
    Name        string     `json:"name"`
    Host        string     `json:"host"`
    Port        int        `gorm:"default:22" json:"port"`
    Username    string     `json:"username"`
    AuthType    string     `json:"auth_type"` // password, key
    Password    string     `json:"-"`         // 加密存储
    PrivateKey  string     `json:"-"`         // 加密存储
    Passphrase  string     `json:"-"`
    GroupID     *string    `json:"group_id"`
    Tags        string     `json:"tags"`      // JSON数组
    LastStatus  string     `json:"last_status"`
    CreatedAt   time.Time  `json:"created_at"`
}

// 服务器分组
type ServerGroup struct {
    ID          string `gorm:"primaryKey" json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    ParentID    *string `json:"parent_id"` // 支持嵌套分组
}
```

### 8.2 前端实现

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S8-2-1 | 服务器列表页面 | Vue3 + NaiveUI | 服务器管理界面 |
| S8-2-2 | 服务器配置表单 | NaiveUI Form | 添加/编辑服务器 |
| S8-2-3 | SSH终端组件 | xterm.js | 复用现有终端组件 |
| S8-2-4 | 批量执行面板 | Vue3 | 多服务器命令执行 |

### 8.3 API设计

```
POST   /api/servers              # 添加服务器
GET    /api/servers              # 服务器列表
GET    /api/servers/:id          # 服务器详情
PUT    /api/servers/:id          # 更新服务器
DELETE /api/servers/:id          # 删除服务器
POST   /api/servers/:id/test     # 测试连接
POST   /api/servers/:id/terminal # 创建SSH终端
GET    /api/server-groups        # 分组列表
```

---

## Sprint 9: 工作流引擎 (依赖: Sprint 7)

### 目标
实现可视化工作流编排，支持任务串联和并行执行

### 9.1 后端实现

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S9-1-1 | 工作流数据模型 | GORM | 工作流定义存储 |
| S9-1-2 | YAML/JSON解析器 | gopkg.in/yaml.v3 | 解析工作流定义 |
| S9-1-3 | 执行引擎核心 | Go Goroutine | 任务调度执行 |
| S9-1-4 | 条件分支处理 | expr-lang/expr | 表达式求值 |
| S9-1-5 | 定时调度器 | robfig/cron | Cron表达式支持 |

**数据模型**:
```go
// 工作流定义
type Workflow struct {
    ID          string    `gorm:"primaryKey" json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Definition  string    `json:"definition"` // YAML/JSON
    Status      string    `json:"status"`     // draft, active, paused
    TriggerType string    `json:"trigger_type"` // manual, scheduled, webhook
    CronExpr    string    `json:"cron_expr"`
    CreatedAt   time.Time `json:"created_at"`
}

// 工作流执行记录
type WorkflowRun struct {
    ID         string     `gorm:"primaryKey" json:"id"`
    WorkflowID string     `gorm:"index" json:"workflow_id"`
    Status     string     `json:"status"` // pending, running, success, failed
    StartedAt  time.Time  `json:"started_at"`
    FinishedAt *time.Time `json:"finished_at"`
    Context    string     `json:"context"` // 执行上下文JSON
}
```

**工作流定义示例**:
```yaml
name: "部署流水线"
trigger: manual
steps:
  - id: clone
    type: terminal
    command: "git clone $REPO_URL"

  - id: build
    type: terminal
    depends_on: [clone]
    command: "npm run build"

  - id: test
    type: parallel
    depends_on: [build]
    steps:
      - id: unit_test
        command: "npm test"
      - id: lint
        command: "npm run lint"

  - id: deploy
    type: condition
    depends_on: [test]
    condition: "$TEST_RESULT == 'success'"
    then:
      command: "npm run deploy"
```

### 9.2 前端实现

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S9-2-1 | 工作流列表页 | Vue3 | 工作流管理 |
| S9-2-2 | 可视化编辑器 | vue-flow | 拖拽式流程设计 |
| S9-2-3 | 节点配置面板 | NaiveUI | 节点属性编辑 |
| S9-2-4 | 执行监控面板 | Vue3 | 实时执行状态 |

---

## Sprint 10: AI Agent编排 (依赖: Sprint 7)

### 目标
实现多AI Agent协同工作，任务智能分发

### 10.1 后端实现

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S10-1-1 | Agent注册表 | GORM | Agent配置管理 |
| S10-1-2 | 任务路由器 | Go | 根据任务类型分发 |
| S10-1-3 | 上下文共享服务 | Redis/内存 | 跨Agent数据共享 |
| S10-1-4 | 结果聚合器 | Go | 多Agent结果合并 |

**Agent配置模型**:
```go
// Agent配置
type AgentConfig struct {
    ID          string `gorm:"primaryKey" json:"id"`
    Name        string `json:"name"`
    Type        string `json:"type"` // claude, codex, gemini
    Capabilities string `json:"capabilities"` // JSON: ["code", "review", "docs"]
    Priority    int    `json:"priority"`
    MaxConcurrent int  `json:"max_concurrent"`
    Enabled     bool   `json:"enabled"`
}

// 任务分发规则
type RoutingRule struct {
    ID        string `gorm:"primaryKey" json:"id"`
    Pattern   string `json:"pattern"`   // 任务匹配模式
    AgentID   string `json:"agent_id"`
    Priority  int    `json:"priority"`
}
```

---

## Sprint 11: DevOps监控 (依赖: Sprint 8)

### 目标
实现服务器监控和AI辅助运维

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S11-1 | 服务器指标采集 | SSH + 命令 | CPU/内存/磁盘 |
| S11-2 | 告警规则引擎 | expr-lang | 阈值告警 |
| S11-3 | AI日志分析 | OpenAI API | 异常检测 |
| S11-4 | 监控仪表盘 | Vue3 + ECharts | 可视化展示 |

---

## Sprint 12: Runbook运维手册 (依赖: Sprint 9)

> 来源: codex-sprint.md Sprint 3

### 目标
实现参数化运维脚本模板和定时调度

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S12-1 | Runbook模型 | GORM | 模板参数、目标选择 |
| S12-2 | 参数化引擎 | text/template | 变量注入到steps |
| S12-3 | 定时调度器 | robfig/cron | cron表达式支持 |
| S12-4 | 并发控制 | Go Semaphore | 按标签限制并发 |

**数据模型**:
```go
type Runbook struct {
    ID          string `gorm:"primaryKey" json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    WorkflowID  string `json:"workflow_id"` // 基于的工作流
    Parameters  string `json:"parameters"`  // JSON: 参数定义
    TargetTags  string `json:"target_tags"` // 目标服务器标签
}

type Schedule struct {
    ID         string     `gorm:"primaryKey" json:"id"`
    RunbookID  string     `json:"runbook_id"`
    CronExpr   string     `json:"cron_expr"`
    Enabled    bool       `json:"enabled"`
    NextRunAt  *time.Time `json:"next_run_at"`
    LastRunAt  *time.Time `json:"last_run_at"`
}
```

---

## Sprint 13: Project/Workspace (依赖: Sprint 8)

> 来源: codex-sprint.md Sprint 4

### 目标
统一描述"在哪做事"，支持复杂项目开发

| ID | 任务 | 技术栈 | 实施细节 |
|----|------|--------|----------|
| S13-1 | Project模型 | GORM | 本地/远程/Git仓库 |
| S13-2 | CLI Profiles | GORM | CLI配置模板 |
| S13-3 | 项目模板库 | JSON/YAML | 内置常用模板 |
| S13-4 | 项目仪表盘 | Vue3 | 项目概览页面 |

**数据模型**:
```go
// 项目/工作区
type Project struct {
    ID          string  `gorm:"primaryKey" json:"id"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Type        string  `json:"type"` // local, remote, git
    LocalPath   string  `json:"local_path"`
    TargetID    *string `json:"target_id"`
    RemotePath  string  `json:"remote_path"`
    GitRepo     string  `json:"git_repo"`
    GitBranch   string  `json:"git_branch"`
    EnvVars     string  `json:"env_vars"` // JSON
}

// CLI配置模板
type CLIProfile struct {
    ID             string `gorm:"primaryKey" json:"id"`
    Name           string `json:"name"`
    Type           string `json:"type"` // claude, codex, gemini, custom
    Command        string `json:"command"`
    DefaultArgs    string `json:"default_args"`
    ResumeStrategy string `json:"resume_strategy"`
    DetectPatterns string `json:"detect_patterns"` // JSON
}
```

---

## 第三部分：未来功能扩展 (Sprint 14+)

### Sprint 14: 插件系统

| ID | 任务 | 描述 |
|----|------|------|
| S14-1 | 插件接口定义 | Go Plugin / WASM |
| S14-2 | 插件生命周期 | 加载/卸载/热更新 |
| S14-3 | 插件市场UI | 浏览/安装/管理 |

**扩展点**:
- 自定义AI检测器
- 自定义审批规则
- 自定义工作流节点
- 自定义监控指标

### Sprint 15: 团队协作

| ID | 任务 | 描述 |
|----|------|------|
| S15-1 | 多用户系统 | 用户注册/邀请/管理 |
| S15-2 | 团队/组织 | 团队创建/成员管理 |
| S15-3 | 资源共享 | 项目/工作流/模板共享 |
| S15-4 | 实时协作 | 多人同时查看终端 |

### Sprint 16: 外部集成

| ID | 任务 | 描述 |
|----|------|------|
| S16-1 | Git集成 | GitHub/GitLab/Gitee |
| S16-2 | CI/CD集成 | Jenkins/GitHub Actions |
| S16-3 | 消息通知 | Slack/钉钉/飞书 |
| S16-4 | 项目管理 | Jira/Linear/Notion |

### Sprint 17: 审计与合规

| ID | 任务 | 描述 |
|----|------|------|
| S17-1 | 操作审计日志 | 全量操作记录 |
| S17-2 | 会话回放 | 终端操作回放 |
| S17-3 | 合规报告 | 自动生成报告 |
| S17-4 | 数据保留策略 | 自动清理/归档 |

### Sprint 18: 移动端支持

| ID | 任务 | 描述 |
|----|------|------|
| S18-1 | 响应式UI | 移动端适配 |
| S18-2 | PWA支持 | 离线访问/推送 |
| S18-3 | 移动终端 | 触屏终端交互 |

### Sprint 19: AI增强

| ID | 任务 | 描述 |
|----|------|------|
| S19-1 | 智能任务分解 | AI自动拆分任务 |
| S19-2 | 代码审查助手 | AI代码Review |
| S19-3 | 故障预测 | 基于历史数据预测 |
| S19-4 | 自然语言运维 | 用自然语言执行运维 |

---

## 技术栈汇总

### 后端新增依赖

```go
// go.mod 新增
require (
    github.com/casbin/casbin/v2 v2.77.0    // RBAC权限
    golang.org/x/crypto v0.14.0             // SSH支持
    github.com/robfig/cron/v3 v3.0.1        // 定时任务
    github.com/expr-lang/expr v1.15.0       // 表达式求值
    github.com/go-redis/redis/v8 v8.11.5    // Redis(可选)
    gopkg.in/yaml.v3 v3.0.1                 // YAML解析
)
```

### 前端新增依赖

```json
{
  "dependencies": {
    "@vue-flow/core": "^1.26.0",
    "@vue-flow/background": "^1.3.0",
    "echarts": "^5.4.3",
    "vue-echarts": "^6.6.1"
  }
}
```

### 新增目录结构

```
backend/
├── service/
│   ├── ssh/              # SSH管理 (Sprint 8)
│   ├── workflow/         # 工作流引擎 (Sprint 9)
│   ├── orchestrator/     # Agent编排 (Sprint 10)
│   └── monitor/          # DevOps监控 (Sprint 11)
├── model/
│   ├── ssh.go            # SSH服务器模型
│   ├── workflow.go       # 工作流模型
│   └── agent.go          # Agent配置模型

frontend/src/
├── views/
│   ├── Servers.vue       # 服务器管理
│   ├── Workflows.vue     # 工作流列表
│   ├── WorkflowEditor.vue # 工作流编辑器
│   └── Monitor.vue       # 监控仪表盘
```

---

## 扩展版本规划

| 版本 | Sprint | 功能 | 依赖 |
|------|--------|------|------|
| v2.3.0 | Sprint 7 | 共享基础设施 | Sprint 6 |
| v3.0.0 | Sprint 8 | SSH多服务器管理 | Sprint 7 |
| v3.1.0 | Sprint 9 | 工作流引擎 | Sprint 7 |
| v3.2.0 | Sprint 10 | Agent编排 | Sprint 7 |
| v3.3.0 | Sprint 11 | DevOps监控 | Sprint 8 |
| v3.4.0 | Sprint 12 | Runbook运维 | Sprint 9 |
| v3.5.0 | Sprint 13 | Project/Workspace | Sprint 8 |
| v4.0.0 | Sprint 14 | 插件系统 | Sprint 13 |
| v4.1.0 | Sprint 15 | 团队协作 | Sprint 14 |
| v4.2.0 | Sprint 16 | 外部集成 | Sprint 15 |
| v4.3.0 | Sprint 17 | 审计合规 | Sprint 15 |
| v5.0.0 | Sprint 18-19 | 移动端+AI增强 | Sprint 17 |

---

## 扩展功能依赖图

```
Sprint 1-6 (现有功能)
        │
        ▼
┌───────────────────────────────────────────┐
│         Sprint 7: 共享基础设施             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐      │
│  │ Auth v2 │ │Terminal │ │EventBus │      │
│  │ Secrets │ │   v2    │ │         │      │
│  └────┬────┘ └────┬────┘ └────┬────┘      │
└───────┼───────────┼───────────┼───────────┘
        │           │           │
        └─────┬─────┴─────┬─────┘
              │           │
    ┌─────────┼───────────┼─────────┐
    ▼         ▼           ▼         ▼
┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐
│Sprint │ │Sprint │ │Sprint │ │Sprint │
│   8   │ │   9   │ │  10   │ │  11   │
│ SSH   │ │工作流 │ │Agent  │ │DevOps │
└───┬───┘ └───┬───┘ └───────┘ └───────┘
    │         │
    ▼         ▼
┌───────┐ ┌───────┐
│Sprint │ │Sprint │
│  13   │ │  12   │
│Project│ │Runbook│
└───┬───┘ └───────┘
    │
    ▼
┌───────────────────────────────────────────┐
│         Sprint 14-19: 未来扩展             │
│  插件系统 → 团队协作 → 外部集成 → 审计     │
│                    ↓                       │
│            移动端 + AI增强                 │
└───────────────────────────────────────────┘
```

---

## 并行开发指南

### Sprint 7 完成后可并行的任务

| 开发者 | Sprint | 模块 | 前置条件 |
|--------|--------|------|----------|
| A | Sprint 8 | SSH管理 | Terminal v2接口 |
| B | Sprint 9 | 工作流引擎 | EventBus |
| C | Sprint 10 | Agent编排 | EventBus + Auth v2 |

### 团队分工建议

**3人团队**:
```
开发者A: Sprint 7(Auth) → Sprint 8(SSH) → Sprint 11(DevOps)
开发者B: Sprint 7(Terminal) → Sprint 9(工作流)
开发者C: Sprint 7(EventBus) → Sprint 10(Agent编排)
```

**2人团队**:
```
开发者A: Sprint 7(Auth+Terminal) → Sprint 8 → Sprint 11
开发者B: Sprint 7(EventBus) → Sprint 9 → Sprint 10
```

---

## 参考资源

### 开源项目参考

| 项目 | 用途 | 链接 |
|------|------|------|
| Wave Terminal | AI终端+SSH | https://waveterm.dev |
| Chaterm | AI SSH客户端 | https://chaterm.ai |
| n8n | 工作流自动化 | https://n8n.io |
| vue-flow | 流程图编辑器 | https://vueflow.dev |

### 技术文档

- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) - Go SSH库
- [Casbin](https://casbin.org/) - RBAC权限管理
- [robfig/cron](https://github.com/robfig/cron) - Go定时任务
