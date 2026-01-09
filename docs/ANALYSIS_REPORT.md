# ACA 现状分析与路线建议（2026-01-09）

本文基于当前仓库代码与界面（`frontend/src/router/index.ts`、`backend/main.go`、`docs/PRD.md` 等）整理：功能全貌、AI 审核与 AI 托管流程、PRD 实现对比、竞品与商业验证假设，并给出下一阶段 Sprint 清单。

---

## 1. 当前功能全貌（以代码为准）

### 1.1 前端主要页面

路由见 `frontend/src/router/index.ts`：

- 工作台：`/`（`Dashboard`）
- 任务：`/kanban`（Kanban）、`/tasks`（列表）、`/task/:id`（详情）
- 终端：`/terminals`（终端会话管理）、工作台内终端面板
- AI 智能：`/ai-intelligence`（代理监控/统计/审批记录/思考日志等聚合）
- 日志：`/logs`（日志管理）
- 服务器：`/servers`（SSH 服务器与分组）
- 工作流：`/workflows`（工作流与模板/运行记录等）
- 系统设置：`/settings`（系统规则、Agent Config、AI Provider、提示词模板、导入导出、日志导出、消息中心、审批记录等）

### 1.2 后端核心能力

入口见 `backend/main.go`：

- 认证与权限：JWT + 角色（admin/user/viewer）
- PTY 终端托管：会话创建、WebSocket 双向通信、日志落库、断线重连
- 任务管理：Kanban 状态流转、任务详情、与终端关联
- AI 代理检测：Claude Code / Codex / Gemini CLI 等状态识别（工作中/等待输入/等待审批）
- 自动化审批：规则（白/黑名单、模式） + AI Provider 辅助决策 + 记录/消息通知
- AI Provider 配置：模型、Base URL、Key、温度等
- 任务自动化/托管：启动 CLI、日志监控、需要用户介入的提示、结束条件/完成标记
- 工作流：工作流定义、模板、运行、AI workflow session
- 运维/管理：SSH 服务器/分组、Secret 管理、日志导出、规则集导入导出
- 系统级提示词模板：各模块提示词不再硬编码，统一在“提示词模板”中配置（见第 2/3 节）

---

## 2. AI 审核（自动化审批）流程梳理

### 2.1 触发入口

- 终端输出通过 WebSocket 流入前端，同时后端落库（`logs` 等）。
- 审批引擎 `backend/service/approval/engine.go` 对输出做检测：
  - `detector.IsApprovalPrompt(output)` 判定是否是“需要确认/需要输入”的提示。

### 2.2 规则集与模式选择

审批配置来源按“终端规则模式”选择（`Engine.GetAutomationConfig`）：

- `terminal.rule_mode = system`：读取系统规则集（`rule_sets.type = system`）
- `terminal.rule_mode = task`：读取任务绑定规则集
- `terminal.rule_mode = custom`：读取终端自定义规则集
- `none/未知`：回退为手动审批默认值

模式（`ApprovalMode`）核心分三类：

- `manual`：只检测并通知，等待人工处理
- `auto_yes`：根据输入类型自动回应（yes/y/enter/option1 等）
- `smart`：在规则基础上调用 AI Provider 做决策（见 2.3）

### 2.3 AI 决策（smart 模式）

- AI 决策入口：`backend/service/ai/provider.go` 的 `AnalyzeForApproval`
- 系统提示词由模板渲染：`prompt.RenderTemplate("approval.system_prompt", { extra_rules })`
  - `extra_rules` 通常来自规则集的 `ai_prompt`（可作为“额外高优先级规则”注入）
- 用户消息通常为终端输出（或上下文日志片段），AI 返回 JSON（action/input/confidence/reasoning）
- 后端解析并归一化动作，输出 `ApprovalResult`

### 2.4 执行与记录

- 自动执行：根据 `ApprovalResult` 将输入发送到终端（tmux send-keys/PTY 输入）
- 记录：`approval_records` 存储每次审批判断与执行结果
- 通知：`messages` 用于前端提醒（例如需要人工介入、被阻止等）

### 2.5 体验风险点（当前最影响闭环的）

- **终端输入可靠性**：输入无法写入/焦点丢失/光标错位会直接破坏“自动审批”闭环（即便 AI 判断正确也无法执行）。
- **规则可解释性**：smart 模式需要清晰展示“命中规则/AI 决策原因/执行了什么输入”，否则用户不敢开自动化。
- **移动端可用性**：审批弹窗/消息中心/日志列表在手机上若不可用，会导致用户只能回到电脑处理，影响“随时随地托管/审阅”价值。

---

## 3. AI 托管（AI Managed Task）流程梳理

### 3.1 创建任务（托管参数）

任务模型在 `backend/model/db.go` 的 `Task` 结构体中包含自动化/托管字段：

