# w6 · m123 — A failed build tells the tenant nothing true: the failure reason is Kubernetes internals, the duration is 0s, and buildkit's error never reaches the log

**Worker:** worker6 **Goal:** a tenant whose build fails reads a sentence that names the cause, over a duration that matches reality, with the failing phase's own error in the log **Status:** in_progress

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Stop shipping the raw Job message; use the failure tail the build already captures           | 60m | —          | — **DONE**
| t002 | Stop stamping `started_at` from a terminal-skip transition, and carry the real duration      | 60m | —          | — **DONE**
| t003 | Find out why buildkit's output never reaches the build log, and fix it                       | 60m | —          |
| t004 | Render parity                                                                                | 25m | t001, t002, t003 |
| t005 | Simplify                                                                                     | 20m | t004       |
| t006 | Test coverage                                                                                | 40m | t004       |
| t007 | Closeout                                                                                     | 15m | t005, t006 |

## Definition of done

- **The failure reason names the cause.** Recreate the exact fixture — a free `web_service` on the **public** repo `github.com/render-examples/express-hello-world` with `dockerfilePath: ./NoSuchDockerfile` — deploy it, and read `failureReason`. It must name the missing Dockerfile and contain **none** of: `PodFailurePolicy`, `bex-build/`, `bld-tea-`, `FailJob rule`, or a bare `exit code 90`. Today it is exactly:

  ```
  build failed: PodFailurePolicy: Container buildkit for pod
  bex-build/bld-tea-d98210cbbpdc73dcrkvg-qa-20260827-badbuild-gen-2-hhtxp
  failed with exit code 90 matching FailJob rule at index 1
  ```

- **The duration is real.** That deploy's `startedAt` is strictly before its `finishedAt` by roughly the build's actual runtime. Today they differ by **one microsecond** (`19:57:15.720548Z` vs `19:57:15.720549Z`) on a build that ran **68 seconds** (`createdAt 19:56:07.854793Z`). The Events tab must stop rendering `0s` for it.
- **The log carries the failing phase's error.** The build log for that deploy contains buildkit's own message naming the Dockerfile. Today the complete log is 11 lines with **zero** buildkit output — not the error, not even buildkit's usual `#1 [internal] load build definition` progress lines.
- **The banner sorts before the output it introduces.** `==> Building from <repo>@<ref>` appears **before** the failing phase's output. Today it carries `finishedAt` and sorts last, so the log reads: clone failure, *then* "Building from".
- **`w7/m82/t003`'s classification split survives.** A tenant-caused and an infra-caused failure still produce different `failure_reason` values and different condition reasons. This milestone improves the message **without** collapsing that distinction.
- **Successful deploys are untouched.** A live deploy's timestamps are unchanged — verified against a real one, because those are correct today and are exactly what a careless change to `DeployStatusStartsExecution` would break.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **57th run**, 2026-08-27, journey 14 (deliberately break a build). Workspace `tea-d98210cbbpdc73dcrkvg`. Every probe below is re-runnable; the purpose-built service was deleted at the end of the run (`deleteService` → `true`, subsequent `GET` → **404**).

  **Evidence base — 3 failed deploys, 2 services, 2 fault phases, 2 repos, 2 causes.** Pre-existing `qa-20260826-webhook-renamed` (`srv-da7o6ovvqdcc73bpn9hg`), private repo, **clone** phase, auth failure:

  ```
  dep-da7rag7vqdcc73bpnin0  build_failed
    createdAt  2026-08-27T03:55:44.475748Z
    startedAt  2026-08-27T03:55:58.679347Z
    finishedAt 2026-08-27T03:55:58.679347Z      <- identical
  dep-da7r98nkrsvc73c3m5mg  build_failed, same shape, startedAt == finishedAt == 03:53:15.734821Z
  ```

  Purpose-built this run — `qa-20260827-badbuild` (`srv-da89ch0ueu1c7395j53g`), **public** repo (no auth in play), **buildkit** phase (not clone), different cause:

  ```
  dep-da89clvm2e9c73ft62i0  build_failed
    createdAt  2026-08-27T19:56:07.854793Z
    startedAt  2026-08-27T19:57:15.720548Z
    finishedAt 2026-08-27T19:57:15.720549Z      <- one microsecond apart
  ```

  The second sample is what proves this is neither clone-specific nor auth-specific.

