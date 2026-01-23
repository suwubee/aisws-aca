# 用户故事与流程核对（基于当前实现）

本文档用“用户故事（User Story）+ 场景剧本（Scenario）”的方式，模拟不同用户在不同环境下使用本系统的关键路径，并对照当前代码实现进行功能盘点与卡点排查。

> 重点概念（本项目语境）
>
> - **AI 托管**：接替人去发布/输入/执行/反馈（任务驱动器），对任务结果负责。
> - **AI 审核**：只在“需要判断的交互点”（y/n、Enter to confirm、选项题、权限提示等）进行智能干预，避免卡住和避免高危误操作。
> - **AI CLI（可选）**：
>   - 当任务明确选择 `automation_mode=cli` 时：以 `cli_type` + **终端输出锚点** 自动确认“已进入 CLI”（不靠输入关键词）。
>   - 当 CLI 未强制（例如仅开启 `ai_managed=true`）时：系统只做“候选预判”（`needs_confirm=true`），需要在终端面板确认 **是/否/不确定**；也可点击 **让 AI 预判** 辅助判断。
>
> 规则：**不会用“输入命令关键词”来判定进入/退出 CLI**；进入/退出以输出锚点/启发式证据为主，并提供人工确认闭环。

相关设计与实现细节参考：`docs/product/AI_AUDIT_AND_MANAGED_FLOW.md`。

---

## 0) 角色画像（Personas）

1. **服务器小白（新手）**：第一次接触 SSH/CLI；希望“点一下就跑起来”，能在卡住时被明确提示下一步。
2. **运维/工程师（熟练用户）**：有自己的终端习惯，可能先手动进入 CLI，再让系统接管后续（托管/审核）。
3. **团队管理员**：关注安全审计、审批留痕、权限边界；希望批量处理审批，避免风险升级。
4. **多服务器执行者**：需要在多台服务器上批量执行脚本/变更，关注可观测、可回滚、可暂停。

---

## 1) 场景矩阵（快速索引）

| 场景 | 你做了什么 | 系统需要识别什么 | 预期结果 | 典型卡点 |
|---|---|---|---|---|
| S1 | 新建 CLI 任务并启动 | 任务选择 CLI + 输出锚点进入 CLI | 自动进入 CLI 并发送 prompt | CLI 未安装/不在 PATH |
| S2 | CLI 启动后先出现权限/信任提示 | `waiting_approval` | AI 审核介入或提示人工 | 回车/选择输入不到 |
| S3 | 终端里你手动进入 CLI，再启用托管 | 输出锚点候选 + 需要确认（是/否/不确定） | 不新开终端，也能开始跟踪 | CLI 状态不确定导致发送被阻止 |
| S4 | CLI 过程中断线/重连 | 终端重连链 + 任务活跃终端更新 | 任务可恢复 | 多终端状态不同步 |
| S5 | 只用 AI 审核（不托管任务） | 识别审批交互点 | 审批中心可一键处理 | 误判/过度匹配 |
| S6 | AI 托管(动态)/agent 模式 | 会话可持续 Resume | 多轮补充信息不中断 | “完成边界吞消息”已修复 |
| S7 | 脚本模式多服务器执行 | 输出标记 + 退出码/暂停信号 | 可观测、可暂停、可归因 | 出错归因/需要人工确认 |
| S8 | 提示弹窗太频繁 | 允许取消提示（不叠加） | 不影响手动操作 | 重复弹出打断节奏 |
| S9 | 同任务多终端切换 | `active_terminal_id` + 终端级 CLI/审批 | 明确“AI活跃终端”，避免串消息 | UI 异步竞态/误发到非活跃终端 |

---

## 2) 用户故事与场景剧本（按核心链路）

### S1. 新手：从 0 到 1 跑通一个 AI CLI 托管任务（Happy Path）

**用户故事**
- 作为服务器小白，我希望选择一台服务器、选一个 AI CLI（例如 Claude Code），输入需求后自动执行并在终端里看到过程与结果。

**前置条件**
- 已在「服务器」页面配置了可用服务器（含“本机也要配置成一条服务器记录”的产品设定）。
- 目标服务器已安装并可执行 `claude` / `codex` / `gemini` 之一（取决于任务 `cli_type`）。

