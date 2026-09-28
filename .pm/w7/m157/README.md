# w7 · m157 — Retain core platform logs across rollouts

**Worker:** worker7 **Goal:** Preserve API and operator incident evidence across pod replacement within bounded platform log storage. **Status:** todo

**Estimate:** 150m implementation; 220m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define bounded core-service labels and storage limits | 30m | — |
| t002 | Extend platform collection to API and operator pods | 45m | t001 |
| t003 | Prove history survives pod replacement and stays isolated | 40m | t002 |
| t004 | Verify rollout ingestion and storage budget | 35m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- [ ] Document finite retention and an explicit storage budget; no tenant identifiers or request payloads become unbounded labels, and credentials are not newly retained.
- [ ] Both new services arrive under type=platform with correct namespace/pod/container attribution and no duplicate ingestion.
- [ ] Both services retain their old markers after replacement; tenant APIs cannot retrieve the platform records and no existing collection loses records.
- [ ] Both services are queryable across a rollout, storage remains within the agreed budget, and retention is enabled rather than inferred from configuration alone.

- [ ] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

## Source + Goal linkage

- **Source:** User-approved platform-log brainstorm, 2026-09-28 UTC (September 27 America/Denver); user requested all five in w7. Read-only collection at source revision 016391810: 342 container streams, 167 pods, 22 namespaces, plus Loki platform history. Requested prior 24 hours; rotation/deleted pods limit coverage. Raw local evidence: /tmp/bex-platform-log-audit/{manifest.json,pods.json,events.json,argo.json,loki-platform.jsonl}; temporary artifacts may expire, so the observed findings are preserved below and must be revalidated at execution.
- **Observed finding:** The audit retrieved 37,977 Loki type=platform records: 37,467 zot, 409 static-server and 101 dashboard. API/operator pod logs began around 2026-09-27 12:08 UTC after replacement. The platform_pods keep rule excludes these components, preventing recovery of earlier deleted-replica logs through this pipeline.
- **Goal linkage:** ADR008 reliable self-hosted Render-compatible hosting, deterministic lifecycle state and platform operability. Preserve API and operator incident evidence across pod replacement within bounded platform log storage.
- **Expected outcome:** Preserve API and operator incident evidence across pod replacement within bounded platform log storage.
- **Why now:** The missing history prevented complete reconstruction of this incident window and recurs on every rollout.
- **Existing work / deduplication:** Extends w4/done/m88/README.md (dashboard stdout and retention) and the registry/static-server platform labels; does not rebuild their collection or introduce external drains.
- **Render parity:** Omitted: platform infrastructure/observability only, with no REST/GraphQL/MCP/UI contract change.
- **Scheduling:** approved priority 5 of five in w7. Infrastructure rollout verification follows m153's root-sync repair; isolated implementation can proceed independently. No dependency on blocked w7 datastore/sandbox walkthroughs is inferred.

## Scope and evidence limits

This is planned work, not an implemented fix. The read-only audit did not authorize implementation during collection. Revalidate the deployed revision and failure before executing these tasks. Raw logs and credentials stay out of Git; preserve only sanitized observations. Do not reopen the accepted PSL finding, duplicate the known blockeden.xyz certificate work, or infer root causes from transient etcd/storage warnings.
