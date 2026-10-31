# w8 · m104 — Supply pg_partman for ordinary partition migrations

**Worker:** worker8 **Goal:** app owners can install and use the documented partition-management extension. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](../../m92/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply pinned SQL-only pg_partman in compatible serving operands](t001.md) | 70m | — |
| t002 | [Audit shared image selection and existing database lifecycles](t002.md) | 30m | t001 |
| t003 | [Verify documented Render behavior across database consumers](t003.md) | 20m | t002 |
| t004 | [Simplify the changed extension supply](t004.md) | 15m | t003 |
| t005 | [Test real extension installation partitions and retained data](t005.md) | 45m | t003 |
| t006 | [Accept original native migrations and close the milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/200m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep428/README.md).

## Definition of done

Compatible reviewed immutable SQL-only pg_partman supply for PostgreSQL14..18 permits normal app-owner on-demand installation and real partition generation, routing and manual maintenance with the background worker disabled. Maintained real CNPG/image/SQL proof covers supported architecture/base/patch choices, default/custom identities/schemas, controlled existing-parent transition and retained data/extension configuration. PG13 and existing vector/pg_stat_statements, restore/upgrade/reconcile/suspend behavior remain correct. After publication both clients' unchanged permitted Free migrations and actual partition operations pass, exact cleanup preserves baselines/quota and removes own material, and unexercised limits remain explicit. Implementation alone does not close this milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep428,2026-10-17: both native SQL clients fail pg_partman installation on a fresh owned Free PostgreSQL18.6, with matching catalogs and healthy extension/partition controls. A separate ordinary-owner localPG17 SQL-only reference works.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md), [managed PostgreSQL ADR009](../../../../docs/ADR009-postgresql-management.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md): supported app migrations should work through normal managed database connections.
- **Expected outcome:** Partition-management migrations and maintenance run without server-file workarounds or changed app authority.
- **Why now:** Two original failures and actual missing catalog entry establish a distinct supply gap; no existing pg_partman work was found. Coordinate the shared image rollout with [m92](../../m92/README.md); The m92 plan does not establish pg_partman supply in its future candidate images.
- **Render parity:** Included because shared database image supply reaches native/REST/GraphQL/MCP/Blueprint/dashboard consumers. No extension-management product or automatic background-worker service is introduced.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m104；共用 m92/design.md 的供应与上线门槛。
- **结论来源：** 现有 Render parity 规则和官方 postgresql-extensions 表复核。按各 major 支持范围提供扩展、普通 owner 按需 `CREATE EXTENSION` 是既定目标；不能以只提供 PostGIS 或默认全预装作为无签字的替代。
- **工程方案：** 采用 m92/design.md 的 1(b)+2(b)：按 major 维护兼容 CNPG 的共享 operand，受限 allowlist 激活并保留现有权限边界。pg_similarity 仅 13–15、pg_duckdb 16+、pg_partman 14+ 且无后台 worker、TimescaleDB 仅 Apache 功能；每项原真实 SQL/恢复/版本验收保留。
- **真正剩余门槛：** 新 operand / preload 会重启既有数据库，这是需要用户接受的生产影响；待单独讨论全库滚动切换及窗口。w7/m156 已把 CNPG manager 升级为 1.30.1 且原地更新，不应再声称可以顺带等待尚未发生的 m65 升级。
- **worker 下一步：** 用户明确 rollout 后，先在隔离环境验证各 major、扩展权限/备份恢复和数据保留，再分批 rollout 并逐项验收。当前没有新 rollout 授权，继续 blocked；不把推荐写成用户决定。


## 用户决定（2026-10-29，`/unblock w8` 主题 2）

- **涉及 ID：** w8/m92、m104、m109、m110、m111、m112
- **结论来源：** 用户明确决定，补充上一条「当前没有新 rollout 授权」的复核记录。用户先问「Render 是怎么做的」，然后选择方案；之后指出「Render 公开的只是按库 CREATE EXTENSION 这一层契约，背后靠白名单机制放行非 trusted 扩展，需要写 ADR」。
- **结论：** 采用 [ADR092](../../../../docs/ADR092-postgres-extension-supply.md)：每个大版本一个 bex 自建 operand 镜像，内置全部 6 个扩展；owner 按需 `CREATE EXTENSION`；用封闭白名单（supautils 或 pgextwlist，按 ADR 的 5 条性质验证后选定）放行非 trusted 扩展。这与上一条记录的工程方案 1(b)+2(b) 一致。
- **重启已获批准：** 用户批准为采用新镜像对全部托管 Postgres pod **滚动重启一次**。CNPG 1.30.1 已由 w7/m156 原地升级完成，不再与 m65 合并。上一条的「剩余门槛：需要用户接受重启」已解除。
- **剩余门槛：** 无用户门槛。本项等 [w8/m92/t008](../../m92/t008.md) 的共享镜像和白名单落地（上游依赖），之后恢复，完成本扩展的回归和线上验收。
