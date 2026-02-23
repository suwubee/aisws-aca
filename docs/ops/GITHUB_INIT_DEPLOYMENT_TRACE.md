# github-init 部署追踪（34007）

> 用途：记录每一步在 `34007` 的可核对事实，支持快速确认“当前跑的是哪版”。

## 字段说明

- `step`: 迭代步骤编号
- `time_utc`: 部署或核对时间（UTC）
- `branch`: 分支名
- `commit`: 提交号（短 hash）
- `pid`: 运行进程 PID
- `binary`: 运行二进制路径
- `port`: 监听端口
- `assets`: 前端主入口 hash
- `status`: `running` / `stopped` / `replaced`
- `notes`: 补充说明

## 记录表

| step | time_utc | branch | commit | pid | binary | port | assets | status | notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| S0 | 2026-02-20 14:29:49 UTC | github-init | 6907c26 | 779197 | `/root/dev/aca/backend/aca-new` | 34007 | `index-BlNZG2dK.js`, `index-CJcjrLAR.css` | running | 基线核对记录（未执行功能发布） |
| S1 | 2026-02-20 16:44:50 UTC | github-init | 6907c26 | 934096 | `/root/dev/aca/backend/aca-new` | 34007 | `index-69AgpyFw.js`, `index-CJcjrLAR.css` | running | 新增运行版本追溯接口与头部版本标签 |
| S2 | 2026-02-20 17:12:00 UTC | github-init | 6907c26 | 936832 | `/root/dev/aca/backend/aca-new` | 34007 | `index-DrJcDMMy.js`, `index-CJcjrLAR.css` | running | 修复会话面板上下文挤压，支持上下文收起与明细稳定滚动 |
| S2.1 | 2026-02-20 17:43:46 UTC | github-init | 6907c26 | 940438 | `/root/dev/aca/backend/aca-new` | 34007 | `index-5AYbHJIN.js`, `index-CJcjrLAR.css` | running | 终端右侧分离“流程轨迹/AI托管控制”tab，任务域新增流程与进度入口 |
