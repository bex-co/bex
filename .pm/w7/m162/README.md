# w7 · m162 — Detect missing platform backup telemetry

**Worker:** worker7 **Goal:** Distinguish unknown platform backup health from healthy backups when archiver metrics or primary targets disappear. **Status:** in progress

**Estimate:** 120m implementation; 190m (~3h10m) including standing closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Define independent platform database telemetry expectations](done/t001.md) — **DONE** | 30m | — |
| t002 | [Add per-cluster backup telemetry-loss alerts](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Expose unknown backup health and record deployed coverage](t003.md) | 45m | t002 |
| t004 | [Simplify](t004.md) | 20m | t003 |
| t005 | [Test coverage](t005.md) | 40m | t003, t004 |
| t006 | [Closeout](t006.md) | 10m | t004, t005 |

## Definition of done

- [ ] All four expected platform databases have an independent coverage expectation, with per-cluster alerts for missing targets or missing archiver timestamp series.
- [ ] A single missing database and complete discovery loss both remain visible; healthy siblings cannot suppress either case.
- [ ] Primary failover within the documented grace interval remains quiet, sustained loss alerts, and recovery resolves it.
- [ ] The existing stale-archiver behavior is preserved; panels distinguish unknown from stale/healthy, and dated runtime evidence records deployed coverage.

- [ ] Standing closing tasks and required checks are complete; production-dependent claims have dated runtime evidence for the tested revision.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w7` proposal 2, materialized by `$pm all for w7` on 2026-09-28. Brainstorm source revision: `7f1e49986`; materialization checkout: `97b70fad0`. `deploy/gitops/base/prometheus.yaml` (`cnpg-platform-db`, `PlatformDatabaseBackupStale`), its existing promtool fixtures, [w2/m27](../../w2/done/m27/README.md), and [w7/m156](../blocked/m156/README.md).
- **Evidence:** PlatformDatabaseBackupStale in deploy/gitops/base/prometheus.yaml compares time() against cnpg_pg_stat_archiver_last_archived_time; an absent series produces no alert. The cnpg-platform-db scrape selects only current primaries for bex-db, kratos-db, hydra-db and openfga-db. w7/m156 proves a separate pg_stat_wal collector failure occurs, but does not prove archiver telemetry is currently absent.
- **Goal linkage:** ADR008 deterministic hosting depends on recoverable control-plane data; ADR031 defines the platform backup obligations.
- **Expected outcome:** Each expected platform database reports healthy evidence, stale archiving, or unknown coverage without disappearing while sibling databases stay green.
- **Why now:** The current age-only rule has a concrete absence blind spot, and primary-only discovery changes during failover. Collector problems already have a recorded operational example.
- **Deduplication:** Complements shipped backup/restore and staleness work. Does not redo restore drills or depend on m156's CNPG upgrade; pg_stat_wal and pg_stat_archiver are explicitly separate signals.
- **Render parity omitted:** Internal platform monitoring only; no tenant-facing REST, GraphQL, MCP or dashboard contract change. Grafana here is the internal operations surface.

## Scheduling and boundaries

Approved priority 2 of five: **m161 → m162 → m163 → m164 → m165**. Keep work sequential in w7 because the milestones share Prometheus, Grafana and validation configuration. Priority is scheduling, not an artificial hard dependency. Implementation can begin independently of the other new milestones; m165 reuses m163/t001's Alloy scrape.

Use worker7's isolated dev-7 environment for applicable local exercises, respecting the harness's shared-cluster boundaries. Inspect current deployed state before runtime-dependent work; historical production observations are not claims of a current outage. Establish failure behavior with bounded isolated fixtures and deployed healthy coverage with dated read-only observations.

This filing schedules the approved work; it does not implement or deploy it. Existing blocked work, including m156, m158, 047 and 060, retains its own scope and completion conditions.

## Out of scope

- CNPG upgrades, WAL statistic resets, or changes to backup retention and storage.
- Changing tenant datastore metrics or tenant-facing recovery contracts.
- Treating monitoring absence as proof that stored backups are corrupt or missing.
