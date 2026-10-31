# w8 · m109 — Make documented DuckDB analytics usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary applications can use the documented PostgreSQL16+ DuckDB capability. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](../../m92/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a compatible pinned managed-Postgres DuckDB package](t001.md) | 65m | — |
| t002 | [Reconcile DuckDB preload and ordinary application usability](t002.md) | 50m | t001 |
| t003 | [Audit shared database lifecycle and other extension suppliers](t003.md) | 40m | t002 |
| t004 | [Verify Render parity across database authoring surfaces](t004.md) | 25m | t003 |
| t005 | [Simplify managed DuckDB delivery](t005.md) | 15m | t004 |
| t006 | [Test real managed analytics and missing-package regressions](t006.md) | 75m | t004 |
| t007 | [Accept fresh native analytics and close milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/290m.

## Definition of done

On supported managed PostgreSQL16–18, an ordinary application connection can execute the unchanged native CREATE EXTENSION IF NOT EXISTS pg_duckdb command with exit0 and use DuckDB for real own-table analytics. Both native clients return independently expected Unicode/signed grouped totals, with actual DuckDB engine-plan evidence. Selected operands contain a reviewed immutable compatible package/dependency closure, required preload/application configuration and appropriate plan resource bounds. Existing vector/pg_stat_statements/normal SQL, data/identity, parameters, backup/export/client and lifecycle behavior are preserved. Actual image/CNPG regressions fail before/pass after; post-publication fresh hosted checks, exact cleanup and required validation pass. Local references, code/config alone and a catalog-presence check cannot close this milestone. Record actual architecture/version/topology/external-feature limits.

## Source + Goal linkage

- **Source:** [Functional native CLI sweep444 finding](finding.md) and [complete evidence](evidence/cli-sweep444/README.md): both native enable commands fail1, both ordinary SQL controls pass and four catalogs omit the package; a real local DuckDB reference succeeds.
- **Goal linkage:** [ADR008 AI-native usable hosting](../../../../docs/ADR008-vision.md), [ADR009 managed PostgreSQL](../../../../docs/ADR009-postgresql-management.md), [ADR018 Render compatibility](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** Applications can use the documented PostgreSQL16+ DuckDB analytics capability through the existing managed database and unchanged clients.
- **Why now:** The standard operand/configuration has no supplier for a documented extension. Packaging alone would leave preload and application use incomplete; explicit lifecycle and shared-export checks prevent the repair from disrupting existing databases.
- **Render parity:** Included because this changes the tenant-visible database capability across REST/GraphQL/MCP/dashboard/Blueprint authoring; preserve existing fields and document any preenabled Bex policy. Coordinate with m92/m104 supplier work while retaining their distinct consumers.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m109；共用 m92/design.md 的供应与上线门槛。
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
