# w7 · m156 — Repair CNPG replica WAL metric collection

**Worker:** worker7 **Goal:** Restore reliable WAL observability for control-plane database replicas through startup and recovery. **Status:** todo

**Estimate:** 120m implementation; 190m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce NULL WAL statistics on the deployed CNPG version | 45m | — |
| t002 | Validate and apply the supported collector correction | 45m | t001 |
| t003 | Verify replica metrics and error disappearance | 30m | t002 |
| t004 | Simplify | 20m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- [ ] A reproducible failing scrape and evidence-backed correction are documented; no unverified upgrade is treated as the fix.
- [ ] The same reproduction collects WAL metrics successfully; database availability, replication and backups remain healthy. Do not reset statistics merely to hide the error.
- [ ] Affected replica WAL metrics remain present through the observation window and no NULL stats_reset scan errors recur; successful recovery was exercised in isolation.

- [ ] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

## Source + Goal linkage

- **Source:** User-approved platform-log brainstorm, 2026-09-28 UTC (September 27 America/Denver); user requested all five in w7. Read-only collection at source revision 016391810: 342 container streams, 167 pods, 22 namespaces, plus Loki platform history. Requested prior 24 hours; rotation/deleted pods limit coverage. Raw local evidence: /tmp/bex-platform-log-audit/{manifest.json,pods.json,events.json,argo.json,loki-platform.jsonl}; temporary artifacts may expire, so the observed findings are preserved below and must be revalidated at execution.
- **Observed finding:** The audit counted 798 errors from bex-system/bex-db-2: while collecting pg_stat_wal; sql: Scan error on column index 4, name "stats_reset": converting NULL to string is unsupported. The stack identifies the CNPG instance metric collector. This establishes a collection failure, not database corruption.
- **Goal linkage:** ADR008 reliable self-hosted Render-compatible hosting, deterministic lifecycle state and platform operability. Restore reliable WAL observability for control-plane database replicas through startup and recovery.
- **Expected outcome:** Restore reliable WAL observability for control-plane database replicas through startup and recovery.
- **Why now:** Repeated collector failures degrade observability of the control-plane database that stores platform state.
- **Existing work / deduplication:** No matching WAL NULL-collector item found. Existing CNPG availability/backup work does not own this collector failure.
- **Render parity:** Omitted: platform infrastructure/observability only, with no REST/GraphQL/MCP/UI contract change.
- **Scheduling:** approved priority 4 of five in w7. Infrastructure rollout verification follows m153's root-sync repair; isolated implementation can proceed independently. No dependency on blocked w7 datastore/sandbox walkthroughs is inferred.

## Scope and evidence limits

This is planned work, not an implemented fix. The read-only audit did not authorize implementation during collection. Revalidate the deployed revision and failure before executing these tasks. Raw logs and credentials stay out of Git; preserve only sanitized observations. Do not reopen the accepted PSL finding, duplicate the known blockeden.xyz certificate work, or infer root causes from transient etcd/storage warnings.
