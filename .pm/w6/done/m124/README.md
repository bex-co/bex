# w6 · m124 — A service that is serving traffic reports "Failed" because its latest build failed — the coarse phase contradicts the running instance, on every surface

**Worker:** worker6 **Goal:** a service that is serving reports that it is serving, on REST, GraphQL, MCP and the dashboard, while its failed deploy stays visible as a deploy fact **Status:** done — closed 2026-08-29: live probes passed (serving service with a failed build on top reports Running on REST+GraphQL while curl returns 200 and the deploy row says build_failed; first-build-fails control reports Failed; dashboard header shows no "Service Failed"); see done/t007.md

## Tasks (in order)

| id   | title                                                                                   | est | depends_on |
| ---- | --------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Re-source the deploy row's build verdict from the Build condition, not the coarse phase — **DONE** | 50m | —          |
| t002 | Gate the terminal `Failed` phase on whether a prior release is still serving — **DONE**    | 50m | t001       |
| t003 | Dashboard: the badge reflects the service; the failed deploy stays its own indicator — **DONE** (fell out of t002: `deriveStatus` maps phase, the latest-deploy link reads the deploy row; no UI change needed) | 35m | t002       |
| t004 | Render parity — **DONE** (decision recorded on ADR018's "Get service" row: parity restored, no new row) | 25m | t003       |
| t005 | Simplify — **DONE** (reason vocabulary moved onto the CRD contract in `lego/types`)        | 20m | t004       |
| t006 | Test coverage — **DONE** (operator: Running/Hibernated/first-failure/non-build cases; backend: verdict + message from the Build condition, stale-condition guard, healthy availability) | 40m | t004       |
| t007 | Closeout — credential-dependent live production DoD re-run (fixture, MCP probe, first-deploy-failure case, deploy-row check), then move the milestone to `done/` — **DONE** | 15m | t005, t006 |

## Implementation note (2026-08-28)

- Operator: `r.fail` now records the Build condition first and, when `Status.Image` is set (a prior release exists), settles the phase via `settleFailedBuildOverPriorRelease` — `Running` with `Ready=True/PriorReleaseServing`, or `Hibernated` (reason `AutoHibernated`/`Suspended`) when the Deployment is parked at 0 replicas, probed from the informer cache. No image ⇒ `PhaseFailed` unchanged. The quiesce gate (`terminalBuildFailureRecorded`) keys on the Build condition, not the phase, so the App still quiesces.
- Backend: the deploy row's `build_failed` verdict was already Build-condition-sourced (`supersededDeployStatus`, w6/m100); the failure **message** now is too (`deployCloseFailureReason` reads the Build condition first, Ready-condition fallback for pre-m100 operators, then the build-window line).
- **Unhealthy ≠ running — the covering signal:** the phase/Ready stamped here describe the release's last observed state; live instance health remains covered by the rollout observation's own Ready reasons (`CrashLoopBackOff`, `ImagePullBackOff`, readiness) feeding `observedServiceStateFor`'s `unhealthy` availability + events, and by the live pod reads behind `GET .../instances` and metrics. The failed build no longer masquerades as that signal.
- All four suites green: `make test` (operator, envtest), backend `go test ./...`, `make lint` (4 modules), dashboard `yarn test` (one pre-existing flake in `agent-sessions/new-session-composer` under full-suite load; passes in isolation; unrelated — no dashboard files changed).

## Definition of done

- **A serving service reports that it is serving.** Recreate the state — a `web_service` with a successful deploy serving traffic, then a failing build on top (the 57th run's recipe works: point `dockerfilePath` at a nonexistent file). With the previous release still answering `200`, REST `GET /v1/services/{id}` and GraphQL `service(id:){phase}` both report a **running** service. Today both report `"Failed"` while `curl` returns `HTTP/2 200` and `GET .../instances` returns 1 instance.
- **The dashboard stops contradicting itself.** The service header does **not** say "Service Failed" while the service serves 200; it still shows "Latest deploy: Build Failed". Today it shows **both at once**.
- **The deploy row still reaches `build_failed`.** Verified on the same fixture via `GET /v1/services/{id}/deploys?limit=1`. This is the regression the fix is most likely to cause — check it explicitly, do not assume it.
- **A genuinely failed service still says Failed.** A build failure with **no** previously deployed release reports a failed service on every surface. Verify with a service whose **first** build fails, so there is no image to fall back to.
- **Unhealthy ≠ running.** A service whose running instance is genuinely unhealthy is still not reported as running — state which signal covers that, since this milestone deliberately stops using "latest build failed" as a proxy for it.
- **The control still passes:** a healthy service reports Running with a live deploy (measured this run on `qa-20260827-env` before deletion).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **58th run**, 2026-08-27, journeys 2 + 14. Workspace `tea-d98210cbbpdc73dcrkvg`. All probes re-runnable.

  **The failing case** — `qa-20260826-webhook-renamed` (`srv-da7o6ovvqdcc73bpn9hg`), a free `web_service` whose last two builds failed but whose previous release is still deployed:

  ```
  $ curl -sS  https://qa-20260826-webhook-svc.onbex.co/
  OK — pushed v3
  $ curl -sSI https://qa-20260826-webhook-svc.onbex.co/
  HTTP/2 200 · content-type: text/plain · content-length: 16

  REST    GET /v1/services/srv-da7o6ovvqdcc73bpn9hg  -> phase "Failed", suspended "not_suspended"
  GraphQL query { service(id:){ phase } }            -> phase "Failed"
  REST    GET .../instances                          -> 1 instance
  REST    GET .../deploys?limit=1                    -> status "build_failed"
  ```

  **The control**, created and destroyed inside the same run — `qa-20260827-env` (`srv-da89qbgueu1c7395j6dg`), free `web_service`, runtime `node`, public repo `github.com/render-examples/express-hello-world`, built successfully: REST phase `Running` · GraphQL phase `Running` · 1 instance · latest deploy `live` · `curl` → 200.

  Same workspace, same plan, same type, both serving 200 from one instance — and they report different phases. The only difference is whether the **latest build** succeeded, not whether the service works.

  **What the user sees** (accessibility tree, `/services/srv-da7o6ovvqdcc73bpn9hg`):

  ```
  heading "qa-20260826-webhook-renamed"
  text: Service Failed                         <- false; it serves 200
  link "Latest deploy: Build Failed"           <- true, and already sufficient
  ```

- **Root cause:** `lego/operator/internal/controller/app_controller.go:3913`, inside `r.fail` — `app.Status.Phase = appv1alpha1.PhaseFailed`, unconditional. It does not consider whether a previous release is still deployed and serving. The reconcile then quiesces deliberately (`:832-839` explains why it must not re-fail), so the App stays `Failed` indefinitely.

- **The target behaviour is already written down in this repo, and already implemented one function away.** `lego/types/v1alpha1/app_types.go:1104-1111`, the doc on `PhaseCanceled`:

  > "`PhaseCanceled`: the user canceled the release that was rolling and no earlier release ever succeeded, so nothing is running and nothing failed. **Reusing `PhaseFailed` here made a service contradict its own deploy history** … a healthy release never reaches this state: **that release keeps serving and the phase stays Running**."

  And the cancel path implements it, `app_controller.go:577-584`:

  ```go
  if app.Status.Image != "" {
      return r.dispatchRuntime(ctx, app, app.Status.Image, port)
  }
  // Canceled, not Failed: ... the coarse phase reused PhaseFailed, so the
  // service reported an error for something the user did on purpose —
  // contradicting its own deploy row, which reads "canceled" (w6/m52).
  app.Status.Phase = appv1alpha1.PhaseCanceled
  ```

  `w6/m52` fixed this exact contradiction for **cancel**, and the guard it used — "is there a previously released image?" — is the predicate the failure path needs. So this is not an open design question: bex decided the rule, documented it on the type, and applied it to the sibling case.

- **The consumer trap — the fix is NOT simply "stop setting `PhaseFailed`".** `lego/backend/internal/store/reconciler.go:1027-1031` derives the **deploy row's** status from the App phase:

  ```go
  case appv1alpha1.PhaseRunning:
      if releaseIsActive(open, app) { return DeployLive }
  case appv1alpha1.PhaseFailed:
      if conditionCurrent {
          switch {
          case appv1alpha1.IsBuildFailureReason(reason): return DeployBuildFailed
  ```

  Keep the phase `Running` after a failed build and this switch never reaches the `PhaseFailed` arm, so the deploy row stops reaching `build_failed` — trading a wrong service status for a wrong deploy status. The verdict must be re-sourced from the **Build condition**, which `r.fail` already sets separately and durably at `app_controller.go:3920-3926`, attributed to the release generation that actually built (`w6/m100`). That condition is the right source; the coarse phase is not.

- **Blast radius — counted.** Scoped to the **App** phase (the `build.PhaseFailed` / `publish.PhaseFailed` hits in the same grep are unrelated local enums in other packages):
  - **Writers: exactly 1** — `app_controller.go:3913`.
  - **Backend readers: 2** — `store/reconciler.go:1027` (the deploy-status derivation above) and `store/reconciler.go:1209` (`if app.Status.Phase == appv1alpha1.PhaseFailed && c.Message != ""`, which sources the failure message).
  - **Dashboard: 1 component** — `dashboard/src/features/services/components/service-status-badge.tsx` renders `deriveStatus(service)` from `features/services/lib/status.ts`. Its own doc says "suspension wins over phase" and that it is "One component for every place a status shows — the list rows, the detail header, and the overview panel — so they can't drift", so one change moves all three placements together.
  - Evidence that phase already carries known nuance: `store/reconciler.go:81` records that an ImagePullBackOff "never makes the App CR's own phase machine reach PhaseFailed — it polls PhaseDeploying".

- **Goal linkage:** [docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md) and [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md) — `phase` is a first-class field on REST, GraphQL and the dashboard, and it is the field a user checks to answer "is my service up?".

- **Expected outcome:** a serving service reports that it is serving, everywhere, while the failed deploy stays visible as a deploy fact.

- **Why now:** this misreports the single most important thing the product says about a service, and in the alarming direction — a tenant seeing "Service Failed" on a service that is actually fine may roll back, redeploy, or escalate for nothing. It compounds `w6/m123` (57th run), where the same failed build also reports an unusable failure reason: the user is told their service failed **and** cannot find out why.

- **Precedent — extend, do not re-litigate.** `w6/m52` fixed the identical contradiction for the cancel path and left its reasoning in `app_types.go:1104-1111` and `app_controller.go:580-584`. Cite it as the decided rule; this milestone applies that rule to the build-failure path. **Not** a regression of m52 — m52's scope was cancel.

- **Render parity:** included (t004). `phase` is exposed on REST and GraphQL and rendered in the dashboard, so all three move together; confirm MCP's service read carries the same phase and moves with it. Render distinguishes service state from deploy state — a failed deploy leaves the previously deployed version serving and the service is not presented as failed — so this restores parity rather than diverging.

- **Adjacent classes:** place every neighbour under the new rule rather than fixing only the observed case — **suspended** (already wins over phase in the dashboard), **auto-hibernated/sleeping** (`w6/m116`–`m119`'s territory; must not be swept into "running"), **`PhaseCanceled`** (already correct; do not disturb), a **first-deploy failure with no prior image** (must stay Failed), and **ImagePullBackOff**, which `reconciler.go:81` says already never reaches `PhaseFailed`.

- **Unverified this run — carried as work, not presented as observation:** `app.Status.Image` was **not** read directly — the GraphQL query errored with `Cannot query field "image" on type "Service"` — so "a prior release image is present" is **inferred** from the service serving 200 from one running instance rather than read off the CR; **MCP's** service read was never probed for phase; the **first-deploy-failure** case (where `Failed` is correct) was not exercised; and no cluster access was available to confirm the Deployment's replica state directly.

## Triage note (2026-08-27) — overlaps `w6/m97/t007`, land t002 first

`w6/m97/t007` (open) fixes the *reporting* half of the same symptom: `observedServiceStateFor` (`lego/backend/internal/store/reconciler.go:1298-1315`) labels every Ready=False reason outside a three-item exclusion list as `readiness_failed`, so a service driven to `Failed` by a pre-pod-evaluation refusal tells the operator's investigator "A running instance stopped passing readiness checks" — a plausible, wrong cause.

This milestone's `t002` ("Gate the terminal Failed phase on whether a prior release is still serving") is the *upstream* fix: with it, a service whose prior release is serving never reaches `PhaseFailed`, so the misleading event is never emitted for that case at all. The two do not duplicate each other — m97/t007 still owns the reason-code mapping for refusals that genuinely have no serving release — but `t002` should land first so m97/t007's remaining scope is measured against the corrected phase behaviour.
