# w6 · m119 — Keep private services out of public-edge auto-sleep

**Worker:** worker6 **Goal:** only services with an observable activity signal and wake path auto-sleep; the dashboard promises wake-on-request only for those services **Status:** done — closed 2026-08-29: live probes passed (free private service with 60s TTL stays Running past the window; worker exclusion holds 14+ min; Settings offers the idle control only where its sentence is true); see done/t008.md

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Decide whether a private service should auto-sleep at all, given it has no wake path — **DONE** | 30m | — |
| t002 | Enforce the decision in the operator — **DONE** | 45m | t001 |
| t004 | Stop the dashboard offering an idle timeout that promises a wake it cannot deliver — **DONE** | 30m | t001 |
| t009 | Re-verify this milestone's own claims with tight sampling — the original method was too coarse — **DONE** | 30m | — |
| t005 | Render parity — **DONE** | 20m | t002, t004, t009 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout — **DONE** | 10m | t006, t007 |

## Definition of done

- **A free private service with a non-zero idle timeout stays running.** Repeat this milestone's probe: create one, set `idleTTLSeconds: 60`, and watch past three minutes; it must remain `Running` with one instance. Its in-cluster traffic does not pass through the public activator, so it has neither the activity signal nor the wake path required for safe auto-sleep.
- **Resume remains the idempotent inverse of explicit suspension.** It clears `spec.suspended`; it does not override replicas chosen by auto-sleep or autoscaling. The rejected t003 proposal would have violated ADR007 and could fight another controller.
- **The Settings idle-timeout control is only offered where its own sentence is true.** It currently renders on a private service reading _"Free services sleep after this idle window, then wake on the next request."_ — the wake half of which that type cannot do.
- **The `background_worker` exclusion still holds**, re-verified rather than assumed: same probe on a free worker keeps it `Running` with 1 instance well past the window (measured 4m48s this run).
- `make test` from `lego/operator/` covers, per type: web (eligible), private/worker/cron/static (ineligible), plus an envtest proving an expired free private service retains one replica and no Ingress.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 49th run, 2026-08-27, journey 10 (private service / background worker). Both fixtures deleted at end of run.

  **The failing case** — `qa-20260827-priv2` (`srv-da85ueoueu1c7395is8g`, free `private_service`, Go). Reached `Running` at `16:03:40Z`; `setIdleTimeout(idleTTLSeconds: 60)` at `16:03:40Z`:

  ```
  16:04:28Z  phase=Running      idleTTL=60  instances=1
  16:05:04Z  phase=Hibernated   idleTTL=60  instances=0      <- ~84s
  16:07:03Z  phase=Hibernated               instances=0      <- no self-recovery
  events: service_hibernated@2026-08-27T16:04:45Z
  ```

  **The control, same protocol, same plan, one type over** — `qa-20260827-worker` (`srv-da85qdvm2e9c73ft5p70`, free `background_worker`), `idleTTLSeconds: 60` set at ~`15:56Z`:

  ```
  15:56:31Z … 16:00:44Z   phase=Running  idleTTL=60  instances=1   (8 samples, 4m48s)
  ```

  Never hibernated, at 4.7× the window the private service needed. So the difference is the type, not the tier, the TTL, or the runtime.

  **Resume does not recover it:**

  ```
  mutation { resumeService(id:"srv-da85ueoueu1c7395is8g"){ id phase suspended } }
  => {"data":{"resumeService":{"phase":"Hibernated","suspended":"not_suspended"}}}
  16:08:14Z (45s later)  phase=Hibernated  instances=0
  ```

  **The only thing that recovered it** was disabling auto-sleep — which the dashboard labels "Platform default" (`w6/m116`):

  ```
  16:08:27Z  setIdleTimeout(idleTTLSeconds: 0)
  16:08:57Z  phase=Running  idleTTL=0  instances=1
  ```

  **And the dashboard offers the setting**, read from the live accessibility tree on that private service's Settings page:

  ```
  Idle timeout   Free services sleep after this idle window, then wake on the next request.
                 [ Platform default ]
  ```

