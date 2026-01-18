# Scripts 使用说明

本项目提供若干脚本用于“快速部署/本地开发/一键启动”。脚本统一位于 `scripts/`。

## 0) 初始化向导（可视化）

适用场景：首次部署，希望用 Web 界面引导完成 `.env`、SQLite / PostgreSQL、默认审核规则、管理员账号，并自动启动服务。

```bash
./scripts/quickstart.sh wizard
```

也可直接运行：
- `./backend/ai-coding-assistant setup`（推荐，使用仓库内置二进制）
- `cd backend && go run ./cmd/setup-wizard`（需要 Go）

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
./scripts/start.sh dev
./scripts/start.sh status
./scripts/start.sh logs
./scripts/start.sh stop
```

说明：
- 若项目根目录存在 `.env`，会自动加载并导出为环境变量（未找到则使用默认值）
- 后端日志：`.aca/logs/backend.log`；前端日志：`.aca/logs/frontend.log`
- PID 文件：`.aca/pids/backend.pid`、`.aca/pids/frontend.pid`
- `./scripts/start.sh dev` 会使用 `go run` 启动后端（用于调试源码）
- 可通过环境变量切换后端启动方式：`ACA_BACKEND_MODE=binary|go-run`
- 若端口被占用，可使用 `--takeover-ports`（或 `ACA_TAKEOVER_PORTS=1`）自动终止占用 `SERVER_PORT/ACA_FRONTEND_PORT` 的进程并重启
- 运行 `./scripts/start.sh`（无参数）会进入分步引导模式（TTY 下默认）

## 2.1) start.bat（Windows 启动脚本）

适用场景：Windows 环境快速启动（默认使用后端内置前端静态资源，因此不强制要求安装 Node）。

```bat
scripts\start.bat
```

说明：
- 默认进入分步引导（setup/start/stop/status/logs）
- 会输出可访问 URL（自动列出本机所有 IPv4 地址的访问地址，而不是只显示 localhost）
- 若 `SERVER_PORT` 被占用，会提示是否接管端口并自动停止占用进程
- 需要可执行文件 `backend\ai-coding-assistant.exe`（如未提供，可使用 Go 构建：`cd backend && go build -o ai-coding-assistant.exe .`）

## 3) 环境变量与默认值

- 参考模板：`.env.example`
- Quickstart 会生成 `.env` 并写入默认值

与脚本相关的常见变量：
- `SERVER_HOST` / `SERVER_PORT`：后端监听
- `DEMO_MODE`：演示模式（只读）
- `DATABASE_TYPE`：数据库类型（默认 `sqlite`；可选 `postgres`）
- `DATABASE_DSN`：
  - SQLite：文件路径（默认 `./data/aca.db`，通常落在 `backend/data/aca.db`）
  - PostgreSQL：连接 DSN（示例：`host=localhost user=aca password=secret dbname=aca port=5432 sslmode=disable`）
- `AUTH_USERNAME` / `AUTH_PASSWORD` / `JWT_SECRET`：认证
- `TERMINAL_DEFAULT_LOGIN_DIR`：终端默认登入目录（新建本地终端会话默认进入，默认 `~/`）
- `ACA_BACKEND_HOST` / `ACA_BACKEND_PORT` / `ACA_FRONTEND_PORT`：仅开发模式下用于 Vite 代理与端口覆盖（可选）
