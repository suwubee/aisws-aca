# AI 审核（审批）与 AI 托管流程梳理

本文以 AISWS-ACA（本仓库）当前实现为准，整理两条主流程：

- **AI 审核（审批）**：当 Claude Code/Codex/Gemini 等 CLI 在终端里弹出确认/权限提示时，系统自动检测并按规则（或 AI）给出处理动作。
- **AI 托管**：基于任务配置自动启动 CLI、自动输入提示、并持续监控终端日志判断完成/异常。

---

## 1) 概念与数据对象

- **Task（任务）**：`backend/model/db.go` 中 `Task`，带 `work_dir/cli_type/initial_prompt` 与 `ai_managed/ai_prompt/ai_end_condition/ai_error_handling`。
- **TerminalSession（终端会话）**：`backend/model/db.go` 中 `TerminalSession`，运行 PTY（本地）或 SSH 会话（远程）。
- **RuleSet（审批规则集）**：`backend/model/db.go` 中 `RuleSet`，支持 `manual/auto_yes/smart` 三种模式，并可配置黑白名单与 AI Provider。
- **ApprovalRecord（审批记录）**：`backend/model/db.go` 中 `ApprovalRecord`，记录每次审批的提示片段/输入/规则命中/AI 决策说明。
- **Message（消息中心）**：`backend/model/db.go` 中 `Message`，用于“需要干预/被阻止/告警”等通知类信息。
- **PromptTemplate（提示词模板）**：`backend/model/prompt_template.go`，系统级提示词统一从数据库读取（非硬编码），支持变量渲染。
- **PromptTemplatePreset（提示词方案）**：`backend/model/prompt_template_preset.go`，每个模板 Key 可保存多个命名方案，并可一键套用。

**任务状态（建议按此理解）**

- `todo`：待办
- `in_progress`：进行中
- `paused`：暂停（通常代表需要用户介入后再继续）
- `done`：完成
- `failed`：失败
- `timeout`：超时
- `archived`：归档

---

## 2) 提示词模板系统（全局配置，不再硬编码）

### 2.1 设计目标

- **避免硬编码**：所有系统级 AI 提示词从数据库读取；代码只保留“默认模板文件”（可恢复默认）。
- **可编辑 + 可选择**：在前端系统设置中可直接编辑模板，并可创建/选择多个“方案（Preset）”。
- **动态信息用变量注入**：规则集、日志上下文、任务字段等通过变量渲染进入模板（而不是拼接固定文案）。

### 2.2 入口与 UI

- 前端入口：系统设置 → `提示词模板`（管理员可见）
- 后端 API（管理员）：`/api/prompt-templates`、`/api/prompt-templates/:key`、`/api/prompt-templates/:key/presets`

### 2.3 当前内置模板 Key（与业务绑定点）

- `approval.system_prompt`：AI 审核系统提示词（变量：`extra_rules`，来自规则集 `ai_prompt`）
- `task_monitor.system_prompt`：任务监控系统提示词（变量：`log_limit`、`max_log_chars`）
- `task.managed_prompt`：AI 任务托管提示词模板（变量：`task_initial_prompt`、`task_ai_prompt`、`task_ai_end_condition`、`task_done_marker`）
- `ai_workflow.system_prompt`：AI 工作流系统提示词（变量：`tools`）
- `ai_workflow.user_goal_prompt`：AI 工作流用户目标包装模板（变量：`user_goal`）

### 2.4 方案（Preset）机制

- 每个模板 Key 内置一个 **默认方案**（`name=默认`，`is_builtin=true`）。
- 套用方案会同时更新：
  - 模板内容 `template`
  - 当前生效方案 `active_preset_id`
- 直接编辑/保存模板会清空 `active_preset_id`（表示当前为自定义内容）。

## 3) AI 审核（审批）流程

### 2.1 触发：检测到“等待审批”提示

入口：`backend/service/terminal/session.go` 的 `detectAndHandle()`

1. PTY 输出进入 `detectAndHandle(data)`。
2. `detector.DetectState(output)` 判断是否命中 `waiting_approval`（如 `(y/n)`、`allow write?` 等）。
3. 若命中，触发 `go s.handleApproval(output)`。

相关实现：

- 审批提示模式：`backend/service/detector/detector.go`（`defaultStatePatterns`）

### 2.2 决策：审批引擎按规则/AI 评估

入口：`backend/service/terminal/session.go` 的 `handleApproval()`

1. 从 scrollback 截取最近上下文（`ContextLines`，上限 200）。
2. 调用 `approvalEngine.EvaluateWithContext(ctx, terminalID, output, fullContext)`。
3. 引擎根据终端 `rule_mode`（none/system/task/custom）解析有效 RuleSet：
   - `backend/service/approval/engine.go`：`GetAutomationConfig()`
4. `EvaluateWithContext()` 先确认是否“确实是审批提示”；若不是则返回并跳过。
5. 根据 `approval_mode` 进入：
   - `manual`：只做黑名单阻断（可通知），其余等待人工
   - `auto_yes`：命中黑名单则等待人工，否则自动输入（yes/y/enter/1）
   - `smart`：白名单直接通过；黑名单阻断；否则调用 AI Provider 判断 approve/reject/input/wait

