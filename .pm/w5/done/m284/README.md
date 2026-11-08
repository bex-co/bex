# w5 · m284 — Restart waits for an in-flight deploy under Render's Wait policy

**Worker:** worker5 **Goal:** a restart requested while a deploy is in flight waits for it, as Render's Wait policy does, instead of cancelling the new deploy. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's restart behavior under Wait and bex's current cancel path — **DONE** | 30m | — |
| t002 | Queue a restart behind the in-flight deploy and pick the release when it runs — **DONE** | 2h | t001 |
| t003 | Use the same policy decision as ordinary deploys — **DONE** | 1h | t002 |
| t004 | Surface the queued restart on REST, GraphQL, MCP and the dashboard — **DONE** | 1h | t002 |
| t005 | Live walk on dev-5: restart during a build — **DONE** | 30m | t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 6h45m**.

## Definition of done

- In a Wait-policy workspace, `POST /v1/services/{id}/restart` (and GraphQL/MCP/dashboard) during an in-flight deploy queues; the in-flight deploy finishes live and the restart then runs on the newly serving release. No deploy row is closed "Canceled by restart".
- Override-policy workspaces keep today's behavior; the policy is resolved by the same rule as ordinary deploys.
- ADR018:95 reads the truth (no "Wait is not implemented"), with evidence from the dev-5 walk.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 2 (2026-11-07); ADR018:95.
- **Goal linkage:** Render parity first (ADR008); git push to deploy (V0 roadmap item 3).
- **Expected outcome:** clicking Restart while a push builds no longer silently drops the push.
- **Why now:** w8/m119/m121 built Wait for ordinary deploys but left restart cancelling, and the row is marked ✅, so parity sweeps miss it.
- **Decision (2026-11-07):** the user approved the round-7 proposal ("them all to w5"). Restart follows Render's rule — Wait for workspaces created on or after 2025-07-14 — through the w8/m121 resolver. This is the parity-sufficient choice under the AGENTS rule "never park parity work on an optional decision"; an always-Wait variant would be a divergence needing sign-off.
- **Render parity:** included.

## Findings (2026-11-08, loopx w5)

- **t001 Render:** [render.com/docs/deploys](https://render.com/docs/deploys) has two policies. Wait ("allow the in-progress deploy to finish, then proceed directly to the most recently triggered deploy", skipping intermediate ones) is the default for workspaces created on or after 2025-07-14. Override applies before that date. A restart "uses the exact same Git commit and configuration as the running instance at the time of the restart". In bex, ordinary deploys already followed Wait (w2/018, w8/m119), but Restart ended the deploy in flight. The release was selected at request time and the App CR was patched at once (`restartSelectedRelease` → `openSelectedRelease`), so simply queuing that patch would have rolled the new release back once it went live.
- **t003 policy decision:** every bex workspace was created after 2025-07-14, so Render's rule gives Wait for all of them. Restart therefore follows the same Wait that ordinary deploys already use, with no per-workspace switch.
- **t002 mechanism:** while a deploy is executing, a Restart records a deferred restart row (migration 0211 `restart_deferred`: status `queued`, latest-pending slot, generation 0) and leaves the App CR untouched. `DeferredRestartWorker` (every 5 s) runs it once the app has no executing deploy: `restartNow` under `store.WithDeferredRestart` selects what serves then, and `createDeploy` promotes the same row in place, keeping its id, trigger time and caller. Inside the trigger lock, a pre-check (`CheckDeferredRestart`) runs before the App changes, so two replicas cannot run the same restart. A newer trigger replaces a waiting restart (`Superseded by`). Cancel, a suspended or deleted service, and a 3 h wait each close the row only while it is still waiting. The reconciler never projects a deferred row; without that skip, a mutation test showed it closing the row as `build_failed`.
- **t004 surfaces:** REST, GraphQL `restartServer`, MCP `restart_service` and the dashboard all go through `deploys.Service.Restart`, and they return the queued restart deploy. The dashboard already renders `queued` and the Restart trigger, so it needed no change.
- **t006 Render parity:** the behavior now matches Render's Wait. ADR018:95 is rewritten. GraphQL still returns a `Deploy` (the existing w2/m30 divergence).
- **t007 Simplify:** `lockApp`, `appHasActiveDeploy` and a `scanDeploy(row, extra...)` helper are shared with `prepareDeployCreate`. The review's three race findings (a double run across replicas, Cancel against promotion, and an unconditional worker cancel) were fixed. One residual remains: a deploy path that opens a release without the trigger lock (`apps` settings and rollout writes) can still land between the pre-check and the promotion. In that case the promotion refuses, and the restart release rolls without its row.
- **t008 tests:** real-Postgres tests `TestPGDeferredRestartWaitsThenPromotes`, `TestPGDeferredRestartIsReplacedByANewerTrigger` and `TestPGCancelDeferredRestart`; `TestReconcileLeavesADeferredRestartWaiting`; and in deploys, `TestRestartWaitsForTheDeployInFlight` (the App stays untouched while the deploy is in flight, then the restart rebuilds the now-serving commit through the same row), `TestDeferredRestartThatCannotRunIsCanceled` and `TestDeferredRestartGivesUpAfterMaxWait`.
- **t005:** dev-5 is unavailable (see w5/m283's CAPD certificate mismatch). The walk runs on production once deployed: start a push build on a QA service, restart it during the build, and record that both run in order.

## Closeout — 2026-11-08 (loopx w5)

- **t005, production walk** (`hello-go` `srv-dbkq3hdhhjfs73eiahe0`, bex-canary, production pinned to `d6666dc65`):
  - `POST /v1/services/{id}/deploys` → `dep-dboes7d88aas73ah2f9g` (commit `ab6a46a6`). Four seconds later, `POST /v1/services/{id}/restart` → `dep-dboesbd88aas73ah2fag`, `queued`, trigger restart. The deploy in flight kept building.
  - **13:48–13:50:** the restart stayed `queued` while the deploy went `build_in_progress` → `update_in_progress`.
  - **13:51:19:** the deploy went `live` and the restart row became `created` on commit `ab6a46a6`, the now-serving release.
  - **13:52:21:** the restart went `live`. No row reads "Canceled by restart".
- **DoD:** all three bullets hold.
