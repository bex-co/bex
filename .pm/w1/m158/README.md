# w1 · m158 — A background worker whose newest release is held (a pre-deploy step or a build) ignores suspend, resume and scale

**Worker:** worker1 **Goal:** a background worker's replicas follow suspend, resume, manual scale and autoscale while a newer release waits on its pre-deploy step or its image. They move on the prior release's pod template, and the held release never rolls. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Worker replica convergence under both holds: a scale-only patch of the prior Deployment plus the worker's parked or running status | 1h | — |
| t002 | Blast radius: every worker transition against the pre-deploy hold and the artifact hold | 30m | t001 |
| t003 | Live: suspend, resume and scale a background worker over a failed pre-deploy step and a failed build | 45m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

Traced, not yet observed. t003 probes each bullet first and records the pre-fix result.

- **Suspend and resume over a failed pre-deploy step.** A worker serving release 1 whose release 2 recorded `pre_deploy_failed`:
  - **Suspend.** Within one reconcile its Deployment scales to 0 and the service reads suspended (Hibernated).
  - **Resume.** It scales back to its instance count on release 1's pod template.
  - **The failed release.** Its deploy stays `pre_deploy_failed`, and no pre-deploy Job runs again.
- **The same over a failed build.** Release 2 recorded `build_failed`.
- **Manual scale while a pre-deploy step runs.** A new instance count takes effect on release 1 while release 2's step is still running, and release 2 rolls once its step passes.
- **Control (must not regress).** A healthy worker suspends, resumes and scales as before. Web and private services keep `w1/m156` and `w1/m157`'s behavior.

## Root cause (line numbers at `710ebe07e`)

- **The pre-deploy hold skips workers.** `holdUnpassedRelease` returns `held=false` for a worker (`lego/operator/internal/controller/app_controller.go:4588`). `reconcileKubernetes` therefore returns on the pre-deploy gate's halt (`:2053-2056`) before the Deployment write (`:2059`) and `reconcileWorkerStatus` (`:2093`).
- **The artifact hold skips workers.** `w1/m157` scopes its hold to web and private services. A worker whose newest build failed or is running halts in `resolveDeployImage` (`:692-696`) the same way.
- **Why it matters.** A worker has no Service, no Ingress and no auto-sleep, but its replica count still follows suspend, resume, manual scale and autoscale (`desiredReplicas`, `:2301-2324`).

## Blast radius

- **Who is hit.** Every background worker whose newest release is held. Suspend is silently ignored, so the worker keeps running and consuming. Resume leaves it at 0. Scale changes are dropped until the owner ships a new release.
- **Severity.** Moderate. Explicit user actions are ignored, but only while a newer release is held.

## Live-verification constraint

- **Plan.** Background workers may require a paid plan. t003 creates one only when the QA workspace can do so without a purchase.
- **Otherwise.** The live step moves to `w1/blocked/` with the exact question, and the unit and envtest coverage stands in for it.

## Dedupe

- `w1/done/103.md`: this milestone's source, promoted.
- `w1/done/m156`: the web and private pre-deploy hold. Its `convergeServingRuntime` gives the replica half.
- `w1/m157`: the web and private artifact hold. Its shared prelude and serving-prior lookup are reused.

## Source + Goal linkage

- **Source:** `w1/103`, promoted 2026-09-15 by user decision during `/loop-worker w1`. `w1/m156` t002 traced it from the code.
- **Goal linkage:**
  - `docs/ADR004-app-deployment.md`: a failed deploy leaves the previous revision serving;
  - `docs/ADR018-render-parity.md`: suspend, resume and scale act on a background worker's running instances.
- **Expected outcome:** suspend, resume and scale always take effect on a worker, whatever state its newest release is in.
- **Why now:** it is the last type left in the held-release class after `w1/m156` and `w1/m157`, and explicit user actions on paid services are silently ignored. It sequences after `w1/m157`, whose shared helpers make it small.
- **Render parity is included** because the phase and instance count surface through REST, GraphQL, MCP and the dashboard.