### 2.3 执行：自动输入或等待人工

回到 `session.go`：

- 若 `result.Action` 为 `approve/input` 且 `result.Input` 非空：`session.Write()` 自动写入终端，并标记 `auto_handled=true`。
- 否则：保持等待，由用户在终端或审批中心手动输入。

### 2.4 记录与推送

1. `approvalEngine.RecordApproval()` 落库到 `approval_records`（并关联 `task_id/server_id`）。
2. 通过终端 WebSocket 推送事件：
   - `type: "approval"`
   - `approval_result: { action, input, reasoning, confidence, rule_matched, ai_decision, auto_handled }`
   - `message`: 选取到的“审批提示片段/上下文”

后端 WS：

- `backend/api/terminal.go`：`/api/terminal/ws` 转发 `StreamEvent`（包含 `approval_result` 与 `ai_log`）。

前端接收：

- `frontend/src/components/Terminal.vue`：处理 `type === "approval"`，将未自动处理的审批写入 `approvalStore`。
- `frontend/src/components/ApprovalCenter.vue`：展示待处理审批并可一键允许/拒绝/自定义输入。

---

## 4) AI 托管流程

入口：`backend/api/task.go` 的 `StartTask()` → `backend/service/task/automation.go` 的 `AutomationService.StartTask()`

### 3.1 启动阶段（创建终端 + 启动 CLI + 发送提示）

1. 创建本地/远程工作目录（可选）。
2. 创建终端会话：
   - 本地：`terminalManager.CreateSession(...)`
   - 远程：`terminalManager.CreateSSHSession(serverID)`
3. 写入：
   - `cd work_dir`
   - 启动 CLI（claude/codex/gemini）
4. 若配置了初始提示：
   - 普通模式：发送 `initial_prompt`
   - **AI 托管模式**：发送 `buildManagedPrompt(task)`（组合“任务目标/执行规则/完成条件”）
5. 启动后台监控：`go monitorTaskCompletion(taskID, terminalID)`

### 3.2 监控阶段（日志分析 → 决策 → 更新状态）

入口：`backend/service/task/automation.go` 的 `monitorTaskCompletion()`

1. 定期读取数据库中的最近日志（默认 200 行）。
2. `TaskMonitor.AnalyzeLogs()`：
   - 优先尝试调用 AI Provider 输出 JSON
   - AI 不可用或解析失败则降级启发式规则（含 `ACA_TASK_DONE` 标记）
3. `MakeDecision()` 输出：
   - `complete`：标记任务 `done`
   - `alert`：需要用户介入；若 `ai_managed=true` 则标记 `paused` 并创建消息
   - `retry`：按 `ai_error_handling` 策略决定继续/暂停/失败
   - `continue`：继续监控

消息通知：

- `backend/service/task/automation.go`：`createTaskMessage()` 写入 `messages`（例如 `approval_needed/warning/error`）

---

## 5) 体验优化要点（本次已落地）

- 任务状态仅接受 `todo/in_progress/paused/done/failed/timeout/archived`，不再兼容 `pending/completed`；`/tasks/by-status` 将 `paused` 归入“进行中”、`failed/timeout` 归入“已完成”列显示。
- 终端 WebSocket 补齐 `approval_result` 与 `ai_log` 字段，前端可正确接收审批事件与 AI 日志。
- 增加 `POST /api/terminals/:id/input`，审批中心可在非终端页面直接发送输入，完成“检测 → 展示 → 手动处理 → 落库”的闭环。
- `reset-data` 会尝试关闭所有终端会话，并清理除 `users` 与内置 `workflow_templates` 外的全部业务数据（任务/终端/日志/审批/消息/服务器/项目/工作流/规则/AI Provider 等），避免“假重置”。
- 所有系统级 AI 提示词从数据库模板读取（不再硬编码），并支持在系统设置中编辑、保存为方案、选择/套用方案（含变量渲染）。
- 终端输入稳定性：前端 xterm 在容器可见时再 open/fit，初次渲染补一帧 fit；WebSocket 断线自动重连，降低“无法输入/光标错位”的概率。
- 任务与终端绑定：同一任务内允许终端重连/重启后继续执行；不同任务禁止复用终端，避免 AI 指令串任务。新增 `POST /api/tasks/:id/bind-terminal`、`POST /api/tasks/:id/resume`。
- 预期断开自动恢复：AI 输出重启类命令时标记 `expect_disconnect`，SSH 断开后自动重连/重启并更新任务活跃终端；连接恢复后仅在因 `terminal_disconnected` 暂停时自动恢复运行。新增 `POST /api/terminals/:id/restart`。
- 审批/AI 日志稳定性：避免组件卸载后仍持续重连 WebSocket，减少异常更新与资源泄漏。
- 移动端体验：Kanban/工作流/日志/AI 决策日志等长列表切换为 card 模式；移动端判定增加 coarse pointer + 横屏覆盖，菜单使用抽屉、弹窗宽度自适应。
- 托管启动安全提示：当 CLI 命令不可用（例如 `claude` 不在 PATH）时自动暂停任务并提示用户手动确认（如尝试 `claude` 或 `npx claude`），避免把提示词误当作 shell 命令执行。
