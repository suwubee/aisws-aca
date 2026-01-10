# ACA 功能全貌与PRD对比分析报告

日期：2026-01-10  
范围：`ai-coding-assistant`（前端 Vue3 + 后端 Go）

> 说明：本报告基于当前代码与 `docs/PRD.md` 进行对照整理，并给出“智能运维/Runbook”视角的节点设计建议与 Sprint 落地清单。

---

## 1. 当前产品功能全貌（按模块）

### 1.1 身份认证与权限
- JWT 登录/鉴权（API 统一鉴权）
- 管理员/普通用户角色区分（管理员可进行系统级配置与数据重置）

### 1.2 终端托管（PTY / tmux / WebSocket）
- 创建本地终端会话：默认以 tmux 会话承载（便于断线重连与 send-keys）
- WebSocket 终端流：前端 xterm.js 连接后端会话流，支持输入/resize/元数据
- Scrollback 缓冲：用于状态检测与审批/监控上下文
- 终端日志：输入/输出日志与查询、清理
- SSH 终端：可在前端打开 SSH 终端窗口，进行远程交互

### 1.3 AI 代理集成与状态检测
- AI Agent Detector：识别 Claude Code / Codex / Gemini CLI 的状态（working / waiting_input / waiting_approval 等）
- 代理监控与统计：集中展示“终端-代理状态”与相关信息

### 1.4 AI 审核（自动化审批）
- 审批提示检测：从终端输出识别 “需要确认/权限请求/选择提示”等
- 审批引擎（规则 + AI）：支持 system/terminal/task/custom 规则模式（黑白名单 + AI 辅助）
- 审批记录：落库保存（可在前端查看记录/AI 决策原因）
- 手动处理入口：审批中心与终端快捷键

### 1.5 AI 托管（任务级托管）
- 任务模型支持：`ai_managed / ai_prompt / ai_end_condition / ai_error_handling`
- 启动自动化任务：自动创建终端、进入工作目录、启动 CLI、等待 CLI 就绪后发送提示
- 任务监控：后台监控任务完成/异常/超时，必要时暂停并提示用户介入

### 1.6 任务与看板（Kanban）
- 任务 CRUD + 状态流转（含扩展状态：paused/failed/timeout/archived）
- 任务-终端关联：同一任务可关联多个终端
- 看板拖拽：桌面端支持拖拽；移动端采用 Tab + Card 的方式呈现

### 1.7 日志与消息
- 日志管理：按会话查看、分页、过滤
- 系统消息：审批阻止/提醒等通知，可统计未读

### 1.8 项目与项目集（Portfolio）
- 项目（Project）/项目集（Project Group）管理：用于任务/工作流分类与默认上下文
- 项目可绑定服务器、路径、Git 仓库/分支等（为 Runbook 节点提供“上下文”基础）

### 1.9 服务器与分组
- SSH 服务器配置、连通性测试、分组管理
- 批量执行入口（多服务器执行同类操作的雏形）

### 1.10 工作流（Workflow）
- 工作流列表/模板/可视化编辑（Vue Flow）
- AI 工作流（对话驱动）
- 节点类型已预留：`server/task/terminal/git/ops_step/ai_agent/condition/parallel/wait`

### 1.11 系统设置（可运营化配置）
- Prompt Templates（可编辑、可套用方案、可恢复默认）：用于 AI 审核/AI 托管/任务监控/AI 工作流等多模块接入
- Key Bindings（全局按键绑定）：Enter（CR）与 Newline（LF）等区分；支持 tmux keys 与 PTY 输入
- 计划任务（Cron/一次性）：可定时运行任务或 AI 工作流
- Reset Data：管理员可重置数据（不需要兼容旧值；后端已有相关测试覆盖）

---

## 2. AI 审核与 AI 托管流程梳理（端到端）

### 2.1 AI 审核（审批）流程
1) **检测触发**：终端输出进入 detector，识别为 `waiting_approval` 后触发审批评估  
2) **规则评估**：黑/白名单匹配（危险操作阻止；安全模式可放行）  
3) **AI 辅助（可选）**：按系统提示词模板（`approval.system_prompt`）+ 规则注入变量进行判断  
4) **输出决策**：`approve/reject/wait/input` + 输入内容  
5) **执行输入**：自动通过时优先 `tmux send-keys`，否则 PTY 写入；手动处理在审批中心可发送  
6) **记录审计**：审批记录/AI 决策理由落库，前端可回看

关键体验点：
- **Enter（CR）与 Newline（LF）必须区分**：Claude Code 的信任/选择提示属于“Enter to confirm”，必须发送 Enter（CR）。  
- **按键统一封装**：应以系统 Key Bindings 的 action（enter/esc/ctrl_c/1/2…）作为“全局一套”协议，避免前端输入框回车=换行的歧义。

