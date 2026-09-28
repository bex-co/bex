# w7 · m154 — Bound cron resource names across lifecycle operations

**Worker:** worker7 **Goal:** Allow accepted long cron service names to execute and preserve existing schedules and lifecycle operations. **Status:** todo

**Estimate:** 120m implementation; 220m including standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Derive bounded collision-resistant CronJob names | 40m | — |
| t002 | Converge all cron lookups and lifecycle paths on the naming rule | 45m | t001 |
| t003 | Verify boundary-length scheduled and manual execution | 35m | t002 |
| t004 | Render parity | 30m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- [ ] Derived names fit the 52-character boundary and remain stable and distinct for different Apps.
- [ ] All relevant paths resolve the same CronJob; previously valid short-name resources keep their identity and long-name Apps do not create duplicate schedules.
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
