# 智能运维工作流（Runbook）设计：节点语义、配置与关系

> 目标：把“工作流”从纯编排（看起来像流程图）升级为可落地的 Runbook 引擎：明确每个节点“在谁的机器上、对哪个项目、用什么权限/目录/参数”执行；支持定时触发、多机并发、结果汇总、AI 分析与下一步决策。

## 1. 现状梳理（基于当前实现）

### 1.1 现有工作流执行引擎（后端）

后端 `WorkflowEngine` 已具备“按节点串行执行 + 条件分支（基于布尔/命令退出码）”的基本能力，并能在 **本机** 或 **SSH 服务器** 上执行命令：

- `server`：选择服务器（写入执行上下文 `currentServerID`）
- `command`：在当前服务器或指定 `server_id` 上执行命令（可拿到输出）
- `terminal`：创建终端会话并向 PTY 写入命令（偏交互，输出不回传给节点）
- `task`：创建/启动 ACA 任务（可选 server/work_dir/cli_type/initial_prompt），并由 `WorkflowAgent` 监控其完成
- `git`：封装 git 命令（本质仍是 command/ssh exec）
- `condition`：支持布尔常量/布尔字符串；或执行命令，以退出码判断 true/false
- `wait`：延迟
- `parallel`：占位（未实现并发编排语义）

核心结论：**后端“可以执行”但“节点契约不够清晰”**（特别是目标服务器、项目目录、交互/非交互差异、输出作为下一步判断的方式）。

### 1.2 当前痛点（与你反馈一致）

1) **“Git 节点到底在哪台机器执行？”**
- 当前引擎逻辑：若节点/上下文有 `server_id` 则在该 SSH 服务器执行；否则在 ACA 服务端本机执行。
- 但前端节点配置目前没有把“目标服务器/目录”显式作为必填或强提示，导致理解困难。

2) **“打开服务器终端后再执行”与“直接 SSH Exec”概念混淆**
- `terminal` 节点偏“交互式”（写入 PTY），用于需要持续会话/AI CLI 的场景。
- 运维 runbook 的大多数动作应使用 `command`（SSH exec，可拿到 stdout/stderr/退出码用于判断）。

3) **前端节点配置与后端引擎期待字段不一致**
- 例如 `git` 节点 UI 目前是 `operation/repo_url/branch`，而引擎优先读取 `command/args/work_dir`。
- `condition` 节点 UI 目前是 `expression`，但引擎需要 `command` 才能“跑命令判断”。

4) **工作流“设计器保存”与后端执行的工作流定义脱节**
- 若节点图未落库，则后端执行时拿不到真实 nodes/edges，用户感觉“节点没有意义/跑不起来”。

## 2. 运维视角的“节点契约”与必要配置项

### 2.1 核心原则：执行目标（Target）必须显式

任何会执行命令/读取日志/做 git 操作的节点，都必须能解析出一个明确的执行目标：

- `target.mode`: `local` | `ssh` | `ssh_group`（扩展项）
- `target.server_id`: 单机目标（ssh）
- `target.server_group_id`: 多机目标（ssh_group）
- `target.work_dir`: 工作目录（可来自 Project）
- `target.env`: 环境变量（可来自 Project/运行参数）

并且 UI 上要展示“该节点最终在谁上跑”的结果（例如 `@ prod-1`）。

### 2.2 项目（Project）与工作目录（WorkDir）的关系

运维中“对哪个项目做操作”通常比“对哪个仓库 URL 做操作”更重要：

- Project（当前已存在）应成为工作流的顶层上下文：`workflow.project_id`
- 节点默认继承 `project_id` 的 `server_id` / `remote_path` / `env_vars`
- 节点允许覆盖：例如某一步在 bastion 上跑、或在另一台 build 机上跑

### 2.3 交互式与非交互式：两类节点要分清

建议将执行节点明确分成两类：

- **Command（非交互）**：SSH Exec 拿输出，适合巡检/检测/批量执行/可重复 runbook
- **Terminal / Task（交互）**：
  - `terminal`：需要“打开会话并写入按键/命令”的场景（例如进入某个交互工具）
  - `task`：需要启动 Claude/Codex/Gemini 等 AI CLI，在终端里持续交互并由系统监控的场景

一个简单判断：**需要把输出作为下一步条件 → 用 command/condition；需要人机/AI持续交互 → 用 task/terminal。**

## 3. 节点设计（面向智能运维的最小可用集合）

### 3.1 Context 类节点（决定“在哪/对谁”）

1) `server`（已有）：设置默认 `server_id`
- 必填：`server_id`
- 作用：后续节点默认在此 server 执行（除非节点自身指定）

2) `project`（建议新增）：设置默认 `project_id`
- 必填：`project_id`
- 作用：后续节点默认继承 project 的 `server_id/remote_path/env_vars`

