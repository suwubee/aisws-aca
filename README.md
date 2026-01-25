# AISWS-ACA（AI超站）

> **🎮 在线演示**
>
> 演示地址：https://aca.aisws.com
>
> 点击登录页底部「点击填充演示账号」按钮自动填充 demo/demo123
>
> ⚠️ **演示模式限制**：工作流功能不可用，仅支持测试单个 AI 任务的基本流程。

> **⚠️ 重要声明**
>
> 1. **项目状态**：本项目仍处于早期开发阶段，可能存在重要 Bug，**请务必在开发模式下充分测试后再用于生产环境**。
> 2. **AI 生成**：整个项目代码由 AI（Claude/Codex/Gemini）辅助编写，欢迎提交 PR 改进，或 Fork 后自行修改。
> 3. **工作流/AI CLI 交互**：涉及服务器文件配置读取等功能仍需大量测试，稳定性待验证。
> 4. **开源协议**：采用 **AGPL-3.0** 协议，要求修改后的代码和基于本项目的网络服务必须开源。如需闭源商用，请联系获取商业授权。

---

AISWS-ACA 是 AISWS（AI SUPER WORKSTATION / AI超站）品牌下的开源产品：一个"AI 驱动的研发/运维超级工作台"，把任务、终端执行、AI 托管、审批治理、审计复盘与工作流/计划任务收敛到一个系统里。

## 核心能力

- **任务与看板**：任务队列 + Kanban 管理，项目/项目集组织与筛选
- **终端与执行面**：本地 PTY / 远程 SSH 终端托管，实时输出 + 日志回放
- **AI 托管**：任务级托管与"动态托管"（按命令返回循环决策下一步）
- **AI 审核（审批治理）**：识别确认/选择提示，规则/AI 决策 `approve/reject/input/ask_user` 并审计
- **工作流与计划任务**：Runbook 雏形（可视化编排 + cron/单次触发）

## 更新记录（Update）

> 以下为从 `ba8d0a9feacb783f410bad6b0810967bedfb06ac`（2026-01-15）到当前版本的更新摘要，按提交顺序列出，便于核对版本差异。

### Update 列表（Commit）

- 2026-01-15 `ba8d0a9` feat: add PostgreSQL database support
- 2026-01-16 `1e31e90` fix: filter internal ACA_CMD markers from terminal output
- 2026-01-16 `0838cdc` feat: bind task terminals and auto-reconnect on restart
- 2026-01-16 `9c4c2d8` docs: update database and terminal restart notes
- 2026-01-16 `a658ef5` fix: prevent terminal task_id corruption
- 2026-01-16 `ec9276a` feat: add server sharing and AI task management
- 2026-01-18 `f88520f` fix: AI托管闭环与任务删除
- 2026-01-18 `22ba63a` fix: AI托管工作流恢复与终端标记过滤
- 2026-01-18 `0703dc0` fix(ai-agent): process queued user messages across completion
- 2026-01-18 `fb855b8` feat(ai-agent): pause after repeated invalid retries

### 详细说明（Highlights）

- `ba8d0a9` PostgreSQL 数据库支持
  - 新增 `DATABASE_DSN` 配置项：支持 PostgreSQL DSN（同时保留 SQLite 模式），并完善数据库初始化/迁移逻辑。
- `1e31e90` 终端输出清爽化：过滤内部命令采集标记
  - `RunCommand` 为了采集输出会注入内部 marker（`__ACA_CMD_BEGIN__/END__`），该版本起对外输出层会过滤，避免用户在终端里看到无关 `echo` 行。
- `0838cdc` 任务↔终端绑定 + “预期断开”自动重连
  - 新增任务绑定终端接口：`/api/tasks/:id/bind-terminal`，避免同一任务反复创建新终端、也避免终端误绑到其他任务。
  - 新增“预期断开”自动重连：当 AI 托管执行重启类命令导致 SSH 断开时，系统会自动尝试重连并恢复任务 AI 状态（超时则暂停）。
- `9c4c2d8` 文档与 Release 配置完善
  - 补充 PostgreSQL / 终端重启与重连的说明，更新 `release/.env.example` 与 Release 运行文档。
- `a658ef5` 终端绑定数据安全修复
  - 修复 `task_id/server_id` 在 fiber/fasthttp “zero-copy string” 场景下可能被污染的问题：对写入 session 的字符串进行复制，并补充回归测试。
- `ec9276a` 服务器与 AI 任务管理增强
  - 服务器管理 API 增强（导入/导出、批量执行、上传 key、创建终端等），并引入 `UserServerShare` 数据结构与前端共享管理入口（共享 API 后续完善）。
  - 任务详情页增加 AI 托管(动态) 会话面板（步骤/状态/追加指令可视化）。
- `f88520f` AI 托管闭环与任务删除修复
  - 任务删除：仅允许在 `done/failed/timeout/archived` 状态删除；并在 PostgreSQL 外键约束下先解绑终端再删除任务，避免删除失败/误报。
  - AI 托管：控制面板/日志展示与交互闭环优化；`<action>/<complete>` 解析更健壮（兜底提取 JSON / 纯文本 summary）。
