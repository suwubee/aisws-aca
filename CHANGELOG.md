# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### v3.2.0 - Sprint 10 AI Agent编排与工作流引擎 (2026-01-08)

#### Sprint 10: AI Agent编排

**核心功能**:
- **S10-1**: AI工作流工具函数 - ReAct框架工具定义
- **S10-2**: ReAct框架AI引擎 - Thought→Action→Observation循环
- **S10-3**: CLI会话管理服务 - 检测claude/codex状态
- **S10-4**: 任务启动流程改进 - 使用detector检测CLI就绪
- **S10-5**: 任务监控服务 - 日志分析/AI决策
- **S10-6**: 并行任务执行器 - Codex MCP集成
- **S10-7**: 前端AI工作流组件 - 对话式交互界面

**后端新增文件**:
- `service/workflow/tools.go` - AI工具定义(ReAct框架)
- `service/workflow/ai_engine.go` - AI驱动的工作流引擎
- `service/workflow/tool_executor.go` - 工具执行器
- `service/workflow/parallel_executor.go` - 并行任务执行器
- `service/cli/session_manager.go` - CLI会话管理
- `service/cli/task_launcher.go` - 任务启动器
- `service/task/monitor.go` - 任务监控服务
- `api/ai_workflow.go` - AI工作流API接口

**前端新增文件**:
- `src/components/AIWorkflowChat.vue` - AI工作流对话组件
- `src/api/ai-workflow.ts` - AI工作流API

**API接口**:
```
POST /api/ai-workflow/start      # 启动AI工作流
GET  /api/ai-workflow/session/:id # 获取会话状态
GET  /api/ai-workflow/sessions    # 列出所有会话
```

**技术亮点**:
- ReAct框架支持任意LLM（无需Function Calling）
- CLI会话检测（Claude UUID / Codex rollout文件）
- 任务启动流程：StartCLI → WaitForReady → SendTask
- AI驱动的日志分析与决策（continue/retry/alert/complete）
- 启发式降级：AI分析失败时自动使用关键词检测

---

### v1.4.0 - Sprint 1-6 完成 (2026-01-07)

#### Sprint 1: Bug修复 & 基础完善
- **S1-1**: 修复ResetData引用不存在的表 (HasTable检查)
- **S1-2**: 实现任务编辑模态框 (TaskEditModal.vue)
- **S1-3**: 实现任务卡片AI状态显示 (从terminal.metadata.ai_assistant获取)
- **S1-4**: 修复密码修改持久化问题 (改用数据库存储)

#### Sprint 2: 审批UI完善
- **S2-1**: 审批提示弹窗组件 (ApprovalPrompt.vue)
- **S2-2/S2-3**: 快捷操作按钮 + 手动输入框
- **S2-4**: WebSocket审批事件监听 (approval.ts store)
- **S2-5**: 审批消息中心组件 (ApprovalCenter.vue)

#### Sprint 3: AI集成增强
- **S3-1**: AI决策结果展示组件 (AIDecisionDisplay.vue)
- **S3-2**: 审批置信度可视化 (进度条+颜色分级)
- **S3-3**: AI决策日志面板 (AIDecisionLog.vue)
- **S3-4**: 智能建议展示 (SmartSuggestion.vue)

#### Sprint 4: 多代理支持
- **S4-3**: 多代理状态监控面板 (AgentMonitor.vue)
- **S4-4**: 代理配置管理界面 (AgentConfig.vue)
- **S4-5**: 代理性能统计 (AgentStats.vue)

#### Sprint 5: 高级功能
- **S5-1/S5-2**: 任务评论API + 前端组件 (TaskComments.vue)
- **S5-3/S5-4**: 日志导出API + 前端功能 (LogExport.vue)
- **S5-5**: 规则导入导出 (RuleImportExport.vue)

#### Sprint 6: 部署优化
- **S6-1**: Docker镜像优化 (多阶段构建Dockerfile)
- **S6-2**: docker-compose配置
- **S6-3**: 多用户数据模型设计 (User扩展Email/Role/Status)
- **S6-4**: 多用户认证改造 (RequireRole中间件)
- **S6-5**: 用户管理界面 (UserManagement.vue)

