# w8 · m112 — Make documented time-series tables usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary applications can use documented Apache-compatible TimescaleDB hypertables and time bucketing. **Status:** blocked — needs the user's managed Postgres extension decision ([decision](../m92/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply compatible pinned TimescaleDB operands per supported major](t001.md) | 50m | — |
| t002 | [Configure Apache-mode loading and appropriate plan resource bounds](t002.md) | 40m | t001 |
| t003 | [Make normal migrations and own-table time-series SQL usable](t003.md) | 30m | t002 |
| t004 | [Audit data lifecycles and shared serving/export consumers](t004.md) | 45m | t003 |
| t005 | [Verify documented Render parity across authoring surfaces](t005.md) | 25m | t004 |
| t006 | [Simplify time-series operand and configuration delivery](t006.md) | 15m | t005 |
| t007 | [Test real hypertables, UTC bucketing and shared lifecycles](t007.md) | 75m | t005 |
| t008 | [Accept both native migrations and close milestone](t008.md) | 20m | t006, t007 |

**Total:**8 tasks/300m.

## Definition of done

On existing supported managed PostgreSQL13–18, with compatible extension releases reviewed per major, ordinary applications can run both unchanged native CREATE EXTENSION IF NOT EXISTS timescaledb commands with exit0 and use real basic hypertables/time bucketing. Actual physical partition and independent UTC/Unicode/signed data-change results agree. Reviewed immutable CNPG-compatible packages and required loading preserve pg_stat_statements, existing extensions, data, readiness, parameters and appropriate plan bounds. Keep Render's Community-feature exclusion explicit. Verify and document existing-parent, suspend/resume, backup/export/import/restore and compatible-upgrade behavior, including actual hypertable metadata and continued writes. Actual selected-operand/CNPG SQL regressions fail before/pass after; fresh hosted acceptance, required checks and exact cleanup pass. A diagnostic image, source/config strings or catalog presence alone cannot close the work. Record actual version/architecture/lifecycle limits.

## Source + Goal linkage

- **Source:** [Functional native CLI sweep453](finding.md), [complete evidence](evidence/cli-sweep453/README.md): both migrations fail1 with package/control-file unavailability while both ordinary SQL controls and a real Apache-mode hypertable reference pass.
- **Goal linkage:** [ADR008 usable hosting](../../../../docs/ADR008-vision.md), [ADR009 managed PostgreSQL](../../../../docs/ADR009-postgresql-management.md), [ADR018 Render compatibility](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** Applications can use documented basic time-series tables and hourly analytics through their existing managed database and unchanged clients.
- **Why now:** A documented module is absent at the normal application boundary. Packaging, loading, plan bounds and metadata-aware lifecycle procedures need coordinated repair rather than a presence-only supplier patch.
- **Render parity:** Included because the capability is tenant-visible across REST/GraphQL/MCP/dashboard/Blueprint database authoring. Preserve current fields/errors/selectors, the closed13–18 set and the Community-feature exclusion. Coordinate common suppliers with m92/m104/m109/m110/m111 without duplicating their distinct consumers.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m112；共用 m92/design.md 的供应与上线门槛。
- **结论来源：** 现有 Render parity 规则和官方 postgresql-extensions 表复核。按各 major 支持范围提供扩展、普通 owner 按需 `CREATE EXTENSION` 是既定目标；不能以只提供 PostGIS 或默认全预装作为无签字的替代。
- **工程方案：** 采用 m92/design.md 的 1(b)+2(b)：按 major 维护兼容 CNPG 的共享 operand，受限 allowlist 激活并保留现有权限边界。pg_similarity 仅 13–15、pg_duckdb 16+、pg_partman 14+ 且无后台 worker、TimescaleDB 仅 Apache 功能；每项原真实 SQL/恢复/版本验收保留。
- **真正剩余门槛：** 新 operand / preload 会重启既有数据库，这是需要用户接受的生产影响；待单独讨论全库滚动切换及窗口。w7/m156 已把 CNPG manager 升级为 1.30.1 且原地更新，不应再声称可以顺带等待尚未发生的 m65 升级。
- **worker 下一步：** 用户明确 rollout 后，先在隔离环境验证各 major、扩展权限/备份恢复和数据保留，再分批 rollout 并逐项验收。当前没有新 rollout 授权，继续 blocked；不把推荐写成用户决定。
