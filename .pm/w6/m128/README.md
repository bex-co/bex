# w6 · m128 — A deploy canceled mid-build leaves an unclosed build in the activity feed: `build_started` with no `build_ended`

**Worker:** worker6 **Goal:** every build that starts also ends in the feed, whatever terminal state its deploy reaches **Status:** in progress — t001 and t005 are done and deployed in `daf84f6e`; t002 outbound verification, t003 parity, t004 simplify and t006 live closeout remain

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Emit the build lifecycle close on the cancel path — **DONE**                    | 45m | —          |
| t002 | Confirm the outbound consumers see a closed pair                               | 35m | t001       |
| t003 | Render parity                                                                   | 25m | t001, t002 |
| t004 | Simplify                                                                        | 20m | t003       |
| t005 | Test coverage — **DONE**                                                         | 40m | t003       |
| t006 | Closeout                                                                        | 15m | t004, t005 |

## Implementation update (2026-08-28)

`daf84f6e` added `store.CanceledBuildLifecycleFacts`, reused it from `Service.Cancel`, and preserved fact idempotency through the existing source keys. The regression matrix covers mid-build cancel, queued cancel, post-build cancel, supersede cancel, image-backed deploys and duplicate insertion; the existing `TestBuildEndedStatus` continues to pin successful and build-failed outcomes. `cd lego/backend && go test ./...` is green. The commit is contained in the production image pinned by `71fe9660`.

t002 and t003 remain open because no live webhook/push subscriber, cross-surface probe, dashboard Events-tab check or Render comparison was performed. t004 also remains open because the landed commit does not record a `/simplify` pass. t006 owns the credential-dependent live closeout.

## Definition of done

- **The canceled build closes.** Create a repo-backed service, let one deploy reach `live`, trigger a second, and cancel it while `build_in_progress` (read the deploy first and confirm that status, as this run did). `GET /v1/services/{id}/events` then shows **both** `build_started` and `build_ended` for the canceled deploy, the ended fact carrying a canceled outcome. Today that deploy has exactly three events — `deploy_started`, `build_started`, `deploy_ended(canceled)` — while the successful deploy in the same service has four.
- **A deploy canceled while still QUEUED emits neither fact** — the current correct behaviour, which the fix must not break.
- **A deploy canceled AFTER its build finished** reports `build_ended` with a **succeeded** outcome, per `buildEndedStatus`'s second branch.
- **The successful and build-failed paths still emit exactly one `build_ended` each** — correct today, and precisely what a careless change to the shared fact helper would break.
- **Cancel's other surfaces are unchanged:** 200 with `status: canceled`, detail/list/`deploy_ended(canceled)` in agreement, and a real non-zero duration (measured this run: `23:55:15.693` → `23:55:32.427`).
- `cd lego/backend && go test ./...` covers the canceled-mid-build case explicitly.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **65th run**, 2026-08-27, journey 3 (deploys). Workspace `tea-d98210cbbpdc73dcrkvg`. Every probe is re-runnable; fixture `qa-20260827-dep` (`srv-da8cre0ueu1c7395jg50`) was deleted at the end of the run (`deleteService` → true, `GET` → 404).

  Created the service from `github.com/render-examples/express-hello-world` (runtime `node`), let its first deploy reach `live`, then triggered a second and cancelled it while `build_in_progress` — confirmed by reading the deploy immediately before cancelling (`beforeStatus: "build_in_progress"`).

  `GET /v1/services/{id}/events?limit=50&startTime=<now-30m>` — the **complete** feed, 7 events:

  ```
  deploy_ended(canceled)   dep-…95jgd0  @2026-08-27T23:55:32Z
  build_started            dep-…95jgd0  @2026-08-27T23:55:06Z
  deploy_started           dep-…95jgd0  @2026-08-27T23:55:06Z
  deploy_ended(succeeded)  dep-…95jg5g  @2026-08-27T23:54:32Z
  build_ended              dep-…95jg5g  @2026-08-27T23:54:32Z   <- the SUCCESSFUL deploy closes its build
  build_started            dep-…95jg5g  @2026-08-27T23:53:45Z
  deploy_started           dep-…95jg5g  @2026-08-27T23:52:24Z
  ```

  The canceled deploy has three events and **no** `build_ended`; the successful one, same service, minutes earlier, has both halves. Re-probed after a further 15s with a 30-minute window and `limit=50` to rule out truncation or a late-arriving fact — still 7 events, still no `build_ended`.

  **Cancel is otherwise correct and must not be disturbed:** it returned 200 with `status: canceled`, and detail, list and `deploy_ended(canceled)` all agree. Its duration is honest — `startedAt 23:55:15.693`, `finishedAt 23:55:32.427`, a real 17 seconds. That incidentally **corroborates `w6/m123`**: the timestamp collapse there happens only on a terminal **skip** with no intermediate observation, and this deploy was observed in `build_in_progress` first.

