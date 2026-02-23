# AISWS-ACA（AI超站）功能与路线图（Features & Roadmap）

> 说明：本文件用于“正式产品发布”的对外口径整理：明确已具备能力、商业化可深化方向、以及中长期技术路线（如 PostgreSQL / React / Windows 可视化操作）。
>
> 当前实现细节与端到端流程分别收敛在：
> - `docs/engineering/ARCHITECTURE.md`
> - `docs/product/AI_AUDIT_AND_MANAGED_FLOW.md`

## 1) 已具备能力（当前版本）

### 任务与看板
- 任务 CRUD、状态流转、移动端卡片化、任务详情聚合（终端/日志/审批/AI 会话）。
- 项目集/项目用于分类与默认上下文（任务/工作流/计划任务可挂载）。

### 终端与执行面
- 本地 PTY 多会话、远程 SSH 终端，会话日志落库与回放。
- 终端 WebSocket 实时推送、重连、光标/输入稳定性优化。

### AI 审核（审批治理）
- 终端输出检测 “等待确认/选择/权限提示”等场景。
- 规则集（system/task/terminal/custom）+ AI Provider 辅助决策（可审计、可回看）。
- 全局按键绑定抽象（Enter(CR)/Newline(LF)/Esc/Ctrl+C/...），降低交互式 CLI 误操作。

### AI 托管（任务级）
- 托管提示词/结束条件/错误处理策略均模板化（不在代码硬编码）。
- 可在工作台终端实时可见 AI 执行命令与结果，并在右侧查看 AI 托管会话日志。

### 工作流与计划任务（Runbook 雏形）
- 可视化工作流设计器、节点执行引擎（含 server/command/terminal/task/git/condition/wait）。
- 计划任务（cron/单次）触发任务或 AI 工作流，记录运行结果。

### 配置与治理
- 系统设置分组化；演示模式（只读）可通过 `.env` 开关。
- Prompt Templates（按模块 Key 管理、可编辑、可保存多个 Preset 并切换）。

## 2) 商业化可深化方向（建议作为 Features 列表管理）

> 下列条目是“商业化常见增值点”，并不代表已实现；建议按 ROI 与风险拆解为里程碑。

### P0：企业可用性（可靠性 + 治理）
- 审计留存与合规导出（不可篡改日志、签名/哈希链、审计报表）。
- 权限模型增强：组织/空间（workspace）、细粒度 RBAC、API Token、最小权限。
- 秘钥与凭据：KMS/Vault 对接、加密存储、轮换、访问审计。
- 通知与告警：Webhook/邮件/企业微信/飞书/Slack，告警策略与抑制。

### P1：Runbook 平台化（智能运维）
- 多机 fan-out / join：一键对多台服务器并行执行、结果汇总后进入下一步 AI 判断。
- 节点级策略：超时/重试/失败分支/回滚策略，变更窗口与审批链。
- “组合节点”（server + git + task/command）：沉淀可复用的 Runbook Primitive。

### P1：平台与生态
- 插件化：新增 CLI 类型、检测器、审批规则、工作流节点类型。
- 多 AI Provider 策略：成本/延迟/质量路由，模型降级与配额管理。

### P2：可视化操作系统（Visual OS Ops）
- Windows：WinRM/PowerShell、RDP 会话编排、截图与录屏、GUI 自动化（受控白名单动作）。
- Linux：SSH +（可选）VNC/X11/Wayland 远程桌面接入，GUI 自动化（同样受控）。
- 安全护栏：所有 GUI 自动化必须可审计、可回放、可限权、可审批。

## 3) 中长期技术路线（Tech Roadmap）

### 数据库
- 当前：SQLite（单机部署友好）。
- 规划：PostgreSQL 18（多实例/并发/审计留存/报表更适合）。建议路径：
  - 先抽象 DB 层与迁移策略（SQLite ↔ Postgres 双写/迁移工具）
  - 再引入 PG 作为可选后端（不强制）

### 前端架构
- 当前：Vue 3 + Naive UI（已可用）。
- 规划：若要强化“企业级后台 + 复杂表单 + 可视化运维控件”，可评估 React + shadcn/ui（或保留 Vue 但引入更强的 design system）。

### 执行与代理
- 强化“命令执行模式”：后端 exec / 终端可见 exec / 远端 tmux/screen 托管。
- 引入 “Keepalive + 可恢复会话” 策略（SSH/终端断线恢复、长任务保活）。

