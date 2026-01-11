# 开发指南

本文档用于本地开发、调试与交付前自检。产品/流程类文档请从 `docs/README.md` 进入。

## 1) 前置要求

- Go 1.21+
- Node.js 18+（项目 Dockerfile 使用 Node 20）
- npm（仓库包含 `frontend/package-lock.json`）

## 2) 本地启动

### 2.1 后端（Go）

```bash
cd backend
go mod tidy
go run .
```

- 默认地址：`http://localhost:34007`
- 健康检查：`GET /api/health`

### 2.2 前端（Vite）

```bash
cd frontend
npm ci
npm run dev
```

- 默认地址：`http://localhost:34001`
- 本地代理：`/api -> http://localhost:34007`（见 `frontend/vite.config.ts`）

### 2.3 脚本启动（可选）

如果你更习惯“一键启停/看日志”的方式，可使用脚本：

```bash
./scripts/start.sh all
./scripts/start.sh status
./scripts/start.sh logs
```

- 若项目根目录存在 `.env` 会自动加载（参考 `.env.example`）
- 可选覆盖 Vite 代理/端口：`ACA_BACKEND_HOST`、`ACA_BACKEND_PORT`、`ACA_FRONTEND_PORT`

### 默认账号
- 用户名：`admin`
- 密码：`admin123`

## 3) 构建与发布前自检

### 3.1 前端构建

```bash
cd frontend
npm run build
```

输出目录：`backend/static`

### 3.2 后端测试

```bash
cd backend
go test ./...
```

## 4) 开发约定（避免返工）

### 4.1 提示词模板：禁止硬编码

- **不要**在前后端代码里硬编码模块提示词。
- **应该**通过“提示词模板（Prompt Templates）”作为单一事实来源（可编辑、可选择方案、支持变量）。

### 4.2 按键语义统一：Enter vs Newline

交互式 CLI（如 Claude Code/Codex/Gemini CLI）对 `CR`/`LF` 的处理可能不同：
- `enter`：回车确认（CR，`\\r`）
- `newline`：文本换行（LF，`\\n`）

所有“自动输入/审批按键/终端快捷键”应统一通过 Key Bindings 解析，避免把字符串 `"enter"` 当作输入内容发送。

### 4.3 配置层级（全局默认 → 覆盖）

在功能设计与 UI 呈现上需要避免“多个入口同时可改但不知道谁生效”的体验：
- 系统设置提供全局默认值
- 任务/终端/工作流可按需覆盖，并明确展示“当前生效来源”

## 5) 常见问题（开发调试）

- 终端交互/审批确认异常：优先检查 Key Bindings 的 `enter/newline` 配置与实际发送链路。
- 移动端显示错位：优先检查是否存在固定高度/overflow 叠加（如底部导航占位、`100vh` 在移动端的差异）。