### 2.2 AI 托管（任务级托管）流程
1) **创建任务**：配置工作目录/CLI 类型/初始目标；可开启 `AIManaged`  
2) **创建终端**：任务启动时创建终端并关联任务  
3) **进入工作目录 + 启动 CLI**：写入 `cd ... && claude/codex/gemini`，并做“命令不存在”快速拦截  
4) **就绪检测**：通过 detector/元数据检测进入 `waiting_input/working`，避免把提示词当 shell 命令执行  
5) **发送托管提示**：通过模板 `task.managed_prompt` 渲染（含动态变量）并发送  
6) **后台监控**：周期性分析输出，判断 `complete/alert/retry/timeout`，并根据 `ai_error_handling` 决定暂停/失败/重试

关键体验点：
- **CLI 启动命令提示**：Claude Code 常见入口为 `claude` 或 `npx claude`；当环境不确定时必须“先向用户确认”，而不是盲目执行错误命令。  
- **托管提示词/监控提示词模板化**：禁止硬编码，所有模块统一走 Prompt Templates + 变量注入。

---

## 3. PRD.md 对比：已实现 / 超越实现 / 待补齐

### 3.1 PRD P0（核心）实现情况
- 用户认证（P0）：已实现
- 终端托管（P0）：已实现（WebSocket + tmux + scrollback），并补强了重连与日志
- AI 代理检测（P0）：已实现 Claude Code，并对 Codex/Gemini 具备检测与配置基础
- 自动化审批（P0）：已实现（检测 + 手动审批 + 自动化执行路径）
- Kanban（P0）：已实现（含移动端卡片化展示）
- 终端输出日志（P0）：已实现（可分页/过滤/清理）

### 3.2 PRD P1/P2（增强）实现情况
- 终端元数据（P1）：已实现并用于“就绪检测/审批状态”
- 审批记录（P1）：已实现（可回看 AI 决策）
- AI 自动决策接口（P1）：已实现 AI Provider 配置 + 审批提示词模板
- 任务评论（P2）：已实现（见 TaskDetail/评论相关 API/UI）
- 日志导出（P2）：已实现（设置中含导出）

### 3.3 超越 PRD 的实现（可视为商业化增强）
- Prompt Templates + Presets：多模块提示词可配置、可套用方案（利于“团队化/行业化交付”）
- Key Bindings：将 Enter/Esc/Ctrl+C 等抽象为系统配置（利于跨终端/跨 CLI 一致体验）
- 计划任务（Cron）：定时执行任务/AI 工作流（向 Runbook/运维自动化迈进）
- 项目/项目集：将任务与工作流纳入项目编排（更贴近真实业务组织）
- 工作流模板 + 可视化编辑：具备 Runbook 雏形
- SSH 服务器分组 + 批量执行入口：具备“多服务器协同”雏形

### 3.4 暂未闭环或需要深化的点
- 工作流节点语义需升级（见第 5 章）：尤其是“Git 节点/在哪台服务器执行”的上下文归属与可视化编排
- 多服务器 fan-out/fan-in、结果聚合、条件分支的工程化能力需补齐
- 面向运维的“凭证/权限/审计”体系仍需强化（RBAC、敏感操作二次确认、审计报表）

---

## 4. 商业验证与潜在价值（从智能运维视角）

### 4.1 可验证的核心价值假设
1) **多终端/多代理统一工作台**显著减少“上下文切换成本”  
2) **审批自动化**减少“机械确认”，并通过审计记录提升团队信任  
3) **任务/项目/工作流**让 AI 从“单点对话工具”升级为“可复用的组织流程资产（Runbook）”

### 4.2 MVP 商业验证建议（可量化指标）
- 时间节省：一次任务从创建到完成耗时、人工确认次数下降比例
- 审批效率：审批触发次数、自动通过比例、误拦截/漏拦截比例
- 复用度：模板/工作流复用次数、项目内任务闭环率
- 运维价值：计划任务触发成功率、告警/异常响应时间

---

## 5. 竞品与差异化（基于常见产品形态归类）

> 说明：竞品按“能力形态”归类，便于定位差异化与产品路线。

### 5.1 Runbook / 自动化编排类
- Rundeck / StackStorm / Jenkins / Ansible AWX：强在 Runbook、权限、节点生态与审计；弱在 AI Coding CLI 的原生交互与“审批语义（Enter/选择提示）”
- Argo Workflows / Airflow / Prefect：强在数据流/任务调度；弱在交互式终端与权限提示的细粒度处理

