# CLI Session Management（Claude/Codex）MVP 方案

> 目标：把 **Claude Code / Codex CLI** 的“会话（session / resume）”纳入 ACA 的统一对象模型，支持在 **项目集 → 项目 → 任务** 下系统化管理，并能在“一个任务可能启动/重连多次”的现实流程里做到可追溯、可恢复、可审计。
>
> 约束：仍处于 MVP 阶段，优先 **复用现有数据表与终端检测能力**，避免引入复杂的“跨任务会话合并/全量索引/远端文件同步”等重量功能。

---

## 1) 现状盘点（ACA 已有但未闭环的能力）

### 1.1 任务与终端执行面（已具备）

- `backend/model/db.go`：`Task` 已具备 `automation_mode=cli`、`cli_type`、`work_dir`、`active_terminal_id` 等字段。
- `backend/service/task/automation.go`：任务启动会创建终端、`cd work_dir`、运行 `claude/codex/gemini` 并（在 CLI ready 后）发送提示词。
- `backend/service/terminal/session.go`：终端输出检测已能识别 AI CLI 类型与状态（`waiting_input/working/waiting_approval/idle`），并驱动审批治理。

### 1.2 AI CLI Session（数据结构已存在，但当前未接入运行时）

ACA 其实已经有两块“未来会用到”的拼图，但目前未形成闭环：

1) **`AISession` 表**（`backend/model/db.go`）  
   - 字段包含 `terminal_id/task_id/ai_type/state/session_id/session_file`，其中 `session_id` 明确用于 `--resume`。

2) **CLI Session 解析器**（`backend/service/cli/session_manager.go`）  
   - 具备从终端输出中提取：
     - Claude Code：`session_id`（UUID）
     - Codex：`rollout-...<uuid>.jsonl`（同时得到 `session_file` + `session_id`）
   - 但当前该模块未被终端 Session 的输出管线调用，因此 DB 里不会产生可用会话数据。

3) **`CLIProfile` 表**（`backend/model/project.go`）  
   - 可用于配置 `command/default_args`，但当前也未被任务启动逻辑使用。

---

## 2) 外部参考（Claude Code Workflow / CCW 的做法要点）

CCW 的可借鉴点是：把“CLI 会话”当作一等对象管理，并且尽量走 **工具原生 resume**：

- Claude Code
  - 会话文件：`~/.claude/projects/<projectPath>/.../*.jsonl`
  - resume：`claude --resume <session-id>` 或 `claude --continue`（继续最近会话）
- Codex
  - 会话文件：`~/.codex/sessions/YYYY/MM/DD/rollout-...-<uuid>.jsonl`
  - resume：`codex resume <uuid>` 或 `codex resume --last`

> 对 ACA 的启示（MVP）：**不必一开始就做“扫描远端 ~/.claude / ~/.codex”**，先把“运行过程中从输出捕获到的 session_id”落库并展示出来，已经能覆盖 80% 真实场景；后续再做“原生会话发现”作为增强。

---

## 3) ACA 的统一抽象：Terminal vs CLI Session

为避免混淆，建议明确两层 session：

- **TerminalSession（连接/承载层）**：PTY/SSH/tmux 连接与输出流（ACA 已有）。
- **CLISession / AISession（对话/恢复层）**：Claude/Codex 生成的可恢复会话（ACA 已有 `AISession`，建议直接复用）。

关系建议（MVP）：

- 一个 `Task` 可以关联多个 `TerminalSession`（已支持）。
- 一个 `TerminalSession` 在其生命周期内可能产生多个 `AISession`（用户重启 CLI、重新开始对话等）。
- 一个 `Task` 的“有效会话”本质是 `AISession` 列表；通过 `task_id` 即可自然归档到项目/项目集视图中。

---

## 4) MVP 架构方案（最小闭环）

### 4.1 数据层（尽量不改表）

直接复用 `AISession`：

- `task_id`：强绑定任务（已要求必填）
- `terminal_id`：定位发生在哪个终端/服务器
- `ai_type`：`claude-code/codex/gemini/unknown`
- `state`：建议按 `cli/session_manager.go` 的 `starting/ready/working/waiting_input`（或兼容现有 detector 状态）
- `session_id`：外部会话 ID（Claude UUID / Codex UUID）
- `session_file`：可选（Codex rollout jsonl / Claude jsonl 路径或文件名）

新增字段（可选、非 MVP 必需）：

- `server_id`（便于直接筛选，不必 join terminal/task）
- `work_dir`（便于后续做“远端会话发现”）
- `ended_at` / `status`（区分“已结束但可 resume” vs “不可恢复”）

> MVP 建议：先不加字段，通过 join Task/Terminal 得到 server/work_dir，先把链路跑通。

### 4.2 运行时：在终端输出管线里创建/更新 AISession

接入点建议在 `backend/service/terminal/session.go` 的 `detectAndHandle()`：

