# w1 · m160 — Release identity: a never-served first release must not read Running, and an autoscaled worker must keep autoscaling

**Worker:** worker1 **Goal:** the phase a service reports after a failed or canceled build reflects what it can actually serve — Running only when a release really served — and a background worker's autoscaling transition ends, so it keeps reading metrics after its first scale. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | `fail` and the canceled-release path decide "a prior release exists" from `status.activeRevision` | 30m | — |
| t002 | A worker's autoscaling transition ends, so autoscaling continues past the first scale | 45m | — |
| t003 | Blast radius: every "a prior release exists" decision, and every path that starts a transition | 30m | t001, t002 |
| t004 | Live: a never-served first release, then a failed build | 45m | t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 45m | t005 |
| t008 | Closeout | 10m | t007 |

## Definition of done

Traced, not yet observed. t004 probes each bullet on production with a throwaway free service and records the pre-fix result first.

- **A never-served first release that then fails its build reads Failed.** A service whose first release crash-loops (so `status.image` is set but `status.activeRevision` is not), given a second release whose build fails, reports `phase: Failed`, not Running. The deploy row still reads `build_failed`.
- **The same for a cancel.** Canceling that second release leaves the phase Failed, not Running.
- **Control (must not regress).** A failed build over a release that did serve still reads Running with the prior release serving — the `w6/m124` behavior `w1/m157` verified live.
- **An autoscaled worker scales more than once.** Its scaling transition reaches Ended, and a later metric change moves replicas again. Autoscaling is a paid-only feature, so this bullet closes on the envtest evidence unless a paid worker is approved; the milestone records which.

## Root cause

- **The phase half (`w1/101`).** `fail` decides "a prior release exists" with `app.Status.Image != ""` (`app_controller.go`, around `:4294` at filing), and the canceled-release path does the same (around `:633`). `status.image` is set by a first release that never served. `settleFailedRollout` and, since `w1/m149`, `failPreDeploy` already key on `status.activeRevision`, which `markRunning` sets only once a release has served.
- **The autoscaling half (`w1/105`).** `completeAutoscalingTransition` (`autoscale.go`) is the only place a Started transition becomes Ended, and `applyAutoscaling` returns the recorded `ToReplicas` without reading metrics while one is Started. Its only callers are `reportKubernetesRunning` (web and private) and `convergeServingRuntime` (the held-release path). `reconcileWorkerStatus` never calls it, so a worker's first scale freezes its autoscaling for good.

## Blast radius

- **Who is hit.** Any service whose first release never served and whose next build fails or is canceled — it reads Running while serving nothing. Every autoscaled background worker, after its first scale event.
- **Severity.** Minor for the phase half (it needs a never-served first release); moderate for the autoscaling half, which is a paid feature that silently stops working.

## Source + Goal linkage

- **Source:** `w1/101` (code review during `w1/m149` t005) and `w1/105` (`w1/m158` t005). Both were traced from code and never probed; promoted together 2026-09-15 during the w1 triage because they are one question — which recorded fact a release's identity is read from.
- **Goal linkage:** `docs/ADR004-app-deployment.md` (a failed deploy leaves the previous revision serving, and the phase says what is running) and `docs/ADR018-render-parity.md` (Render reports a service that is not serving as failed, and keeps autoscaling live).
- **Expected outcome:** the phase never claims Running for a service that never served, and an autoscaled worker keeps following its metrics.
- **Why now:** both sit in the code `w1/m156`–`w1/m158` just reworked, so the context is loaded and the tests are already in place to extend.
- **Render parity is included** because the phase is read through REST, GraphQL, MCP and the dashboard header.
