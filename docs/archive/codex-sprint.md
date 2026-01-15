# Codex Sprint：多服务器 SSH + Workflow + 多 CLI 会话（路线图，归档）

> 目标：在现有 AISWS-ACA（简称 ACA；任务/终端/规则/审批/日志）基础上，引入“多目标服务器管理 + 统一终端服务 + 工作流（workflow）”，用于复杂项目开发与自动化运维。
>
> 说明：当前运行环境网络受限，本文件中的“其他实现途径/参考项目”来自常见业界方案与本仓库已有参考工程（如 `CodeKanban/`、`vibe-kanban/`），并非本次在线检索结果。

---

## 1. 新增能力（产品层）

### 1.1 多服务器接入（SSH Targets）
- 在 UI 里维护“目标服务器（Target）”清单：地址、端口、用户名、认证方式（密码/私钥/Agent）、标签、默认工作目录。
- 支持对任意 Target 创建“远程终端会话”，并复用现有终端面板/日志/审批/规则能力。

### 1.2 自动化运维（Runbook / Ops）
- “一次性任务”：对某个 Target 执行命令、上传脚本、收集输出、写入日志与审计。
- “可复用运行手册（Runbook）”：参数化脚本（如：部署、回滚、巡检）。
- “定时任务”：基于 cron/interval 触发 runbook，带告警/消息。

### 1.3 以任务为单点，形成 Workflow
- 单个 Task 可升级为 Workflow：一组步骤（Steps）按依赖关系执行（DAG），每一步可绑定本地终端或某个远程 Target。
- Workflow 能沉淀复杂项目开发流程（拉取代码→安装依赖→跑测试→启动 AI CLI→收集结果→审批→发布）。

### 1.4 多 CLI 会话管理（AI/工具/脚本）
- 支持多个 CLI 类型：Claude/Codex/Gemini/自定义命令。
- 会话元数据统一：会话类型、运行目录、resume 信息、检测状态（working/waiting_input/waiting_approval/idle）。
- 让“同一个任务/工作流”能串起多个会话（例如：一个终端跑编译，一个终端跑 AI CLI，一个终端跑部署）。

---

## 2. 架构原则：先统一公共系统，再并行开发

为保证后续模块能并行推进，先把以下“公共底座”稳定下来：

### 2.1 统一账户与权限（Auth/RBAC）
- 目标：所有资源（Tasks/Terminals/Targets/Workflows/Secrets）都用同一套账号体系鉴权与审计。
- MVP：保留单用户模式，但把密码与策略落到 DB（避免重启丢失），并为后续多用户预留 RBAC。

### 2.2 统一终端服务（Terminal Service）
- 抽象一个“终端驱动层”，把本地 tmux/PTY 与远程 SSH 统一到同一套生命周期/日志/事件流：
  - Driver = `local_tmux` / `local_pty` / `ssh`.
  - Session = 统一 WebSocket 协议（data/metadata/exit/approval/message）。

### 2.3 统一 API 边界（API Contract）
- 后端对前端暴露稳定协议（REST + WS），所有扩展（SSH、Workflow、Ops）都走同一套 token 验证与审计。
- 明确“事件模型”：TerminalEvent、WorkflowEvent、MessageEvent，统一落库与推送。

### 2.4 统一 Secrets（凭据/密钥）
- SSH 私钥、密码、API Key 等统一走 Secrets 模块，避免散落在表字段或配置文件里。
- 目标：加密存储（至少 AES-GCM），并支持“主密钥轮换”（env 提供 master key）。

### 2.5 开发环境一致性（DevX）
- Node/TypeScript 版本需要锁定：前端 `vue-tsc` 对 TS/Node 版本较敏感，建议在 Sprint 0 固化可运行组合（例如固定 TS 版本或升级 vue-tsc 生态）。
- Go 构建缓存路径建议可配置：在受限环境中可通过 `GOCACHE` 指向工作区目录，避免权限问题。

---

## 3. 技术栈建议（落地导向）

### 3.1 后端（Go/Fiber）
- 框架：沿用 Fiber + GORM + SQLite（现状）。
- SSH 客户端：`golang.org/x/crypto/ssh`（交互式 shell / exec）、SFTP：`github.com/pkg/sftp`（上传下载）。
- 任务/工作流执行器：
  - MVP：内置 worker（goroutine + DB 轮询/锁），避免引入 Redis。
  - P1：若需要更强调度与重试，可选 `asynq`（Redis）或引入 Temporal（成本更高）。
