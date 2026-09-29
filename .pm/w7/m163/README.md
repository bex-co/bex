# w7 · m163 — Alert on recurring registry garbage-collection failures

**Worker:** worker7 **Goal:** Detect recurring Zot garbage-collection failures before stranded build artifacts consume registry headroom. **Status:** todo

**Estimate:** 120m implementation; 190m (~3h10m) including standing closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Expose bounded Zot GC failure metrics and a private Alloy scrape](t001.md) | 45m | — |
| t002 | [Add registry GC recurrence alerts and repair navigation](t002.md) | 45m | t001 |
| t003 | [Make GC alert state reliable across replay reload and recovery](t003.md) | 30m | t002 |
| t004 | [Simplify](t004.md) | 20m | t003 |
| t005 | [Test coverage](t005.md) | 40m | t003, t004 |
| t006 | [Closeout](t006.md) | 10m | t004, t005 |

## Definition of done

- [ ] The known GC error signature becomes a bounded operational metric through the existing Zot log pipeline; the Alloy metrics scrape remains private.
- [ ] A failing fixture triggers the intended recurrence signal while healthy GC logs do not; the first event is not silently lost.
- [ ] Collector reload, historical replay, inactivity and recovery have documented and tested semantics, with collection loss distinct from zero errors.
- [ ] Every alert has a panel and ADR060 diagnosis/repair link; dated runtime evidence confirms the deployed scrape and rules without recreating the defect in production.

- [ ] Standing closing tasks and required checks are complete; production-dependent claims have dated runtime evidence for the tested revision.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w7` proposal 3, materialized by `$pm all for w7` on 2026-09-28. Brainstorm source revision: `7f1e49986`; materialization checkout: `97b70fad0`. [w7/m158](../blocked/m158/README.md), [ADR060 D4](../../../docs/ADR060-build-worker-reliability-and-performance.md), `deploy/gitops/base/log-shipper.yaml`, and [Alloy stage.metrics](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.process/#stage.metrics).
- **Evidence:** w7/m158 recorded 12 GC failures in 24 hours for an empty repository left across a Zot restart: removeTagsPerRetentionPolicy could not read repository metadata. The repository was repaired and later removed after successful GC. Prevention remains blocked on the upstream-versus-patched-build decision. Existing Zot alerts cover PVC capacity rather than this known failure signature; Zot platform logs already reach Loki.
- **Goal linkage:** ADR008 deploy-from-chat and dependable push-to-deploy need a reclaimable image registry, not just a registry that is currently reachable.
- **Expected outcome:** A recurrence of the known GC defect raises an actionable alert linked to the existing repair procedure before capacity alarms become the first signal.
- **Why now:** The failure signature and bounded repair are known now; monitoring can ship without deciding how to carry the upstream prevention fix.
- **Deduplication:** Adds detection only. m158 continues to own prevention, and its remaining DoD is not satisfied by this milestone. m26's Zot capacity alerts and the 047 build-cache trial stay separate.
- **Render parity omitted:** Internal platform monitoring only; no tenant-facing REST, GraphQL, MCP or dashboard contract change. Grafana here is the internal operations surface.

## Scheduling and boundaries

Approved priority 3 of five: **m161 → m162 → m163 → m164 → m165**. Keep work sequential in w7 because the milestones share Prometheus, Grafana and validation configuration. Priority is scheduling, not an artificial hard dependency. Implementation can begin independently of the other new milestones; m165 reuses m163/t001's Alloy scrape.

Use worker7's isolated dev-7 environment for applicable local exercises, respecting the harness's shared-cluster boundaries. Inspect current deployed state before runtime-dependent work; historical production observations are not claims of a current outage. Establish failure behavior with bounded isolated fixtures and deployed healthy coverage with dated read-only observations.

This filing schedules the approved work; it does not implement or deploy it. Existing blocked work, including m156, m158, 047 and 060, retains its own scope and completion conditions.

## Out of scope

- Patching/forking Zot, filing an upstream issue/PR, or selecting the prevention strategy.
- Automatically pushing repair artifacts or deleting registry data in response to an alert.
- Changing build-cache enablement or retention policy; repository names as unbounded metric labels.
