# w2 · m99 — A queued deploy says why it waits

**Worker:** worker2 **Goal:** while a deploy row is `queued`, every log surface states the operator's current reason for the wait, so a capacity wait is distinguishable from a stuck deploy. The operator already writes the reason to the App's `Ready` condition; today bex-api drops it until the row times out. **Status:** t001–t006 done (code, copy, cross-surface proof, parity, simplify, tests); **t007 blocked on live production verification** — this run had no working production credential (the `.env` QA password and the CLI token are both stale), so no Definition-of-done bullet could be re-probed live.

## Tasks (in order)

| id   | title                                                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | While a deploy row is `queued`, the progress follower narrates the App's `Ready` condition reason and re-emits on change — **DONE**| 35m | —          |
| t002 | Tenant-neutral copy for the cluster cap and scheduler wait with no counts; "preparing registry credentials" for `RegistryCredsPending` — **DONE**| 20m | t001       |
| t003 | Confirm REST `GET /v1/logs`, GraphQL `logs`, MCP and the SSE tail carry the identical line; `failureReason` stays terminal-only — **DONE**| 20m | t001       |
| t004 | Render parity — **DONE**| 15m | t002, t003 |
| t005 | Simplify — **DONE**| 15m | t004       |
| t006 | Test coverage — **DONE**| 30m | t004       |
| t007 | Closeout                                                                                                                               | 10m | t006       |

## Definition of done

Run each bullet on production, workspace `bex` / `tea-d98210cbbpdc73dcrkvg`, with throwaway fixtures created and deleted inside the run. `BEX_MAX_CONCURRENT_BUILDS` is `"2"` (`lego/operator/config/manager/manager.yaml:178-179`).

- **The workspace-cap wait is narrated.** With two builds already `build_in_progress` in the workspace, create a third service. Within one poll of its deploy row entering `queued`, the build log (REST `GET /v1/logs?type=build`, GraphQL `logs`, MCP log read, and the SSE tail) carries a line of the shape `==> Waiting for a build slot: this workspace has 2/2 builds running`, and the deploy page `/services/<id>/deploys/<dep-id>` shows that line under **Queued**. At filing time the whole 8¾-minute wait showed only `==> Build queued`.
- **A reason change emits a new line.** When the operator's `Ready` message changes while the row is still `queued` (for example one slot frees and the message goes from `2/2` to another reason, or the wait moves from `BuildQueued` to `RegistryCredsPending`), a new narration line appears; the same reason is not repeated every tick.
- **The cluster cap leaks nothing.** A wait whose operator message names the `cluster` noun (or the scheduler wait, `waiting for build capacity: …`) is narrated as "waiting for platform build capacity" with no `N/N` counts and no other tenant's activity. Verified by the unit test in t006 and, if a cluster-cap wait can be induced, live.
- **`failureReason` stays terminal-only.** Throughout the wait, `deploys { failureReason }` on GraphQL and `failureReason` on REST `GET /v1/services/{id}/deploys/{depId}` remain empty. The `==> Build queued` line at `CreatedAt` and the `==> Building from …` line at `StartedAt` are unchanged.

## Source + Goal linkage

