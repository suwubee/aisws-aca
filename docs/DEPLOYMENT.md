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

## 2) 直接运行（二进制）

1. 构建前端并输出到 `backend/static`
2. 构建并运行后端（会同时提供 API + 静态资源）

示例：
```bash
cd frontend && npm ci && npm run build
cd ../backend && go build -o aca .
./aca
```

## 3) 数据与备份

默认数据库为 SQLite：

- 文件位置：`./data/aca.db`（相对项目根）
- 备份方式：停止服务后复制该文件即可

建议同时备份：
- `./data/aca.db`
- 业务侧自建的服务器密钥/凭证（如有单独存放）

## 4) 升级建议

- Docker 部署：先备份 `./data/aca.db`，再 `docker compose pull && docker compose up -d`
- 二进制部署：先备份数据库文件，再替换可执行文件并重启