- **Defect A — the tenant-facing failure reason is raw Kubernetes internals.** `lego/operator/internal/controller/app_controller.go:849`:

  ```go
  return halt(r.fail(ctx, app, view.reason, fmt.Errorf("%s: %s", view.message, obs.Message)))
  ```

  `view.message` is the curated half from `buildFaultViews` (`build_failure.go:36-51`) — "build failed" / "build failed for an infrastructure reason and was retried" / "build exceeded its time limit". `obs.Message` is `execution.JobFailureMessage(...)` (`build/build.go:563`), Kubernetes' own Job condition text. Concatenating them ships the internal build namespace (`bex-build`), the internal Job naming scheme with the workspace id embedded (`bld-tea-<workspace>-<svc>-gen-N-xxxxx`), a bare `exit code 90`, and `matching FailJob rule at index 1` — a sentence about bex's own PodFailurePolicy config that is meaningless to a tenant and discloses internal scheduling policy.

  **The house style already exists in the same file.** The **runtime** path, `app_controller.go:3635-3645`, produces: _"container exited shortly after start and is restarting repeatedly (last exit code N) — check the service logs for the crash output. If the crash is a port bind: the process must listen on $PORT (8080), and tenant containers cannot bind ports below 1024 (all Linux capabilities are dropped)."_ Tenant-oriented, actionable, no internals. That is both the control and the target.

  **The mechanism to do better is built and unused.** `captureFailureTail` (`build/build.go:1249-1259`) sets `TerminationMessagePolicy = FallbackToLogsOnError` on every build init- and app-container, commented "so a failing container's output is retrievable from the pod status". `grep -rn "TerminationMessage\|Terminated.Message\|LastTerminationState" lego/operator lego/backend --include='*.go' | grep -v _test` returns only the setters plus the unrelated CrashLoopBackOff reader at `app_controller.go:3635`. **Nothing reads the build pod's termination message** — the platform captures the exact tail that would say "failed to read dockerfile", then discards it in favour of Kubernetes boilerplate.

- **Defect B — `started_at` is stamped at failure, so the duration is 0s.** `lego/backend/internal/store/deploy_lifecycle.go:61-63`:

  ```go
  func DeployStatusStartsExecution(status string) bool {
      return status != DeployQueued && status != DeployCanceled && status != DeployDeactivated
  }
  ```

  `build_failed` is none of those, so it returns true and `TransitionDeploy` stamps `started_at = clock_timestamp()` on the `queued → build_failed` transition. When a build fails before any reconcile pass observes it mid-build, that transition **is** the first "executing" status seen, so start and finish land on the same instant. `CanTransitionDeploy`'s own doc (`deploy_lifecycle.go:70-74`) says the table "permits forward phase skips because the backend samples Kubernetes state rather than receiving every operator edge" — the skip is designed for; the timestamp is not.

  **The control is exact and came back in the same responses.** Every `live` and `deactivated` deploy has a distinct `startedAt` (e.g. created `00:31:48` → started `00:31:58` → finished `00:33:15`), on the QA service **and** on an unrelated healthy service (`srv-d9nqg9dcavls73fp8m2g`, 4 deploys, all distinct). And `canceled` — which the predicate **does** exclude — has **no** `startedAt` key at all, which is precisely what the predicate produces, confirming the mechanism rather than merely correlating with it.

  **The true duration exists and is thrown away.** `Observation.RunSeconds` is "terminal build run duration from the Job's own status timestamps" (`build/build.go:431`), computed at `:559` and `:564` via `jobRunSeconds(cur)`. Its only consumers are `recordBuildRunSeconds` (`app_controller.go:819,843`) → a Prometheus histogram (`build_metrics.go:157-164`). It never reaches the App status or the deploy row.

  Two visible symptoms: the Events tab renders `Deploy ended … 0s` for a build that ran over a minute; and `lego/backend/internal/logs/progress.go:101` stamps the synthetic `==> Building from` banner with `d.StartedAt`, so it sorts **below** the real failure output. Live, within a single deploy:

  ```
  03:53:06.760  ==> Build queued
  03:53:08.179  remote: Invalid username or token. Password authentication is not supported for Git operations.
  03:53:08.179  fatal: Authentication failed for 'https://github.com/bex-co/bex-hello-go-live.git/'
  03:53:15.734  ==> Building from https://github.com/bex-co/bex-hello-go-live.git@main   <- 7s AFTER the failure
  03:53:15.734  ==> Build failed
  ```

