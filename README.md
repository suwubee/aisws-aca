# AISWS-ACA（AI超站）

AISWS-ACA 是 AISWS（AI SUPER WORKSTATION / AI超站）品牌下的开源产品：一个“AI 驱动的研发/运维超级工作台”，把任务、终端执行、AI 托管、审批治理、审计复盘与工作流/计划任务收敛到一个系统里。

## 核心能力

- **任务与看板**：任务队列 + Kanban 管理，项目/项目集组织与筛选
- **终端与执行面**：本地 PTY / 远程 SSH 终端托管，实时输出 + 日志回放
- **AI 托管**：任务级托管与“动态托管”（按命令返回循环决策下一步）
- **AI 审核（审批治理）**：识别确认/选择提示，规则/AI 决策 `approve/reject/input/ask_user` 并审计
- **工作流与计划任务**：Runbook 雏形（可视化编排 + cron/单次触发）

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

## License / Brand

- License: Apache-2.0（见 `LICENSE` / `NOTICE`）
- Trademarks: `TRADEMARKS.md`
- Commercial: `COMMERCIAL.md`

## Contributing / Security

- Contributing guide: `CONTRIBUTING.md`
- Security policy: `SECURITY.md`
