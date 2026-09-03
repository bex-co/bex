# w4 · m91 — Managed-datastore disk metrics show the logical (billed) capacity, not the physical PVC floor

**Worker:** worker4 **Goal:** the Metrics disk panel judges fullness against the datastore's billed/autoscaled disk size, so a user near their quota sees that they are near it — instead of the fixed 10 GiB physical PVC that makes an 81%-full 1 GB database look 8% full **Status:** done

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Backend: `disk_capacity` returns the datastore's logical provisioned size, not `kubelet_volume_stats_capacity_bytes` | 50m | — — **DONE**     |
| t002 | Dashboard: DatastoreMetricsPanel capacity label + reference line reflect the logical size                    | 30m | t001 — **DONE**  |
| t003 | KeyValue parity (+ scope compute-service disks)                                                              | 30m | t001 — **DONE**  |
| t004 | Render parity                                                                                                | 25m | t001, t002, t003 — **DONE** |
| t005 | Simplify                                                                                                     | 20m | t004 — **DONE**  |
| t006 | Test coverage                                                                                                | 40m | t004 — **DONE**  |
| t007 | Closeout                                                                                                     | 15m | t005, t006 — **DONE** |

## Outcome (2026-09-02)

Shipped. The user-facing `disk_capacity` DatastoreMetric now reports the datastore's **logical/billed** disk size instead of `kubelet_volume_stats_capacity_bytes` (the physical PVC, a fixed 10 GiB Hetzner Cloud-Volume floor), so an 81%-full 1 GB Postgres reads ~81% of ~1 GiB instead of ~8% of 10 GiB.

- **Single source of truth.** New `tiers.Postgres.EffectiveStorageGB` / `tiers.Valkey.EffectiveStorageGB` (`lego/types/tiers/tiers.go`) compute `max(plan floor, spec.storageGB, status.allocatedStorageGB)` — the billed/autoscaled high-water. `postgres.DatabaseStorageHighWater` (which drives Details "Storage") now delegates to it, so the metric and the Details/autoscaling numbers **cannot drift**. `metrics.DatastoreMetrics` (`datastore.go`) resolves the logical GB per kind and returns a flat, config-shaped `disk_capacity` series (`logicalDiskCapacitySeries`, `StorageGB × 1 GiB`), the same single-point way `cpu_limit`/`memory_limit` are — needing no Prometheus source. **Service disks folded in** (t003): a service's `spec.disk.sizeGB` high-water shares the same split, so `kind=service` capacity is now logical too.
- **`disk` USED is unchanged** — still `kubelet_volume_stats_used_bytes`; only the capacity denominator changed. `NewPrometheusDiskUsageSource` was simplified to used-only and the now-dead `DiskUsageRequest.Metric`/capacity branch removed.
- **All three surfaces agree.** REST `/v1/metrics/disk-capacity`, GraphQL `datastoreMetrics(DISK_CAPACITY)`, and MCP `get_datastore_metrics` all route through the one verb — pinned byte-identical by `TestDiskCapacityIdenticalAcrossRESTGraphQLMCP`.
- **Ops alert stays physical.** `PersistentVolumeFillingUp` deliberately keeps `kubelet_volume_stats_capacity_bytes` (operations care about the real volume). A new promtool unit test (`deploy/gitops/base/rules/alerts_test.yml`) pins that it fires on the physical series and that a managed 1 GB Postgres on the 10 GiB floor (8% physical) does **not** fire it. Validated locally with promtool 2.55.1 (`promtool test rules` SUCCESS, 43 rules).
- **Dashboard** (t002): the panel already renders `latestValue(disk_capacity)` as the "Capacity" label + reference line, so it reflects the logical value automatically; the misleading en/zh locale annotation ("the PVC's total capacity") was corrected to "the datastore's billed/provisioned disk size". Regression test added.
- **Render parity** recorded in `docs/ADR018-render-parity.md` (Metrics row): matches Render charting managed-Postgres disk usage against the plan's disk size; ops-alert divergence noted as deliberate.
- **Suites:** operator `make test`, backend `go test ./...` (61 pkgs), `make lint` (0 issues, all four modules + dead-code), dashboard typecheck + lint + `yarn test` (2871), and the promtool alert pack — all green.

**Live-verification residual (carried, same constraint as m28/m31/m32/m81):** the DoD's live checks against the specific prod databases (`dpg-d9nqg95cavls73fp8m20` 1 GB → ~1 GiB, `dpg-d9rs3ee0ccis738kc7c0` 5 GB → ~5 GiB) and a live KeyValue instance were **not** run this session — no cluster/prod access. The behavior is pinned exhaustively offline (logical size = `max(plan, spec, allocated)` for Postgres 1/5 GB, spec-override, and autoscaled high-water; KeyValue; service disk; cross-surface identity; ops-alert stays physical). The remaining step is an operator running the Metrics → Disk panel against those live datastores.

