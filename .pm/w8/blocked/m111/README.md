# w8 · m111 — Make documented text similarity usable in managed Postgres

**Worker:** worker8 **Goal:** ordinary applications can use documented PostgreSQL13–15 text-similarity functions and operators. **Status:** blocked — waits for [w8/m92/t008](../../m92/t008.md) to deliver qualified per-major operand images and the allowlist; the single ADR092 D4 rolling restart is already approved; no user-decision gate

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply pinned pg_similarity for the documented PostgreSQL majors](t001.md) | 40m | — |
| t002 | [Make text matching usable through normal application connections](t002.md) | 35m | t001 |
| t003 | [Audit the shared supplier and database lifecycles](t003.md) | 35m | t002 |
| t004 | [Verify Render parity across database authoring surfaces](t004.md) | 20m | t003 |
| t005 | [Simplify managed text-matching delivery](t005.md) | 15m | t004 |
| t006 | [Test actual similarity functions, operators and lifecycle neighbors](t006.md) | 55m | t004 |
| t007 | [Accept both native migrations and close milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/220m.

## Definition of done

On the documented managed PostgreSQL13–15 operands, both unchanged native clients' original CREATE EXTENSION IF NOT EXISTS pg_similarity commands exit0 and normal applications execute real similarity functions/operators with independently expected results. Reviewed immutable CNPG-compatible packages and managed activation preserve existing vector, pg_stat_statements, data, readiness and shared export/backup/client behavior. Preserve the13–15 contract boundary and explicitly verify/document existing-parent, restore/import/export/upgrade and major-transition constraints. Actual image/CNPG/SQL regressions fail before/pass after; fresh hosted acceptance, required checks and exact cleanup pass. Record actual version/architecture/lifecycle limits. Local compilation or catalog presence alone does not meet this definition.

## Source + Goal linkage

- **Source:** [Functional native CLI sweep450 finding](finding.md), [complete evidence](evidence/cli-sweep450/README.md): both migrations exit1 with missing package/control file, both ordinary SQL controls and a real source-compiled local function/operator reference pass.
- **Goal linkage:** [ADR008 usable hosting](../../../../docs/ADR008-vision.md), [ADR009 managed PostgreSQL](../../../../docs/ADR009-postgresql-management.md), [ADR018 Render compatibility](../../../../docs/ADR018-render-parity.md).
- **Expected outcome:** Applications can use documented text-similarity migrations and SQL through their existing database and unchanged clients.
- **Why now:** Both native consumers connect and execute ordinary SQL, but the documented module is absent. Package delivery, normal usability and lifecycle/major compatibility require coordinated work.
- **Render parity:** Included because the database capability is tenant-visible across REST/GraphQL/MCP/dashboard/Blueprint authoring. Preserve fields/errors/selectors and coordinate the existing shared supplier work from m92/m104/m109/m110; those separate modules do not provide this consumer.

## 解除阻塞复核（2026-10-29，基于 `99ddf393061a`）

- **涉及 ID：** w8/m111；共用 m92/design.md 的供应与上线门槛。
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

## Shared operand qualification checkpoint — 2026-10-31

The corrected shared operand/restore source and maintained tests shipped in `fa85c4418`. Historical SQL results remain partial evidence: the renewed complete ordinary-owner restore and original IMMV maintenance checks, actual CNPG 1.30.1 with Free resources, every supported major on both architectures, published-artifact review/adoption and this milestone's original hosted SQL acceptance remain open. The dependency is qualified [w8/m92/t008](../../m92/t008.md) output, not another product or restart decision. Preserve the existing ADR092 D4 single rolling-restart approval, every original DoD and all prior evidence.