### 5.2 AIOps / 观测类
- Datadog / New Relic / Dynatrace / PagerDuty AIOps：强在指标/日志/告警与自动化响应；弱在“开发侧 AI 代理 + 终端交互”的闭环

### 5.3 AI 工具链与 Prompt 管理
- Langfuse / LangSmith / PromptLayer：强在 prompt/version/评测与 LLM 可观测；弱在终端、SSH、多服务器 Runbook 执行

### 5.4 ACA 的潜在差异化
- **把“交互式 AI Coding CLI”当作一等公民**：统一托管、状态检测、审批、审计、快捷键
- **把“运维 Runbook”当作下一阶段**：以服务器/项目/Git/任务为上下文，支持定时与多服务器协作
- **把“提示词与按键协议”产品化**：Prompt Templates + Key Bindings 为团队落地与交付提供标准化入口

---

## 6. 工作流节点设计优化建议（智能运维/Runbook）

### 6.1 核心原则：上下文必须显式
“Git 操作在哪台服务器执行？”不是节点本身的问题，而是 **缺少执行上下文**：  
- **Server**：执行环境（本地/SSH server）、凭证、shell、tmux、工作目录根  
- **Repo**：git_url、branch、workspace_path（可来自 Project）  
- **Task/Command**：具体动作与参数、超时、输出采集  

建议将“服务器/项目/Git/执行动作”组合为更贴近业务的 **OpsStep（复合节点）**：
- `ops_step` 节点配置：
  - `server_id`（或 “local”）
  - `project_id`（可选，自动补齐 git_repo/路径/分支）
  - `workspace_path`（可选，覆盖 project 默认）
  - `git`: { `url`, `branch`, `depth`, `strategy`(pull/clone), `clean` }
  - `run`: { `command` | `script`, `env`, `timeout`, `cwd` }
  - `approval`: { `mode`, `ruleset_id`, `requires_manual` }
  - `outputs`: { `capture_stdout`, `capture_stderr`, `parse`(regex/json) }
  - `on_result`: 条件分支/重试/通知

### 6.2 多服务器协同（fan-out/fan-in）
建议引入两类节点：
- `parallel`：对 server 列表 fan-out（并发、最大并行度、失败策略）
- `condition`：聚合后判断（任一失败/全部成功/阈值）进入下一步

### 6.3 “AI 自行开启终端并执行操作”的边界
建议分层：
- L0：AI 提建议（仅输出计划/命令，不执行）
- L1：AI 执行但需审批（默认）
- L2：白名单自动执行（规则 + 审计）
- L3：全自动托管（需要更严格的 RBAC、审计、回滚策略）

现阶段建议以 L1/L2 为主，避免“自动化过强导致误操作”影响商业信任。

---

## 7. Sprint 落地清单（建议）

### Sprint 0：稳定性与体验（已在近期迭代中覆盖/进行中）
- 移动端卡片化：任务/服务器/终端/工作流/项目等列表适配
- 统一 isMobile 判定：避免 mounted 后切换视图导致渲染异常
- 审批按键协议化：Enter/Esc/Ctrl+C/选项键支持，避免“回车=换行”误操作
- Reset Data 回归：后端测试覆盖并通过

### Sprint 1：Runbook 节点语义升级（核心）
- 引入 `ops_step` 节点并在 Workflow Editor 中提供可视化配置
- Project 作为默认上下文：一键带出 server/git/path
- Git 节点语义收敛：要么并入 ops_step，要么强制依赖 server/project 上下文

### Sprint 2：多服务器协同与调度闭环
- 工作流 `parallel + condition`：fan-out、聚合判断、失败策略
- 计划任务支持选择 workflow + 参数化（server/project/变量注入）
- 运行记录/审计报表（面向运维）

### Sprint 3：商业化能力
- RBAC（项目维度权限）、审计导出、敏感操作二次确认
- 模板市场/团队 preset：Prompt/Workflow/KeyBinding 的可交付体系

---

## 8. 暂时不必要/建议后置的功能（避免分散）
- 过度复杂的 AI 工作流节点类型（在 ops_step 未稳定前先收敛语义）
- 大而全的观测集成（先把“SSH/终端/审批/Runbook”闭环打通，再对接外部 APM）

---

## 9. 结论
- 现有实现已覆盖 PRD 核心，并在 Prompt/KeyBinding/计划任务/项目化/工作流模板等方面明显“超越 PRD”，具备走向 Runbook/智能运维的基础。  
- 下一阶段的关键不是继续堆节点类型，而是把“执行上下文（server/project/git）”收敛为可复用的 ops_step，并完善多服务器协同与审计闭环。  

