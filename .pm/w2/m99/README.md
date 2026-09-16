# w2 · m99 — A queued deploy says why it waits

**Worker:** worker2 **Goal:** while a deploy row is `queued`, every log surface states the operator's current reason for the wait, so a capacity wait is distinguishable from a stuck deploy. The operator already writes the reason to the App's `Ready` condition; today bex-api drops it until the row times out. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | While a deploy row is `queued`, the progress follower narrates the App's `Ready` condition reason and re-emits on change               | 35m | —          |
| t002 | Tenant-neutral copy for the cluster cap and scheduler wait with no counts; "preparing registry credentials" for `RegistryCredsPending` | 20m | t001       |
| t003 | Confirm REST `GET /v1/logs`, GraphQL `logs`, MCP and the SSE tail carry the identical line; `failureReason` stays terminal-only        | 20m | t001       |
| t004 | Render parity                                                                                                                          | 15m | t002, t003 |
| t005 | Simplify                                                                                                                               | 15m | t004       |
| t006 | Test coverage                                                                                                                          | 30m | t004       |
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
