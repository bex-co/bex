# w4 · m91 — Managed-datastore disk metrics show the logical (billed) capacity, not the physical PVC floor

**Worker:** worker4 **Goal:** the Metrics disk panel judges fullness against the datastore's billed/autoscaled disk size, so a user near their quota sees that they are near it — instead of the fixed 10 GiB physical PVC that makes an 81%-full 1 GB database look 8% full **Status:** todo

## Tasks (in order)

| id   | title                                                                                                       | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Backend: `disk_capacity` returns the datastore's logical provisioned size, not `kubelet_volume_stats_capacity_bytes` | 50m | —                |
| t002 | Dashboard: DatastoreMetricsPanel capacity label + reference line reflect the logical size                    | 30m | t001             |
| t003 | KeyValue parity (+ scope compute-service disks)                                                              | 30m | t001             |
| t004 | Render parity                                                                                                | 25m | t001, t002, t003 |
| t005 | Simplify                                                                                                     | 20m | t004             |
| t006 | Test coverage                                                                                                | 40m | t004             |
| t007 | Closeout                                                                                                     | 15m | t005, t006       |

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
