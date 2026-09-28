# w7 · m159 — Detect tenant-node disk pressure before workload disruption

**Worker:** worker7 **Goal:** Hosted workloads need actionable infrastructure pressure signals before eviction escalates. A healthy current snapshot must not erase the evidence of a recent incident. **Status:** todo

**Estimate:** 120m implementation; 190m including closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Attribute the eviction and select available pressure signals | 40m | — |
| t002 | Add bounded pressure alerts and a diagnosis runbook | 40m | t001 |
| t003 | Verify pressure, recovery, missing-series behavior and delivery | 40m | t002 |
| t004 | Simplify | 20m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- [ ] The pressure source, coverage limits and signal availability are documented; ephemeral node storage is distinguished from PVC usage.
- [ ] Sustained pressure produces an actionable node-specific alert; the runbook identifies relevant storage consumers and recent incident evidence.
- [ ] The alert fires under sustained pressure, resolves on recovery, does not silently declare missing telemetry healthy, and recent transient pressure remains diagnosable.

- [ ] Required checks and closing tasks are complete, with dated runtime evidence for production-dependent outcomes.

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
