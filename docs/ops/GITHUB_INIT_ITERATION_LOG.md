# github-init 迭代日志（门禁记录）

> 规则：每个 Step 完成后必须追加一条记录，且必须包含“承上启下”的下一步前置条件。  
> 分支策略：默认在 `github-init` 持续迭代，按 Step 收敛后再讨论合并。

## 记录模板

```md
## [Sx] 标题
- 时间(UTC):
- 分支/提交:
- 目标:
- 非目标:
- 关键变更文件:
- 接口/行为变更:
- 本地验证:
- 部署记录(34007):
- 风险更新:
- 你方验收结果:
- 下一步前置条件:
```

---

## [S0] 基线固化与记录体系

- 时间(UTC): `2026-02-20 14:29:49 UTC`
- 分支/提交: `github-init` / `6907c26`
- 目标:
1. 建立统一门禁计划文档。
2. 建立风险台账。
3. 建立迭代日志模板并记录当前基线。
4. 固化 34007 运行快照，避免版本歧义。
- 非目标:
1. 不改业务逻辑与接口行为。
2. 不进行 UI/日志/状态机功能修复。
- 关键变更文件:
1. `docs/ops/GITHUB_INIT_ITERATION_PLAN.md`
2. `docs/ops/GITHUB_INIT_RISK_REGISTER.md`
3. `docs/ops/GITHUB_INIT_ITERATION_LOG.md`
4. `docs/ops/GITHUB_INIT_DEPLOYMENT_TRACE.md`
5. `docs/README.md`
- 接口/行为变更: 无。
- 本地验证:
1. 读取当前分支与提交号。
2. 核对 `.aca_server.pid`、`ps`、`ss -ltnp` 的 34007 监听状态。
3. 核对静态资源主入口 hash（`index-BlNZG2dK.js` / `index-CJcjrLAR.css`）。
- 部署记录(34007):
1. 当前运行 PID: `779197`（`/root/dev/aca/backend/aca-new`）。
2. 监听端口: `0.0.0.0:34007`。
3. 静态主资源: `backend/static/assets/index-BlNZG2dK.js`、`backend/static/assets/index-CJcjrLAR.css`。
- 风险更新:
1. 新建并初始化风险台账 `R-001` 到 `R-012`。
2. 当前优先级：`R-001 > R-005 > R-002/R-006 > R-003/R-004/R-007/R-008`。
- 你方验收结果: 待验收。
- 下一步前置条件:
1. 你确认 S0 记录体系可接受。
2. 进入 S1：实现运行版本可追溯（接口/UI/部署记录联动）。

## [S1] 运行版本可追溯（接口 + 页面可见）

- 时间(UTC): `2026-02-20 16:44:50 UTC`
- 分支/提交: `github-init` / `6907c26`
- 目标:
1. 增加可公开查询的运行版本接口（无需登录）。
2. 在主界面头部展示运行版本摘要，减少“当前是哪版”不确定性。
3. 将静态资源入口 hash 纳入运行版本信息，支持前后端一致性核对。
- 非目标:
1. 不处理 AI 托管会话滚动/日志/状态机问题（留在 S2-S4）。
2. 不改任务/终端业务流程。
- 关键变更文件:
1. `backend/buildmeta/buildmeta.go`
2. `backend/api/runtime_version.go`
3. `backend/api/runtime_version_test.go`
4. `backend/main.go`
5. `frontend/src/api/types.ts`
6. `frontend/src/api/index.ts`
7. `frontend/src/views/MainLayout.vue`
- 接口/行为变更:
1. 新增 `GET /api/runtime/version`，返回 `branch/commit/build_time/static_index_assets/pid` 等运行信息。
2. `GET /api/health` 保持兼容（`status/version`），并补充 `git_branch/git_commit/build_time/static_source`。
3. 页面头部新增版本标签（hover 可看完整信息）。
- 本地验证:
1. 后端测试：`/usr/local/btgo/bin/go test ./api -run RuntimeVersion -count=1` 通过。
2. 兼容回归：`/usr/local/btgo/bin/go test ./... -run TestRegisterStaticRoutes -count=1` 通过。
3. 前端构建：`/www/server/nodejs/v24.13.1/bin/pnpm run build` 通过。
4. 运行验证：
   - `curl http://127.0.0.1:34007/api/health` 返回 `git_branch/git_commit/build_time`。
   - `curl http://127.0.0.1:34007/api/runtime/version` 返回 `static_index_assets` 与 `pid`。
