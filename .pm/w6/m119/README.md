# w6 · m119 — A free private service auto-hibernates with no wake path, and Resume silently fails to bring it back

**Worker:** worker6 **Goal:** a private service can never be driven into a state it cannot come back from, and Resume brings back anything that is scaled to zero **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Decide whether a private service should auto-sleep at all, given it has no wake path  | 30m | —          |
| t002 | Enforce the decision in the operator                                                  | 45m | t001       |
| t003 | Make Resume recover an auto-hibernated service — broken independently of t001         | 45m | —          |
| t004 | Stop the dashboard offering an idle timeout that promises a wake it cannot deliver    | 30m | t001       |
| t009 | Re-verify this milestone's own claims with tight sampling — the original method was too coarse | 30m | —          |
| t005 | Render parity                                                                         | 20m | t002, t003, t004, t009 |
| t006 | Simplify                                                                              | 20m | t005       |
| t007 | Test coverage                                                                         | 30m | t005       |
| t008 | Closeout                                                                              | 10m | t006, t007 |

## Definition of done

- **A free private service with a non-zero idle timeout is never left in a state nothing can recover.** Repeat this milestone's probe: create one, set `idleTTLSeconds: 60`, wait three minutes. Today it reaches `phase: "Hibernated"` with `instances: 0` in ~84 seconds and stays there. Whichever way `t001` decides — exclude the type, or give it a wake path — the observable end state must not be "scaled to zero forever".
- **`resumeService` brings back a service that is scaled to zero, whatever scaled it.** Today it returns `200` with `phase: "Hibernated"` and changes nothing (capture below). This bullet holds independently of `t001`: even for a `web_service`, Resume is the control a user reaches for when a service is down, and it must not report success while doing nothing.
- **The Settings idle-timeout control is only offered where its own sentence is true.** It currently renders on a private service reading _"Free services sleep after this idle window, then wake on the next request."_ — the wake half of which that type cannot do.
- **The `background_worker` exclusion still holds**, re-verified rather than assumed: same probe on a free worker keeps it `Running` with 1 instance well past the window (measured 4m48s this run).
- `make test` from `lego/operator/` covers, per type: web (sleeps and is wakeable), private (per `t001`'s decision), worker (never sleeps), cron and static (never reach the path at all) — red before the change for the type `t001` moves.

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
- **Why Resume does not help:** auto-hibernation deliberately scales to zero **without** touching `spec.suspended` (`parkKubernetes:1862-1866`, "so manual-suspend semantics are preserved"), and Resume clears `spec.suspended`. On an auto-hibernated App there is nothing for it to clear, so it is a no-op by construction — correct in isolation, and wrong as the control a user presses when their service is down. That is a separate defect from the routing one, which is why `t003` does not depend on `t001`.
- **The message the operator stamps is itself untrue for this type:** `parkKubernetes:1865` writes `"idle ≥%ds on free tier; wakes on next request"` onto the hibernated App — for a private service, there is no request that can.
- **This is not an unnoticed type, it is a mis-drawn line.** `.pm/w6/done/m43/done/t002.md:35,55,64` states as settled design that _"Only `web_service`/`private_service` have `idleTTLSeconds`-driven auto-sleep"_ and instructs "leave the free-tier web/private sleeping behavior byte-identical". So private-service auto-sleep is intended; what was never checked is whether anything can wake one. `t001` is where that gets decided, not assumed.
- **Goal linkage:** [ADR003](../../docs/ADR003-control-plane.md)'s `sleep = free` (idle → hibernate, request → wake via the activator) and [ADR007](../../docs/ADR007-restart-suspend-and-resume.md), which defines resume as manual for an explicitly suspended service and auto-hibernate/wake-on-request as the activator's job — the two halves this milestone finds disconnected for one type.
- **Expected outcome:** no service type can be put into a permanently-zero state through a control the product offers, and Resume is honest about whether it did anything.
- **Why now:** it is reachable from the dashboard in two clicks on a free private service, the recovery button silently fails, and the only working escape is a dropdown value labelled "Platform default" — a user would have no reason to try it. A private service is by definition something other services depend on, so the failure is silent from the outside: the dependent service starts failing, and the private service's own page says `Hibernated`, which reads like a normal free-tier state.
- **Render parity:** included (t005). Render's free tier spins down web services and wakes them on request; Render has no private-service auto-sleep to mirror, so bex's `private_service` sleeping is a bex extension — which makes `t001`'s decision a bex product call rather than a parity fix. Record it in [ADR018](../../docs/ADR018-render-parity.md) either way. Resume's behavior (`t003`) is Render-shaped and must stay wire-compatible.
- **Blast radius, enumerated exhaustively from one switch** (`app_controller.go:1700-1710`): `cron_job` returns early to `reconcileCronJob` and `static_site` to `reconcileStaticSite`, so neither ever reaches `desiredReplicas`; `background_worker` is excluded by `!worker` (live-proven above); `web_service` is eligible **and** has an Ingress, so the activator wakes it (`w6/m94`, `w6/m116`). `private_service` is the only type that is eligible without a wake path. `desiredReplicas` has one call site (`:1716`); `autoSleepEligible` has three (`:1642` scheduling, `:2031` Ingress backend, `:2098` the `last-active` stamp) — `t002` must decide which of the three the fix belongs in, since they answer different questions.
- **Adjacent classes:** `t003` changes what Resume does to a service that is scaled to zero. Place the neighbours: an explicitly **suspended** service (Resume must keep clearing `spec.suspended` exactly as today), a service scaled to zero by **autoscaling** at `minInstances: 0` if that is reachable, and a service mid-rollout with zero ready pods (Resume must not mask a genuine failure as a resume).
- **Unverified this run:** (1) that a hibernated private service is genuinely unreachable in-cluster — the pod count is 0 and the k8s Service has no endpoints, but this hunt has no in-cluster vantage point to attempt `qa-20260827-priv2:3000` and confirm the connection is refused rather than queued; `t001` should confirm before choosing between "exclude the type" and "give it a wake path". (2) Whether an in-cluster request could ever stamp `last-active` — `:2098` stamps it on the Running reconcile, and nothing observed re-stamps it afterwards for this type, but the negative was inferred from the code, not instrumented. (3) `resumeService`'s no-op behavior was observed only on an auto-hibernated **private** service; whether it equally fails to recover an auto-hibernated **web** service (which the activator would wake anyway) was not tested, and `t003` should check both.
