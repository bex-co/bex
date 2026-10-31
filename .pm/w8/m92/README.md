# w8 · m92 — Make PostGIS usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary managed PostgreSQL app connections can complete PostGIS migrations and geospatial queries. **Status:** in progress — t008 implemented and verified locally on 13–18 (2026-10-31); publication via `pg-operand-image.yml`, then t001 pins and rolls out (ADR092 D4)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t008 | [Build the per-major operand image and extension allowlist (ADR092)](t008.md) | 240m | — |
| t001 | [Supply compatible pinned PostGIS operand images](t001.md) | 70m | t008 |
| t002 | [Reconcile PostGIS in the existing application Database child](t002.md) | 45m | t001 |
| t003 | [Audit shared image, extension and lifecycle callers](t003.md) | 35m | t002 |
| t004 | [Verify Render parity across database consumers](t004.md) | 20m | t003 |
| t005 | [Simplify the changed supply and reconciliation code](t005.md) | 15m | t004 |
| t006 | [Test actual PostGIS supply activation and existing data](t006.md) | 45m | t004 |
| t007 | [Accept fresh native migrations and close the milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/250m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep403/README.md).

## Definition of done

Reviewed compatible PostGIS operand supply plus retained CNPG application-Database reconciliation makes the original migration and real spatial data/distance/GiST operations work under normal app identities. Maintained real CNPG/image/SQL checks cover supported major/architecture/patch boundaries, default/custom names and a controlled owned pre-fix existing-parent transition without data/extension loss. Vector, pg_stat_statements, hstore, export/restore compatibility, retention, suspension and optional extension-status behavior remain correct. After publication both native clients' fresh permitted Free migrations and spatial queries pass; exact cleanup preserves baselines/quota and removes own connection material. Distinguish integration from hosted acceptance and retain unverified limits. Code alone does not close the milestone.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep403,2026-10-17: both unchanged native psql clients fail the original PostGIS migration on one fresh Bex-created Free PostgreSQL18.6 database; independent SQL/hstore/point controls pass and four catalogs remain identical.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [managed data ADR009](../../../docs/ADR009-postgresql-management.md) and [ADR018](../../../docs/ADR018-render-parity.md): supported geospatial migrations should work through ordinary managed database connections.
- **Expected outcome:** Applications can store/query geospatial data without installing server libraries themselves or changing authority/CLI requests.
- **Why now:** Actual catalog absence and two matching native errors establish this separate gap while vector and ordinary extension/SQL behavior already work. No PostGIS repair is scheduled in the searched board/history.
- **Render parity:** Included because the shared managed database image/extension change reaches REST/GraphQL/MCP/Blueprint/dashboard/native consumers. Core PostGIS activation only; no general extension catalog/API product.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m92；共用 m92/design.md 的供应与上线门槛。
- **结论来源：** 现有 Render parity 规则和官方 postgresql-extensions 表复核。按各 major 支持范围提供扩展、普通 owner 按需 `CREATE EXTENSION` 是既定目标；不能以只提供 PostGIS 或默认全预装作为无签字的替代。
- **工程方案：** 采用 m92/design.md 的 1(b)+2(b)：按 major 维护兼容 CNPG 的共享 operand，受限 allowlist 激活并保留现有权限边界。pg_similarity 仅 13–15、pg_duckdb 16+、pg_partman 14+ 且无后台 worker、TimescaleDB 仅 Apache 功能；每项原真实 SQL/恢复/版本验收保留。
- **真正剩余门槛：** 新 operand / preload 会重启既有数据库，这是需要用户接受的生产影响；待单独讨论全库滚动切换及窗口。w7/m156 已把 CNPG manager 升级为 1.30.1 且原地更新，不应再声称可以顺带等待尚未发生的 m65 升级。
- **worker 下一步：** 用户明确 rollout 后，先在隔离环境验证各 major、扩展权限/备份恢复和数据保留，再分批 rollout 并逐项验收。当前没有新 rollout 授权，继续 blocked；不把推荐写成用户决定。


## 用户决定（2026-10-29，`/unblock w8` 主题 2）

- **涉及 ID：** w8/m92、m104、m109、m110、m111、m112
- **结论来源：** 用户明确决定，补充上一条「当前没有新 rollout 授权」的复核记录。用户先问「Render 是怎么做的」，然后选择方案；之后指出「Render 公开的只是按库 CREATE EXTENSION 这一层契约，背后靠白名单机制放行非 trusted 扩展，需要写 ADR」。
- **结论：** 采用 [ADR092](../../../docs/ADR092-postgres-extension-supply.md)：每个大版本一个 bex 自建 operand 镜像，内置全部 6 个扩展；owner 按需 `CREATE EXTENSION`；用封闭白名单（supautils 或 pgextwlist，按 ADR 的 5 条性质验证后选定）放行非 trusted 扩展。这与上一条记录的工程方案 1(b)+2(b) 一致。
- **重启已获批准：** 用户批准为采用新镜像对全部托管 Postgres pod **滚动重启一次**。CNPG 1.30.1 已由 w7/m156 原地升级完成，不再与 m65 合并。上一条的「剩余门槛：需要用户接受重启」已解除。
- **剩余门槛：** 无用户门槛。worker 从 [t008](t008.md) 开始：先在隔离环境按 ADR092 D3 验证各大版本，再按 D4 低峰滚动并逐项验收。
