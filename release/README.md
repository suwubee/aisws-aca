# AISWS-ACA（AI超站）Release 包（离线一键启动）

该目录用于“可复制/可压缩分发”的运行包：包含后端可执行文件 + 前端静态资源（`static/`）。

## 目录结构（生成后）

- `ai-coding-assistant`：Linux 可执行文件
- `ai-coding-assistant.exe`：Windows 可执行文件
- `static/`：前端静态资源（构建产物）
- `.env.example`：环境变量示例（复制为 `.env` 使用）
- `.aca/`：运行时文件（启动后生成，不进 git）
  - `logs/backend.log`
  - `pids/backend.pid`

## 快速启动

首次运行建议先走一次可视化初始化向导（会输出访问地址/引导配置）：

- Linux：`./start.sh setup`
- Windows：`start.bat setup`

启动服务：

- Linux：`./start.sh`（默认进入分步引导）
- Windows：`start.bat`（默认进入分步引导）

默认端口由 `.env` 控制（未配置则使用默认值 `SERVER_PORT=34007`）。

## License / Brand

- License: Apache-2.0（见 `../LICENSE` / `../NOTICE`）
- Trademarks: `../TRADEMARKS.md`

## 配置

1. 复制 `.env.example` 为 `.env`
2. 按需修改（生产环境务必更换 `JWT_SECRET` / 管理员账号密码 / 数据库路径等）
