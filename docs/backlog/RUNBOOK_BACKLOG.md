# Sprint Backlog（智能运维 / Runbook 方向）

目标：把 AISWS-ACA（简称 ACA）从“任务+终端”升级为可落地的智能运维平台：项目体系（项目集/项目）、可视化 Runbook（节点编排）、定时触发、跨多服务器执行、结果汇总与 AI 决策。

## Sprint 1：工作流基础打通与节点语义清晰化（已落地）

- [x] 工作流设计器保存/加载与后端 `workflow.nodes/edges` 落库打通
- [x] `command` 节点一等公民（支持输出，便于 condition 判断）
- [x] `git`/`condition` 配置字段与后端引擎对齐（server/work_dir/contains/regex 等）
- [x] workflow 支持绑定 `project_id`，引擎执行上下文继承 project 默认 server/work_dir
- [x] 新增 WeDo 风格组合节点 `ops_step`：一个节点内完成 server/work_dir + git 操作 + 动作（command/terminal/task）
- [x] 文档沉淀：`docs/ops/ops-workflow-design.md`

## Sprint 2：项目/项目集（Portfolio）与任务分类（已落地）

- [x] 新增 ProjectGroup（项目集）模型与 API：`/api/project-groups`
- [x] Project 增加 `group_id`
- [x] Task 增加 `project_id`，任务列表/看板返回 project + group 信息
- [x] 任务创建/编辑支持选择项目（TaskForm/TaskEditModal）
- [x] 任务管理/看板支持按项目集/项目过滤，并在卡片上展示项目标签
- [x] 新增「项目管理」页面（项目/项目集 CRUD，移动端卡片模式）

## Sprint 3：Runbook 执行能力升级（建议）

- [ ] Schedule 支持触发 WorkflowRun（不仅是 task / ai_workflow）
- [ ] server group fanout + join（多机并行执行、结果汇总到下一步）
- [ ] `notify` 节点（消息中心/外部 webhook/告警）
- [ ] `vars` 节点（变量输入/环境变量/模板渲染；用于命令与 AI 提示词）
- [ ] 节点级超时/重试/失败策略（continue_on_error / retries / backoff）
- [ ] 运行报告增强（每个节点的 stdout/stderr/exit_code 结构化存储）

## Sprint 4：终端交互可靠性与全局按键（建议）

- [ ] 统一 Enter/换行、Esc、Ctrl+C 等快捷键：按“CLI 类型”做穷举映射，但保持“全局一套”入口
- [ ] 终端输入焦点与光标错位问题复现与修复（移动端优先）
- [ ] AI 操作复查：对“需要确认”的提示弹窗与按键动作进行强约束（Enter=confirm vs Enter=newline）

## 待确认的问题（用于继续深化 Runbook）

- “git 操作”默认是对 **project 的仓库** 还是对 **work_dir 已存在的 repo**？（建议两者都支持，优先继承 project）
- “任务/工作流”与 “项目” 的关系：是否需要项目级权限与成员管理？（后续可加 RBAC）
- 看板列是否允许项目级自定义（todo/in_progress/done 之外的 blocked/monitoring 等）？