- `22ba63a` AI 托管恢复与终端标记过滤完善
  - 修复 AI 托管恢复（避免 “session is not resumable”），并统一沿用旧的 `[AI][type] ...` 日志写入/解析方式。
  - 进一步完善终端输出过滤，避免内部 marker 影响用户终端体验。
- `0703dc0` AI 托管并发追加指令：完成/暂停边界不丢消息
  - 用户在“完成判断/总结尚未结束”时追加的新问题不再被吞：会在当前轮结束后自动继续处理。
- `fb855b8` AI 托管防止无效重试（默认熔断）
  - 连续 3 次无效响应（解析失败/未输出 action/complete）或重复失败会自动暂停并 ask_user 让用户补充确认，避免长任务无效消耗 token。
  - 同步强化 “AI 托管(动态)” 系统提示词：目标不明确优先追问、连续无法推进主动请求用户确认。

## AI 托管与 AI 审核

### 前置条件：配置 AI Provider

使用 AI 托管或 AI 审核的"智能模式"前，需先在 **系统设置 → AI Provider** 中配置至少一个 AI 服务：

1. 进入系统设置页面
2. 添加 AI Provider（支持 OpenAI 兼容接口、Anthropic、Google 等）
3. 填写 API Key、Base URL、模型名称
4. 测试连接后保存

### AI 审核（审批治理）

当终端中运行的 AI CLI（Claude Code/Codex/Gemini CLI）弹出确认提示（如 `(y/n)`、`allow write?`）时：

1. 系统自动检测审批提示
2. 根据规则集配置决定处理方式：
   - **手动模式**：等待用户在审批中心手动处理
   - **自动模式**：自动输入 yes/y/enter（黑名单除外）
   - **智能模式**：调用 AI Provider 分析上下文，给出 `approve/reject/input/ask_user` 决策
3. 所有审批记录落库，支持审计追溯

配置入口：**系统设置 → 规则集管理**

### AI 托管

任务级 AI 托管可自动启动 CLI、发送提示、监控执行状态：

1. 创建任务时勾选"AI 托管"
2. 配置初始提示词、完成条件、异常处理策略
3. 启动任务后系统自动：启动 CLI → 发送提示 → 监控日志 → 判断完成/异常

详细流程见：`docs/product/AI_AUDIT_AND_MANAGED_FLOW.md`

## 快速开始（推荐：Release 一键启动）

1) 构建 Release（需要 Node.js + Go）：

```bash
./scripts/build_release.sh
```

如需“单文件”分发（不生成 `release/static/`，前端由 Go embed 提供）：

```bash
./scripts/build_release.sh --single-binary
```

2) 运行（Release 目录不依赖 Node；后端负责提供前端静态资源）：

```bash
cd release
./start.sh setup   # 首次运行：可视化初始化向导
./start.sh start   # 启动服务（默认监听 0.0.0.0:34007）
```

Windows：进入 `release/` 后运行 `start.bat`。

## 开发模式

- 开发/调试指南：`docs/engineering/DEVELOPMENT.md`
- 脚本说明（quickstart/start/build_release）：`docs/engineering/SCRIPTS.md`

默认端口：
- 后端：`34007`
- 前端 dev server：`34001`

## 文档入口

- 文档索引：`docs/README.md`
- 产品概览：`docs/product/PRODUCT_OVERVIEW.md`
- 功能与路线图：`docs/product/FEATURES_AND_ROADMAP.md`

### 文档结构

```
docs/
├── README.md                    # 文档索引（从这里开始）
├── product/                     # 产品文档
│   ├── PRODUCT_OVERVIEW.md      # 产品概览
│   ├── FEATURES_AND_ROADMAP.md  # 功能清单与路线图
│   ├── PRD.md                   # 产品需求文档
│   ├── AI_AUDIT_AND_MANAGED_FLOW.md  # AI 审核/托管流程详解
│   └── UX_NAVIGATION_AND_SETTINGS.md # UX 设计说明
├── engineering/                 # 研发文档
│   ├── DEVELOPMENT.md           # 开发/调试指南
│   ├── DEPLOYMENT.md            # 部署/升级指南
│   ├── ARCHITECTURE.md          # 系统架构
│   ├── PROJECT_LAYOUT.md        # 项目结构
│   └── SCRIPTS.md               # 脚本使用说明
├── ops/                         # 运维文档
│   └── ops-workflow-design.md   # 工作流/Runbook 设计
├── backlog/                     # 规划文档
│   ├── SPRINT_BACKLOG.md        # Sprint Backlog
│   └── RUNBOOK_BACKLOG.md       # Runbook Backlog
└── research/                    # 研究文档
    └── ANALYSIS_REPORT.md       # 功能分析报告
```

## License / Brand

- License: AGPL-3.0（见 `LICENSE` / `NOTICE`）
- Trademarks: `TRADEMARKS.md`
- Commercial: `COMMERCIAL.md`

## Contributing / Security

- Contributing guide: `CONTRIBUTING.md`
- Security policy: `SECURITY.md`
