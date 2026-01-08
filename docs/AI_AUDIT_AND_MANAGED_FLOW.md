# AI 审核（审批）与 AI 托管流程梳理

本文以 `ai-coding-assistant` 项目当前实现为准，整理两条主流程：

- **AI 审核（审批）**：当 Claude Code/Codex/Gemini 等 CLI 在终端里弹出确认/权限提示时，系统自动检测并按规则（或 AI）给出处理动作。
- **AI 托管**：基于任务配置自动启动 CLI、自动输入提示、并持续监控终端日志判断完成/异常。

---

## 1) 概念与数据对象

- **Task（任务）**：`backend/model/db.go` 中 `Task`，带 `work_dir/cli_type/initial_prompt` 与 `ai_managed/ai_prompt/ai_end_condition/ai_error_handling`。
- **TerminalSession（终端会话）**：`backend/model/db.go` 中 `TerminalSession`，运行 PTY（本地）或 SSH 会话（远程）。
- **RuleSet（审批规则集）**：`backend/model/db.go` 中 `RuleSet`，支持 `manual/auto_yes/smart` 三种模式，并可配置黑白名单与 AI Provider。
- **ApprovalRecord（审批记录）**：`backend/model/db.go` 中 `ApprovalRecord`，记录每次审批的提示片段/输入/规则命中/AI 决策说明。
- **Message（消息中心）**：`backend/model/db.go` 中 `Message`，用于“需要干预/被阻止/告警”等通知类信息。

**任务状态（建议按此理解）**

- `todo`：待办
- `in_progress`：进行中
- `paused`：暂停（通常代表需要用户介入后再继续）
- `done`：完成
- `failed`：失败
- `timeout`：超时
- `archived`：归档

---

## 2) AI 审核（审批）流程

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

## 3) AI 托管流程

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

## 4) 体验优化要点（本次已落地）

- 任务状态仅接受 `todo/in_progress/paused/done/failed/timeout/archived`，不再兼容 `pending/completed`；`/tasks/by-status` 将 `paused` 归入“进行中”、`failed/timeout` 归入“已完成”列显示。
- 终端 WebSocket 补齐 `approval_result` 与 `ai_log` 字段，前端可正确接收审批事件与 AI 日志。
- 增加 `POST /api/terminals/:id/input`，审批中心可在非终端页面直接发送输入，完成“检测 → 展示 → 手动处理 → 落库”的闭环。
- `reset-data` 会尝试关闭所有终端会话并清理相关表数据，避免仅删库导致“列表仍有终端/进程未结束”的假重置。
- 移动端支持：菜单改为抽屉、表格支持横向滚动、弹窗宽度自适应。
- 终端输入稳定性：避免在隐藏容器里初始化 xterm、切换/展开面板时自动 fit，并修复长粘贴导致输入发送失败。
