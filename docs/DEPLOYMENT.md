# 部署指南

## 1) Docker Compose（推荐）

仓库自带 `docker-compose.yml`，默认将数据挂载到宿主机 `./data`，便于备份与升级。

```bash
docker compose up -d --build
```

- 服务端口：`34007`（见 `docker-compose.yml`）
- 健康检查：`GET /api/health`

### 1.1 关键环境变量（生产必改）

- `JWT_SECRET`：务必替换为强随机值
- `AUTH_USERNAME` / `AUTH_PASSWORD`：替换默认账号密码
- `DATABASE_DSN`：默认 `./data/aca.db`（容器内路径），通常无需修改
- `TERMINAL_DEFAULT_LOGIN_DIR`：默认 `~/`（新建本地终端会话默认进入目录）

## 2) Quickstart（推荐：非 Docker）

适用场景：你在一台机器上直接运行项目（不走 Docker），并希望通过 `.env` 配置所有变量。

```bash
./scripts/quickstart.sh up
```

说明：
- 首次运行会交互式生成 `.env`（可用 `./scripts/quickstart.sh init --force` 覆盖）
- 会构建前端到 `backend/static`，并启动后端预编译二进制 `backend/ai-coding-assistant`

## 3) 直接运行（二进制）

适用场景：你希望把程序打包/复制到某台服务器上运行。

关键约定：
- 后端会优先读取“可执行文件同级目录的 `static/`”作为前端静态资源（便于只更新前端无需重编后端）
- 若未找到磁盘 `static/`，会回退到后端二进制内置（embed）的静态资源

示例（从源码构建）：
```bash
cd frontend && npm ci && npm run build
cd ../backend && go build -o ai-coding-assistant .
./ai-coding-assistant
```

示例（复制部署）：
```bash
mkdir -p /opt/aca
cp backend/ai-coding-assistant /opt/aca/
cp -r backend/static /opt/aca/static
cd /opt/aca && ./ai-coding-assistant
```

## 4) 数据与备份

默认数据库为 SQLite：

- 文件位置：由 `DATABASE_DSN` 决定（Docker Compose 默认落在宿主机 `./data/aca.db`；仓库内本地运行常见落在 `backend/data/aca.db`）
- 备份方式：停止服务后复制该文件即可

建议同时备份：
- `./data/aca.db`
- 业务侧自建的服务器密钥/凭证（如有单独存放）

## 5) 升级建议

- Docker 部署：先备份 `./data/aca.db`，再 `docker compose pull && docker compose up -d`
- 二进制部署：先备份数据库文件，再替换可执行文件并重启