**用户操作**
1. 进入「任务」→ 新建任务，选择：
   - `automation_mode = cli`
   - `cli_type = claude|codex|gemini`
   - `server_id`（或绑定到目标服务器）
   - `work_dir`（可空，系统会生成默认目录）
   - `initial_prompt`（需求描述）
2. 点击「启动任务」。

**系统行为（当前实现核对）**
- 后端 `POST /api/tasks/:id/start` → `backend/api/task.go:StartTask()`：
  - 校验 `automation_mode=cli` 必须有 `server_id`。
  - 幂等：若任务 `in_progress/paused` 且已有 running 终端，会返回该终端而不是重复创建。
- `backend/service/task/automation.go:StartTask()`：
  1. 创建 SSH 终端（`terminalManager.CreateSSHSession`），并 `LinkTask` 写回任务绑定。
  2. `cd work_dir`，执行 CLI 命令（例如 `claude`）。
  3. **等待 CLI 就绪后才发送 prompt**（避免把 prompt 当 shell 命令执行）：
     - 进入 CLI 的标记来自 `metadata.ai_assistant.detected=true`。
     - `detected` 的来源：终端输出锚点命中（`backend/service/terminal/session.go:557` + `backend/service/detector/detector.go:106`）。
     - 只有“任务选择了 CLI 且启用 CLI/托管跟踪”（`automation_mode=cli` 或 `ai_managed=true`）才开启检测（`backend/service/terminal/session.go:1535`）。
  4. 发送 `initial_prompt`（若 `ai_managed=true` 会发送更结构化的 `buildManagedPrompt`）。
  5. 启动后台监控 `monitorTaskCompletion` 做“完成/需要人工/失败/超时”判断。

**成功标准（用户可见）**
- 终端面板显示 AI CLI 已进入（`metadata.ai_assistant.detected=true`，且 state=waiting_input/working）。
- 任务状态变为 `in_progress`，AI 状态 `ai_status=running`。
- 终端中出现 AI 的执行过程与最终结果，任务最终被标记为 `done` 或提示需要人工处理。

**可能卡点（本场景）**
1. 新手不知道“本机也要在服务器里配置”→ 启动任务报错 “Server is required (local must be configured in Servers)”。
2. CLI 输出锚点未命中（版本差异、语言差异、ANSI 清屏导致采样不到）→ 就绪检测超时 → 任务暂停。

---

### S2. 新手：CLI 未安装/不在 PATH（Fail Fast + 可恢复）

**用户故事**
- 作为新手，我希望当服务器没有安装 AI CLI 时，系统不要乱输入，而是明确告诉我怎么做。

**触发**
- 任务启动后执行了 `claude`/`codex`/`gemini`，终端输出出现 `command not found`。

**系统行为（当前实现核对）**
- `backend/service/task/automation.go:366`：
  - 启动 CLI 后短暂等待，读取 scrollback，若发现 `command not found`：
    - 立刻将任务置为 `paused`
    - 返回 `needs_user_action=true` + `user_action_hint`（前端会 toast）
    - 并创建消息 `approval_needed`（用于消息中心/审批中心提示）

**成功标准**
- 不会继续发送 prompt（避免误把 prompt 当 shell 命令）。
- 用户得到明确提示：例如 Claude 建议 `claude` 或 `npx claude`。

**卡点/改进建议**
- 目前只提示用户手动尝试；对新手更友好：可以在设置里提供“CLI 启动命令模板”（例如允许配置 claude 为 `npx -y @anthropic-ai/claude-code`）。

---

### S3. 熟练用户：先手动进入 CLI，再从终端面板启用 AI 托管（不新开终端）

**用户故事**
- 作为熟练用户，我希望在已有 SSH 终端里先完成环境准备（进入目录、export token、手动启动 CLI），然后系统只负责后续 prompt/审核/监控。

**用户操作**
1. 在「终端」里手动 SSH 到服务器，进入目录并启动 AI CLI（例如输入 `codex`）。
2. 在右侧面板点击「启用 AI 托管」。

---

### S9. 多终端：同一任务开 A/B 终端，AI 托管只绑定活跃终端（避免“串消息”）

**用户故事**
- 作为多终端使用者，我希望在同一任务下开两个终端（A 做 AI 托管执行，B 做人工观察/排障），切换时不会把 A 的 AI 控制/提示误显示到 B，也不会把输入误发到错误终端。

