# w7 · m164 — Warn before tenant-node storage reaches DiskPressure

**Worker:** worker7 **Goal:** Provide actionable node filesystem headroom warnings before tenant workloads reach kubelet eviction thresholds. **Status:** in progress

**Estimate:** 180m implementation; 250m (~4h10m) including standing closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Identify tenant-node backing filesystems and eviction thresholds](done/t001.md) — **DONE** | 45m | — |
| t002 | [Add scoped filesystem collection and missing-node coverage](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [Add pre-eviction storage alerts and capacity panels](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Record deployed node coverage collection cost and response limits](t004.md) | 30m | t003 |
| t005 | [Simplify](t005.md) | 20m | t004 |
| t006 | [Test coverage](t006.md) | 40m | t004, t005 |
| t007 | [Closeout](t007.md) | 10m | t005, t006 |

## Definition of done

- [ ] Each intended tenant node exposes the correctly identified node/image backing filesystem, free bytes, free inodes and relevant coverage state.
- [ ] Representative fixtures warn before the documented kubelet eviction thresholds; pseudo-filesystems, duplicate mounts and PVC-only usage do not cause noise.
- [ ] Missing target/filesystem telemetry remains visible; recovered headroom clears the corresponding warning.
- [ ] Collection access, resource cost and node coverage are recorded at the deployed revision, and the runbook distinguishes node pressure, PVC capacity and per-pod limits.
- [ ] Forecasts are labelled advisory and have conservative history/threshold guards; no production disk is filled to establish the acceptance proof.

- [ ] Standing closing tasks and required checks are complete; production-dependent claims have dated runtime evidence for the tested revision.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w7` proposal 4, materialized by `$pm all for w7` on 2026-09-28. Brainstorm source revision: `7f1e49986`; materialization checkout: `97b70fad0`. [w7/m159](../done/m159/README.md), `deploy/gitops/base/prometheus.yaml`, and [node_exporter selective collectors](https://github.com/prometheus/node_exporter#collectors).
- **Evidence:** w7/m159 observed the September 28 tenant-node DiskPressure episode and shipped condition/recovery/missing-condition alerts. Its closeout explicitly records that no node filesystem or fill-rate series are scraped, so there is no advance headroom warning. prometheus-node-exporter remains disabled in prometheus.yaml at materialization.
- **Goal linkage:** ADR008 reliable self-hosted hosting and the existing platform roadmap's capacity/elasticity goals: operators need time to respond before storage pressure evicts tenant workloads.
- **Expected outcome:** The real filesystems backing tenant-node ephemeral/image storage expose bounded capacity/inode warnings and advisory trends, with explicit missing coverage.
- **Why now:** A documented pressure episode establishes the need, while m159 deliberately stopped at the available kubelet condition. This is the identified advance-warning residual.
- **Deduplication:** Extends m159 rather than reopening it. Existing PVC/Zot capacity alerts and per-pod ephemeral-storage limits do not measure node-local filesystem headroom. The historical no-node-exporter stance allowed adding it when a needed rule cannot be supported otherwise.
- **Render parity omitted:** Internal platform monitoring only; no tenant-facing REST, GraphQL, MCP or dashboard contract change. Grafana here is the internal operations surface.

## Scheduling and boundaries

Approved priority 4 of five: **m161 → m162 → m163 → m164 → m165**. Keep work sequential in w7 because the milestones share Prometheus, Grafana and validation configuration. Priority is scheduling, not an artificial hard dependency. Implementation can begin independently of the other new milestones; m165 reuses m163/t001's Alloy scrape.

Use worker7's isolated dev-7 environment for applicable local exercises, respecting the harness's shared-cluster boundaries. Inspect current deployed state before runtime-dependent work; historical production observations are not claims of a current outage. Establish failure behavior with bounded isolated fixtures and deployed healthy coverage with dated read-only observations.

This filing schedules the approved work; it does not implement or deploy it. Existing blocked work, including m156, m158, 047 and 060, retains its own scope and completion conditions.

## Out of scope

- Automatic image/data deletion, disk expansion, node replacement or capacity purchases.
- Duplicating PVC alerts or mistaking per-pod limit eviction for node DiskPressure.
- Claiming a forecast guarantees advance notice of sudden writes, or enabling a broad unrelated host-metric suite.

## Implementation verification — 2026-09-29 UTC

- `GITOPS_TEST_FILESYSTEM_COLLECTOR=1 bash scripts/gitops-validate.sh`: passed, including locked Helm render, all rule fixtures, coverage and eight actual-image filesystem cases for shared/split/missing mounts and local overlay behavior. CI opts into that same render rather than downloading/rendering a second time. The sole warning is the pre-existing optional FGA model drift check skipped because `fga` is absent; no authz files changed.
- Eighteen focused alert scenarios and twenty-two panel scenarios passed. Thirteen isolated rule mutations and five collector/scrape mutations failed behaviorally. After sharing the forecast predicate, the focused suite and two relevant forecast mutations passed/failed as expected. The panel's missing-inventory fallback mutation also failed.
- `/simplify`: reuse, quality and efficiency reviews completed. Shared the advisory predicate between alert and panel, removed redundant panel guards, and consolidated CI's collector exercise into its existing render. No broad host metrics or automatic disk action was added.
- Authenticated read-only topology inspection covered every intended production node and cleaned up all temporary pods. Pressure/recovery/split-storage claims come from isolated fixtures; no production disk was filled. Deployed coverage and actual collector overhead remain for t004.
