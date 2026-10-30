# w8 · m110 — Make incremental views usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary applications can use documented incremental views on managed PostgreSQL. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](../m92/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply pinned CNPG-compatible incremental-view packages](t001.md) | 50m | — |
| t002 | [Reconcile preload and normal application activation](t002.md) | 45m | t001 |
| t003 | [Audit lifecycle and incremental-view metadata restoration](t003.md) | 55m | t002 |
| t004 | [Verify Render capability parity across authoring surfaces](t004.md) | 25m | t003 |
| t005 | [Simplify incremental-view delivery](t005.md) | 15m | t004 |
| t006 | [Test actual immediate maintenance and shared data lifecycles](t006.md) | 75m | t004 |
| t007 | [Accept unchanged native migrations and close milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/285m.

## Definition of done

On supported managed PostgreSQL13–18, ordinary application connections can run the original native CREATE EXTENSION IF NOT EXISTS pg_ivm command with exit0 and create a real IMMV whose contents update immediately through insert, update and delete. Both unchanged clients produce every independently expected Unicode/signed aggregate snapshot without refresh calls. Reviewed immutable compatible operands supply module/control/SQL/helper files and required preload/application setup. Preserve existing extension catalogs, rows/identity, readiness/parameters and shared backup/export/client/lifecycle behavior. Explicitly validate and document required IMMV metadata handling through restore/import/upgrade. Actual image/CNPG/SQL regressions fail before/pass after, post-publication hosted checks and exact cleanup pass, and actual version/architecture/lifecycle limits are recorded. Local references, configuration strings or catalog presence alone cannot close the milestone.

## Source + Goal linkage

- **Source:** [Native functional CLI sweep447](finding.md), [complete evidence](evidence/cli-sweep447/README.md): both native enable commands fail1 with missing control file, both ordinary SQL controls and all8 data snapshots pass; a real source-compiled local IMMV reference succeeds.
- **Goal:** [ADR008 usable hosting](../../../../docs/ADR008-vision.md), [ADR009 managed PostgreSQL](../../../../docs/ADR009-postgresql-management.md), [ADR018 Render compatibility](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** Applications can maintain documented incremental views through their existing managed database and unchanged clients.
- **Why now:** A documented capability is absent from the ordinary operand/configuration. Packaging/preload/activation and metadata restoration require a coordinated repair; a package-presence change alone would leave correctness gaps.
- **Render parity:** Included because the database capability is tenant-visible across REST/GraphQL/MCP/dashboard/Blueprint authoring. Preserve current fields and document Bex's provisioning policy; compose supplier work with m92/m104/m109 without duplicating their distinct consumers.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m110；共用 m92/design.md 的供应与上线门槛。
- **结论来源：** 现有 Render parity 规则和官方 postgresql-extensions 表复核。按各 major 支持范围提供扩展、普通 owner 按需 `CREATE EXTENSION` 是既定目标；不能以只提供 PostGIS 或默认全预装作为无签字的替代。
- **工程方案：** 采用 m92/design.md 的 1(b)+2(b)：按 major 维护兼容 CNPG 的共享 operand，受限 allowlist 激活并保留现有权限边界。pg_similarity 仅 13–15、pg_duckdb 16+、pg_partman 14+ 且无后台 worker、TimescaleDB 仅 Apache 功能；每项原真实 SQL/恢复/版本验收保留。
- **真正剩余门槛：** 新 operand / preload 会重启既有数据库，这是需要用户接受的生产影响；待单独讨论全库滚动切换及窗口。w7/m156 已把 CNPG manager 升级为 1.30.1 且原地更新，不应再声称可以顺带等待尚未发生的 m65 升级。
- **worker 下一步：** 用户明确 rollout 后，先在隔离环境验证各 major、扩展权限/备份恢复和数据保留，再分批 rollout 并逐项验收。当前没有新 rollout 授权，继续 blocked；不把推荐写成用户决定。