**前置条件**
- A 与 B 都关联到同一个任务（`TerminalSession.task_id` 相同）。
- 该任务启用了 AI 托管或 Agent 模式，且 `task.active_terminal_id` 指向 A（系统正在 A 上推进任务）。

**用户操作**
1. 在 A 上启用 AI 托管并开始执行。
2. 切换到 B 终端继续查看/手动操作。
3. 打开右侧「AI 托管控制」面板，尝试查看日志/处理提示/发送补充信息。

**系统行为（当前实现与建议）**
- AI 托管状态是 **任务级别**（`ai_managed/ai_status`），因此 B 看到“AI运行中”并不代表 AI 在 B 执行。
- 控制面板需要显式展示：**AI活跃终端 = A**，并提供一键切换到 A。
- 异步轮询/WS 输出需要做“终端作用域校验”，避免在快速切换时出现“上一终端结果覆盖当前终端”的串消息现象。

**成功标准**
- 切换终端时，AI 控制面板的提示/日志不会跨终端错位。
- 用户发送补充信息时，系统能清晰指向“控制目标终端”（活跃终端），避免误发到 B。

**系统行为（当前实现核对）**
- 前端 `frontend/src/components/TerminalPanel.vue:978`：
  - 若当前终端已绑定任务：直接 `updateTask(ai_managed=true, ai_status=running)`。
  - 若无任务：会自动创建一个任务并绑定当前终端（`automation_mode=none`, `ai_managed=true`），再置 `ai_status=running`。
- 后端 `UpdateTask`：
  - 会刷新该终端的 CLI 跟踪配置（`backend/api/task.go:774` 调 `terminalManager.LinkTask`）。
- 终端侧：
  - 在启用跟踪后，**只有当终端输出命中“所选 cli_type 的锚点”** 才会把 `detected=true`（`DetectAgentWithType`）。
  - 前端在发送 prompt 前会检查 `assistant.detected` 和 `assistant.state===waiting_input`（`TerminalPanel.vue:960`）。

**成功标准**
- 不新开终端；
- 当 CLI 真正进入交互态（输出锚点命中）后，可以从右侧输入框持续给 AI 发送指令。

**关键卡点（高概率）**
1. **默认 cli_type 可能与用户实际启动的 CLI 不一致**  
   - 右侧面板自动创建任务时没有让用户选择 `cli_type`，很可能默认成 `claude`。  
   - 若用户实际启动的是 `codex`：由于检测是“按 expectedType 精确匹配”，会一直 `detected=false`，前端会提示“未检测到 AI CLI 就绪”。  
   - 这符合“不能用关键词判定”的原则，但 UX 上会显著卡住新手/熟练用户。

**建议（不改原则的前提下）**
- 右侧“启用 AI 托管”时增加一步：让用户选择 `cli_type`（claude/codex/gemini）或沿用上次选择。
- 或者：若 `cli_type` 为空/unknown，则提示用户先在任务里明确选择后再启用（宁可让用户明确选择，也不猜测）。

---

### S4. CLI 过程中出现 y/n、Enter to confirm、选项题：AI 审核介入

**用户故事**
- 作为管理员/普通用户，我希望在 CLI 卡在确认题时系统能提示我并尽量自动处理简单问题，同时对高危操作更保守。

**触发**
- 终端输出命中审批提示（`detector.StateWaitingApproval`），例如：
  - `(y/n)`, `[y/n]`, `Enter to confirm`, `Esc to cancel`
  - `allow read/write/execute?`
  - `sudo password:`

**系统行为（当前实现核对）**
- 终端输出进入 `waiting_approval` 时：
  - `backend/service/terminal/session.go:603` 异步 `handleApproval`
  - `approval.Engine` 会评估该提示（可用规则/AI），得到 approve/reject/input/wait
  - 若需要人工处理，会通过 WebSocket 发送 `approval`/`approval_needed` 事件
  - 前端：
    - `ApprovalCenter.vue` 集中展示待处理审批，可一键允许/拒绝/自定义输入/发送按键（Esc/Ctrl+C/数字选项等）
    - `Terminal.vue` 会把审批事件写入 `approvalStore`，供审批中心展示