1) 当 `cliTrackingEnabled=true` 且检测到 AI CLI 进入（`AIAssistant.Detected` 从 false→true，或 `startupComplete`）时：
   - 创建一个新的 `AISession` 记录（或恢复/复用“最近一次未完成会话”，见 4.4）。
2) 每次收到输出 chunk：
   - 调用 `cli.SessionManager.UpdateFromOutput()` 更新 `ai_type/state/session_id/session_file` 并落库。
3) 当检测到“疑似退出 CLI”（从 CLI 状态回到 shell prompt）：
   - 仅更新 `state`（不建议直接删除/终止记录，避免误判丢会话）。

并在 `SessionMetadata` 中附带一个轻量快照（MVP UI 只需要几个字段）：

- `metadata.ai_assistant.session_id`
- `metadata.ai_assistant.session_file`
- `metadata.ai_assistant.ai_session_id`（ACA 内部主键，用于审批记录关联）

### 4.3 审批治理：ApprovalRecord 绑定到 AISession

在 `backend/service/terminal/session.go` 触发审批时，补充把“当前活跃 AISessionID”传入：

- `backend/service/approval/engine.go#RecordApproval()` 已支持 `aiSessionID *string`。

这样可以实现：

- 同一终端多次启动 CLI 时，审批记录按会话拆分；
- 任务复盘时能知道某个审批发生在“哪次对话/哪次 resume”。

### 4.4 “一个任务多次 resume”的组织方式（MVP 推荐规则）

不做复杂的“会话合并”，用简单规则即可：

- **每次 CLI 重新进入交互界面** → 创建一条新的 `AISession`（run 记录）
- 如果输出解析到的 `session_id` 与历史某条一致：
  - 视为“同一外部对话的再次连接”，仍保留多条 run 记录（便于审计）
  - UI 侧可以按 `session_id` 分组展示

额外可选（更智能但仍不复杂）：

- 如果同一 `terminal_id` 在短时间内重复进入/退出，且 `session_id` 相同，则复用上一条记录（避免刷屏）。

---

## 5) API 与 UI（最小可用）

### 5.1 API（建议新增）

- `GET /api/tasks/:id/ai-sessions`：列出任务下所有 `AISession`（按 `updated_at desc`）
- `GET /api/tasks/:id/ai-sessions/discover`：扫描本机/远端服务器上原生会话文件，返回候选列表（不落库）
- `POST /api/tasks/:id/ai-sessions/import`：把某条候选会话导入到任务，落库为 `AISession`（用于统一管理/一键 resume）
- `POST /api/tasks/:id/ai-sessions/collect`：**一键收纳**（discover + import），把“与该任务 work_dir/cli_type 关联的历史会话”直接纳入任务
- `GET /api/terminals/:id/ai-sessions`：列出终端下所有 `AISession`
- `POST /api/tasks/:id/ai-sessions/:aiSessionId/resume`
  - 行为：向任务的 `active_terminal_id` 注入“原生 resume 命令”
  - Claude：`claude --resume <session_id>`（或继续最近：`claude --continue`）
  - Codex：`codex resume <session_id>`（或最近：`codex resume --last`）

> MVP 也可以先不做 `resume` API，只在 UI 展示可复制的 resume 命令（风险更低）。

### 5.2 UI（建议落在 TaskDetail）

在 `frontend/src/views/TaskDetail.vue` 增加一个卡片 `CLI 会话`：

- 列表字段：`tool(ai_type)` / `session_id` / `state` / `updated_at` / `terminal`
- 操作：
  - 一键复制 resume 命令
  - 打开对应终端
  - （可选）在当前活跃终端执行 resume
  - **收纳历史**：一键扫描并导入（适用于“已经跑过 codex/claude，但当时没被 ACA 追踪”的情况）

并在终端顶部状态栏展示当前会话快照（从 metadata 取）：

- `Claude Code · <session_id> · waiting_input`

---

## 6) 多服务器/多项目并行托管：MVP 的边界与演进

### 6.1 MVP 可支持的并行模型

- **并行**：多个任务（每个任务一个 active terminal + 多个 CLI session run）并行运行。
- **串行**：同一任务内多次 resume / 重启终端，形成可审计链路。

### 6.2 下一阶段（非 MVP）再做的增强

- **原生会话发现增强**（远端）：目前已支持远端 `find` 扫描并导入；后续可增强：
  - 更精确的 codex `cwd` 过滤（远端读取首行 `session_meta`）
  - 更强的 Claude “项目路径”识别（从元数据推断真实路径，而非仅 project_key）
- **CLIProfile 真正落地**：把 `Task.cli_type` 映射到某个 profile（命令、参数、权限模式、json 输出等），实现“不同服务器/项目的启动命令不同”。
- **跨终端/跨服务器 guardrail**：resume 时校验会话是否属于同一 server/user/home，避免误在错误机器上 resume。
