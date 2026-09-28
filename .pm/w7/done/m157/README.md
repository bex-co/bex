# w7 · m157 — Retain core platform logs across rollouts

**Worker:** worker7 **Goal:** Preserve API and operator incident evidence across pod replacement within bounded platform log storage. **Status:** done

**Estimate:** 150m implementation; 220m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define bounded core-service labels and storage limits — **DONE** | 30m | — |
| t002 | Extend platform collection to API and operator pods — **DONE** | 45m | t001 |
| t003 | Prove history survives pod replacement and stays isolated — **DONE** | 40m | t002 |
| t004 | Verify rollout ingestion and storage budget — **DONE** | 35m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- [x] Document finite retention and an explicit storage budget; no tenant identifiers or request payloads become unbounded labels, and credentials are not newly retained.
- [x] Both new services arrive under type=platform with correct namespace/pod/container attribution and no duplicate ingestion.
- [x] Both services retain their old markers after replacement; tenant APIs cannot retrieve the platform records and no existing collection loses records.
- [x] Both services are queryable across a rollout, storage remains within the agreed budget, and retention is enabled rather than inferred from configuration alone.

- [x] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

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

## Evidence (2026-09-28 UTC, worker7)

**Scope/budget (t001):** bex-api (`app.kubernetes.io/name=bex-api`, container `api`, a few std-log lines/hour) and the operator manager (`control-plane=controller-manager`, zap console, ≈2.45 MB/6h raw). Credential patterns (JSON secret keys, bearer tokens, DSNs with passwords, provider token prefixes) in 6h of both sources: **0**. Loki: 20 GiB PVC, 1.9 GB used, ≈31 MB/day across all streams, shared 21-day retention (compactor `retention_enabled: true`).

**Implemented (t002), shipped `46f6f442a`:** `platform_pods` keep extended to exactly those two selectors with closed `service=bex-api|operator`; zap console level parsed via `stage.regex`; activator, SNI proxies, egress meter, ssh-gateway and other namespaces' controller-managers stay excluded; no tenant-id label. gitops-validate guards the rules; ADR010 records scope and budget.

**Tests (t005):** real-Alloy test routes synthetic pod targets through the production `platform_pods` relabel block: bex-api/operator/static-server/dashboard arrive with correct service/namespace/pod/container/level and no `app` label; activator, ssh-gateway and a foreign controller-manager are dropped; zero diagnostics. Pre-change pipeline fails the test.

**Isolation (t003):** tenant log selectors always pin `namespace=<tenant ns>` plus `app`/`database`/`keyvalue` resolved server-side (`lego/backend/internal/logs/loki.go` `lokiSelectorFor`); platform streams live in `bex-system` with no `app` label, so no tenant query can select them.

**Production (t003/t004):** log-shipper reloaded 07:22–07:23Z; the first discovery back-filled the pods' retained output from 05:38Z. After the 09:11Z operator/bex-api rollout, the deleted pods remain queryable — `bex-controller-manager-55fd899d9b-lvl88` (last marker `cron reconciled` 09:11:06Z, 5,398 lines) and `bex-api-74c545c74f-{948jw,cgxnt}` — alongside every later replica (3 operator, 10 bex-api pods retained in 4h). Levels: operator info/error parsed; unknown lines are stack-trace continuations (no severity of their own). Measured ingestion for both services: 2.72 MB in 4h (≈16 MB/day, above the ≈10 MB/day estimate because it includes the backfill and today's rollout churn) — still ≈0.3 GB over 21 days against 18 GB free. No duplicate ingestion: these pods carry no `app.bex.co/app` label, so `app_pods` never selected them.

**Simplify:** inline review; one keep rule, three service stamps.