- CLI：`cli_type`（claude/codex/gemini）
- 工作目录：`work_dir`（可自动创建）
- 目标描述：`initial_prompt`
- 托管开关：`ai_managed`
- 托管提示词：`ai_prompt`
- 结束条件：`ai_end_condition`
- 错误处理策略：`ai_error_handling`

### 3.2 启动与进入 CLI

`backend/service/task/automation.go` 会在 `StartTask` 里：

- 创建/准备工作目录
- 创建并关联终端会话
- 尝试启动对应 CLI
- 如果 CLI 环境未就绪，会返回 `needs_user_action=true` 并给出提示（例如 `claude` / `npx claude`）

### 3.3 托管提示词渲染（关键：不硬编码）

托管提示词由模板系统渲染（`buildManagedPrompt`）：

- 模板 key：`task.managed_prompt`
- 变量：`task_initial_prompt` / `task_ai_prompt` / `task_ai_end_condition` / `task_done_marker`
- 目标：把“任务目标 + 执行规则 + 完成条件 + 完成标记”结构化，便于 CLI 代理稳定执行与收敛

### 3.4 监控与状态判断

任务监控使用 AI + 启发式兜底组合：

- AI 监控系统提示词：`task_monitor.system_prompt`（变量：log_limit/max_log_chars）
- AI 不可用/解析失败时回退 heuristic，保证系统稳定

### 3.5 体验风险点

- **结束条件可验证性**：建议强制“可机读”的 Done Marker（目前默认 `ACA_TASK_DONE`）来降低误判。
- **托管中断与续跑**：需要明确“暂停点（等待输入/等待审批/失败）”与“恢复方式（继续/重试/回滚）”。
- **多任务并行时的注意力管理**：移动端/桌面端都需要清晰的“谁在等我/我该做什么”提示与快捷入口。

---

## 4. PRD.md 实现对比（已实现 / 超越实现 / 待补齐）

对照 `docs/PRD.md`：

### 4.1 PRD 核心功能实现情况（摘要）

| 模块 | PRD 重点 | 当前状态 | 备注 |
|---|---|---|---|
| 用户认证 | JWT、鉴权、权限 | 已实现 | 角色权限已引入（admin/user/viewer） |
| 终端托管 | PTY、WS、scrollback、并行 | 已实现 | 终端重连/日志等能力在界面可见 |
| AI 代理集成 | Claude/Codex/Gemini 检测 | 已实现 | 代理配置（启用/优先级/检测模式）已做成设置项 |
| 自动化审批 | 检测 + 手动审批 + 记录 +（预留 AI） | 已实现且增强 | 已有 AI Provider + 规则集 + 记录/消息中心 |
| Kanban 任务 | CRUD、流转、拖拽 | 已实现 | 同时提供任务列表页与详情页 |
| 日志记录 | 终端输出日志、检索、导出 | 已实现且增强 | 已有日志管理、日志导出（JSON/CSV） |

### 4.2 明显“超越 PRD”的功能

PRD 未明确要求但仓库已具备（或已落在设置中）：

- AI Provider 完整配置与默认选择（不仅“预留接口”）
- 规则集体系（system/task/custom），并能导入导出
- SSH 服务器管理与分组（远程项目/远程托管基础能力）
- Secret 管理（为后续接入 Provider/API Key 打基础）
- 工作流（模板、运行、节点等），以及 AI workflow session
- 系统级提示词模板（按模块拆分、支持变量、支持“方案”）
- 消息中心/未读计数/审批记录列表（注意力管理体系雏形）

### 4.3 需要补齐/体验未闭环的点（PRD + 现状差距）

- **移动端体验**：Kanban/长列表/弹窗/终端在手机端仍需系统性适配（见 Sprint 建议）。
- **终端输入稳定性**：光标错位、输入失效这类问题会显著影响托管/审批闭环。
- **“托管”闭环指标**：目前具备能力，但缺少对“成功率/卡点/平均介入次数/任务完成时间”的量化与可视化。

---

## 5. 商业验证与潜在价值（建议用数据验证）

### 5.1 价值主张（谁会付费，为什么）

ACA 的差异点是“让人类用更低成本管理多个 CLI 代码代理”：

- 多任务并行：把多个代理跑在多个终端里，并用任务/状态/消息中心串起来
- 审批自动化：减少 yes/no/enter 等重复确认成本，并可追溯
- 托管可控：用户可以在“需要介入点”接管，而不是全自动黑盒

适合的 ICP（优先从强需求人群验证）：

- 同时跑 2+ 个 Claude Code / Codex / Gemini CLI 的独立开发者
- 小团队里有“review/把控风险”的人（希望集中审阅与审批）
- 远程服务器上跑 agent，需要 Web UI 统一管理的用户

### 5.2 建议的商业验证指标

建议在后端/前端增加可量化埋点（不涉及隐私内容）：

