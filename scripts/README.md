# Scripts

常用脚本：

- `./scripts/quickstart.sh`：推荐的“生产式本地运行”（生成 `.env`、构建前端到 `backend/static`、启动后端二进制）
- `./scripts/quickstart.sh wizard`：初始化向导（可视化 Web 引导，一次性完成配置并启动）
- `./scripts/start.sh`：开发便捷脚本（前后端分别启动/停止/查看状态/日志）
- `./scripts/start.bat`：Windows 启动脚本（内置前端模式无需 Node；提供分步引导/端口接管/URL 输出）
- `./scripts/build_release.sh`：生成 `release/` 分发目录（后端二进制 + 前端静态资源 + 一键启动脚本）

详细说明见：`docs/engineering/SCRIPTS.md`