- 部署记录(34007):
1. 当前运行 PID: `934096`（`/root/dev/aca/backend/aca-new`）。
2. 监听端口: `0.0.0.0:34007`。
3. 静态主资源: `index-69AgpyFw.js`、`index-CJcjrLAR.css`。
- 风险更新:
1. `R-001` 从 `open` 更新为 `mitigating`（等待你确认后转 `controlled`）。
- 你方验收结果: 待验收。
- 下一步前置条件:
1. 你确认 S1 的版本追溯可用（接口 + 页面显示 + 部署核对）。
2. 进入 S2：AI 托管会话面板滚动与布局稳定化。

## [S2] AI 托管会话面板滚动/布局稳定化（可读性修复）

- 时间(UTC): `2026-02-20 17:12:00 UTC`
- 分支/提交: `github-init` / `6907c26`
- 目标:
1. 修复会话上下文长文本挤压明细区的问题。
2. 让“流程事件/CLI日志/步骤”面板在窄空间下仍可持续浏览。
3. 给用户明确的“上下文收起/展开”交互，避免信息层互相抢占高度。
- 非目标:
1. 不改日志聚合逻辑、去重逻辑与加载策略（留在 S3）。
2. 不改托管状态机与动作解析（留在 S4）。
- 关键变更文件:
1. `frontend/src/components/AIWorkflowSessionPanel.vue`
- 接口/行为变更:
1. 新增“收起上下文/展开上下文”按钮（会话头部）。
2. 会话上下文区改为独立滚动，并限制最大占高，避免压缩 tabs 区。
3. tabs 明细区设置最小高度，保证至少可见一屏事件列表。
4. `user_goal` / `summary` 长文本改为默认折叠，可展开查看。
- 本地验证:
1. 前端构建：`/www/server/nodejs/v24.13.1/bin/pnpm run build` 通过。
2. 静态产物更新：`AIWorkflowSessionPanel-Bd89M6fx.js`、`AIWorkflowSessionPanel-DzIvoOlq.css`。
- 部署记录(34007):
1. 当前运行 PID: `936832`（`/root/dev/aca/backend/aca-new`）。
2. 启动日志显示监听 `0.0.0.0:34007`（见 `.aca_server.log` 17:11:38）。
- 风险更新:
1. `R-005` 从 `open` 更新为 `mitigating`。
- 你方验收结果: 待验收。
- 下一步前置条件:
1. 你确认右侧会话明细区可持续查看（可收起上下文、tabs 可滚动）。
2. 进入 S3：流程事件/CLI 日志加载顺序、连续性与去重稳定化。

## [S2.1] 终端右侧“AI托管控制/流程轨迹”分离 + 任务域历史入口

- 时间(UTC): `2026-02-20 17:43:46 UTC`
- 分支/提交: `github-init` / `6907c26`
- 目标:
1. 将终端右侧 AI 托管控制与流程轨迹拆分为独立 tab，避免语义冲突。
2. 在任务域补齐“流程与进度”入口，符合“项目集-项目-任务-服务器-终端”主链路。
3. 保持 Workflows 页历史能力不丢失，同时允许任务域与工作流域分别跳转。
- 非目标:
1. 不改后端日志采集/去重策略（S3）。
2. 不改托管状态机与解析容错（S4）。
- 关键变更文件:
1. `frontend/src/components/TerminalPanel.vue`
2. `frontend/src/components/AIWorkflowSessionPanel.vue`
3. `frontend/src/views/Tasks.vue`
4. `frontend/src/components/WorkflowHistoryList.vue`
5. `frontend/src/views/Workflows.vue`
- 接口/行为变更:
1. 终端右侧新增 segmented tab：
   - `流程轨迹`（仅流程/执行链路/CLI日志/步骤）
   - `AI托管控制`（会话摘要/暂停/继续对话）
2. `AIWorkflowSessionPanel` 新增 `panelMode`：`full | control | flow`。
3. 工作清单新增 `流程与进度` tab（嵌入执行历史筛选面板）。
4. 执行历史组件新增 `sessionJumpTarget`，支持任务域与工作流域不同跳转目标。
- 本地验证:
1. 前端构建：`/www/server/nodejs/v24.13.1/bin/pnpm run build` 通过。
2. 后端运行：`/api/health` 可访问。
- 部署记录(34007):
1. 当前运行 PID: `940438`（`/root/dev/aca/backend/aca-new`）。
2. 静态主资源: `index-5AYbHJIN.js`、`index-CJcjrLAR.css`。
- 风险更新:
1. `R-009` 保持 `mitigating`，待你确认任务域入口是否满足筛选和追踪诉求。
- 你方验收结果: 待验收。
- 下一步前置条件:
1. 你确认右侧分离 tab 交互与任务域历史入口符合预期。
2. 进入 S3：流程事件与 CLI 日志连续性、顺序与去重治理。
