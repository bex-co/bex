# w7 · m83 — Build-plane observability + the two structural guards

**Worker:** worker7 **Goal:** the next capacity claim is read from a series instead of argued from arithmetic, and the two thin invariants the build plane silently depends on are guarded. **Status:** done

## Tasks (in order)

| id   | title                                                                                       | est | depends_on                            |
| ---- | ------------------------------------------------------------------------------------------- | --- | ------------------------------------- |
| t001 | `bex_build_queue_seconds` + `bex_builds_queued` / `bex_builds_active` — **DONE**              | 45m | —                                     |
| t002 | Alert on build queue p95 breach / build `Pending` beyond N minutes — **DONE**                 | 30m | w7/m83/t001                           |
| t003 | Prewarm follows the build pools: build-taint toleration + completeness guard — **DONE**       | 35m | —                                     |
| t004 | `clusterapi-validate.sh` arithmetic guard: CA hint ≥ build request + DaemonSet floor — **DONE** | 45m | w7/m83/t003                           |
| t005 | D6: global `BEX_MAX_ACTIVE_BUILDS` ceiling + re-count-after-create overshoot fix — **DONE**   | 45m | w7/m83/t001                           |
| t006 | D7: check the build baseline env into the gitops path — **DONE**                              | 20m | w7/m83/t005                           |
| t007 | Simplify the code this milestone changed — **DONE**                                           | 30m | w7/m83/t002, w7/m83/t004, w7/m83/t006 |
| t008 | Test coverage for the shipped behavior — **DONE**                                             | 40m | w7/m83/t002, w7/m83/t004, w7/m83/t006 |
| t009 | Closeout — **DONE**                                                                           | 15m | w7/m83/t007, w7/m83/t008              |

## Definition of done

Queue time and run time are separate series, with an alert on the former. The prewarm DaemonSet is scheduled on every node carrying the build taint, and its guard turns CI red without the toleration. The arithmetic guard turns CI red if any build pool's cluster-autoscaler hint drops below the build request plus the DaemonSet floor. Admission has a global active-build ceiling and self-corrects overshoot. The build baseline env is readable from git rather than set out of band.

**Verified clause by clause, 2026-08-18:**

- **Separate series + an alert on queue time.** `bex_build_queue_seconds` (admission → first placement) sits alongside `bex_build_run_seconds`, with `bex_builds_active` / `bex_builds_queued` / `bex_build_queue_oldest_seconds` for the live picture and `bex_build_push_seconds` / `_errors_total` / `bex_build_retries_total{reason}` (the m82 t004 deferral) off the same pod read. `BuildQueuedTooLong` fires at ~10 minutes of queueing — eight minutes before bex-api's 18m `DeployGateTimeout` closes the deploy — and `BuildQueueTimeHigh` covers the chronic case; both are promtool-unit-tested in `deploy/gitops/base/rules/alerts_test.yml`.
- **Prewarm toleration + guard.** `deploy/gitops/base/build-image-prewarm.yaml` tolerates `bex.co/build-only=true:NoSchedule`; `scripts/clusterapi-validate.sh` fails closed when any build-tainted MachineDeployment lacks a matching prewarm toleration, and `scripts/clusterapi-validate.test.sh` proves it red for a missing toleration, a wrong-taint toleration, and a drifted image, green for a keyless `Exists`.
- **Arithmetic guard.** The floor is computed — build request `sed`-read from `build.go`, plus every repository-declared DaemonSet that tolerates the build taint, plus the Alloy chart's off-repo sidecar allowance carried with the chart itself — and each build pool's CA hint must clear it, must not exceed measured allocatable (simulation stays conservative), and must run a server type whose allocatable is recorded. Proven red in five directions.
- **Global ceiling + overshoot self-correction.** `BEX_MAX_ACTIVE_BUILDS` (production `4`) refuses a dispatch past the cluster-wide cap with `BuildQueued`; a racing double-create re-counts after create and sheds the **newest** build. envtest + unit tests, and `0` asserted byte-identical.
- **Baseline in git.** All three variables are explicit in `lego/operator/config/manager/manager.yaml` — the path Argo syncs — with `BEX_APP_RECONCILE_WORKERS`' post-D1 meaning stated.

