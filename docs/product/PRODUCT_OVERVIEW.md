# AISWS-ACA（AI超站）产品概览

AISWS-ACA 是 AISWS（AI SUPER WORKSTATION / AI超站）品牌下的开源产品，定位为“AI 驱动的研发/运维超级工作台”：把 **任务**、**终端执行**、**AI 托管**、**审批治理**、**审计复盘**、**工作流/计划任务** 收敛在一个系统中，形成可观测、可控、可回放的闭环。

## 1) 解决的问题（Why）

- 多个 AI CLI（Claude/Codex/Gemini/自定义命令）并行运行时，缺少统一的任务队列、终端视图、审计与复盘。
- 交互式 CLI 频繁弹窗确认（Enter/Yes/No/选项）导致打断，且存在误操作风险。
- “命令执行→结果判断→下一步动作”难以标准化沉淀（Runbook / 工作流 / 计划任务）。

## 2) 核心闭环（How）

- **任务**：以任务作为执行单元，绑定项目/服务器/工作目录/AI 托管策略。
- **终端**：以终端作为执行载体（本地 PTY / 远程 SSH），支持实时输出、日志与重连。
- **AI 托管**：可选择“记录/脚本/AI CLI”模式，或“AI 托管(动态)”按命令返回循环决策下一步。
- **AI 审核（审批治理）**：检测终端输出中的确认/选择提示，通过规则或 AI 给出 `approve/reject/input/ask_user`，并落库审计。
- **审计与复盘**：终端日志、审批记录、AI 决策日志、登录记录等聚合，支持导出与追溯。

详细端到端流程见：
- `docs/product/AI_AUDIT_AND_MANAGED_FLOW.md`

## 3) 工作流与 Runbook（Ops 视角）

Runbook 的关键是“节点契约清晰”：明确每一步 **在谁的机器上、对哪个项目/目录、用什么权限/参数** 执行，并把 stdout/stderr/exit_code 结构化记录为后续判断依据。

节点设计建议与现状对齐见：
- `docs/ops/ops-workflow-design.md`

## 4) 开源与商业化（License / Brand）

- 代码许可：Apache-2.0（见 `LICENSE` / `NOTICE`）
- 商标与品牌：AISWS / AISWS-ACA / AI超站（见 `TRADEMARKS.md`）
- 商业化与企业支持方向（非绑定）：见 `COMMERCIAL.md`

## 5) 规划入口（Roadmap）

- 业务/产品 backlog：`docs/backlog/SPRINT_BACKLOG.md`、`docs/backlog/RUNBOOK_BACKLOG.md`
- 市场/价值分析（历史沉淀）：`docs/research/PRD_GAP_COMPETITOR_VALUE_REPORT.md`