- **Defect C — buildkit's failure output never reaches the build log (cause unverified).** The complete log for the missing-Dockerfile failure (`GET /v1/logs?resource=…&type=build&limit=30&startTime=…&endTime=…`) is 11 lines: two `==> Build queued`, six successful git-clone lines, `==> Deploy canceled` (the superseded first deploy's banner), `==> Building from …`, `==> Build failed`. **Zero** buildkit output. The clone succeeded twice and **is** logged, so the log pipeline itself works; buildkit's specifically is absent. Combined with Defect A, a tenant with a wrong Dockerfile path has no surface that names the cause. **The cause was not traced this run — `t003` owns it.** The lead: the clone phase's stderr *did* ship, so whatever differs between the two containers is the thing to find.

- **Goal linkage:** [docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md) (deploy lifecycle) and [docs/ADR060-build-worker-reliability-and-performance.md](../../../docs/ADR060-build-worker-reliability-and-performance.md) D2, whose classification this **builds on** rather than replaces.

- **Expected outcome:** a tenant whose build fails reads a sentence naming the cause and can act on it, with a duration that matches reality and a log containing the failing phase's own error.

- **Why now:** this is the first thing a user hits when a deploy breaks, and the three defects compound — the reason is boilerplate, the log is empty, and the timeline says the build took no time. `.pm/w5/048.md` records the same class of complaint on the agent-session surface ("Users cannot remediate without logs"), so this is a recurring shape rather than a one-off.

- **Precedent — extend, do not re-litigate.** `m79` shipped the `failure_reason` **channel** (operator → deploy record → REST/GraphQL/MCP → dashboard → failure email). `w7/m82/t002` introduced the `podFailurePolicy` that produces this Kubernetes text, and `w7/m82/t003` carried the **classification** to the deploy record. **Both of their DoDs are met** — t003's acceptance was "a tenant-caused build failure and an infra-caused one produce different condition reasons and different deploy `failure_reason` values", and they do. Neither addressed the raw Job message being concatenated onto the curated one, which is Defect A. This is a gap alongside them, not a regression of them.

- **Render parity:** included (t004). `failureReason` rides `m79`'s channel across REST, GraphQL, MCP, the dashboard **and the failure email**, so a message change moves all of them together. Render's build failures name the failing step and show the builder's output, so this restores parity rather than diverging. The email is the one surface this hunt never opened.

- **Blast radius:** `r.fail` at `app_controller.go:849` is the shared terminal-failure path — grep its callers before changing the message shape, since a non-build caller may depend on the current composition. `DeployStatusStartsExecution` has consumers beyond the stamp site: `buildStartedAt` (`store/reconciler.go:803`) reads `open.StartedAt`, so changing it moves the `build_started` **event** and its webhook payload as well as the column. `w6/035` tuned exactly that timing and its reasoning at `reconciler.go:790-802` must be re-read first. Deploys that behave correctly today (`live`/`deactivated`/`canceled`) need regression tests, not just the failing one.

- **Adjacent classes:** place every fault class under the new message — `FaultTenant`, `FaultInfra` (retried), `FaultTimeout`, and `unclassifiedBuildFailure` (the kpack/unmodelled fallback at `build_failure.go:57-62`) — and say what each reads when no termination-message tail is available; a timeout in particular has no failing-container tail to quote. Also separate tenant-facing from operator-facing: the internal Job name is genuinely useful in operator logs and must keep going there, just not to the tenant.

- **Unverified this run — carried as work, not presented as observation:** Defect C's **cause** (observed, not traced — `t003` owns it); the failure **email** surface was never opened; the `FaultInfra` and `FaultTimeout` message paths were never triggered live, only read in code, so their exact tenant-facing strings are unconfirmed; and `failureReason`'s dashboard rendering was read through the Events tab and the API, not on the deploy detail page. **Separately, and not part of this milestone:** the clone-auth failure on `qa-20260826-webhook-renamed` ("Invalid username or token") is an observation whose cause was not established — `.pm/w6/040.md:106` records the same repo cloning fine on 2026-08-26, so it is either an expired credential (the ops class `w6/m105` was dropped under) or something in open milestone `w6/m97`'s clone-secret territory. It was this run's **fixture**, not its finding; a fix here must not assume it.