- **Root cause — the cancel verb closes the row directly, and the fact path only ever sees OPEN deploys.** `lego/backend/internal/deploys/service.go:706`, the last step of `Service.Cancel`:

  ```go
  won, err := s.Store.CloseDeploy(ctx, deployID, store.DeployCanceled, "")
  ```

  That writes the terminal status straight to the deploy row and emits no build lifecycle fact of its own. The facts come from `lego/backend/internal/store/reconciler.go:630-665` (`recordDeploy`), which derives `status := observedDeployStatus(open, cur, …)` from the **App CR** and calls `recordLifecycleFacts` (`:653`) or `buildLifecycleFacts` (`:659`). By the time the reconciler next runs, the row is already terminal (`FinishedAt` set), so it is no longer an open deploy and `recordDeploy` never processes it.

- **The intent is documented and implemented — just unreachable from this path.** `reconciler.go:751-759`, the doc on `buildLifecycleFacts`:

  > "…build_ended's outcome is read from where the deploy has reached — a build phase means still building (no ended fact yet), any later phase means the build succeeded, build_failed means it failed, and **a cancel while still building means it was canceled**."

  and `buildEndedStatus` (`reconciler.go:832-851`) implements exactly that:

  ```go
  case DeployCanceled:
      if isBuildPhase(current) {
          return EventStatusCanceled
      }
      return EventStatusSucceeded // build had finished before the cancel landed
  ```

  with `isBuildPhase` = `created | queued | build_in_progress` (`:853-854`). The observed deploy was `build_in_progress`, so this branch **would** return `EventStatusCanceled` — if anything called it for this deploy. The only route that does is the supersede path at `reconciler.go:654-659`, whose own comment reads "A lower-generation row can be settled canceled after the reconciler missed its final observation". An explicit user cancel never reaches it. **So the target behaviour needs no designing** — emit the fact the code already knows how to compute, on the path that skips it.

- **Blast radius — counted.** `grep -rn "CloseDeploy(" internal/ --include='*.go' | grep -v _test` returns 4 lines, of which exactly **one** is a caller:

  ```
  internal/deploys/service.go:706   <- the Cancel verb (the only caller)
  internal/deploys/service.go:65    interface declaration
  internal/store/store.go:540       interface declaration
  internal/store/store.go:2061      PGStore implementation
  ```

  The fix is contained to a single call site; nothing else closes a deploy row outside the reconciler. The shared helpers `buildLifecycleFacts`/`buildEndedStatus` have 2 call sites between them, both in the reconciler (`:653` via `recordLifecycleFacts`, `:659` direct) — correct today, and needing regression coverage rather than change.

- **Precedent — extend, do not re-litigate.** `w7/m66` created these events, and its own task title names the boundary: t001 is "`build_started` / `build_ended` events from the BuildKit Job lifecycle **(reconciler-emitted facts)**". Reconciler-emitted is precisely what the cancel verb is not. m66 delivered what it scoped; this milestone extends the guarantee to the one lifecycle transition that does not pass through the reconciler. **Not** an m66 regression.

- **Goal linkage:** [docs/ADR004-app-deployment.md](../../../docs/ADR004-app-deployment.md) and [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md). The events package doc calls the feed a "TRUTHFUL 1:1 record of what happened to a service"; a build that starts and never ends is not that.

- **Expected outcome:** every build that starts also ends in the feed, whatever terminal state its deploy reaches.

- **Why now:** a small, contained fix (one call site) to a durable record that outbound integrations consume. It also compounds the deploy-observability work already queued — `w6/m123` (failure reason, 0s duration) and `w6/m124` (service phase) are the same surface, so a reviewer touching the deploy lifecycle will already be in this code.

- **Render parity:** included (t003). The vocabulary is Render's (`build_started`/`build_ended` are Render type names) and the feed is exposed on REST, GraphQL and MCP, so the change is visible on all three plus the dashboard. **Confirm Render closes the build on a canceled deploy** — if it does not, this becomes a deliberate bex divergence and belongs in `ADR018` rather than being silently added.

- **Adjacent classes:** canceled while **queued** (no build dispatched — emit neither); canceled **after** the build finished (`buildEndedStatus` returns succeeded); canceled by **supersede** rather than by the user (already handled at `reconciler.go:654-659`); **build_failed** (unaffected); and an **image-backed** deploy, which has no build phase and must keep emitting neither fact.

- **Unverified this run — carried as work, not presented as observation:** whether an outbound **webhook** subscriber actually receives these facts (none was subscribed; the path is established from the `factEvents` map in `webhooks/service.go`, not observed here — and note push is selective, `push_worker.go:661-667`); whether the **supersede** path genuinely emits `build_ended(canceled)` today (read from `reconciler.go:654-659`, never triggered); whether a deploy canceled while **queued** behaves as described (only `build_in_progress` was exercised); and how the dashboard **Events tab** renders the unclosed pair — the API feed was read directly, the tab was not opened.

- **Also verified this run and deliberately not filed:** rollback works end to end (`POST /v1/services/{id}/rollback` → 201, new deploy `dep-da8cu5gueu1c7395jgm0`, `trigger: "rollback"`, reached `live` with a real 16-second duration), and cancel's own terminal state is consistent across detail, list and events.
