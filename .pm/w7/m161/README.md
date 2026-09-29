# w7 · m161 — Detect stalled platform GitOps delivery

**Worker:** worker7 **Goal:** Surface stalled platform configuration delivery before it silently withholds hosting fixes or monitoring rules. **Status:** todo

**Estimate:** 150m implementation; 220m (~3h40m) including standing closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Add internal Argo metrics collection and expected application coverage](t001.md) | 45m | — |
| t002 | [Add sustained sync and health alerts with actionable panels](t002.md) | 60m | t001 |
| t003 | [Activate collection and record root and child application coverage](t003.md) | 45m | t002 |
| t004 | [Simplify](t004.md) | 20m | t003 |
| t005 | [Test coverage](t005.md) | 40m | t003, t004 |
| t006 | [Closeout](t006.md) | 10m | t004, t005 |

## Definition of done

- [ ] Every intended platform Application, including bex-platform-prod and its children, is represented or explicitly reported missing; the scrape endpoint stays internal.
- [ ] A persistent failed/out-of-sync or unhealthy Application triggers an alert naming the Application and diagnosis path, while a normal rollout shorter than the chosen grace interval does not.
- [ ] Recovery clears the alert; exporter/target loss and missing required application series cannot appear healthy.
- [ ] Dated deployed-revision evidence records root/child coverage and rule availability; isolated failure/recovery fixtures establish behavior without breaking production sync.

- [ ] Standing closing tasks and required checks are complete; production-dependent claims have dated runtime evidence for the tested revision.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w7` proposal 1, materialized by `$pm all for w7` on 2026-09-28. Brainstorm source revision: `7f1e49986`; materialization checkout: `97b70fad0`. [w7/m153](../done/m153/README.md), `deploy/gitops/base/prometheus.yaml`, and [Argo controller metrics](https://argo-cd.readthedocs.io/en/stable/operator-manual/metrics/).
- **Evidence:** At brainstorm revision 7f1e49986, Prometheus has no Argo application scrape or sync-health alerts. w7/m153 recorded bex-platform-prod OutOfSync/Degraded after an unmanaged analytics Secret blocked an early sync wave; child monitoring changes were withheld until the ownership repair. This is a known failure mode, not a claim that the repaired root Application is currently unhealthy. Materialization rechecked the absent scrape/alerts at 97b70fad0.
- **Goal linkage:** ADR008 reliable self-hosted hosting and deterministic convergence: code/configuration must actually reach the platform that runs tenant workloads.
- **Expected outcome:** Persistent platform Application sync/health failure and loss of its monitoring produce distinct actionable signals; ordinary rollout transitions remain quiet.
- **Why now:** The incident-specific ownership repair shipped, but another resource failure can block the same delivery path without application-aware alerting.
- **Deduplication:** Extends m153 with detection. Does not repeat its Secret adoption or duplicate CI red-streak/scheduled-workflow checks, which cannot establish Argo reconciliation health.
- **Render parity omitted:** Internal platform monitoring only; no tenant-facing REST, GraphQL, MCP or dashboard contract change. Grafana here is the internal operations surface.

## Scheduling and boundaries

Approved priority 1 of five: **m161 → m162 → m163 → m164 → m165**. Keep work sequential in w7 because the milestones share Prometheus, Grafana and validation configuration. Priority is scheduling, not an artificial hard dependency. Implementation can begin independently of the other new milestones; m165 reuses m163/t001's Alloy scrape.

Use worker7's isolated dev-7 environment for applicable local exercises, respecting the harness's shared-cluster boundaries. Inspect current deployed state before runtime-dependent work; historical production observations are not claims of a current outage. Establish failure behavior with bounded isolated fixtures and deployed healthy coverage with dated read-only observations.

This filing schedules the approved work; it does not implement or deploy it. Existing blocked work, including m156, m158, 047 and 060, retains its own scope and completion conditions.

## Out of scope

- Repairing analytics Secret ownership again or changing its credential custody.
- Tenant deployment alerting, a replacement GitOps engine, or automatic forced sync/pruning.
- Changing runner hosts, CI trust boundaries, or tenant-facing APIs.
