# AI 审核（审批）与 AI 托管流程梳理

本文以 AISWS-ACA（本仓库）当前实现为准，整理两条主流程：

- **AI 审核（审批）**：当 Claude Code/Codex/Gemini 等 CLI 在终端里弹出确认/权限提示时，系统自动检测并按规则（或 AI）给出处理动作。
- **AI 托管**：让 AI 替代人工去“接收目标 → 下发输入/命令 → 观察输出 → 决策下一步/结束/求助”。

> 重要：本项目里 **AI 托管** 与 **AI 审核** 是两件事。
>
> - **AI 托管（Managed/Handoff）**：AI 是“驾驶员”，负责推进任务（主动输入、执行与反馈闭环）。
> - **AI 审核（Approval/Audit）**：AI 是“安全员/加速器”，只在出现确认/选择题等需要人工判断的交互点时，按规则/AI 做 `approve/reject/input/ask_user`，避免卡死与降低高危误操作。

---

## 1) 概念与数据对象

- **Task（任务）**：`backend/model/db.go` 中 `Task`，带 `work_dir/cli_type/initial_prompt` 与 `ai_managed/ai_prompt/ai_end_condition/ai_error_handling`。
- **TerminalSession（终端会话）**：`backend/model/db.go` 中 `TerminalSession`，运行 PTY（本地）或 SSH 会话（远程）。
- **RuleSet（审批规则集）**：`backend/model/db.go` 中 `RuleSet`，支持 `manual/auto_yes/smart` 三种模式，并可配置黑白名单与 AI Provider。
- **ApprovalRecord（审批记录）**：`backend/model/db.go` 中 `ApprovalRecord`，记录每次审批的提示片段/输入/规则命中/AI 决策说明。
- **Message（消息中心）**：`backend/model/db.go` 中 `Message`，用于“需要干预/被阻止/告警”等通知类信息。
- **PromptTemplate（提示词模板）**：`backend/model/prompt_template.go`，系统级提示词统一从数据库读取（非硬编码），支持变量渲染。
- **PromptTemplatePreset（提示词方案）**：`backend/model/prompt_template_preset.go`，每个模板 Key 可保存多个命名方案，并可一键套用。

### 1.1 AI 托管的两种执行引擎（务必区分）

同样叫“托管”，但底层执行路径不同：

1) **CLI 托管（automation_mode=cli）**
   - 依赖目标服务器终端里安装的 AI CLI（Claude Code/Codex/Gemini 等）。
   - 系统负责：创建/绑定终端 → 启动 CLI → 发送提示 → 监控日志 →（配合 AI 审核处理 y/n 等）。
   - 适合：目标环境已装好 AI CLI，用户希望“让 CLI 自己做事”，系统做编排与治理。

2) **AI 托管(动态) / Task Agent（automation_mode=agent）**
   - 不依赖服务器安装 AI CLI；由本系统配置的 AI Provider（OpenAI/Anthropic/…）直接驱动“ReAct 循环 + 工具调用”。
   - 系统负责：创建可观测终端（可选）→ AI 规划/决策 → 调用工具执行（SSH/终端 RunCommand 等）→ 记录步骤与状态 → 在需要时 ask_user 暂停。
   - 适合：服务器小白/不想装 AI CLI、希望“AI 直接接管执行面”的用户。

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

### 3.5 统一的“终端交互点”状态机（托管与审核共用）

托管与审核能否顺畅，核心在于：**终端当前到底是在“等你输入”、还是在“等你确认”、还是在“忙”**。

本项目用 `backend/service/detector/detector.go` 把终端输出粗分为：

- `waiting_approval`：出现 y/n、Enter to confirm、选择列表等“需要判断的交互点”
  - **AI 审核**：触发审批引擎 `approve/reject/input/wait`（可落库审计）
  - **AI 托管(动态)**：若走 terminal 执行面，可让审批引擎自动处理或 ask_user 暂停
- `waiting_input`：出现可输入提示符（CLI prompt / 输入框）
  - **CLI 托管**：此时才安全发送 prompt（避免误当 shell 命令）
- `working`：模型/工具执行中
  - **托管**：允许用户追加消息，但会排队到“下一轮边界”处理，保证对话顺序
- `idle`：更像 shell prompt（$/#/%）
  - **CLI 托管**：通常表示不在 AI CLI 交互态（或已退出/未进入）

> 备注：状态机是“最佳努力”启发式；因此对高风险输入默认更保守（宁可暂停 ask_user，也不误输入）。

---

## 4) AI 托管流程