## Definition of done

Each bullet is a click or command the next person can repeat and watch succeed or fail.

- **1 GB Postgres reads ~1 GiB capacity.** On a `basic-256mb` Postgres (logical `diskSizeGB=1`, e.g. `beancount-forum-db` / `dpg-d9nqg95cavls73fp8m20`), the Metrics → Disk panel's "Capacity" and reference line read **~1 GiB** — the same value the same page's Details "Storage" and autoscaling ("1 GB current") show — and the used/capacity fullness (~828 MiB / 1 GiB ≈ 81%) matches the "At 90% full, storage grows…" autoscale basis. It must **not** read 10 GiB.
- **5 GB Postgres reads ~5 GiB, not the same 10 GiB.** On a `basic-1gb` Postgres (`diskSizeGB=5`, e.g. `blockeden-forum-db` / `dpg-d9rs3ee0ccis738kc7c0`) the capacity reads **~5 GiB**. The two databases no longer report an identical 10 GiB.
- **All three API surfaces agree on the logical value.** `GET /v1/metrics/disk-capacity?resource=<dpg>&kind=postgres`, GraphQL `datastoreMetrics(metric: DISK_CAPACITY)`, and MCP `get_datastore_metrics` each return the logical size in bytes (e.g. 1 GiB / 5 GiB), byte-identical across the three — not `kubelet_volume_stats_capacity_bytes`.
- **KeyValue matches its own StorageGB.** A managed KeyValue instance's Metrics "Capacity" equals its logical `StorageGB`, not the physical `data-<name>` PVC size.
- **Disk USED is unchanged.** The disk-used series still comes from `kubelet_volume_stats_used_bytes` (it was always accurate); only the capacity/denominator changes.
- **Ops alert stays physical.** The `PersistentVolumeFillingUp` Prometheus alert continues to evaluate against `kubelet_volume_stats_capacity_bytes` (operations correctly care about the real volume filling). The change is scoped to the user-facing `disk_capacity` metric only — demonstrate the alert expression is untouched.
- **Regression pinned.** A test asserts `disk_capacity` equals the logical `StorageGB` (converted to bytes) for both Postgres and KeyValue, and fails if the metric reverts to the kubelet capacity series.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 2026-09-02, workspace `tea-d98210cbbpdc73dcrkvg` (read-only; nothing created). Two managed Postgres databases with different logical disk sizes (1 GB and 5 GB) both showed the Metrics "Disk → Capacity" as a fixed **10 GiB**, contradicting Details "Storage", the autoscaling section, and billing on the same page.
- **Mechanism (confirmed):** `lego/backend/internal/metrics/source.go:441-442` sources `MetricDiskCapacity` from `kubelet_volume_stats_capacity_bytes` (the physical PVC). The operator sets CNPG `spec.storage.size` to the logical size (`database_controller.go:149`; `database_test.go:198` asserts `"1Gi"`), but the Hetzner Cloud Volume floor makes the physical PVC 10 GiB regardless — corroborated by the repo's own prod drill evidence `w2/m86/t004.md:39` ("PVC bound … 10→11Gi"). Dashboard chain: `dashboard/src/features/metrics/components/datastore-metrics-panel.tsx:81,100,140-143,154` renders `latestValue(diskCapacity.series)` as both the "Capacity {value}" label (`metrics/locales/en.ts:233` — annotation literally says "showing the PVC's total capacity") and the chart reference line.
- **Goal linkage:** ADR009 (Postgres) / ADR021 (KeyValue) managed-data honesty + ADR006 REST/GraphQL/MCP consistency + ADR018 Render parity — a hosting metric that misleads about disk fullness undercuts trust in the managed-data product.
- **Expected outcome:** the disk Metrics panel and the `disk_capacity` API report the billed/autoscaled disk size, so fullness is truthful and self-consistent with autoscaling, Details, and billing; a database near its quota is shown as near its quota.
- **Why now:** for a 1 GB database at ~81% used, the panel currently shows ~8% of 10 GiB — a user believes they have ample room right up to the moment autoscaling silently grows (and bills) their disk. The defect is on the shipped managed-Postgres/KeyValue surface today.
- **Render parity task included:** yes — the fix changes the `disk_capacity` REST/GraphQL/MCP metric and the dashboard panel; Render shows managed-Postgres disk usage against the plan's (logical) disk size, so parity confirmation applies.
- **Unverified (carried from the hunt):** KeyValue confirmed in code only, not exercised live this run; the Hetzner 10 GiB floor is corroborated by `w2/m86/t004` but not re-read from infra this run; whether compute-service disks (ADR082, `types.ts:52` says `disk`/`disk_capacity` now apply to services too) share the logical-vs-physical split — `t003` scopes this.