## Implementation update 2026-08-31

t001 + t002 are implemented, tested, and green locally (`make test`, backend `go test ./...` against real Postgres, `make lint` — the only backend failure is the pre-existing repeat-run flake in `TestPGGitWebhookReplayClaims`, which uses fixed digests with no cleanup and passes on a fresh database). t003's cause is established and its fix is in the tree, pending live verification.

- **Defect A (t001 — done):** `buildFailureMessage` (`lego/operator/internal/controller/build_failure.go`) composes the tenant sentence per fault class; the raw Job text goes to the operator log only (`app_controller.go` failed branch keeps Job identity + Kubernetes verdict there). The unread capture is finally read: `failureDetail` (`build/build.go`) pulls the failing container's termination message (bounded by `failureTail`, 800 bytes, end-biased) and its phase's tenant-facing name off the pod status. kpack failures keep their builder text via the same `Tail` channel. Pinned by `TestBuildFailureMessageNeverLeaksJobInternals` (every class × tail presence × the five internal markers) and `TestBuildFailureClassesStayDistinct` (w7/m82/t003's split survives).
- **Defect B (t002 — done):** `started_at` now stamps from the clock only on dispatch-observing transitions (`DeployStatusStampsDispatch`); a `build_failed` skip applies the operator's recorded window — new CRD field `status.buildRun` (generation-attributed, written in the same status update as the Build condition) — or stays honestly NULL. Bonus: `Observation.RunSeconds` was 0 for **every** failed Job (Kubernetes sets `completionTime` only on success), so the Prometheus histogram was wrong too; `jobRunWindow` + `execution.JobFailedAt` fix both. The `build_started`/`build_ended` facts ride the same evidence (`buildStartedAt(open, newStatus, observedStart)`) — w6/035's tests still pass; live/deactivated/canceled are pinned by `TestTransitionDeployFailureSkipStartedAt` (+ its PG variant, which also proves a real in-progress stamp is never overwritten) and `TestDeployStatusStampingSplit`.
- **Defect C (t003 — cause established, fix in tree, live verification pending):** two legs. **Stored log:** `loki.source.kubernetes` opens one kubelet follow stream per discovered container; a sequential init phase that starts minutes in (buildkit after clone/cache-restore) answers "waiting to start", and the retry backoff can outlive a fast-failing phase's entire run — a terminal pod is then not tailed at all. Clone is running at discovery, so it ships; that is exactly the observed split, and consistent with buildkit lines shipping on multi-minute builds in `docs/render-artifacts/live-deploy-following.md` (2026-07-18) but never on the seconds-lived fixture. Fix: the `build_pods` pipeline now tails `/var/log/pods` files (`local.file_match` + `loki.source.file` + `stage.cri`; `alloy.mounts.varlog: true`) — files exist from container start, survive until pod deletion (Job TTL 1h), and `stage.cri` keeps original line timestamps (`deploy/gitops/base/log-shipper.yaml`). **Live follow:** `followBuildLogs` only streamed containers running at subscribe time, so a subscriber attached during clone never saw buildkit; it now watches the pod and attaches each phase as it starts (`lego/backend/internal/logs/service.go`).
- **Banner ordering:** rides t002 — `==> Building from` is stamped from the row's now-real `started_at`, sorting before the failing phase's output; with no evidence there is no banner rather than a mis-sorted one.
- **Dashboard:** no change needed — `formatDeployDuration` returns null for an absent `startedAt` (renders an em-dash), and `0s` came only from the collapsed timestamps.

**Remains for closeout:** deploy operator + backend + shipper config, then t003's live acceptance (the fixture's log carries buildkit's error; a successful build shows progress output), t004's parity probes (REST/GraphQL/MCP, deploy detail page, and the **email** — never opened by the hunt), t005/t006 residue if any, and t007's live DoD re-run + board close.