3) `vars`（建议新增）：定义/覆盖变量
- 配置：`vars: { key: value }`
- 作用：给 AI 提示词、命令模板、通知等提供变量输入

### 3.2 Action 类节点（真正干活）

4) `command`（已有 legacy，但建议作为运维首选）
- 必填：`command`
- 可选：`server_id`（覆盖上下文），`work_dir`，`timeout`，`retries`，`env`
- 输出：stdout/stderr/exit_code（至少要能记录 stdout）

5) `git`（保留但必须“编译”为明确命令）
- 必填（取决于 operation）：例如 `clone` 需要 `repo_url`；`pull` 需要 `work_dir`
- 必选/强提示：`server_id/work_dir`（明确在哪个 repo 上操作）
- 本质：生成一条或多条 `command`（例如 `cd repo && git pull`）

6) `task`（已有）：启动 AI CLI 执行一段目标（可在远端）
- 必填：`title` 或 `task_id`
- 可选：`server_id/work_dir/cli_type/initial_prompt/ai_managed/...`
- 语义：适用于“AI 自己打开终端登录执行操作”的高阶自动化（系统可监控状态并辅助输入）

7) `notify`（建议新增）：发送消息中心通知
- 配置：title/content/level/related_task_or_run
- 用途：告警、变更通知、人工介入提醒

### 3.3 Control 类节点（让流程可控）

8) `condition`（已有，但建议增强为“检测节点”）
- 模式建议：
  - `exit_code`：命令退出码为 0 则 true
  - `contains/regex`：对输出做匹配
  - `ai_decision`：把输出交给 AI 做风险/是否继续判断（需可配置 PromptTemplate）
- 输出：bool + 原始输出（用于日志与下一步解释）

9) `wait`（已有）

10) `parallel/fanout/join`（后续迭代）
- 运维多机任务的关键：对 server group fanout 执行，再汇总结果进入下一步。

## 4. 典型业务用例（你提到的场景如何落地）

### 4.1 定时在某台服务器执行命令 → AI 监控 → 条件判断 → 下一步

- Schedule（定时触发）→ WorkflowRun
- Node A：`project/server`（选定目标）
- Node B：`command`（例如 `tail -n 200 /var/log/app.log | grep -i error`）
- Node C：`condition`（regex/contains 判断是否出现关键错误）
  - true → Node D：`task`（AI 执行修复/回滚/扩容等操作）
  - false → Node E：`notify`（可选，记录“健康”）

### 4.2 AI 发起多台服务器任务，联合处理

最小可用的方式（不引入并发编排）：
- 由一个 `task` 节点启动 AI CLI，让 AI 在任务里使用“批量执行/多 server”能力做联合处理（需要工具层支持多机执行）。

更理想的方式（Runbook 原生支持）：
- Node A：`server_group`
- Node B：`fanout`（对 group 成员并发执行子流程：command/task）
- Node C：`join`（汇总结果）
- Node D：`ai_decision`（对汇总结果做判断并选择后续分支）

## 5. 看板（Kanban）与工作流/项目的融合建议

### 5.1 任务状态是否应可配置？

从运维视角，常见状态不仅是 todo/in_progress/done：
- `blocked`（等待依赖/权限）
- `paused`（等待人工确认）
- `monitoring`（持续观测）
- `failed`（失败待处理）
- `rollback`（回滚中）

建议做成“可配置列”：
- 系统默认提供基础列（兼容现状）
- 项目级可自定义列（适配不同团队/流程）

### 5.2 项目组/项目集（Portfolio）是否需要？

建议引入层级：
- Portfolio（项目集/业务线）
  - ProjectGroup（项目组，可选）
    - Project（具体系统/仓库/服务）
      - Tasks（具体事项）
      - Workflows（Runbook/流程）
      - Schedules（定时巡检/发布窗口）

### 5.3 融合点：Runbook “对项目做事”

- Workflow 绑定 `project_id`（已有字段）
- Task 也应支持 `project_id`（目前缺失，建议补）
- 节点默认继承 project 的 target/workdir/env，减少重复配置

## 6. 落地优先级（建议 Sprint 切分）

Sprint A（让现有能力“可用且不困惑”）
- 工作流设计器保存/加载与后端工作流 nodes/edges 落库打通
- `command` 节点在 UI 中作为一等公民（可拿输出用于 condition）
- `git/condition` 节点字段对齐（明确 server/work_dir/command/匹配方式）
- UI 展示“节点最终执行目标”（@server 或 local）

Sprint B（智能运维的关键能力）
- server group + 批量/并发执行（fanout + join）
- 条件节点增强（contains/regex/JSON 解析/AI decision）
- Schedule 支持触发 workflow（而不仅是 task/ai_workflow）

Sprint C（看板与项目体系融合）
- Task 增加 `project_id`，看板支持按项目/项目集过滤
- 可配置状态列（系统/项目级）
- Workflows/Schedules 与项目视图融合（Runbook 中心）