- 安全：统一从 JWT 获取 userID/username；WS 必须验证 token（与 HTTP 同规则）。

### 3.2 前端（Vue 3）
- 终端：沿用 xterm.js；新增“Target 选择器”“Workflow 画布/步骤列表”“Runbook 执行面板”。
- Workflow 可视化：
  - MVP：列表式 Step 编辑器（避免引入复杂 DAG 依赖）。
  - P1：引入图形化库（如 vue-flow/dagre 等）再升级为 DAG 画布。

### 3.3 API/WS 草案（先约束后开发）

#### REST（建议新增）
- Targets
  - `GET /api/targets`
  - `POST /api/targets`
  - `GET /api/targets/:id`
  - `PUT /api/targets/:id`
  - `DELETE /api/targets/:id`
  - `POST /api/targets/:id/test`（连通性/认证测试）
- Secrets（最小化：仅创建/更新/删除，不返回明文）
  - `GET /api/secrets`（仅返回 meta/用途，不返回 ciphertext 明文）
  - `POST /api/secrets`
  - `PUT /api/secrets/:id`
  - `DELETE /api/secrets/:id`
- Workflows
  - `GET /api/workflows`
  - `POST /api/workflows`
  - `GET /api/workflows/:id`
  - `PUT /api/workflows/:id`
  - `DELETE /api/workflows/:id`
  - `POST /api/workflows/:id/run`
  - `GET /api/workflow-runs/:id`（含 steps 状态）
- Runbooks/Schedules（Sprint 3）
  - `GET /api/runbooks` / `POST /api/runbooks` / …
  - `GET /api/schedules` / `POST /api/schedules` / …

#### WebSocket（统一事件流）
- 终端 WS：仍为 `/api/terminal/ws?sessionId=...&token=...`
- 服务端事件：`ready` / `data` / `metadata` / `exit` / `approval` / `message`
- 关键字段建议稳定：
  - `approval`: `approval_result: { action, input, reasoning, confidence, rule_matched, auto_handled }`
  - `message`: `message: { id, type, title, content, status, priority, created_at }`

---

## 4. 数据模型草案（最小必要字段）

### 4.1 Targets（远程目标）
- `targets`：
  - `id`, `name`, `host`, `port`, `username`, `auth_type(password|private_key|agent)`, `secret_id`, `tags`, `default_work_dir`, `created_at`, `updated_at`

### 4.2 Secrets（统一密钥）
- `secrets`：
  - `id`, `type(ssh_password|ssh_private_key|api_key)`, `ciphertext`, `meta(json)`, `created_at`, `updated_at`

### 4.3 TerminalSession 扩展（统一驱动）
- `terminal_sessions` 增加：
  - `driver`（local/ssh）
  - `target_id`（可空）
  - `remote_work_dir`（可空）
  - `auth_context`（可选：指向 secret 或 agent）

### 4.4 Workflow
- `workflows`: `id`, `name`, `task_id?`, `definition(json)`, `created_at`, `updated_at`
- `workflow_runs`: `id`, `workflow_id`, `status`, `started_at`, `ended_at`, `created_by`
- `workflow_step_runs`: `id`, `run_id`, `step_key`, `status`, `started_at`, `ended_at`, `logs_ref`

---

## 5. Sprint 路线图（先底座后并行）

### Sprint 0（P0）：平台加固（并行前置条件）
| ID | 模块 | 目标 | 依赖 | 可并行 |
|---|---|---|---|---|
| S0-1 | Auth | 密码持久化（DB 存储 hash），为 RBAC 预留字段 | - | ✅ |
| S0-2 | Terminal/WS | WebSocket token 校验 + 统一 WS 事件协议（含 approval/message） | - | ✅ |
| S0-3 | Secrets | 新增 secrets 表 + AES-GCM 加密 + master key 环境变量 | S0-1 | ✅ |
| S0-4 | API Contract | 定义并固化 REST/WS schema（版本化/向后兼容策略） | S0-2 | ✅ |

实施细节（建议路径）：
- S0-1（Auth 持久化）
  - 登录：从 DB 读取 user 与 `password_hash` 校验（bcrypt），不再用内存配置做最终判定；保留 env 作为“首次初始化/重置”入口。
  - 改密：仅更新 DB（并让现有 token 在过期后自然失效）；可选提供“强制登出全部会话”开关（通过变更 `JWT_SECRET` 或维护 token version）。
  - 为 RBAC 预留：在 user 上预留 `role` 字段或单独 `user_roles` 表（先不启用）。
