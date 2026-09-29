# w7 · m165 — Detect failures in durable log ingestion

**Worker:** worker7 **Goal:** Expose log delivery and ingestion failures before they silently remove tenant diagnostics and platform incident history. **Status:** done

**Estimate:** 150m implementation; 220m (~3h40m) including standing closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Collect deployed Loki and Alloy ingestion-health metrics](done/t001.md) — **DONE** | 45m | w7/m163/t001 |
| t002 | [Alert on unexpected ingestion loss and expose diagnosis panels](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [Exercise bounded log delivery failure and recovery](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Simplify](done/t004.md) — **DONE** | 20m | t003 |
| t005 | [Test coverage](done/t005.md) — **DONE** | 40m | t003, t004 |
| t006 | [Closeout](done/t006.md) — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] Deployed Loki and intended Alloy collectors expose the selected native delivery/rejection/target-health series through private scrapes, with no duplicate Alloy scrape.
- [x] Unexpected loss/rejection, transient retriable failure and missing collectors have documented distinct behavior, and failures can alert even while process readiness remains healthy.
- [x] Intentional filtering, including cnpg_instance_manager drops, remains quiet; tenant/repository identifiers are not added as monitoring labels.
- [x] A bounded isolated failure/recovery exercise demonstrates the chosen alerts and identifies buffering/retry/loss limits; existing tenant synthetics remain unchanged.
- [x] Dated runtime evidence records deployed targets, rules and panels; recovery is observable without claiming that already-dropped logs were recovered.

- [x] Standing closing tasks and required checks are complete; production-dependent claims have dated runtime evidence for the tested revision.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w7` proposal 5, materialized by `$pm all for w7` on 2026-09-28. Brainstorm source revision: `7f1e49986`; materialization checkout: `97b70fad0`. `deploy/gitops/base/loki.yaml`, `deploy/gitops/base/log-shipper.yaml`, `deploy/gitops/base/prometheus.yaml`, [w7/m157](../m157/README.md), [w3/m83](../../../w3/done/m83/README.md), and [Loki/Alloy native monitoring](https://grafana.com/docs/loki/latest/operations/meta-monitoring/).
- **Evidence:** Loki's checked-in values disable bundled self-monitoring and Prometheus has no Loki/Alloy scrape at brainstorm/materialization. w7/m157 added API/operator retention. Existing periodic tenant-view/request-log probes verify some end-to-end results but do not directly identify rejected ingestion, exhausted delivery retries or absent collectors. This is a coverage gap, not evidence of a current ingestion outage.
- **Goal linkage:** ADR008 reliable hosting and agent-readable state require trustworthy diagnostic history; ADR010 supplies the durable logging contract.
- **Expected outcome:** Unexpected ingestion loss, delivery failures and missing collectors become distinct actionable signals even when workload readiness remains green.
- **Why now:** Incident reconstruction now depends on the expanded retained streams, and recent shipper changes make prompt failure attribution valuable. Existing periodic synthetics stay as independent end-to-end evidence.
- **Deduplication:** Complements m157 retention and m83 tenant synthetics. Reuses m163/t001's Alloy metrics scrape instead of creating a second one; this milestone covers transport/ingestion health while m163 covers registry GC semantics.
- **Render parity omitted:** Internal platform monitoring only; no tenant-facing REST, GraphQL, MCP or dashboard contract change. Grafana here is the internal operations surface.

## Scheduling and boundaries

Approved priority 5 of five: **m161 → m162 → m163 → m164 → m165**. Keep work sequential in w7 because the milestones share Prometheus, Grafana and validation configuration. Priority is scheduling, not an artificial hard dependency. The real cross-milestone dependency is `w7/m165/t001` → `w7/m163/t001`, which supplies the shared private Alloy scrape.

Use worker7's isolated dev-7 environment for applicable local exercises, respecting the harness's shared-cluster boundaries. Inspect current deployed state before runtime-dependent work; historical production observations are not claims of a current outage. Establish failure behavior with bounded isolated fixtures and deployed healthy coverage with dated read-only observations.

This filing schedules the approved work; it does not implement or deploy it. Existing blocked work, including m156, m158, 047 and 060, retains its own scope and completion conditions.

## Out of scope

- External telemetry drains or a replacement/multi-component Loki architecture.
- Changing tenant log API shapes, retention policy or intentional CNPG chatter filtering.
- Paging on every intentionally dropped line or treating absence of tenant traffic as loss.

## Implementation verification — 2026-09-29 UTC

- Reused the private Alloy job and added one private single-binary Loki target. Locked chart renders verify all-node collector placement, Loki single-tenant/single-replica assumptions and collision-safe metric filtering in both overlays. Seven filtering mutations were rejected, including the actual Zot component identity regression found during review.
- Twenty-eight alert/record scenarios and twenty panel scenarios pass. Thirteen syntax-valid rule mutations and one missing-panel-coverage mutation were rejected. The new eight alerts are covered by Platform availability panels 21–28; the full coverage guard reports 84 covered and three existing context-only waivers.
- The actual pinned Alloy/Loki lifecycle fixture passes: transient 503 buffering recovers; one permanent 400 and one exhausted 503 batch are dropped; subsequent delivery succeeds without recovering discarded lines. Both processes stay ready during rejection. Intentional CNPG filtering stays separate, and restart resets counters while fresh delivery resumes. Two rendered-config mutations fail behaviorally. CI runs this fixture through `python3 -m unittest scripts/test_log_delivery.py -v`.
- `/simplify` reuse, quality and efficiency reviews completed. Shared recording rules and rendering helpers are reused; the native Zot metric shape caught and corrected a scrape regression. Missing native evidence and zero running components remain unknown instead of looking healthy. No further substantive simplification was needed.
- `bash scripts/gitops-validate.sh` passed after final integration, including both overlay renders, all rule fixtures and alert/panel coverage. Markdown formatting and `git diff --check` passed. The only warning is the pre-existing optional FGA model check skipped because its CLI is absent; no authz files changed. The Application growth budget passes at 1,204,301 bytes (production) and 1,226,404 bytes (local), below 1,310,720 bytes.
- Deployment verification is complete in t003. Isolated fixtures establish failure semantics; no production stream was interrupted. First-positive counters require a 15m marker baseline, and missing evidence, resets and between-scrape loss limit completeness as documented in ADR010.

## Deployed closeout — 2026-09-29 UTC

Verified 2026-09-29 06:32:23–06:34:21 UTC at 8d1cecba9: root, Prometheus and Grafana are Synced/Healthy; complete Prometheus ConfigMap and Platform availability dashboard/panels 21–28 match the committed render, with loaded rule/configuration semantics verified. Exactly one Alloy job covers all eleven current nodes and eleven Ready DaemonSet pods; expected, UP and healthy records each contain eleven nodes with empty coverage differences. One Loki target is UP with process marker and WAL counter present; completeness and inventory are one. All nine records evaluate healthily and all eight alerts are healthy/inactive. Selected native series total 84 for Alloy and five for Loki, within the declared label allowlists. Historical ingester_error drop totals were 212 and 41,469 on two tenant collectors, with no drop/retry increases observed during the rollout; these undated lifetime totals did not create false new incidents. Zot heartbeat advanced and was 6.94s old in the final snapshot, with collection healthy. The 15m marker baseline is still warming; fixtures establish first-positive behavior after that baseline. No production log stream was deliberately interrupted.
