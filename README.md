# AI Coding Assistant

一个可视化多任务并行的AI编程助手管理平台，用于统一管理和监控多个AI编程代理（如Claude Code、Codex、Gemini CLI等）的执行状态。

## 功能特性

- **单用户登录认证** - 简单安全的用户认证
- **终端托管** - PTY会话管理，支持多终端并行
- **Kanban任务看板** - 可视化任务状态管理
- **AI代理检测** - 自动检测Claude Code等AI工具的运行状态
- **自动化审批接口** - 预留OpenAI API集成接口，支持自动化审批

## 技术栈

### 后端
- Go 1.21+
- Fiber (Web框架)
- GORM + SQLite (数据库)
- WebSocket (实时通信)
- PTY (终端托管)

### 前端
- Vue 3 + TypeScript
- Naive UI (组件库)
- Pinia (状态管理)
- xterm.js (终端组件)
- Vite (构建工具)

## 快速开始

### 前置要求
- Go 1.21+
- Node.js 18+
- pnpm (推荐) 或 npm

### 安装和运行

1. **克隆项目**
```bash
git clone <repository-url>
cd ai-coding-assistant
```

2. **后端**
```bash
cd backend
go mod tidy
go run main.go
```
后端服务将在 http://localhost:3007 启动

3. **前端开发模式**
```bash
cd frontend
pnpm install
pnpm dev
```
前端开发服务器将在 http://localhost:3000 启动

4. **构建生产版本**
```bash
cd frontend
pnpm build
```
前端将构建到 `backend/static` 目录

### 默认登录账号
- 用户名: admin
- 密码: admin123

## 项目结构

```
ai-coding-assistant/
├── backend/                 # Go后端
│   ├── main.go             # 入口文件
│   ├── api/                # API路由
│   ├── config/             # 配置管理
│   ├── middleware/         # 中间件
│   ├── model/              # 数据模型
│   ├── service/            # 业务逻辑
│   │   ├── terminal/       # 终端管理
│   │   └── ai_detector/    # AI代理检测
│   ├── utils/              # 工具函数
│   └── static/             # 前端静态文件
├── frontend/               # Vue前端
│   ├── src/
│   │   ├── api/           # API调用
│   │   ├── components/    # 组件
│   │   ├── stores/        # Pinia状态
│   │   ├── views/         # 页面
│   │   └── router/        # 路由
│   └── package.json
└── docs/                   # 文档
    ├── PRD.md             # 产品需求文档
    └── DEVELOPMENT_PLAN.md # 开发计划
```

## API接口

### 认证
- `POST /api/auth/login` - 用户登录
- `POST /api/auth/logout` - 用户登出
- `GET /api/auth/me` - 获取当前用户

### 任务
- `GET /api/tasks` - 获取任务列表
- `POST /api/tasks` - 创建任务
- `GET /api/tasks/:id` - 获取任务详情
- `PUT /api/tasks/:id` - 更新任务
- `DELETE /api/tasks/:id` - 删除任务
- `POST /api/tasks/:id/move` - 移动任务

### 终端
- `GET /api/terminals` - 获取终端列表
- `POST /api/terminals` - 创建终端
- `POST /api/terminals/:id/close` - 关闭终端
- `WebSocket /api/terminal/ws` - 终端WebSocket连接

### 自动化（预留接口）
- `POST /api/automation/analyze` - 分析日志
- `POST /api/automation/execute` - 执行命令
- `GET /api/automation/config` - 获取配置
- `PUT /api/automation/config` - 更新配置

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| SERVER_HOST | 0.0.0.0 | 服务器监听地址 |
| SERVER_PORT | 3007 | 服务器端口 |
| DATABASE_DSN | ./data/aca.db | 数据库文件路径 |
| JWT_SECRET | (内置默认值) | JWT密钥 |
| AUTH_USERNAME | admin | 登录用户名 |
| AUTH_PASSWORD | admin123 | 登录密码 |
| TERMINAL_SHELL | /bin/bash | 默认Shell |
| LOG_LEVEL | info | 日志级别 |

## 开发计划

详见 [开发计划文档](docs/DEVELOPMENT_PLAN.md)

## 参考项目

- [CodeKanban](https://github.com/fy0/CodeKanban) - 终端托管和AI检测参考
- [Claude-Code-Workflow](https://github.com/catlog22/Claude-Code-Workflow) - 多代理工作流参考
- [vibe-kanban](https://github.com/BloopAI/vibe-kanban) - Kanban UI参考

## License

MIT
