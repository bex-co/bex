# w7 · m154 — Bound cron resource names across lifecycle operations

**Worker:** worker7 **Goal:** Allow accepted long cron service names to execute and preserve existing schedules and lifecycle operations. **Status:** in-progress

**Estimate:** 120m implementation; 220m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Derive bounded collision-resistant CronJob names — **DONE** | 40m | — |
| t002 | Converge all cron lookups and lifecycle paths on the naming rule — **DONE** | 45m | t001 |
| t003 | Verify boundary-length scheduled and manual execution | 35m | t002 |
| t004 | Render parity — **DONE** | 30m | t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 40m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- [x] Derived names fit the 52-character boundary and remain stable and distinct for different Apps.
- [x] All relevant paths resolve the same CronJob; previously valid short-name resources keep their identity and long-name Apps do not create duplicate schedules.
- [ ] Long accepted names execute on schedule and manually; lifecycle actions target the right resource and cleanup leaves no orphan schedule. Record actual runtime evidence, not only fake-client success.

- [ ] All required closing tasks are complete; production-dependent claims have dated runtime evidence, not just green unit tests.

## Source + Goal linkage

- **Source:** User-approved platform-log brainstorm, 2026-09-28 UTC (September 27 America/Denver); user requested all five in w7. Read-only collection at source revision 016391810: 342 container streams, 167 pods, 22 namespaces, plus Loki platform history. Requested prior 24 hours; rotation/deleted pods limit coverage. Raw local evidence: /tmp/bex-platform-log-audit/{manifest.json,pods.json,events.json,argo.json,loki-platform.jsonl}; temporary artifacts may expire, so the observed findings are preserved below and must be revalidated at execution.
- **Observed finding:** The audit counted 150 operator reconciliation errors for tea-daif693dqjvc73e7as3g-qa-20260922-cd9361-longcronxx: CronJob metadata.name must be no more than 52 characters. The App reported Failed. lego/operator/internal/controller/app_controller.go:3619 copies app.Name into CronJob metadata; the backfill lookup at approximately :1785 also assumes the App name.
- **Goal linkage:** ADR008 reliable self-hosted Render-compatible hosting, deterministic lifecycle state and platform operability. Allow accepted long cron service names to execute and preserve existing schedules and lifecycle operations.
- **Expected outcome:** Allow accepted long cron service names to execute and preserve existing schedules and lifecycle operations.
- **Why now:** A service accepted by the platform cannot create its scheduled execution resource.
- **Existing work / deduplication:** No matching naming milestone found in the board audit. w4/blocked/m114 concerns run concurrency and terminal state, not this resource-name admission failure.
- **Render parity:** Included: this fix affects tenant-facing cron lifecycle or log-level filtering; validate the corresponding ADR018 row and all exposed adapters.
- **Scheduling:** approved priority 2 of five in w7. Infrastructure rollout verification follows m153's root-sync repair; isolated implementation can proceed independently. No dependency on blocked w7 datastore/sandbox walkthroughs is inferred.

## Scope and evidence limits

This is planned work, not an implemented fix. The read-only audit did not authorize implementation during collection. Revalidate the deployed revision and failure before executing these tasks. Raw logs and credentials stay out of Git; preserve only sanitized observations. Do not reopen the accepted PSL finding, duplicate the known blockeden.xyz certificate work, or infer root causes from transient etcd/storage warnings.

## Evidence (2026-09-28 UTC, worker7)

**Revalidated in production (read-only, 06:52Z):** App `tea-daif693dqjvc73e7as3g/tea-daif693dqjvc73e7as3g-qa-20260922-cd9361-longcronxx` (54 chars) is `Failed`; `app_controller.go` named the CronJob `app.Name` verbatim in create (`convergeCronRuntime`), the imagePullSecrets backfill and the pending-artifact hold. Same latent bug in `keyValueBackupName`/`diskBackupName`, which bounded CronJobs at 63, not 52.

**Implemented (t001/t002):** `k8sname.FitCronJob` (52-char bound on the existing Stable hash-bound truncation) and `appv1alpha1.CronJobName`; every App CronJob create/get/patch path uses it; KV/disk backup CronJob names use `FitCronJob`. `ManualCronRunJobName` now `k8sname.Fit`s to 63 (a 54-char App produced a 67-char manual Job name). Names that already fit are returned unchanged, so no existing CronJob or run id is renamed; names over 52 could never have been admitted, so no orphan or duplicate schedule arises. Reported App ⇒ CronJob `tea-daif693dqjvc73e7as3g-qa-20260922-cd-111410f14aed` (52), manual run Job `…-longcr-5f1401434db2` (63).

**Locally verified (t005):** envtest against a real kube-apiserver (`cron_long_name_envtest_test.go`): the reported 54-char name creates exactly one owned CronJob across repeated reconciles, reaches `Running`, suspend/resume toggles it, a manual run materializes its Job and appears in `status.runs`, and the schedule pauses during it; a 52-char name keeps its exact identity. Mutation (pre-fix `app_controller.go`) reproduces the production error verbatim: `must be no more than 52 characters`. `k8sname` unit test covers boundary, determinism, and distinctness across shared prefixes. `make test` (operator), `go test ./...` (types), backend `internal/apps`, `make lint` (all modules): pass. Local cluster admission check: old name rejected, derived CronJob and manual Job admitted; scheduled execution could **not** be observed locally — the dev cluster's kube-controller-manager is crash-looping on apiserver lease timeouts (the shared-VM capacity condition gating m149–m151), so probes were deleted.

**Render parity (t004):** no API shape change; all surfaces derive run ids through the one shared `ManualCronRunJobName`, unchanged for names that fit. Recorded in `docs/render-artifacts/cron-runs.md`.

**Simplify (t005 of the standing set):** inline self-review; the three call sites share one helper and the old KV-specific truncation was deleted.

**Remaining (t003):** after the operator rolls out, recheck the reported production App read-only: CronJob admitted, App leaves `Failed`, a scheduled run executes.
