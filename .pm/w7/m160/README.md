# w7 · m160 — Add etcd latency and scrape-health coverage

**Worker:** worker7 **Goal:** Deterministic deployment reconciliation depends on a responsive Kubernetes control plane. Monitor degraded latency before it becomes an opaque hosting failure. **Status:** todo

**Estimate:** 180m implementation; 250m including closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Establish secure per-member metrics access | 60m | — |
| t002 | Add latency, quorum/leader and scrape-health coverage | 60m | t001 |
| t003 | Validate thresholds and deployed member coverage | 60m | t002 |
| t004 | Simplify | 20m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- [ ] Each member has an authenticated or otherwise appropriately isolated metrics path; no public listener or cluster-admin credential is introduced for monitoring.
- [ ] Every member is represented; sustained latency, cluster-health symptoms and telemetry loss have distinguishable actionable alerts.
- [ ] All members are scraped; alerts fire and resolve for their tested conditions, and missing monitoring is visible independently of etcd health.

- [ ] Required checks and closing tasks are complete, with dated runtime evidence for production-dependent outcomes.

## Source + Goal linkage

- **Source:** user-approved second platform-log brainstorm, 2026-09-28; `$pm all for w7`. Observations were captured against 016391810; materialization checkout is 242fe0c83. Revalidate before implementation.
- **Evidence:** The September 28 capture contains 356 slow-apply warnings, 23 delayed ReadIndex responses, three slow fdatasync warnings and two delayed leader heartbeats. The checked-in Prometheus configuration at brainstorm revision 016391810 has no explicit etcd scrape job or etcd alert rules. This is evidence for visibility, not proof that disks must be replaced.
- **Local evidence:** `/tmp/bex-platform-log-audit/` (manifest, events, node snapshots, container logs and Loki history). Temporary files can expire; the sanitized finding is preserved here.
- **Goal linkage:** ADR008 reliable self-hosted hosting and deterministic platform operation. Deterministic deployment reconciliation depends on a responsive Kubernetes control plane. Monitor degraded latency before it becomes an opaque hosting failure.
- **Expected outcome:** All members are scraped; alerts fire and resolve for their tested conditions, and missing monitoring is visible independently of etcd health.
- **Why now:** Deterministic deployment reconciliation depends on a responsive Kubernetes control plane. Monitor degraded latency before it becomes an opaque hosting failure.
- **Deduplication:** Complements w3/m6 node readiness alerts and existing etcd backup work. m159 handles tenant-node pressure; this milestone covers etcd members, latency and quorum/scrape health only.
- **Render parity omitted:** internal registry/platform monitoring only; no tenant-facing REST/GraphQL/MCP/UI contract change.

## Scheduling and boundaries

Approved second-round priority 3; keep existing m154–m157 work intact. The first round's root GitOps blocker m153 is now done; verify current sync before relying on it. These milestones do not depend on one another, although changes to shared Prometheus configuration must be coordinated. Implementation uses isolated dev-7 where applicable; do not disturb other workstreams' stacks. This filing performs no production mutation.