- **Root cause:** `lego/operator/internal/controller/app_controller.go:1710` — `worker := app.Spec.Type == appv1alpha1.TypeBackgroundWorker` — and `desiredReplicas` (`:1983`) gates auto-hibernation on `autoHibernating = !worker && …`. The exclusion is implemented as "is this the worker type" while the reason it states is about routing. Its own doc comment (`:1981-1982`) says: _"A worker never auto-hibernates — **it has no Ingress, so a request could never wake it**."_ A `private_service` has no Ingress either — established live in `w6/063`/run 46: its platform host returns Traefik's default 404 with no certificate, and `POST …/custom-domains` is refused with _"a private_service has no ingress and cannot have custom domains"_. The wake path is the activator, and the activator is reached as an **Ingress backend** (`:2031`), so a type with no Ingress cannot be woken by a request no matter what `autoSleepEligible` says. The predicate excluded the type that motivated the comment and not the other type the comment's reasoning covers.
- **Why Resume does not help — and should not be changed:** auto-hibernation deliberately scales to zero **without** touching `spec.suspended`, while Resume clears `spec.suspended`. A no-op on an already non-suspended App is the documented idempotent result. Broadening Resume into "force replicas above zero" would conflate manual suspension with auto-sleep/autoscaling, fight level-triggered ownership, and mask real rollout failures. The invalid t003 task was deleted during triage.
- **The message the operator stamps is itself untrue for this type:** `parkKubernetes:1865` writes `"idle ≥%ds on free tier; wakes on next request"` onto the hibernated App — for a private service, there is no request that can.
- **This is not an unnoticed type, it is a mis-drawn line.** `.pm/w6/done/m43/done/t002.md:35,55,64` states as settled design that _"Only `web_service`/`private_service` have `idleTTLSeconds`-driven auto-sleep"_ and instructs "leave the free-tier web/private sleeping behavior byte-identical". So private-service auto-sleep is intended; what was never checked is whether anything can wake one. `t001` is where that gets decided, not assumed.
- **Goal linkage:** [ADR003](../../docs/ADR003-control-plane.md)'s `sleep = free` (idle → hibernate, request → wake via the activator) and [ADR007](../../docs/ADR007-restart-suspend-and-resume.md), which defines resume as manual for an explicitly suspended service and auto-hibernate/wake-on-request as the activator's job — the two halves this milestone finds disconnected for one type.
- **Expected outcome:** only public web services can auto-sleep, private services remain available in-cluster, and Resume retains its documented manual-suspension scope.
- **Why now:** it is reachable from the dashboard in two clicks on a free private service, the recovery button silently fails, and the only working escape is a dropdown value labelled "Platform default" — a user would have no reason to try it. A private service is by definition something other services depend on, so the failure is silent from the outside: the dependent service starts failing, and the private service's own page says `Hibernated`, which reads like a normal free-tier state.
- **Render parity:** included (t005). Render's free tier spins down web services and wakes them on request; Render has no private-service auto-sleep behavior to mirror. Bex therefore stores `idleTTLSeconds` for wire compatibility but applies it only to free web services. Resume behavior and wire shapes remain unchanged.
- **Blast radius, enumerated exhaustively from one switch** (`app_controller.go:1700-1710`): `cron_job` returns early to `reconcileCronJob` and `static_site` to `reconcileStaticSite`, so neither ever reaches `desiredReplicas`; `background_worker` is excluded by `!worker` (live-proven above); `web_service` is eligible **and** has an Ingress, so the activator wakes it (`w6/m94`, `w6/m116`). `private_service` is the only type that is eligible without a wake path. `desiredReplicas` has one call site (`:1716`); `autoSleepEligible` has three (`:1642` scheduling, `:2031` Ingress backend, `:2098` the `last-active` stamp) — `t002` must decide which of the three the fix belongs in, since they answer different questions.
- **Adjacent classes:** an explicitly suspended service still resumes by clearing `spec.suspended`; autoscaling and rollout readiness retain ownership of their own replica state. No lifecycle verb was broadened to mask either condition.
- **Verification boundary after triage:** (1) t009 observed zero private-service instances; Kubernetes Service routing has no serving endpoint at zero replicas. A fresh local in-cluster connection probe was attempted, but the pre-existing kind/CAPD clusters were stopped with stale API endpoints, so no new live connection result is claimed. (2) Repository tracing confirms private traffic bypasses the public activator and no other component refreshes `last-active`; m119 removes private services from both stamping and requeue. (3) The proposed generic Resume experiment was rejected: ADR007 defines Resume only as clearing explicit suspension, and changing it to override any zero-replica cause would fight auto-sleep/autoscaling ownership. The deployed free-private/free-worker probe remains t008.