---

### v1.1.0 - 规则系统重构 (2026-01-05)

#### Bug Fixes
1. **Settings.vue fetchSystemConfig 报错修复**
   - 问题: `Cannot read properties of undefined (reading 'approval_mode')`
   - 原因: API返回数据结构变化后未正确处理null值
   - 解决: 添加空值检查，使用新的统一API接口

2. **系统规则保存404错误修复**
   - 问题: `PUT /api/automation/system-config 404 (Not Found)`
   - 原因: 旧API端点已废弃
   - 解决: 使用新的 `/api/automation/system-rule` 端点

3. **审批引擎编译错误修复**
   - 问题: `service/approval/engine.go` 引用已删除的 `model.TerminalAutomation`
   - 解决: 重构引擎使用新的 `EffectiveConfig` 结构，支持多级规则继承

#### Architecture Redesign - RuleSet统一模型

**新数据模型**:
```go
// RuleSet 规则集模型 - 可被系统、任务、终端复用
type RuleSet struct {
    ID                string    // 主键
    Name              string    // 规则名称
    Type              string    // system, task, terminal
    ApprovalMode      string    // manual, auto_yes, smart
    AutoInputType     string    // yes, y, enter, option1
    WhitelistPatterns string    // JSON数组
    BlacklistPatterns string    // JSON数组
    AIProviderID      *string   // 关联的AI Provider
    AIPrompt          string    // AI判断提示词
    ContextLines      int       // 上下文行数
    DetectClaudeCode  bool      // 检测Claude Code
    DetectCodex       bool      // 检测Codex
    DetectGemini      bool      // 检测Gemini CLI
    NotifyOnBlock     bool      // 阻止时通知
    NotifyOnApprove   bool      // 自动通过时通知
}

// TerminalSession 新增字段
type TerminalSession struct {
    RuleMode  string   // none, system, task, custom
    RuleSetID *string  // 关联的规则集ID
}

// Task 新增字段
type Task struct {
    RuleSetID *string  // 关联的规则集ID
}
```

**规则继承机制**:
- `none`: 不使用规则，默认手动审批
- `system`: 继承系统级规则
- `task`: 继承关联任务的规则（需终端已关联任务且任务有规则）
- `custom`: 终端独立的自定义规则

**新API端点**:
```
# 系统规则
GET  /api/automation/system-rule         获取系统规则
PUT  /api/automation/system-rule         更新系统规则

# 规则集CRUD
GET  /api/automation/rulesets            获取规则集列表
POST /api/automation/rulesets            创建规则集
GET  /api/automation/rulesets/:id        获取规则集详情
PUT  /api/automation/rulesets/:id        更新规则集
DELETE /api/automation/rulesets/:id      删除规则集

# 终端规则模式
GET  /api/automation/terminals/:id/rule-mode    获取终端规则模式
PUT  /api/automation/terminals/:id/rule-mode    更新终端规则模式
POST /api/automation/terminals/:id/custom-rule  创建终端自定义规则

# 默认规则模板
GET  /api/automation/patterns/defaults   获取默认白名单/黑名单模板
```

#### Changed Files

**Backend**:
- `model/db.go` - 新增RuleSet模型，更新Task和TerminalSession模型
- `api/automation.go` - 完全重写，实现新API
- `api/task.go` - 支持rule_set_id字段
- `service/approval/engine.go` - 重构使用EffectiveConfig

**Frontend**:
- `src/api/index.ts` - 新增RuleSet类型和API方法
- `src/views/Settings.vue` - 使用新API
- `src/components/TerminalRuleConfig.vue` - 支持多种规则模式

---

## [1.0.0] - MVP Release

### Added
- 用户认证系统 (JWT)
- 终端托管核心 (PTY + WebSocket)
- Kanban任务管理
- 任务-终端关联
- AI代理检测 (Claude Code, Codex, Gemini CLI)
- 审批系统后端
- 自动化配置系统
- AI Provider配置