- **Source:** `w1/087` (absorbed, now `.pm/w1/done/087.md`), from the live `/qa-find-bugs` hunt 2026-09-14 pass 5, journeys 2/3; proposed by `/pm-brainstorm for w1` 2026-09-15 (proposal #6) and materialized into w2 by user direction.
- **Goal linkage:** pillar 2 (agent-readable state). The operator writes `phase Building`, reason `BuildQueued`, message `"<noun> has <active>/<limit> concurrent builds active; waiting for a slot"` (`lego/operator/internal/controller/app_controller.go:796-797`), and `observedDeployStatus` keeps only the status (`lego/backend/internal/store/reconciler.go:868-885`); the message surfaces only through `failureReasonFor` after a timeout (`:1320-1323`). The narration has one line for the whole wait (`lego/backend/internal/logs/progress.go:84-90`).
- **Expected outcome:** a user (or agent) reading any log surface during a queued deploy sees why it waits and can tell a capacity wait from a stuck deploy, without `failureReason` being repurposed for a live wait.
- **Why now:** `w6/done/m95` made `queued` vs `build_in_progress` honest and closes a timed-out queued row with the operator's reason, but never surfaced the reason during the wait; the whole live wait is still silent. The change sits in the one core verb the progress follower already runs each tick, so all four log surfaces move together. Promoted to a milestone rather than worked as a note because carrying the message may need a stored column (t001 decides and records).
- **Render parity:** included. The build-log narration is served on REST `GET /v1/logs`, GraphQL `logs`, MCP, and the SSE tail, so t004 confirms the four surfaces carry the identical line, and Render's own copy for a queued build (concurrency limits by plan) must be captured and compared rather than assumed.

## Implementation record (t001–t006, 2026-09-15)

### Carrier decision (t001): live `Ready` read, no stored column, no migration

The wait reason is read from the App CR's `Ready` condition, not carried on the `deploys` row.

- The logs service already holds the App CR at both narration call sites — `QueryLogs` fetches it through `AuthorizeApp` before `synthesizeProgress`, and `FollowLogs` does the same before `followBuildLogs`. So the read path costs **zero** extra work: the condition comes off an object already in hand.
- The live tail re-reads the App through `s.Client` (`progressFollower.refreshWait`) **only while the newest row is `queued`** — the one window in which the reason can change and matter. A normal build tail pays nothing; a queued tail pays one cached `Get` per existing wait-loop tick, alongside the pod list it already does.
- A stored column would have been strictly worse: a migration, a second writer in the reconciler, and a value that is stale between projector passes — for a fact that is only ever interesting *right now*. The condition is the operator's own live statement of the wait; copying it into Postgres would add latency and a drift class for no reader.
- Currency rule matches the deploy projector's (`store.failureReasonFor`): only a `Ready` condition whose `observedGeneration == app.Generation` is narrated, so a reason left over from a superseded spec is never spoken.
- Timestamp: the wait line is stamped at the row's `CreatedAt` (the moment the wait began), which keeps line ids deterministic across reads. That is what makes re-emission correct for free — the follower's existing `logID` dedupe means an unchanged reason collapses to the same id (emitted once, never per poll) while a changed reason is a new id (emitted exactly once more). No new dedupe state was added.

### Copy (t002)

| operator `Ready` reason + message                                   | tenant-visible line                                                  |
| ------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `BuildQueued` + `workspace has <a>/<l> concurrent builds active; …` | `==> Waiting for a build slot: this workspace has <a>/<l> builds running` |
| `BuildQueued` + `cluster has <a>/<l> …`                             | `==> Waiting for platform build capacity`                            |
| `BuildQueued` + `waiting for build capacity: <scheduler message>`   | `==> Waiting for platform build capacity`                            |
| `BuildQueued` + anything else (incl. empty, or a future operator's) | `==> Waiting for platform build capacity`                            |
| `RegistryCredsPending` + any message                                | `==> Preparing registry credentials`                                 |

The mapping is an **allow-list, not a deny-list**: `workspaceCapCounts` requires an exact prefix *and* suffix match and then requires the middle to be two digit runs around one slash. So the only operator-authored bytes that can reach a tenant are that workspace's own two numbers. The cluster cap (which counts other tenants' builds) and the scheduler wait (which names nodes, taints and quotas) both fall through to a fixed constant with no digits in it. Because the neutral line is a constant, a count change under it produces the *same* rendered line and therefore no re-emission.

### Surfaces (t003) — no adapter changed

Traced and confirmed: REST `GET /v1/logs` (`logs/rest.go`), GraphQL `logs` (`logs/graphql.go`) and MCP `list_logs` (`logs/mcp.go`) all call `Service.QueryLogs` and render the same `LogEntry.Message` through `render.go`; none filters, rewrites or truncates a narration line. The SSE tail (`GET /v1/logs/subscribe`) calls `Service.FollowLogs` → `followBuildLogs` → `progressFollower.emitReached`, which renders through the **same** `progressLines` verb. Zero adapter edits were needed. `TestQueuedWaitLineIsIdenticalOnEveryLogSurface` drives REST, GraphQL and MCP in one test and compares the line across them rather than asserting each in isolation.

`failureReason` was not touched. It is populated only by `store.deployCloseFailureReason`, which is reached only on a status close; `TestQueuedWaitNeverPopulatesFailureReason` pins that a live `BuildQueued`/`RegistryCredsPending` wait yields `("", "")` while still asserting the control — a *timed-out* queued row does close with the operator's reason. `TestQueuedWaitNarrationIsNotSourcedFromFailureReason` pins the other direction: the narration is never rendered from `DeployProgress.FailureReason`.

### Render comparison (t004), checked 2026-09-15

- [render.com/docs/build-pipeline](https://render.com/docs/build-pipeline) documents only "Each Render service can have only one active build at a time. Whenever a new build is initiated, Render cancels any in-progress build for the same service." — a cancellation rule, not a queue — and prices the pipeline in **minutes** per workspace plan (Hobby 500 / Pro 1,000 / Scale 5,000), with Starter/Performance tiers describing compute size, not concurrency.
- [render.com/docs/deploys](https://render.com/docs/deploys) says nothing about queued builds, concurrency caps, or what the deploy page shows while a build waits.
- The only documented Render queueing is **Workflows**, a separate product, whose per-workflow compute limits queue excess runs.
- **Verdict: bex is ahead, with no Render copy to mirror.** Recorded as a one-line entry in `docs/ADR018-render-parity.md` § bex ahead of Render. No other ADR018 row was touched.
- Not captured: Render's *dashboard* copy on a build actually queued behind a concurrency limit — that needs a live Render account with two concurrent builds, which this run has no credential for.

### Simplify (t005)

- The reason→line table was folded into the existing narration helpers (`progressLines` / `waitLine`), not a parallel path.
- `progressLines`, `synthesizeProgress`, `followBuildLogs` and `newProgressFollower` each lost three positional strings (`repo, branch, serviceType`) in favor of one `progressContext` value, which also carries the wait and the object key — so the read path and the tail path cannot drift.
- No new dedupe state: re-emission rides the follower's existing `logID` map.
- `make lint` (all four modules + whole-program dead-code) and the full backend suite are green.

### Known limitation (not a regression)

`awaitBuildPod` still answers `ErrBuildNotRunning` when no build pod is pending or running, which is the state of a wait gated **before** the build Job is created (the workspace/cluster cap and `RegistryCredsPending`). So an SSE subscriber during such a wait receives the queued + wait lines at subscribe and then the terminal "no running build" event, rather than a long-lived narrating tail. The REST/GraphQL/MCP reads narrate the wait for its whole duration. Teaching the tail to park while the row is `queued` is a behavior change to `ErrBuildNotRunning` semantics and was deliberately left out of m99 — file it as a follow-up note if the deploy page wants a persistent stream.

### What could NOT be verified live, and why

**No working production credential this run** — the `.env` QA password and the CLI token are both stale — so none of the four Definition-of-done bullets was re-probed against production, and no throwaway fixtures were created or deleted. All four are currently backed by unit/adapter tests only:

| DoD bullet                         | covering test                                                                 |
| ---------------------------------- | ----------------------------------------------------------------------------- |
| workspace-cap wait is narrated     | `TestQueuedDeployNarratesTheWorkspaceBuildSlotWait`, `TestQueuedWaitLineIsIdenticalOnEveryLogSurface` |
| a reason change emits a new line   | `TestBuildTailReEmitsOnlyWhenTheWaitReasonChanges`                            |
| the cluster cap leaks nothing      | `TestQueuedWaitCopyNeverLeaksAnotherTenantsActivity`                          |
| `failureReason` stays terminal-only | `TestQueuedWaitNeverPopulatesFailureReason`, `TestQueuedWaitNarrationIsNotSourcedFromFailureReason` |

t007 stays open on exactly this: re-probe every bullet on production after the deploy.

### Files changed

- `lego/backend/internal/logs/progress.go` — `buildWait`, `progressContext`, `appBuildWait`, `waitLine`, `workspaceCapCounts`; wait line in `progressLines`; `refreshWait` on the follower.
- `lego/backend/internal/logs/service.go` — three call sites now pass `newProgressContext(app)`.
- `lego/backend/internal/logs/progress_wait_test.go` (new) — six tests.
- `lego/backend/internal/store/queued_wait_failure_reason_test.go` (new) — the terminal-only guard.
- `docs/ADR018-render-parity.md` — one new bullet in § bex ahead of Render.