- S0-2（WS 鉴权 + 协议闭环）
  - 把 `/api/terminal/ws` 纳入 token 校验（query token/Authorization 均可），避免“绕过认证直连终端”。
  - 扩展 WSMessage：序列化 `approval` 与 `message` 事件；前端 `Terminal.vue` 增加对应 `case`，触发弹窗/通知或写入 store。
- S0-3（Secrets）
  - master key 由 env 提供（例如 `ACA_MASTER_KEY`），使用 AES-GCM 加密明文后入库，`ciphertext` 存储 base64。
  - 元信息（meta）存储用途/指纹（如 ssh public key fingerprint），以便 UI 展示与审计。
  - key rotation：允许同时接受旧 key 解密（短期），保存后用新 key 重新加密（中期）。
- S0-4（API Contract）
  - 把新模块（targets/secrets/workflows）的请求/响应类型在前端 `src/api` 里集中定义并版本化。
  - 对 WS 事件做向后兼容：新增字段只增不改，旧客户端忽略未知字段。

### Sprint 1（P0）：SSH Terminal MVP（远程终端纳入统一终端服务）
| ID | 模块 | 目标 | 技术栈 | 依赖 | 可并行 |
|---|---|---|---|---|---|
| S1-1 | Targets | Target CRUD（列表/新增/测试连接） | Fiber+GORM | S0-3 | ✅ |
| S1-2 | Terminal Driver | 新增 `ssh` driver：PTY + WindowChange + stdout/stderr 流 | x/crypto/ssh | S0-2 | ✅ |
| S1-3 | UI | Target 管理页 + 终端创建时选择 Target | Vue+Naive UI | S1-1 | ✅ |
| S1-4 | Logs/Audit | 远程终端日志/审批记录与本地一致落库 | 复用现有 logs | S1-2 | ✅ |

实施细节（ssh driver 核心点）：
- `ssh.Client` 持久连接 + keepalive（定时发送 ignore 包）。
- `session.RequestPty("xterm-256color", rows, cols, modes)` 后 `session.Shell()`。
- 读写：`stdinPipe`/`stdoutPipe`/`stderrPipe` 合并为统一 stream（与现有 scrollback/log 同步）。
- resize：前端 `resize` → 后端 `session.WindowChange(rows, cols)`。
- S1-1（Targets）
  - 认证方式：先支持 password/private_key；Agent 放到 P1。
  - 连接测试：`ssh.Dial` 成功即通过；可选读取 `uname -a` 作为健康信息。
  - HostKey 校验：MVP 可先支持 TOFU（首次连接记录指纹），后续再做严格校验/替换策略。
- S1-3（UI）
  - Settings 增加 Targets Tab：列表/新增/编辑/删除/测试连接。
  - 创建终端：增加“本地/远程”选择；远程需选择 target 与初始目录（cd）。
- S1-4（Logs/Audit）
  - `terminal_sessions` 落库带上 `driver/target_id`；日志/审批记录沿用现有表，保证按 terminal_id 过滤即可回放。
  - 针对运维场景：在 `logs` 的 `task_id` 为空时，用 `system`/`workflow_run_id`（后续）来归档。

### Sprint 2（P0）：Workflow MVP（先列表式步骤，后 DAG）
| ID | 模块 | 目标 | 技术栈 | 依赖 | 可并行 |
|---|---|---|---|---|---|
| S2-1 | Workflow Model | workflow/workflow_run 表与基础 API | Fiber+GORM | S0-1 | ✅ |
| S2-2 | Runner | Step 执行器（串行版）：ssh_exec / local_exec / terminal_attach | Go worker | S1-2 | ✅ |
| S2-3 | UI | Workflow 编辑器（列表 steps）+ 运行视图（run 状态/日志） | Vue | S2-1 | ✅ |
| S2-4 | Approval Bridge | workflow step 可触发“需要审批”并阻塞/继续 | 复用 approval engine | S0-2 | ✅ |

实施细节（runner 设计建议）：
- Step 类型最小集：`ssh_exec`（非交互），`terminal_session`（创建/附着交互终端），`wait_approval`（等待消息处理）。
- Runner 状态机：`queued → running → blocked → success/failed/canceled`。
- 输出归档：将 step output 作为 `logs` 的 `log_type=system` 或新建 `workflow_logs` 表。
- S2-1（Workflow 定义）
  - MVP 用 JSON 存储 `definition`，包含 steps（key/name/type/target_id/command/env/depends_on/timeout）。
  - 校验：服务端保存时校验 step key 唯一、依赖存在且无环（MVP 可先限制“仅串行”避免 DAG 复杂度）。
