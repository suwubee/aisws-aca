# Scripts 使用说明

本项目提供若干脚本用于“快速部署/本地开发/一键启动”。脚本都位于 `scripts/`（少量历史脚本位于项目根目录）。

## 1) Quickstart（推荐：生产式本地运行）

适用场景：你只想在一台机器上快速跑起来（前端静态资源 + 后端二进制），并通过 `.env` 配置变量。

```bash
./scripts/quickstart.sh up
```

说明：
- 首次运行会交互式生成项目根目录 `.env`（可用 `./scripts/quickstart.sh init --force` 覆盖）
- 会构建前端到 `backend/static`，然后启动 `backend/ai-coding-assistant`
- 访问地址：`http://<SERVER_HOST>:<SERVER_PORT>`（默认 `http://0.0.0.0:34007`）

常用命令：
- 初始化 `.env`：`./scripts/quickstart.sh init`
- 仅构建前端：`./scripts/quickstart.sh build`
- 仅启动后端：`./scripts/quickstart.sh start`

## 2) start.sh（开发便捷：前后端分别启动/停止/查看日志）

适用场景：你需要前端 Vite dev server（热更新），同时启动后端二进制。

```bash
./scripts/start.sh all
./scripts/start.sh status
./scripts/start.sh logs
./scripts/start.sh stop
```

说明：
- 若项目根目录存在 `.env`，会自动加载并导出为环境变量（未找到则使用默认值）
- 后端日志：`/tmp/aca-backend.log`；前端日志：`/tmp/aca-frontend.log`
- PID 文件：`/tmp/aca-backend.pid`、`/tmp/aca-frontend.pid`

## 3) keepbackend.sh（历史脚本：Go run + Vite dev）

适用场景：后端需要 `go run` 方式调试（会启用工作区内 GOCACHE）。

```bash
./keepbackend.sh
```

## 4) 环境变量与默认值

- 参考模板：`.env.example`
- Quickstart 会生成 `.env` 并写入默认值

与脚本相关的常见变量：
- `SERVER_HOST` / `SERVER_PORT`：后端监听
- `DATABASE_DSN`：SQLite 路径（容器默认 `/app/data/aca.db`；仓库内运行默认落在 `backend/data/`）
- `AUTH_USERNAME` / `AUTH_PASSWORD` / `JWT_SECRET`：认证
- `TERMINAL_DEFAULT_LOGIN_DIR`：终端默认登入目录（新建本地终端会话默认进入，默认 `~/`）
- `ACA_BACKEND_HOST` / `ACA_BACKEND_PORT` / `ACA_FRONTEND_PORT`：仅开发模式下用于 Vite 代理与端口覆盖（可选）