入口：`backend/api/task.go` 的 `StartTask()` → `backend/service/task/automation.go` 的 `AutomationService.StartTask()`

### 4.0 多终端一致性：Task 级 AI 状态 vs Terminal 级 CLI/审批（避免“串消息”）

本项目允许 **一个任务关联多个终端**（`TerminalSession.task_id`），但 **AI 托管运行态是任务级别**：

- **Task 级别（全局）**：`ai_managed`、`ai_status`、`agent_session_id`、`active_terminal_id`
- **Terminal 级别（局部）**：`metadata.ai_assistant.*`（是否进入 CLI、waiting_input/working 等）、审批提示与输入（Approval/Ask-user）

因此在多终端场景中，常见误解是：

- 在 A 终端启用 AI 托管后，切到 B 终端仍看到 “AI运行中”，以为 AI 也在 B 上执行；
- UI 轮询/WS 未做“终端切换竞态保护”，导致 A 的异步结果覆盖到 B 的面板上，形成“串消息”的错觉。

**建议的闭环原则（当前前端已按此方向修复）**

1. 当任务 `active_terminal_id` 存在且任务处于 AI 托管/Agent 模式时，AI 控制面板需要显式展示“AI活跃终端”并提供一键切换。
2. AI 控制面板的日志订阅/审批轮询需要按“当前控制目标终端”做作用域隔离，并在异步请求返回时校验作用域，避免跨终端覆盖。
3. 用户点击 AI 控制入口时，若当前查看终端不是任务的 `active_terminal_id`，应优先引导/切换到活跃终端，避免把“查看终端”和“控制终端”混淆。

### 4.1 CLI 托管：启动阶段（创建终端 + 启动 CLI + 发送提示）

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

#### 4.1.1 CLI 就绪检测（避免误把 prompt 当 shell 命令）

为了避免“CLI 未进入交互界面 → 系统把提示词当作 shell 命令执行”的高风险误操作，启动阶段会做 **就绪检测**：

- **CLI 托管（`automation_mode=cli`）**：必须在任务里明确选择 `cli_type`，终端会开启“AI CLI 状态跟踪”，并使用 **所选 CLI 的输出锚点** 命中后将 `metadata.ai_assistant.detected=true` 作为“已进入 CLI 模式”的标记。
- **CLI 可选（`ai_managed=true` 但 `automation_mode!=cli`）**：CLI 不强制。系统会基于输出锚点做“候选预判”（`needs_confirm=true`），并允许用户在终端面板手动确认（是/否/不确定），必要时也可触发 AI 预判（见 `/api/terminals/:id/ai-assistant/evaluate` 与 `/api/terminals/:id/ai-assistant/confirm`）。
- 进入/退出 CLI 的自动判断仍以 **终端输出锚点/启发式证据** 为主；不会把“输入命令关键词”直接当作确定进入/退出的依据（只作为用户确认/AI 判断时的上下文线索之一）。
- 随后基于终端输出的状态检测（`waiting_input/working/waiting_approval`）确认 CLI 已进入可交互态，再发送提示词。
- 若超时仍无法确认，就暂停任务并提示用户手动确认 CLI 安装/启动方式（例如 `claude` 或 `npx claude`）。

### 4.2 CLI 托管：监控阶段（日志分析 → 决策 → 更新状态）

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

### 4.3 AI 托管(动态) / Task Agent：ReAct 循环（可多轮持续接收用户补充）

入口：`backend/api/task.go` 的 `StartTask()`（automation_mode=agent）→ `backend/service/workflow/task_agent.go` 的 `StartTaskAgent()`

1. 为任务创建 AIWorkflowSession（ReAct 模型：`<thought>/<action>/<complete>`）。
2. 可选创建/复用一个“可见终端”用于观测与介入（终端不是执行唯一通道，执行由工具层决定）。
3. 进入执行循环：AI 决策下一步工具调用 → 工具执行 → 结果作为 observation 回灌 → 继续下一轮。
4. 遇到不确定/风险/信息不足时输出 `ask_user` 并将会话置为 `paused`，等待用户补充后继续。

**多轮补充信息的关键保障（避免“多次回复失效”）**

- 前端通过 `POST /api/ai-workflow/session/:id/message` 追加用户消息。
- 后端在执行中会把消息先进入 pending 队列，确保消息顺序不会插入到“正在生成的 assistant 回复”之前。
- 为避免在会话完成/暂停的边界窗口丢消息，执行器在退出前会进入 closing 保护，并在必要时落库/重启循环，确保用户追加信息不会被吞。

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
