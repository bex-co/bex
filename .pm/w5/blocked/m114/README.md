# w5 · m114 — Suspend ends the release it interrupts, not just its deploy row

**Worker:** worker5 **Goal:** Suspending mid-rollout cancels the in-flight release the way Cancel does, so Resume returns to the prior live release and the deploy history stays truthful. **Status:** blocked — t001–t005 and t007–t009 done 2026-10-05; t006's resume half needs the operator change deployed; t010 closeout follows

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Research Render's suspend-during-deploy and resume behavior — **DONE** | 20m | — |
| t002 | Reproduce suspend mid-rollout followed by resume — **DONE** | 45m | t001 |
| t003 | One release-ending verb for Cancel and suspend — **DONE** | 45m | t002 |
| t004 | Make the close paths agree on pre-deploy status and cancel reason — **DONE** | 30m | t003 |
| t005 | Enforce the closed-row invariant in the database — **DONE** | 45m | t004 |
| t006 | Verify suspend and resume live on dev-5 | 30m | t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 30m | t007, t008 |
| t010 | Closeout | 10m | t009 |

## Definition of done

- Suspending mid-rollout cancels both the deploy row (with the suspend reason) and the release (annotation stamped). Resume serves the prior live release, the canceled release's pre-deploy never runs, and a later deploy works.
- A gate-closed pre-deploy step reads `failed`, and `cancelPendingDeploys` rows carry a cancel reason.
- After a backfill, a database CHECK forbids closed rows with `pre_deploy_status = 'running'`.
- A dev-5 replay is recorded.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `60ae617b3` (w4/m171). Code-verified asymmetry: Cancel stamps `AnnotationCanceledReleaseGeneration` (`deploys/service.go:948`) and suspend doesn't. On Resume, the operator's `holdUnservedRelease` rolls the still-requested release with no open deploy row (reviewer reading; no resume test exists). m171's own DoD says Resume returns to the prior live release.
- **Goal linkage:** ADR004 app deployment lifecycle; ADR018 Render parity for deploy statuses.
- **Expected outcome:** Suspend and resume never ship code the user saw canceled, and deploy history stays truthful.
- **Why now:** Shipped on 2026-10-04: any suspend during a deploy followed by resume can run a canceled release's migration.
- **Render parity included:** deploy statuses and cancel reasons are user-visible on every surface.

## Research (t001), 2026-10-05

Render's docs (`render.com/docs/deploys`, the API reference for suspend/resume/cancel, the dashboard changelog) say nothing about what suspending does to an in-progress deploy, or which deploy a resumed service runs. What they do say:

- Canceling an in-progress deploy keeps traffic on the existing instances.
- `deploy_ended` reports `canceled`.
- A restart reuses the running instance's commit and configuration.

Chosen behavior, matching w4/m171's own definition of done: a suspend ends the interrupted release the way Cancel does, and resume serves the prior live release.

## Evidence — 2026-10-05

**Cause, confirmed live (dev-5).** The w4/m171 reconciler closed the suspended rollout's row canceled but never canceled the release. A second, operator-side cause turned up:

- The suspend's own spec write moves `metadata.generation` past the release.
- `requestedReleaseGeneration` then falls back to `metadata.generation`, because the release is already adopted.
- So even a stamped cancel would not match.

**Fix.**

- **Backend:** `store.CancelRelease` is now the one way a release ends.
  - It re-reads the App, stamps `AnnotationCanceledReleaseGeneration` under an optimistic lock, and never overwrites an equal or newer cancel.
  - It then stops a repo-backed build (Job and kpack Image), best effort; a kind the cluster doesn't serve counts as gone.
  - Only the stamp can fail it.
  - `deploys.Cancel` uses it.
  - The reconciler's suspend close calls it before recording anything about the close, wired with `BEX_BUILD_NAMESPACE`.
  - A new level-triggered rule (`releaseCanceledFor`) closes any open row whose release the stamp names. That heals a close that failed after its stamp: once the operator settles, it rewinds the release generation and hides the row from every other close rule.
- **Operator:** `canceledReleaseIsLatest` accepts a stamp equal to the backend's release-generation annotation while no later release has been adopted. `canceledOverServed`, which also skips the pre-deploy, shares it.
- **Deviation from t003 as written:** `suspendEndsRollout` was kept, not deleted. Git-push autodeploy, Blueprint sync and config-change rollouts all open rows on a suspended service without refusing, and the reconciler is the one place that sees every such row. It now ends their releases too.
- **t004:**
  - A pre-deploy step running when its deploy closes `pre_deploy_failed` (the gate timing it out) reads `failed`, not `canceled`.
  - Every superseded trigger names its replacement: the coalesced pending slot (`cancelPendingDeploys`) and a late trigger for an older generation both carry "Superseded by dep-…".
