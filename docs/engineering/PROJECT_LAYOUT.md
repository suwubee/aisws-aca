# 项目结构与产物说明

目标：明确“哪些是源码、哪些是构建产物、哪个可执行文件才是入口”，避免目录里出现多个二进制/多份 dist 造成混乱。

## 1) 标准入口（你应该运行哪个）

- **后端可执行文件（唯一标准）**：`backend/ai-coding-assistant`
  - 生产式本地运行：`./scripts/quickstart.sh up`
  - 开发模式（前端 dev server）：`./scripts/start.sh all`
  - 调试后端源码：`./scripts/start.sh dev`（使用 `go run`）

> 如果你在 `backend/` 下看到 `aca` / `aca_server` / `server` 等其他二进制，它们属于本地历史构建残留（不在 git 中），应删除并统一使用 `backend/ai-coding-assistant`。

## 2) 前端构建产物放哪里

- 前端源码：`frontend/`
- **前端构建输出（唯一标准）**：`backend/static/`
  - Vite 配置：`frontend/vite.config.ts` 的 `build.outDir`
  - 构建命令：`(cd frontend && npm run build)`

> `frontend/dist` / `frontend/dist-local` 不作为本项目的发布产物目录；`dist-local` 已从仓库移除并被 `.gitignore` 忽略。

## 3) 运行时文件放哪里（不进 git）

为避免依赖系统目录（如 `/tmp`），脚本统一把运行时文件写到项目根目录：

- `.aca/logs/`：脚本启动的后端/前端日志
- `.aca/pids/`：脚本启动的 PID 文件
- `.aca/go-build-cache/`：`go run` 使用的构建缓存（可删除）

对应脚本：`scripts/start.sh`

## 4) 不再提供 Docker

本仓库不再维护 Docker 相关文件（Dockerfile / docker-compose），部署方式以 Quickstart / 二进制运行 为准：

- `docs/engineering/DEPLOYMENT.md`
- `docs/engineering/SCRIPTS.md`