- 每任务：完成率、平均耗时、人工介入次数、介入原因分布（等待输入/审批/失败）
- 每终端：审批提示次数、自动通过率、阻断率、误判率（用户撤销/纠正）
- 每用户：并行任务数、日活、复用规则/模板次数、模板方案切换次数

### 5.3 最小化实验（2~4 周内可做）

- 让 5~10 位重度 CLI 代理用户试用一周，记录：
  - “每天节省的确认次数/时间”
  - “最烦的卡点”（通常是终端输入稳定性 + 移动端不可用）
- 以“自动审批 + 托管”作为主卖点，观察是否能提高留存/付费意愿。

---

## 6. 竞品扫描（与 ACA 的差异）

> 以下基于公开 README/官网介绍做摘要（并非完整功能清单）。

### 6.1 Vibe Kanban（BloopAI）

来源：`https://github.com/BloopAI/vibe-kanban` README（摘要）：

- 定位：Orchestrate multiple coding agents（Claude Code/Gemini CLI/Codex/Amp 等）
- 能力点：多代理切换、并行/串行编排、快速 review、启动 dev server、集中配置 MCP、远程 SSH 打开项目

与 ACA 的关键差异机会：

- ACA 优势：Go 单体后端 + 自带 PTY 托管 + 自动化审批/规则体系更“贴近终端交互风险控制”
- 需要补齐：移动端体验、编排能力“可理解性”、以及对 MCP/多项目的统一配置能力

### 6.2 OpenHands

来源：`https://github.com/All-Hands-AI/OpenHands` README（摘要）：

- 定位：AI-driven development（SDK/CLI/Local GUI/Cloud/Enterprise 多形态）
- 强项：更完整的 agent 技术栈与产品形态（含多用户、RBAC、协作、集成等方向）

与 ACA 的关键差异机会：

- OpenHands 偏“通用软件工程 agent 平台”，门槛更高、形态更重
- ACA 可以更聚焦：“CLI 代码代理的可控托管与审阅”，做得更轻、更快、更适合个人/小团队自托管

### 6.3 IDE 内置型助手（Cursor / Copilot / JetBrains AI 等）

- 优势：编辑器内体验极强、上下文丰富、学习成本低
- 不足：对“多终端并行 + 远程服务器 + 审批自动化 + 任务编排”的覆盖有限

ACA 的定位建议：

- 不与 IDE 助手正面竞争补全“写代码体验”，而是做 **“多代理运行与治理层”**：
  - 编排、审批、追溯、指标、成本控制、远程运行

---

## 7. 哪些功能可深化 / 暂时闭环有问题 / 不必要（建议）

### 7.1 可深化（建议投入）

- 移动端信息架构：Kanban/长列表统一 Card 模式；详情页优先；减少横向滚动
- 终端可靠性：输入焦点、fit 时机、隐藏/显示导致的光标错位；自动化执行的回放与撤销
- 提示词模板体系：按模块拆分（已具备），加“方案版本/灰度/导入导出/一键回滚”
- 托管指标面板：每任务的卡点、成功率、平均介入次数（把“托管”做成可验证能力）

### 7.2 暂时闭环有问题（需要先修）

- 终端命令无法输入/显示错位：直接影响审批与托管闭环（优先级最高）
- 手机端“看得到但用不了”：例如数据表格、弹窗、终端，影响“随时审阅/介入”价值

### 7.3 可能不必要/可降级（视 ICP 决定）

- 复杂工作流（节点/运行）如果 ICP 主要是个人用户，可考虑先降级为“任务模板 + 批量启动/串行队列”
- 过早做多团队协作（除非已验证团队付费意愿）

---

## 8. 建议 Sprint 清单（下一阶段）

### Sprint A（1~2 周）：移动端可用性闭环

- 统一“列表类页面”移动端 Card 模式（任务/终端/服务器/日志/消息/审批记录）
- Kanban：移动端默认进入“分组卡片列表”，支持快速切换状态与搜索
- 详情页：信息优先级重排（状态/终端/审批/日志/操作）适配小屏

### Sprint B（1~2 周）：终端输入与显示稳定性

- 解决“输入失效/光标错位/高度变化导致 fit 错误”的主因
- 自动化输入前后：记录输入内容、回放、失败原因
- 增加端到端自检：在 UI 提供“终端自检”按钮（发送回车/打印光标位置/检测 focus）

### Sprint C（1~2 周）：托管闭环与可观测性

- 任务托管“状态机可视化”（运行中/等待输入/等待审批/完成/失败）
- 每任务“卡点摘要”与“一键定位到对应终端/日志/审批记录”
- 关键指标埋点与简单报表（完成率、介入次数、平均耗时）

### Sprint D（可选）：产品化与商业验证

- Onboarding：新用户引导（如何安装/进入 CLI、如何启用托管、如何配置 Provider）
- 模板/规则/Provider 的导入导出与分享
- 定价与试用：若面向团队，先做“审阅/审批/追溯”卖点验证

