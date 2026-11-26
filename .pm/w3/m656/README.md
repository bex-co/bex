# w3 · m656 — A Postgres database warns that it is almost full, as Render's does

**Worker:** worker3 **Goal:** when a Postgres database has used more than 80% of its storage and has less than 5 GiB left, its Info and Plan pages show Render's "almost full" warning, with a link to the place that grows its storage. GraphQL's `Database` carries Render's `storageUsedPercent`, `storageUsed` and `storageTotal`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's almost-full warning and settle bex's used and total numbers | 20m | — |
| [t002](t002.md) | GraphQL `storageUsedPercent`, `storageUsed` and `storageTotal` on `Database` | 1h30m | t001 |
| [t003](t003.md) | The warning on Info and Plan, in Render's words, with its two links | 1h | t002 |
| [t004](t004.md) | Verify locally by filling a database past the line, and on production | 45m | t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 40m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Total: **8 tasks, about 5h**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-25 without sign-in (`index-D5V4ZgbE.js`, sha256 `8d15889ace2d…`). Chunk names change with every Render deploy, so find the code by its strings, not by file name.
- **Data:** the `database(id)` query in the `database-*.js` chunk asks for `storageUsedPercent`, `storageUsed` and `storageTotal` beside `...databaseFields`.
- **Rule:** with `GiB = 1073741824`, the warning shows when `storageUsedPercent > 80` and `(1 − storageUsedPercent / 100) × storageTotal / GiB < 5`; when either number is missing it does not show.
- **Copy:** "Your database is almost full. This may lead to errors reading from or writing to your database and can prevent automated backups…", then a link that either increases its storage (`/plan#storage`) or upgrades to a compute plan with more storage; t001 records the full sentence and both link variants.
- **Placement:** the same component renders at the top of the database's Info page and its Plan page.
- **Public API:** Render's public REST `postgres` and `postgresDetail` objects carry only `diskSizeGB` and `diskAutoscalingEnabled` (`lego/backend/internal/api/openapi/render-public-api-1.json`), and Render's MCP has no usage field. The numbers are a dashboard GraphQL read only.

## bex today

Verified 2026-11-25 on main `c196617a8b`.

- The dashboard has no such warning (`dashboard/src` has no "almost full" and no `storageUsed`). A database's usage appears only in its Metrics charts.
- The Metrics page's disk numbers come from `lego/backend/internal/metrics/datastore.go`:
  - used is `kubelet_volume_stats_used_bytes` (`NewPrometheusDiskUsageSource` in `source.go`);
  - capacity is the logical, billed size from the spec (`logicalDiskCapacitySeries`, w4/m91), not `kubelet_volume_stats_capacity_bytes`, because the physical PVC has a fixed 10 GiB floor on Hetzner and made "an 81%-full 1 GB database read 8% full".
- The operator's disk autoscaler reads the fullest CNPG instance PVC (`DatabaseDiskUsage` in `lego/operator/internal/controller/database_autoscale.go`, w6/m467): an HA standby and each replica hold a full copy. The backend cannot import the operator (`operator → types ← backend`), so it needs its own instant query.
- GraphQL's `Database` type is built in `lego/backend/internal/postgres/graphql.go` (`PostgresView`, `diskSizeGB`).
- The Plan page has `#plan` and `#high-availability` sections; the storage size control sits inside the plan section (`database-plan-section.tsx`, `database-storage-size`) and has no `#storage` anchor. `features/databases/lib/storage.ts` has `planStorageFixed`.

## Definition of done

1. On a local dev-3 Postgres filled past 80% of its storage with less than 5 GiB free, Info and Plan both show Render's warning at the top. Its link opens Plan scrolled to the storage control, or to the plan picker when the plan's storage is fixed.
2. Below the line, the warning does not show. When Prometheus cannot answer, it also does not show, and the database's pages still load with no error.
3. `database { storageUsedPercent storageUsed storageTotal }` returns the fullest instance's usage for an HA database or one with replicas, and `storageTotal` agrees with the Metrics page's capacity. REST `GET /v1/postgres/{id}` and MCP's Postgres objects are unchanged.
4. On production, once a pin carries the milestone, the three fields read for a Postgres in `bex-canary` (`tea-daif693dqjvc73e7as3g`) created for the check (disposable), and Info and Plan load without the warning.
5. ADR018's Postgres file has a row for the warning, and the m641 record `docs/render-artifacts/postgres-dashboard.md` names it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-25), item 2, checked against Render's bundle and bex's code before filing. The brainstorm's conflict check against w1/m645–m648 found no overlap. w1/m645's operation ledger points the `database` query's three storage fields at this milestone (added to its t002 when this was filed).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), and the data-safety promise of a managed database: Render's copy says a full disk breaks reads, writes and backups.
- **Expected outcome:** a user sees that a database is filling before writes start failing and backups stop, on the two pages where they would grow it, as on Render.
- **Why now:** disk autoscaling is opt-in (`docs/render-artifacts/postgres-disk-autoscaling.md`). A database without it fills with no warning in the dashboard today. This is the cheapest item from the brainstorm and the one with the most direct user harm.
- **Render parity task:** included (t005). GraphQL and the dashboard change. REST and MCP stay as Render's public API is, with no usage field, and t005 confirms that.