**成功标准**
- CLI 不再因为简单 y/n 停滞；
- 审批动作可审计（审批记录、规则命中、AI 决策）。

**可能卡点**
1. **审批提示误判**：输出里出现类似 `y/n` 字样但并非交互点（例如文档/日志）→ 会触发审核流程。
2. **密码类提示**：`sudo password:` 被识别为审批，但系统无法替用户输入密码；应更明确地提示“必须人工输入”并避免误触自动输入。

---

### S5. 只用 AI 审核（不启用 AI 托管）

**用户故事**
- 作为团队管理员，我希望团队成员日常操作终端时，遇到确认题不会卡住；但不希望系统自动做复杂决策。

**用户操作**
1. 用户在终端里手动执行命令（不启用任务托管）。
2. 当出现 y/n 选择题时，在审批中心点击“允许/拒绝/自定义输入”。

**核对点**
- 当前实现：审批检测是“对所有输出生效”，不要求进入 AI CLI（即不依赖 `detected`），见 `backend/service/terminal/session.go:599`。
- 这能覆盖更多场景，但也会带来“误判”的可能（需靠审批引擎/规则约束）。

---

### S8. 取消提示：不打断终端手动操作（避免叠加）

**用户故事**
- 作为用户，我希望当系统提示“需要手动接管/需要审批”时，如果我准备在终端里自己处理，可以先把提示收起；提示不应无限叠加，但如果仍未解决也应能再次提醒。

**当前实现**
- 终端审批：
  - `TerminalPanel.vue` / `ApprovalCenter.vue` 提供 **取消提示** 按钮；
  - 前端 `approvalStore.dismissPendingApproval()` 会移除当前待处理项，并对“同一终端 + 同一提示内容”做短暂抑制（TTL）以避免马上被重复弹回。
- 工作流/Agent 的 `approval_needed` 消息：
  - 终端面板提供 **取消提示**，调用后端 `dismissMessage`，避免列表持续堆积。

**成功标准**
- 提示不会“越弹越多”打断节奏；
- 用户仍可选择在审批中心/终端里手动处理，必要时系统也能在后续再次提醒。

---

### S6. AI 托管(动态)/Agent：多轮持续接收用户补充，不丢消息

**用户故事**
- 作为用户，我希望在 AI 正在执行任务时，能不断追加补充信息；即使 AI 刚完成/刚暂停，也不会出现“多次回复失效”。

**用户操作**
1. 新建任务：`automation_mode=agent`，填写 `initial_prompt`（目标）。
2. 启动任务后，在任务/面板里多次追加新消息。

**系统行为（当前实现核对）**
- `backend/service/workflow/ai_engine.go`：
  - 对 in-flight session 用 `pending` 队列保证消息顺序；
  - 在完成边界引入 `closing + done`，避免“刚结束时追加消息被吞”，并会在必要时重启 loop 继续执行。

**成功标准**
- 用户追加信息都会进入下一轮执行，不会因为“AI 判断任务完成了”而直接失效。

---

### S7. 脚本模式：多服务器执行与暂停信号

**用户故事**
- 作为多服务器执行者，我希望一次性在多台服务器跑脚本，并能在需要人工确认/重启时暂停。

**用户操作**
1. 新建任务：`automation_mode=script`，选择 `target_server_ids`，填写 `script`。
2. 启动任务。

**系统行为（当前实现核对）**
- 每台服务器会创建一个 SSH 终端，分别执行脚本。
- 支持通过输出 marker（如 `ACA_TASK_EXIT_CODE:`、`ACA_TASK_PAUSE`）或关键字（reboot/restart）触发暂停/归因（见 `backend/service/task/automation.go:537`）。

**卡点**
- 错误归因与回滚路径需要更明确的 UX（例如：哪台机器失败、失败原因摘要、下一步操作按钮）。

---

## 3) 功能盘点（对照当前实现）

### 3.1 服务器（Servers）
- ✅ 服务器 CRUD、测试连接、创建 SSH 终端（前端 `Servers.vue` + 后端 `backend/api/server.go` 等）
- ⚠️ “本机也要配置为服务器”对新手不直观（但目前是系统的硬约束）

