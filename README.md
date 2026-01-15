# AISWS-ACA（AI超站）

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
