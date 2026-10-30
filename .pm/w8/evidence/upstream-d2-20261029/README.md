# D2：有帮助的 Render CLI 修复已提交

- **授权：** 用户明确允许提交对 Render CLI 有帮助的修复（2026-10-29）。
- **结果：** 19 个 board ID 整理为 17 个独立 PR；079+118 共用子进程流处理，m68+108 共用服务输入归一化与测试入口。所有 17 个 PR 已经实际公开，remote head 和文件清单逐项匹配已测试提交，均为 open。
- **上游基线：** `c31e586eb6f38138af3cc986b079e46a78c61de7`；在独立 checkout 审核，近期全状态 58 项 issue/PR 索引未发现同一修复。现有 #35 的 clone/IP-plan 限制与本批的跨类型默认字段过滤是不同原因。
- **验证：** 原版与各候选的 57 个默认开发包均通过；两组组合分支也通过完整开发测试。git whitespace、gofmt、上游文件名检查通过。golangci-lint 固定 v2.14.0；原版完整 lint 有 92 项既有发现，候选相对基线新增为 0。079 的 stdin/diagnostics、084 的 API 状态、103 的精确路径还做了去掉修复必失败、恢复通过的行为反向验证。
- **范围：** PR 只改 Go 产品代码/测试，没有 workflow、生成 client 或依赖修改。GitHub 连接器无权向 Render 开 PR，因此使用已登录 puncsky 的 GitHub CLI；为避免旧 fork 基线引入 OAuth workflow-scope 拒绝，用连接器从已有上游提交建立 feature 分支后再做 Go-only 快进。没有放宽授权、强推、修改 workflow 或 bex main。
- **剩余门槛：** upstream review/merge/release、Bex 审阅 pin、单独授权 CLI release 和原 native/shared-surface/cleanup 验收。提交不等于已发布，本批 19 项继续 blocked。

| Board ID | 修复 | PR | 证据 |
| --- | --- | --- | --- |
| w8/079, w8/118 | 非交互 psql 输入与成功诊断 | [#59](https://github.com/render-oss/cli/pull/59) | [补丁/验证](079/README.md) |
| w8/084 | Postgres restart 失败不能误报成功 | [#60](https://github.com/render-oss/cli/pull/60) | [补丁/验证](084/README.md) |
| w8/103 | secret-file 精确读取文件路径 | [#61](https://github.com/render-oss/cli/pull/61) | [补丁/验证](103/README.md) |
| w8/077 | 重复 StringArray 参数字节保真 | [#62](https://github.com/render-oss/cli/pull/62) | [补丁/验证](077/README.md) |
| w8/082 | 日志筛选保留小数时间边界 | [#63](https://github.com/render-oss/cli/pull/63) | [补丁/验证](082/README.md) |
| w8/072 | 镜像 cron 的命令更新 | [#64](https://github.com/render-oss/cli/pull/64) | [补丁/验证](072/README.md) |
| w8/076 | IP 描述按 Unicode 字符计数 | [#65](https://github.com/render-oss/cli/pull/65) | [补丁/验证](076/README.md) |
| w8/085 | 交互错误面板显示底层原因 | [#66](https://github.com/render-oss/cli/pull/66) | [补丁/验证](085/README.md) |
| w8/086 | 空项目筛选保持空范围 | [#67](https://github.com/render-oss/cli/pull/67) | [补丁/验证](086/README.md) |
| w8/087 | 清除数据连接器的项目范围 | [#68](https://github.com/render-oss/cli/pull/68) | [补丁/验证](087/README.md) |
| w8/091 | SSH 项目筛选返回服务选择 | [#69](https://github.com/render-oss/cli/pull/69) | [补丁/验证](091/README.md) |
| w8/093 | 可解析形似 ID 的项目/环境名 | [#70](https://github.com/render-oss/cli/pull/70) | [补丁/验证](093/README.md) |
| w8/107 | 显式输出格式覆盖无效环境默认值 | [#71](https://github.com/render-oss/cli/pull/71) | [补丁/验证](107/README.md) |
| w8/m57 | 表单保留字符串列表元素 | [#72](https://github.com/render-oss/cli/pull/72) | [补丁/验证](m57/README.md) |
| w8/m60 | 复制命令保留搜索和时间参数 | [#73](https://github.com/render-oss/cli/pull/73) | [补丁/验证](m60/README.md) |
| w8/075 | 克隆过滤目标不支持的默认字段 | [#74](https://github.com/render-oss/cli/pull/74) | [补丁/验证](075/README.md) |
| w8/m68, w8/108 | 命令文本保真与克隆显式清空 | [#75](https://github.com/render-oss/cli/pull/75) | [补丁/验证](m68/README.md) |

[远端 head 与文件清单逐项核对](remote-verification.json)。

**清理：** 临时 checkout / worktree 和测试 config 没有使用生产凭据；完成归档后删除本轮临时 checkout。公共 fork 分支按 PR 评审需要保留。D3 没有创建数据库或 workspace。