### 3.2 终端（Terminals）
- ✅ 本地 tmux 终端（可恢复）+ SSH 终端（不可重启恢复，但支持任务自动重连链路）
- ✅ WebSocket 输出流 + UTF-8 分片修复 + 自动重连（`frontend/src/components/Terminal.vue`）
- ✅ 终端日志与 AI 日志（`TerminalApprovals.vue` 支持从 ws + 持久化日志加载）

### 3.3 任务（Tasks）
- ✅ 任务 CRUD、Kanban、详情页、状态流转（todo/in_progress/paused/done/failed/timeout/archived）
- ✅ `automation_mode`: none/cli/script/agent
- ✅ `cli_type`：claude/codex/gemini（cli 模式强校验）
- ✅ 终端绑定：`POST /api/tasks/:id/bind-terminal`（前端在终端面板可用）

### 3.4 AI CLI 状态跟踪（Enter/Exit + Ready）
- ✅ 进入 CLI：
  - `automation_mode=cli`：以 `cli_type` 限定的输出锚点确认进入（`detected=true`）
  - CLI 未强制：输出锚点只标记为候选（`needs_confirm=true`），由用户在终端面板确认“是/否/不确定”
- ⚠️ 退出 CLI：以 `state -> idle` 启发式回落为主，并通过“需要确认：AI CLI 状态”闭环避免误判带来的误输入
- ✅ 启动阶段安全：CLI ready 检测失败会暂停并提示人工确认（避免误输入）

### 3.5 AI 审核（Approval）
- ✅ `waiting_approval` 检测、审批引擎评估、审批中心展示、输入/按键发送
- ⚠️ 误判与密码类提示的策略需更明确（默认应更保守）

### 3.6 AI 托管(动态)/Agent
- ✅ 会话创建、默认终端可观测、可持续 Resume（多轮补充不丢）
- ⚠️ agent 与 CLI 的组合策略需要产品层明确：是否允许 agent 任务在不选择 `cli_type` 的情况下进入 CLI（目前默认不会跟踪）

---

## 4) 结论：当前最容易卡住的点（按优先级）

1. **终端面板“启用 AI 托管”未让用户选择 `cli_type`**  
   - 自动创建任务时默认 `claude`，用户实际用 `codex` 可能 detected=false → 无法自动进入“可输入态”。
   - 目前的兜底：终端面板提供“需要确认：AI CLI 状态（是/否/不确定/让 AI 预判）”，可人工确认后继续发送；但从 UX 角度仍建议补齐 `cli_type` 选择入口。
2. **CLI 输出锚点覆盖面不足导致 ready 超时**  
   - 版本/语言/主题变化会导致“未命中锚点”，系统会安全暂停但体验受损。
3. **CLI 退出检测过于粗糙（idle=退出）**  
   - 可能出现误清 `detected`，导致 UI/自动发送逻辑认为“不在 CLI”。
4. **“本机也要配置成服务器”对新手理解成本高**  
   - 建议在首次启动/创建任务时给明确引导（setup wizard 或弹窗）。
5. **非 AIManaged 的 CLI 任务遇到“alert”不会自动暂停**  
   - 监控遇到 `MonitorActionAlert` 时只有 AIManaged 才会进一步判断/暂停；否则可能一直等到超时。

---

## 5) 建议的迭代任务清单（从主干闭环出发）

1. **托管入口更清晰（新手优先）**
   - 在“启用 AI 托管/开始托管”时显式选择：`agent`（无需装 CLI）或 `cli`（需要装 claude/codex/gemini），并把必填项在 UI 上一次性补齐。
2. **CLI 状态确认闭环完善**
   - “让 AI 预判”结果可一键应用为“确认（是/否）+ 类型”（仍保留人工二次确认开关）。
   - 对“候选预判”增加更强证据（例如多锚点/连续多帧/上下文窗口），降低误触发频率。
3. **AI 审核与托管的模块边界**
   - 把 AI 审核做成 AI 托管的可插拔模块：同一套“交互点检测 + 动作执行”能力，既可独立启用（只审核），也可被托管引擎调用（托管时自动处理审批点）。
4. **服务器/本机引导**
   - 首次启动引导（wizard）：解释为什么“本机也要配置成服务器”，并提供一键创建本机 Server 的按钮。
5. **可观测与回溯**
   - 在任务详情页给出“最后一次卡点”的摘要（例如：等待审批/CLI 未就绪/断线重连中），并提供直接跳转到终端/审批中心的按钮。