- **t005, migration 0142:**
  - It backfills every closed row whose step reads `running` by the deploy's outcome. It also corrects rows w4/187 already settled as `canceled` beside a `pre_deploy_failed` close.
  - It disables the analytics trigger `product_deploy_event` during the backfill, so purged events aren't resurrected.
  - It then adds `CHECK (finished_at IS NULL OR pre_deploy_status <> 'running')`. `cancelPendingDeploys` settles a running step too, so a trigger racing the reconciler can't trip it.
  - Violating-row count: dev-5's database is fresh (0 deploys), the other dev stacks are down, and production isn't reachable from this session. The migration settles any number of rows, and its test seeds every class.

**Tests.**

- **Operator:**
  - `TestSuspendedCanceledReleaseStaysCanceledThroughResume` and `TestReleaseAdoptedPastTheCancelIsNotCanceled` (unit).
  - `Suspending mid-rollout, then resuming` (envtest). Release 1 serves; release 2's pre-deploy starts; suspend, stamp, resume; the pre-deploy then completes. Release 1's template still serves.
  - Under the old rule the envtest rolls `nginx:2`.
- **Backend:**
  - `TestUserSuspendCancelsTheInterruptedRollout`, now asserting the stamp, red before the fix.
  - `TestSuspendCloseSurvivesAFailedBuildDelete`, which also checks `BuildNamespace` and that a repeat stamp doesn't patch.
  - `TestCancelReleaseNeverOverwritesANewerCancel`, `TestCancelReleaseSucceedsOnceStamped` (including NoMatch), and `TestStampedReleaseClosesItsStrandedRow`.
  - PG: `TestPGTerminalCloseSettlesARunningPreDeploy` (+ gate case), `TestPGSupersededTriggerNamesItsReplacement`, `TestPGCoalescedRowSettlesItsRunningStep` and the race test's reason check.
  - Migration: `TestClosedDeployPreDeploySettledMigration` covers backfill, CHECK, trigger silence and re-enable, and down.
- **Mutation checks:** every rule above was reverted alone, and its test failed.
- **Suites:** the backend store, deploys, apps, rollout and cmd/api suites pass on fresh dependencies, along with the types module.

**dev-5 replay (t006, backend half).** The fixture was `nginx-unprivileged` with a `sleep 300` pre-deploy.

1. Deploy B (generation 3) was in `pre_deploy_in_progress` with its Job running when the suspend landed.
2. B closed `canceled` with "Canceled: the service was suspended". The config-change row the PATCH opened closed "Superseded by dep-…B".
3. The CR then read generation 4, release 3, canceled 3, status release 3. That's exactly the shape the old operator misreads.
4. After resume, the shared local operator (which predates this change) rolled release 3 (`rev-3`, Running, pre-deploy `Succeeded`) while both rows read canceled. That's the original bug, seen live.
5. The fixture was deleted (`DELETE` 204, then `GET` 404). Its App CR waits on the finalizer (w5/084).

**Render parity (t007).** `cancelReason` and `preDeployStatus` are bex extensions, carried identically by REST (deploy view), GraphQL (`cancelReason`) and MCP (`toRenderDeploy`, the REST shape). The dashboard renders them in the deploy header, list and failure-reason components. Render reports these closes as `canceled`, as bex does. No drift.

**`/simplify` (t008), applied:**

- Reuse: `stampedReleaseGeneration` in the operator; `newTestReconciler`, `openLifecyclePG` and shared envtest accessors in the tests; the `gen-` spelling test moved to `types`.
- Quality: the stale-stamp guard and optimistic lock; best-effort deletes for both callers; the stranded-row rule; stamp before facts; the adoption bound; the extra backfill class; contract and comment refresh (`AnnotationCanceledReleaseGeneration`, `Cancel`, `cancel_reason`, the cancel metric).
- Efficiency: the race through `cancelPendingDeploys`; the trigger kept out of the backfill; the repeat-stamp no-op.
- Measured as fine: `prepareDeployCreate`'s scan, and 0142's single-transaction locking at current table sizes.

**Known and filed.** An in-flight pre-deploy is left to finish rather than killed mid-migration, as user Cancel always did. Its closed row then reads "canceled" although the migration ran: w5/085.

## Blocked — t006 resume replay (after deploy)

Every dev-N stack shares the one in-cluster bex operator, which this workstream may not replace. So the resume half runs after the release pipeline deploys this change, on an owned `qa-` web service in `bex-canary`, or on dev-5 once the local operator is current:

1. Deploy A.
2. Set a slow pre-deploy (`sleep 300`) and deploy B.
3. Suspend while B's pre-deploy runs.
4. Expect B canceled with the suspend reason, and the CR stamped with B's release generation.
5. Resume, and expect A to serve: active revision stays A's, status release generation returns to A's, and B never becomes active even after its pre-deploy Job completes.
6. A later deploy goes live.
7. Delete the fixture, record the evidence here, then run t010.

