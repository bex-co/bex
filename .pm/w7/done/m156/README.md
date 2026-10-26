# w7 · m156 — Repair CNPG replica WAL metric collection

**Worker:** worker7 **Goal:** Restore reliable WAL observability for control-plane database replicas through startup and recovery. **Status:** done

**Estimate:** 120m implementation; 190m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce NULL WAL statistics on the deployed CNPG version — **DONE** | 45m | — |
| t002 | Validate and apply the supported collector correction — **DONE** | 45m | t001 |
| t003 | Verify replica metrics and error disappearance — **DONE** | 30m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] A reproducible failing scrape and evidence-backed correction are documented; no unverified upgrade is treated as the fix.
- [x] The same reproduction collects WAL metrics successfully; database availability, replication and backups remain healthy. Do not reset statistics merely to hide the error.
- [x] Affected replica WAL metrics remain present through the observation window and no NULL stats_reset scan errors recur; successful recovery was exercised in isolation.

- [x] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

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

The original read-only audit did not authorize implementation during collection. The subsequent user instructions authorized execution and shipping; dated checkpoints below record both local and production acceptance. Raw logs and credentials stay out of Git; only sanitized observations are preserved. This milestone does not reopen the accepted PSL finding, duplicate the known blockeden.xyz certificate work, or infer root causes from transient etcd/storage warnings.

## Evidence (2026-09-28 UTC, worker7) — t001 done, parked

**Reproduced in production (read-only, 07:10Z):** operator `ghcr.io/cloudnative-pg/cloudnative-pg:1.30.0` (chart `cloudnative-pg` 0.29.0, `deploy/helm-artifacts.lock:20`), every instance on `postgresql:18.4-system-trixie`. The instance-manager exporter logs `while collecting pg_stat_wal … "stats_reset": converting NULL to string is unsupported` ~60×/hour on **`bex-db-2` only** — the current `bex-db` primary; the other 10 instance pods across 7 clusters log none in the last hour. Not corruption: both `bex-db` instances report healthy.

**Evidence-backed correction:** upstream bug — since PostgreSQL 18 `pg_stat_wal.stats_reset` can be NULL and CNPG scanned it into a Go string. Fixed by [cloudnative-pg#11207](https://github.com/cloudnative-pg/cloudnative-pg/pull/11207) (merge `953536e7`, coalesces to `-infinity`, metric label format unchanged), shipped in **CNPG 1.30.1** (release notes line "Handled a NULL stats_reset value in the pg_stat_wal metric", 2026-09-23); Helm chart **0.29.1** (`appVersion: 1.30.1`). Resetting WAL statistics would hide the error and is excluded by the DoD.

**Prepared change (not applied):** bump `deploy/helm-artifacts.lock` `cloudnative-pg` to `0.29.1` (new archive sha + deterministic mirror digest via `scripts/helm-artifact.sh mirror`) and point `deploy/gitops/base/cnpg-operator.yaml` `targetRevision` at it.

**Why parked:** Argo auto-syncs `cnpg-operator` from `main`, so merging the bump *is* the production rollout. `cnpg-controller-manager-config` does not set `ENABLE_INSTANCE_MANAGER_INPLACE_UPDATES`, so an operator upgrade restarts every instance pod — all **7** Postgres clusters, including **3 single-instance tenant databases** (brief customer downtime) and switchovers of the 4 two-instance platform clusters — to fix one collector on one pod. The DoD also requires the startup/recovery path to be exercised in isolation first, and the local substrate cannot do that today (dev cluster `kube-controller-manager` crash-looping on apiserver lease timeouts, the shared-VM capacity gate on m149–m151).

## Unblock execution — 2026-10-25 America/Los_Angeles

The user instructed “go clear them”, delegating execution of the proposed unblock work. The original product/upgrade decision gate is resolved. No commit or push was made because the repository requires an explicit `$ship` invocation. Production access also failed closed: one discovered control-plane node disagrees with the local pinned SSH key, and two others have no trusted entry. No pin was replaced and no host-key check was disabled.

Full local evidence, tested digests, reproduction commands and subsequent production acceptance are in [the upgrade drill](../../../../docs/drills/2026-10-25-w7-infrastructure-upgrades.md). At this initial checkpoint the milestone remained open for production evidence; that acceptance is now complete below.

The prepared change selects reviewed chart 0.29.1 and the digest-pinned 1.30.1 operator, with string-valued `ENABLE_INSTANCE_MANAGER_INPLACE_UPDATES: "true"`. A separate disposable kind cluster avoided touching the degraded shared CAPD cluster or other dev-N stacks. The 1.30.0 baseline had in-place updates disabled; the upgrade enabled them at the same time as replacing the operator. All three database Pod UIDs/container IDs/restart counts and the single-instance postmaster start time were preserved. All 120 SQL reads succeeded. A NULL timestamp injected only into the disposable test view reproduced the original collector failure on 1.30.0 and produced valid WAL counters on 1.30.1 while NULL remained present. The standby was then recreated and rejoined with the fixture data intact.

The remaining acceptance at that checkpoint was shipping and current production verification. The local fixture did not configure Barman/S3; backup health is established separately by the production evidence below.

## Shipping checkpoint — 2026-10-25

The user explicitly authorized “ship and continue”. The implementation and verification record were pushed to `origin/main` at `418e5ccfd` (separate registry, CNPG and cache commits). Pre-push gates and rebased GitOps validation passed. Delivery was initially queued behind an older deployment; protected CI host-key fingerprints subsequently restored strict local access, and the reviewed-chart mirror published the locked artifact.

## Production acceptance and closeout — 2026-10-26 UTC

- Reviewed chart 0.29.1 and digest-pinned CNPG 1.30.1 are live with in-place updates enabled; Argo reports Synced/Healthy. The nine-cluster / 13-instance inventory includes five single-instance tenant databases. Explicit PostgreSQL operand images are unchanged.
- All 13 actual instance managers report 1.30.1. From 02:18:55 through 02:49:32 UTC, 33 consecutive estate observations passed, covering SQL availability, unchanged postmaster start times, Pod UIDs, PostgreSQL container IDs/restart counts, all nine clusters' readiness and continuous archiving. The monitor passed after 1,828 continuous seconds without errors.
- The affected instance's direct exporter passed 108 consecutive WAL scrapes over 1,804 seconds (02:18:07–02:48:11 UTC), with WAL counters present and collector error zero. SQL still returned NULL from `pg_stat_wal.stats_reset`; no statistics reset or production view change concealed the failure. A final untruncated log check through 02:51:43 found zero NULL WAL scan errors across all 13 instances. Prometheus intentionally drops these collector/WAL series, so acceptance used the exporter directly.
- Post-upgrade Barman plugin backup `bex-system/w7-cnpg-upgrade-zjz4h` completed from standby `bex-db-2`, 02:21:03–02:22:47 UTC, backup ID `20261026T022103`. It remains under existing retention. All nine clusters had healthy continuous archiving. Standby restart/recovery was exercised only in the disposable local fixture.
- Simplify, behavioral coverage, full operator tests, reviewed-chart rendering and GitOps validation passed. The temporary local cluster was removed without disturbing the shared CAPD cluster. All six tasks and the milestone are complete, with original IDs retained and no blocked-path stub.