## Outcome

**Beyond the DoD — the series were unreadable.** The manager's metrics endpoint was disabled (controller-runtime's `--metrics-bind-address` defaults to `0`), the checked-in metrics Service pointed at a port nothing bound, and `deploy/gitops/base/prometheus.yaml` had no operator scrape job at all. Every `bex_build_*` series existed only inside the process, so **w2/m72's build-outcome alerts had never been able to fire** and neither could this milestone's. Fixed end to end: HTTP metrics on `:8080`, the Service repointed, and a `bex-operator` scrape job with a bounded keep-list. This is what makes "read from a series" true rather than aspirational.

**`/simplify` found a real metering bug.** The failure branch's once-per-build gate never closed: `r.fail` deliberately does not stamp `Status.ObservedGeneration` (a failed build must not advance the last-good release `successfulReleaseGeneration` derives from it) and it returns an error, so the branch re-enters ~20×/hour until the Job's TTL — re-observing the same queue wait into the histogram the new p95 alert pages on, and inflating m82's outcome counters. Fixed with a durable marker read off the Ready condition `r.fail` already persists, pinned by an envtest proven red against the pre-fix tree. The same pass also caught a sub-second-queue-time drop (metav1.Time is second-granular, so warm-node builds — the regime the low buckets exist to resolve — were silently excluded, biasing p95 upward), an unconditional pod listing on an idle build plane, a chart sidecar allowance that outlived the chart it belonged to, and three duplicated definitions of "was this pod placed on a node".

**No hardware change**, per `builder-issues.md` §4 — the capacity thread's only survivor is the guard.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.3, §3.4, §3.5 (P2/P3/P4) and §4 (the measured capacity close-out); [docs/ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md) D5/D6/D7. **Retires [`.pm/w7/024.md`](../done/024.md)**, whose branch 3 was refuted by measurement — t004 is that note's surviving residue.
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #2 (basic obs for operation) and #6 (add & remove physical machine).
- **Expected outcome:** cold-node builds stop paying the image-pull tax; a 305 MiB scheduling margin can no longer erode silently; and "should we add machines?" becomes a query rather than an argument.
- **Why now:** this milestone's own investigation is the evidence. With no queue series, the 2026-08-17 capacity question had to be settled by hand against a live cluster — and the first answer was **wrong** (it assumed cilium and friends carried meaningful memory requests; measured, they request 0). Two prior incidents in the same class (2026-08-08 wedged build, 2026-08-11 22-minute Pending) also presented as silence. Separately, D8 tainted the build pools on 2026-08-15 and the prewarm DaemonSet was left behind on the serving pool, which measurement confirmed.
- **Render parity task omitted:** yes — this is operator/infra mechanism only (Prometheus series, a DaemonSet toleration, CI guards, an admission ceiling, and gitops env). Nothing here changes a REST, GraphQL, MCP or dashboard surface.

## Residuals (not blockers)

- **Not live-verified.** The DoD is deliberately repo-verifiable (the `m77` lesson). The prod control-plane SSH host keys have rotated since they were last recorded, so `scripts/fetch-app-kubeconfig.sh` fails closed; the checked-in baseline was verified against the GitOps source of truth instead (Argo syncs `lego/operator/config/default` with `selfHeal: true`). The new scrape job, the metrics port, and the prewarm toleration take effect on the next Argo sync + operator deploy.
- **`BEX_MAX_ACTIVE_BUILDS=4` is a new ceiling**, not a mirror of current production (which runs uncapped). It is set to the D8 pools' physical concurrency (2 warm lg + 2 elastic burst), so it cannot throttle below what the pools can actually run.
- **kpack dispatch is not trimmed.** `TrimOvershoot` counts kpack Images toward the cap but sheds only Jobs; a buildpack build that overshoots is bounded by the pre-create gate alone.
