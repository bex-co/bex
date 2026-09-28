# w7 · m159 — Detect tenant-node disk pressure before workload disruption

**Worker:** worker7 **Goal:** Hosted workloads need actionable infrastructure pressure signals before eviction escalates. A healthy current snapshot must not erase the evidence of a recent incident. **Status:** done

**Estimate:** 120m implementation; 190m including closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Attribute the eviction and select available pressure signals — **DONE** | 40m | — |
| t002 | Add bounded pressure alerts and a diagnosis runbook — **DONE** | 40m | t001 |
| t003 | Verify pressure, recovery, missing-series behavior and delivery — **DONE** | 40m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] The pressure source, coverage limits and signal availability are documented; ephemeral node storage is distinguished from PVC usage.
- [x] Sustained pressure produces an actionable node-specific alert; the runbook identifies relevant storage consumers and recent incident evidence.
- [x] The alert fires under sustained pressure, resolves on recovery, does not silently declare missing telemetry healthy, and recent transient pressure remains diagnosable.

- [x] Required checks and closing tasks are complete, with dated runtime evidence for production-dependent outcomes.

## Source + Goal linkage

- **Source:** user-approved second platform-log brainstorm, 2026-09-28; `$pm all for w7`. Observations were captured against 016391810; materialization checkout is 242fe0c83. Revalidate before implementation.
- **Evidence:** The September 28 capture contains EvictionThresholdMet on bex-tenant-0-bdn2q-bnzl6, last seen at 03:38 UTC; the accumulated event count is four since September 24, not four incidents in the audit day. Later node snapshots show DiskPressure=False. At brainstorm revision 016391810, Prometheus has readiness/PVC alerts but no DiskPressure rule; node-exporter is disabled.
- **Local evidence:** `/tmp/bex-platform-log-audit/` (manifest, events, node snapshots, container logs and Loki history). Temporary files can expire; the sanitized finding is preserved here.
- **Goal linkage:** ADR008 reliable self-hosted hosting and deterministic platform operation. Hosted workloads need actionable infrastructure pressure signals before eviction escalates. A healthy current snapshot must not erase the evidence of a recent incident.
- **Expected outcome:** The alert fires under sustained pressure, resolves on recovery, does not silently declare missing telemetry healthy, and recent transient pressure remains diagnosable.
- **Why now:** Hosted workloads need actionable infrastructure pressure signals before eviction escalates. A healthy current snapshot must not erase the evidence of a recent incident.
- **Deduplication:** Extends shipped w3/m6 alerting. Node-local ephemeral storage is separate from PVC capacity and m160 control-plane latency; do not duplicate Zot PVC alerts or w7/m72 build storage bounds.
- **Render parity omitted:** internal registry/platform monitoring only; no tenant-facing REST/GraphQL/MCP/UI contract change.

## Scheduling and boundaries

Approved second-round priority 2; keep existing m154–m157 work intact. The first round's root GitOps blocker m153 is now done; verify current sync before relying on it. These milestones do not depend on one another, although changes to shared Prometheus configuration must be coordinated. Implementation uses isolated dev-7 where applicable; do not disturb other workstreams' stacks. This filing performs no production mutation.

## Evidence (2026-09-28 UTC, worker7)

**Attribution (t001, production read-only):** `EvictionThresholdMet` Events had already expired (1h TTL). kube-state-metrics' `kube_node_status_condition{condition="DiskPressure",status="true"}` retained the episode: `bex-tenant-0-bdn2q-bnzl6` = 1 for 16 samples, 03:39:15Z–03:43:00Z; the node's DiskPressure condition last transitioned (to False) at 03:43:08Z. No `node_filesystem_*`, `kubelet_evictions` or cAdvisor fs-usage series are scraped (no node-exporter), so the kubelet's own condition is the minimal available signal and there is no fill-rate early warning — recorded in ADR010. Node ephemeral storage is distinguished from PVCs (`PersistentVolumeFillingUp`). Separately observed: a tenant pod evicted every minute for exceeding its **own** 1Gi ephemeral limit — pod-level, not node pressure; the runbook calls that distinction out.

**Implemented (t002):** `NodeDiskPressure` (critical, 2m), `NodeDiskPressureRecovered` (info → `null` receiver, 6h window), `NodeDiskPressureSignalMissing` (warning, 15m; per-node `kube_node_info unless …` plus `absent(...)`); ADR010 rows + non-destructive diagnosis runbook; Cluster capacity panel "Node DiskPressure". Shipped `6a6b4d0d9`.

**Verified (t003/t005):** promtool unit tests — sustained pressure fires at 8m not 6m and clears at 12m; the recovered alert appears only after recovery; a node without a series and no series at all each raise SignalMissing. Mutation of the recovery `unless` clause fails the suite. `gitops-validate.sh` (promtool + alert→panel audit) passes. Against live production data before rollout: the 03:39–03:43Z episode (≈4m) satisfies NodeDiskPressure's `for: 2m`; SignalMissing is empty across all 11 nodes. **After rollout (07:45Z):** Prometheus has all three rules loaded (NodeDiskPressure inactive, SignalMissing inactive, Recovered **firing**), and Alertmanager holds `NodeDiskPressureRecovered{node="bex-tenant-0-bdn2q-bnzl6",severity="info"}` routed to `null`. No production disk was filled; live critical-page delivery was not exercised (the receiver path is shared with existing critical alerts).

**Simplify (t004):** inline review; three rules share one series and one panel.
