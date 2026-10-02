# m109 isolated dev-5 live evidence

Date: 2026-10-02 UTC. Fixture: `srv-davjt41jg4r0n61fs8hg`, workspace `tea-davjt2hjg4r0n61fs8gg`, App/CronJob `tea-davjt2hjg4r0n61fs8gg-w5-m109-cron`.

## Outcome

Passed with real dev-5 API, GraphQL, MCP, dashboard and Kubernetes Jobs/Pods. Unmodified production source generated **43,200 seconds** for both the CronJob template and an executing manual Job, with `backoffLimit=0`. A temporary scoped Go process overlaid only the deadline constant to **12 seconds**; no tracked source or board file was changed by the live agent.

| Execution                                  | Observed result                                                                                                                                                                                                                                    |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Baseline manual `…-run-e2fb6740`           | Actual Pod Running; deadline 43,200; canceled through REST, Job deleted.                                                                                                                                                                           |
| Natural scheduled `…-29848641`             | CronJob controller's scheduled timestamp and Job startTime both 05:21:00Z. Existing active Job adopted 12 seconds at 05:21:15Z, retaining UID/startTime. `FailureTarget/DeadlineExceeded` 05:21:15Z; terminal `Failed/DeadlineExceeded` 05:21:47Z. |
| Manual `…-run-10b9ca11`                    | API trigger 05:21:50Z; created with deadline 12 and Job startTime 05:21:52Z. `FailureTarget/DeadlineExceeded` 05:22:04Z; terminal `Failed/DeadlineExceeded` 05:22:35Z.                                                                             |
| Later natural scheduled `…-29848644`       | Quick command deployed; schedule set to the next UTC minute (`24 05 * * *`). Real CronJob controller tick at 05:24:00Z; succeeded 05:24:03Z, one Pod, exit 0, `W5_M109_QUICK` log.                                                                 |
| Later manual `…-run-86d1824b`              | Start 05:24:08Z; succeeded 05:24:12Z, one Job UID and one Pod, exit 0, `W5_M109_QUICK` log.                                                                                                                                                        |
| Later manual cancellation `…-run-db27795d` | REST DELETE returned 204 with empty body at 05:24:23Z; accepted cancellation remained `Canceled`, Job/Pod deletion completed and scheduling released.                                                                                              |
| Suspend/resume                             | Controller actually reached `Hibernated` with CronJob suspended at 05:25:00Z, then `Running` with CronJob unsuspended at 05:25:01Z.                                                                                                                |

During manual deadline termination the CronJob remained suspended. It resumed only after the backing Job became terminal. Both timeout Pods were observed Running with deletion requested, then absent when Job status became Failed (`failed=1`, `terminating=0`). The Pods were deleted promptly; their final exit codes were not retained. A total of 234 observations between 05:20:41Z and 05:25:17Z saw at most one active-or-terminating Job.

## Public history

REST, GraphQL and MCP return identical IDs, status, start and finish timestamps. The two deadline runs are **`unsuccessful`**, not success or cancellation:

- Scheduled: `crr-j5nnhqlu34d4sunnh79b`, 05:21:00Z–05:21:47Z.
- Manual: `crr-kmd0g15uvqv682mavdkk`, 05:21:52Z–05:22:35Z.

During `FailureTarget`/Pod grace, the surfaces remained `pending`; the manual schedule stayed paused. Dashboard Events rendered both terminal runs as **Failed**, with durations 47s and 43s. See the `ui-timeouts` and `surfaces-timeouts` entries in `runtime.json`. Final API history is canceled, successful, successful, unsuccessful, unsuccessful, canceled. Dashboard's five-row page showed Canceled, Succeeded, Succeeded, Failed, Failed (`ui-final` in `runtime.json`). Final cancellation was accepted before its deadline; Kubernetes also emitted FailureTarget during foreground-deletion grace, while public history correctly retained the accepted user cancellation.

## Method and limits

The service was created through dev-5 GraphQL using cached `docker.io/library/busybox:1.36`. A temporary test harness called the real AppReconciler only for this fixture, using the dev-5 kubeconfig and namespaced write guard. No production or other dev stack was touched. This is actual API → App → scoped reconciler → Kubernetes execution and API/UI readback, not a full deployed manager/event-watch test.

The original-source process first proved actual 43,200-second objects. It was fully stopped before launching a Go `-overlay` process changing `cronRunActiveDeadlineSeconds` from `12 * 60 * 60` to `12`. The first scheduled run exercised adoption of an existing active Job; the manual run exercised a new 12-second Job. Both scheduled runs were generated naturally by Kubernetes, with scheduled timestamps; no fabricated terminal status or manually created scheduled Job was used.

This is **not a twelve-hour-duration test or exact Render timing parity**. Kubernetes counts from Job startTime, including scheduling/image pull, and honors Pod termination grace. The observed 43s/47s include grace and controller timing, and the initial scheduled Job was already 15s old when the temporary process adopted the shorter deadline. Scheduled overlap, grace and skipped/catch-up tick behavior retain Kubernetes semantics.

## Cleanup and review

Completed at 05:25:33Z: service deleted through REST; App cleanup reconciled; fixture and automatically created identity workspace deleted; both namespaces confirmed absent; identity confirmed 404; private credentials removed; observer and both scoped reconciler processes stopped; temporary deadline source and overlay mapping removed. dev-5 API health remained 200. See `cleanup-proof` in `runtime.json`.

Read-only Simplify QUALITY review of `/private/tmp/w5-m109-review.diff`: **clean**. Shared constant and ownership helper keep the policy direct; adoption uses the existing Job-list pass; no redundant persistent state, unnecessary abstraction or clarity issue found. No edits or additional scope.

`runtime.json` combines sanitized snapshots, surface results, public fixture identifiers, review result and cleanup proof. Selected snapshots preserve the observed Job/Pod conditions; the full raw polling trace remains local to the temporary harness.
