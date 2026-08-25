# w6 · m95 — A fresh service's first deploy can hang at `build_in_progress` forever with no backing build job

**Worker:** worker6 **Goal:** a deploy row's `build_in_progress` status always implies a real, currently-scheduled BuildKit build job/pod backs it — never a permanently frozen row with nothing running underneath it, discoverable only by a human clicking Cancel. **Status:** todo

## Tasks (in order)

| id   | title                                                                                            | est | depends_on           |
| ---- | -------------------------------------------------------------------------------------------------- | --- | --------------------- |
| t001 | Root-cause, with cluster/operator-log access, why the App CR stays `PhaseBuilding` with no backing build pod | 90m | —                     |
| t002 | Fix the confirmed mechanism so a `build_in_progress` row with no backing build reaches a terminal state within the gate timeout | 90m | t001                  |
| t003 | Close (or explicitly rule out) the `supersededDeployStatus` orphan-timeout gap guarded on `ReleaseGeneration > 0` | 45m | t001                  |
| t004 | Add `recover()` around the reconciler's `ReconcileOnce` pass in `core.PollWake`/`pollLoop` so one bad row can't silently halt the gate-timeout sweep for every app | 30m | t001                  |
| t005 | Render parity across REST/GraphQL/MCP/UI                                                          | 30m | t002, t003, t004      |
| t006 | Simplify the touched code                                                                          | 30m | t005                  |
| t007 | Test coverage for the fixed behavior                                                               | 45m | t005                  |
| t008 | Closeout                                                                                            | 10m | t007                  |

## Definition of done