- S2-3（UI）
  - 列表式编辑器：增删 steps、设置 target、设置命令/超时/重试策略。
  - 运行视图：展示每个 step 的状态、输出片段、跳转到相关 terminal/logs。
- S2-4（Approval Bridge）
  - step 可声明 `requires_approval=true`：当检测到 approval 事件时，把 workflow_run 置为 blocked，并在 UI 给出“允许/拒绝/输入”操作。
  - 操作写入审批记录与消息中心，保证审计一致。

### Sprint 3（P1）：自动化运维（Runbook + Schedule）
| ID | 模块 | 目标 | 技术栈 | 依赖 | 可并行 |
|---|---|---|---|---|---|
| S3-1 | Runbook | runbook CRUD（模板参数、目标标签选择） | Go+Vue | S2-1 | ✅ |
| S3-2 | Scheduler | cron/interval 调度 + 重试/超时/并发限制 | robfig/cron 或自研 | S3-1 | ✅ |
| S3-3 | Notifications | 失败/阻塞推送到消息中心 + 未读统计 | 复用 messages | S0-2 | ✅ |

实施细节（建议路径）：
- Runbook 本质可以是 Workflow Template：参数化注入到 steps（比如主机列表、版本号、环境）。
- Scheduler MVP：
  - 使用 cron 表达式计算 next_run_at；
  - worker 拉取到期任务执行，写入 workflow_run；
  - 并发限制按 target 标签/组控制（避免同一台机器并发部署）。

### Sprint 4（P1）：多 CLI 会话与复杂项目开发（Project/Workspace）
| ID | 模块 | 目标 | 技术栈 | 依赖 | 可并行 |
|---|---|---|---|---|---|
| S4-1 | Project | Project/Workspace 概念（本地/远程 repo/workdir） | Go+Vue | S1-1 | ✅ |
| S4-2 | Multi-CLI | CLI profiles（claude/codex/gemini/custom）+ 会话恢复 | 复用 detector + 扩展 model | S0-2 | ✅ |
| S4-3 | Workflow Templates | “复杂项目开发”模板（clone→install→test→AI→PR） | Workflow JSON | S2-1 | ✅ |

实施细节（建议路径）：
- Project/Workspace：
  - 统一描述“在哪做事”：local_path / target_id + remote_path / git_repo / env；
  - Task/Workflow/Terminal 都可挂在 Project 下，便于复用与统计。
- Multi-CLI：
  - 抽象 `cli_profiles`：`id/name/type/command/default_args/resume_strategy/detect_patterns`；
  - 将现有 `task.cli_type` 迁移为引用 profile（兼容旧字段一段时间）。
- 模板库：
  - 内置“Go/Vue/Node/DevOps”常见模板（拉取→安装→测试→运行→AI→提交 PR），减少重复配置。

---

## 6. 其他实现途径（可参考的方向）

### Web Terminal / SSH 方向
- gotty / ttyd / wetty：典型的“终端转 WebSocket”思路（可参考协议、流控、resize）。
- Teleport / bastion：更重的“统一入口 + 审计”产品形态（可参考权限/审计/会话回放）。

### Workflow / Ops 方向
- Rundeck / AWX(Ansible) / Jenkins：runbook/任务编排/审计的成熟形态（可借鉴 UI/权限/执行模型）。
- n8n：低代码 workflow（可借鉴节点编辑器交互）。
- Temporal：企业级 workflow 引擎（若未来要强一致/重试/补偿，可评估引入成本）。

### 建议检索关键词（给后续调研用）
- Web terminal: `web terminal ssh websocket resize xtermjs`
- Go SSH: `golang ssh RequestPty WindowChange keepalive hostkey tofu`
- Workflow: `workflow engine dag executor retry compensation temporal rundeck`
- 运维平台: `runbook scheduler audit bastion teleport awx`

---

## 7. 并行开发分工建议（按角色）

### 平台底座（先做）
- 后端 A：S0-2（WS 鉴权/协议）+ S0-4（API schema）
- 后端 B：S0-1（Auth 持久化）+ S0-3（Secrets）

### SSH 与 Workflow（底座完成后并行）
- 后端 A：S1-2（ssh driver）+ S2-2（runner）
- 后端 B：S1-1（targets）+ S2-1（workflow model）
- 前端 A：S1-3（targets UI）+ S2-3（workflow UI）
- 前端 B：S2-4（审批桥接 UI/消息中心联动）
