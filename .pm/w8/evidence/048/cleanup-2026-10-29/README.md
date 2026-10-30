# D1：三个旧 QA Free cron 清理验收

- **授权：** 用户 2026-10-29 明确选择 D1 删除。
- **入口：** 正常 `bex services delete <id> --confirm -o json`；先逐 ID GET / preview 验证名称、cron_job、Free 及 bex-canary 归属。
- **结果：** 三个删除命令均 exit 0；每个详情 GET 404，全量分页 service list 不再含这三个 ID。services.used 14→11，terminating 0；剩余 11 个服务 ID 与删除前排除批准目标的基线完全相同，Postgres / Key Value 配额完全不变。
- **凭据和进程：** 使用私有临时 CLI config 选择 workspace，未改用户的原 workspace 配置；临时凭据副本已删除，没有新建或撤销其他 session/grant，CLI 子进程正常退出。临时运行目录在证据归档后删除。
- **边界：** 这是原 paid-only 线上验收之后的存量清理，不是重新部署或购买付费 cron。未宣称新的全局 SQL inventory 或直接逐 Kubernetes child 检查；通过正常删除路径和 quota/terminating 的实际收敛验收。

[完整无凭据回执](receipt.json)。
