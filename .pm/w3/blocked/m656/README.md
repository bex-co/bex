# w3 · m656 — A Postgres database warns that it is almost full, as Render's does

**Worker:** worker3 **Goal:** when a Postgres database has used more than 80% of its storage and has less than 5 GiB left, its Info and Plan pages show Render's "almost full" warning, with a link to the place that grows its storage. GraphQL's `Database` carries Render's `storageUsedPercent`, `storageUsed` and `storageTotal`. **Status:** blocked (t001–t003 and t005–t007 done; t004 walked locally, its production read waits for a pin carrying the code)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's almost-full warning and settle bex's used and total numbers — **DONE** | 20m | — |
| [t002](done/t002.md) | GraphQL `storageUsedPercent`, `storageUsed` and `storageTotal` on `Database` — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | The warning on Info and Plan, in Render's words, with its two links — **DONE** | 1h | t002 |
| [t004](t004.md) | Verify locally by filling a database past the line, and on production | 45m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 40m | t005 |
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
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), and the data-safety promise of a managed database: Render's copy says a full disk breaks reads, writes and backups.
- **Expected outcome:** a user sees that a database is filling before writes start failing and backups stop, on the two pages where they would grow it, as on Render.
- **Why now:** disk autoscaling is opt-in (`docs/render-artifacts/postgres-disk-autoscaling.md`). A database without it fills with no warning in the dashboard today. This is the cheapest item from the brainstorm and the one with the most direct user harm.
- **Render parity task:** included (t005). GraphQL and the dashboard change. REST and MCP stay as Render's public API is, with no usage field, and t005 confirms that.

## Local walk (2026-11-25, t004 steps 1–2)

bex-api and the dashboard ran from this tree on dev-3 (`scripts/dev-env.sh 3 up`), with dev-3's Prometheus and its local-path volume-usage exporter standing in for the kubelet series. A user registered for the walk created a Free Postgres `almost-full` (disposable, deleted after), filled it with an unlogged `generate_series` table and later moved it to Basic 256MB. Google Chrome for Testing, headless through `playwright-core`, at 1440×900 and 390×900.

| Point | Seen | Verdict |
| --- | --- | --- |
| The three fields | `storageTotal` `"1073741824"`, `storageUsed` `"936996864"`, `storageUsedPercent` 87.26 at 87% used; `storageTotal` equal to `datastoreMetrics(name: "disk_capacity")`'s 1073741824 | holds |
| Free (fixed storage) | Render's sentence above Info's and Plan's headings and above Recovery's card, ending "upgrade to a compute plan with more storage", linking `/databases/<id>/plan`; none on Logs, Metrics or Apps; at both widths | holds |
| Basic 256MB (storage can grow) | the same, ending "increase its storage", linking `/databases/<id>/plan#storage`; following it from Info lands on Plan with the Storage row in view (scrolled to it at 390) | holds |
| Render's link on a full load | `/databases/<id>/plan#storage`, `/d/<id>/plan#storage` and `/databases/<id>#storage` each open Plan with the Storage row in view, at both widths | holds |
| Prometheus cut off | the port-forward stopped: the usage fields read null in 0.03 s, bex-api logged the failed read once, and all six pages loaded with no warning and no error | holds |
| Below the line | the table dropped: 7.7% used, and no page draws the warning | holds |

Not walked: the "contact support" ending (storage at its plan's 16 TB ceiling), which the page tests cover.

## Decisions and deviations

- **Placement.** Render also draws the warning on Recovery, under Point-in-Time Recovery and above Export; bex draws it above Recovery's one card, which holds both. On Info and Plan it sits above the page heading, as on Render's Info.
- **The support ending.** Render's `isMaxPlan` has no bex field. bex shows "contact support to create a custom compute plan" when the storage is already at its plan's maximum, as plain text: bex has no support link.
- **Who reads usage.** `storageUsed` and `storageUsedPercent` follow `can_view_activity`, the permission the Metrics page's disk graph reads under, so a Billing member gets null and no warning. `storageTotal` is always there.
- **`storageTotal` with no usage.** The task said all three fields read null when Prometheus fails; `storageTotal` is the database's own size and needs no Prometheus, so only the two usage fields go null.
- **No new instant query.** The usage read reuses the disk graph's range source over the last ten minutes, newest sample per volume, fullest volume (`metrics.CurrentDiskUsed`), which the service-disk sweeper now shares, instead of a second Prometheus query shape.
- **A query of its own.** The dashboard reads the three fields in `DatabaseStorage`, not in the page's `Database` read, so a slow Prometheus never delays a database page.
- Follow-up filed: [w3/130](../../130.md), Render's 1 TB export limit.

## Parked (2026-11-25)

Shipped in `fa9a9913e2`. Everything but the production read is done: t004 step 3 reads `database { storageUsedPercent storageUsed storageTotal }` on production as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`) for a Postgres created for it (disposable), opens its Info and Plan at 1440 and 390 without the warning, and deletes it, once the newest `chore(deploy): pin platform images to <sha>` carries `fa9a9913e2`. Then t008 closes the milestone. Nothing is needed from the user.