- Creating a fresh web service (Docker runtime, a subdirectory `rootDir`/`dockerContext`, either GitHub-App or Public-Git-URL source) and letting its first deploy sit for at least `BuildGateTimeout` (35m, `lego/backend/internal/store/reconciler.go:84`) past its `startedAt` either reaches a terminal status (`live`/`build_failed`) or, if genuinely still building, `GET /v1/logs/subscribe?resource=<id>&type=build` returns real, currently-arriving log lines — never a sustained immediate `{"error":"no running build is available to follow"}` past the gate timeout.
- `GET /v1/services/{id}/deploys` for such a row shows `updatedAt` eventually advancing past `startedAt` within `BuildGateTimeout`, instead of staying byte-identical indefinitely.
- t001 explicitly names, with cluster/operator-log evidence (not just the dashboard/REST vantage point this hunt was limited to), which candidate mechanism below is the real cause, before t002 writes a fix to the wrong layer.
- The already-present, narrower orphan/timeout backstop in `reconciler.go`'s `supersededDeployStatus` (only fires when `app.Status.ReleaseGeneration > 0`, current lines 937/965/985/992) is explicitly re-examined by t003: either confirmed unrelated to this symptom (this hunt's 3 repros all had matching generations, so the normal `PhaseBuilding` switch branch fired, not the orphan branch) and hardened as its own gap, or confirmed the same mechanism and closed.
- A regression test reproduces a `build_in_progress` row whose backing Job/Pod is gone (deleted out-of-band, or never scheduled) and asserts the row reaches a terminal state within the gate timeout — red pre-fix, green post-fix.
- Render parity: compare against Render's own behavior (a build that cannot be scheduled surfaces as a failed/errored deploy, never an infinite spinner); record in `docs/ADR018-render-parity.md` if this changes wire semantics.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co` hosting surfaces, 2026-08-25. Workspace "bex", project `qa-20260825-project` (deleted, cleanup verified against the pre-hunt Overview baseline).

  **Reproduced 3/3** on independent fresh web services (Free plan, Docker runtime, `examples/hello-go` as `rootDir`/`dockerContext` against the public `bex-co/bex` repo — confirmed public via `curl -sSI https://github.com/bex-co/bex` → `200` and `api.github.com/repos/bex-co/bex` → `"private": false`), across **both** source methods (ruling out Public-Git-URL vs GitHub-App as the variable):

  1. `srv-da6un2u7kmsc739k8ju0` / `dep-da6un2u7kmsc739k8jug` (Public Git URL) — stuck `build_in_progress` from `startedAt` `2026-08-25T19:23:05.933587Z` through at least 19:27:07 (4 minutes, when manually Canceled).
  2. `srv-da6upp67kmsc739k8k80` / `dep-da6upp67kmsc739k8k8g` (Public Git URL) — stuck from `19:28:49.999680Z`, `updatedAt` byte-identical to `startedAt` re-checked at +20s and +95s, deleted while still stuck.
  3. `srv-da6ur3u7kmsc739k8ke0` / `dep-da6ur3u7kmsc739k8keg` (GitHub App source) — stuck from `19:31:35.910708Z`, confirmed frozen at +45s, deleted while still stuck.

  **Durable evidence — the probe itself**, run from inside the authenticated dashboard page (`fetch(..., {credentials:'include'})`), identical on all 3:

  ```
  GET https://api.bex.co/v1/logs/subscribe?resource=<srv-id>&type=build
  200 OK, content-type: application/x-ndjson

  {"message":"==> Build queued", ...}
  {"message":"==> Building from https://github.com/bex-co/bex.git@f0d2a2e", ...}
  {"error":"no running build is available to follow"}
  <connection closes, ~7-20ms total>
  ```

  That string is `ErrBuildNotRunning` (`lego/backend/internal/logs/service.go:70`), returned by `awaitBuildPod` (`service.go:944-994`) when `s.BuildPods(ctx, q.App, s.BuildNamespace)` lists **zero** Pending-or-Running pod for the App — not a scheduling delay (a Pending pod is deliberately not refused, per the comment at `service.go:933-943`).

  Raw REST confirms the deploy row agrees and never moves:

  ```
  GET https://api.bex.co/v1/services/<srv-id>/deploys?limit=5
  {"deploy":{"status":"build_in_progress","startedAt":"2026-08-25T19:28:49.99968Z","updatedAt":"2026-08-25T19:28:49.999682Z", ...}}
  ```

  (no `finishedAt`; `updatedAt` == `startedAt` to the microsecond, unchanged across polls minutes apart.)

  **Control case, checked as hard as the failing case:** at the same time, the real production service `eden-dash-v3` (`srv-d9ndt8hmcglc739fkp50`, unrelated to this hunt, the workspace's own dogfooding team's service) had a deploy genuinely mid-build. The identical probe against it returned real, live-arriving BuildKit push log lines (`Copying blob sha256:...`) with a real build-pod instance id (`bld-tea-d98210cbbpdc73dcrkvg-block-eden-mono-gen-29-gjcxz`) — proving the build pipeline is not universally broken, and that the "no running build" response for the 3 QA services reflects a genuinely absent build pod, not a probe artifact. That deploy went on to reach `Live` normally.

  **Code trace on current `main`** (`lego/backend/internal/store/reconciler.go:867-922`, `observedDeployStatus`): with matching release generations (confirmed — none of the 3 repros hit the orphan/supersede branch at `:924-969`), the deploy row's `build_in_progress` is a faithful projection of the App CR's own `status.Phase == PhaseBuilding` with a current-generation Ready condition. So the App CR itself believes it is building, with nothing contradicting that from this hunt's dashboard/REST vantage point — meaning either the BuildKit Job was created and then lost/reaped before ever producing a pod the log-follow could see, or the App controller's reconcile loop stopped re-evaluating this specific App after the initial `PhaseBuilding` transition. **This hunt could not distinguish between these without cluster/operator-log access — that is t001.**

  **Candidate mechanisms, in priority order (t001 must name which is confirmed before t002 fixes it):**

  1. `lego/operator/internal/build/build.go`'s `EnsureBuild`/`BuildJob` (`:535-573`, `:708-988`) dispatches via `execution.EnsureOwnedJob` (`lego/operator/internal/execution/job.go:53-78`), described as self-healing (Get→NotFound→re-Create) — confirm whether that self-heal actually fires on the next reconcile pass for an App whose Job vanished, or whether something (owner-ref mismatch, namespace mismatch, the `BuildPods` query filter) prevents it.
  2. Whether the App controller's reconcile loop is being re-triggered **at all** for these specific Apps after the initial `PhaseBuilding` write (watch/requeue not firing, vs. firing but making a no-op decision).
  3. The already-present, narrower gap in `supersededDeployStatus` (`:965-968`, guarded on `app.Status.ReleaseGeneration > 0`) fixed by `2da39122` and `2f9db5d4` for a **different** trigger (App CR delete+recreate; push-superseding-redeploy) — confirm whether this is the same class of gap here or a red herring (generations matched in this hunt's repro, so this exact guarded branch was not the one observed firing).
  4. Whether the 35-minute `BuildGateTimeout` watchdog (`deployTimedOut`/`timedOutDeployStatus`, `reconciler.go:842-860`/`:971-982`) actually fires for this scenario once elapsed — this hunt's repros were only observed for 20s-4min, well short of 35m, so the watchdog's effectiveness here is **unverified**, not confirmed broken. Confirming or refuting this changes the severity from "infinite hang" to "up to 35 minutes with a misleading log stream," and must happen before anything else.

  **Related work — lineage, not duplication:**

  - `2da39122` (`fix(control-plane): close deploy rows orphaned by App CR recreation`) and `2f9db5d4` (`fix(backend): stop stranding deploy rows when a push supersedes an open deploy`) fixed two prior instances of "a `build_in_progress` row loses its match to live App-CR evidence and never resolves" — both already on `main`, and (dated Jul 19/30, well before the Aug 24 production deploy freeze) already deployed. This is a plausible third instance of the same recurring gap class, not a duplicate.
  - `w6/done/m46` fixed a related-but-distinct symptom on the same "first-ever deploy" lifecycle moment (a spurious generation bump from a post-create spec write falsely canceling a first deploy) — different mechanism, different symptom (false-`Canceled`, not stuck-forever); do not conflate.
  - While in this area, t004 also closes a contributing-risk hypothesis (not confirmed as this incident's cause): the reconciler goroutine (`lego/backend/cmd/api/main.go:1252`, `core.PollWake`/`pollLoop` at `lego/backend/internal/core/poll.go:73-84`) has no `recover()` around `ReconcileOnce` — a panic on one malformed row would crash `bex-api` and silently stop the gate-timeout check for every app, not just this one.

  **Deliberately not filed here (deploy-lag, already covered):** canceling the first stuck deploy caused `GET /v1/services/<id>` to read `"phase":"Failed"` instead of `"Canceled"` — this is exactly `w6/done/m52` (`lego/operator/internal/controller/app_controller.go:552-573`, `settleCanceledRelease`), already fixed on `main` but **not yet deployed to production** per the open `w6/040.md` (production has not deployed successfully since 2026-08-24, blocked on a broken self-hosted CI runner building the opensandbox-controller image). This hunt's live repro independently reconfirms that diagnosis; not re-filed, not re-opened.

- **Goal linkage:** ADR004 (app deployment state machine) and ADR060 (build-worker reliability) — a deploy's status must always correspond to real backing infrastructure.
- **Expected outcome:** no customer's first deploy can silently hang forever with a misleading "Building" state and no recourse but a manual Cancel.
- **Why now:** blocker severity — this blocks the single most basic hosting journey (create a service, watch it deploy) for any customer who happens to hit it, reproduced 3/3 this run, and is a plausible live recurrence of a bug class already fixed twice before (`2da39122`, `2f9db5d4`).
- **Render parity:** included — this is a fix to `lego/backend/internal/store/reconciler.go`, read identically by REST, GraphQL, and MCP, plus the dashboard's deploy-status rendering.
