# w4 · m214 — Recover datastore cleanup from transient lock conflicts

**Worker:** worker4 **Goal:** an ordinary project deletion survives an unrelated datastore update, and persistent contention gives a truthful reload-before-retry explanation. **Status:** blocked — t001–t006 done 2026-10-16; t007 needs the release pipeline and the live replay

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Retry conditional datastore cleanup after unrelated lock conflicts — **DONE** | 35m | — |
| t002 | Explain interrupted cleanup with a safe public conflict — **DONE** | 25m | t001 |
| t003 | Audit shared cleanup consumers and preserve ordinary assignment — **DONE** | 25m | t001, t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 10m | t004 |
| t006 | Test coverage — **DONE** | 35m | t004 |
| t007 | Closeout | 15m | t005, t006 |

## Definition of done

- Replay [the native journey](finding.md): disposable project, Dev/Prod, two Free image web services and Free private PostgreSQL/Key Value, all ready. Preserve the successful occupied-target mixed move and filtered return controls. A late batch result may truthfully report WRITE_OUTCOME_UNKNOWN under m211; read actual placement before any next mutation.
- Project Settings → Delete Project → confirm preserves all four children and leaves them unassigned; project and both environments return404. The successful retry control at09:44:25Z establishes the intended final state. A fresh390px native replay is additional required verification, not a claimed failure from this hunt.
- In deterministic local composition, one unrelated resourceVersion update between fetch and patch is recovered inside each conditional-clear call for both datastore kinds and both grouping labels. Preserve unrelated fields/resource-owned rules, UID identity and expected-label fencing; a newer reassignment stays intact.
- Exhausted contention gives a bounded public conflict with a safe explanation that cleanup may be partial and current placement must be reloaded before retrying. It is never generic internal error or false delete success. Non-conflict errors and deadlines retain their existing semantics. Earlier writes may remain; no distributed transaction is promised.
- Preserve direct environment deletion, successful omitted-member removals, ordinary Set behavior, all existing m151/m152 admission/reassignment protections and three adapter error/state contracts. These local/shared checks are required implementation work, not claimed live failures.
- Required checks pass; exact owned fixtures/public routes disappear, all seven resource inventory ID sets return to baseline and only the run's QA session is revoked.

## Source + Goal linkage

- **Source:** continuous user-requested functional `$qa-find-bugs w4`, production a698, 2026-10-16 UTC. [Finding](finding.md), [28 exact native/API receipts, diagnostic source/output and cleanup](evidence.json). This hunt changes no product code.
- **Goal linkage:** ADR008 dependable hosting, ADR006 coherent API outcomes and ADR032 project/environment preserve-and-detach lifecycle.
- **Expected outcome:** unrelated datastore churn no longer forces a manual delete retry; sustained contention tells the user how to recover without implying earlier writes were undone.
- **Why now:** the15.2s native delete failed after datastore environment detachment; fresh reload/retry succeeded. An exact owned PostgreSQL lock conflict was logged, and four local two-kind/two-label probes isolate the one-attempt gap without production fault injection.
- **Sizing:** 165m across scoped producer retry, truthful interruption classification, shared consumer audit and standing closing tasks. This is more than an hour across multiple tasks.
- **Render parity included:** REST/GraphQL/MCP/dashboard outcomes change. Keep the documented Bex deletion divergence; authenticated Render retry/timing behavior was not measured.
- **Relationship:** m151 explicitly permits partial cleanup and protects concurrent reassignment; preserve that contract. m211 owns batch deadlines, m206 group lifecycle recovery and287 App cancellation retry. None owns this conditional datastore-clear conflict gap.

## Implementation record (2026-10-16)

- **t001:** `core.RetryPlacementClear` (`lego/backend/internal/core/placementclear.go`) wraps all four conditional clears (Postgres/Key Value `ClearProjectID`/`ClearEnvironmentID`). It uses client-go `retry.OnError(DefaultBackoff)` on Kubernetes conflicts only. Every attempt does a fresh authorized fetch, the UID identity check, the expected-label recheck, and the same optimistic patch. A concurrent reassignment makes the retried clear a no-op instead of a raw conflict.
- **t002:** exhausted conflicts and a replaced resource answer `PLACEMENT_CLEANUP_CONFLICT` (`core.ErrConflict`; 409 on REST, an extension code on GraphQL, a tool error on MCP). The message says earlier cleanup steps may already have been applied and to reload current placement before retrying. A canceled or expired context returns its own error: client-go's `OnError` otherwise reports a done context as the last conflict. Non-conflict errors are unchanged. The dashboard toast already relays the refusal (`mutationErrorMessage`), so no dashboard change was needed.
- **t003:** callers are the project-delete loop (`ClearProjectID`), plus environment teardown and omitted-member removal (`ClearEnvironmentID`, via the two typed `leave` adapters). Teardown serves both direct environment Delete and the project cascade. Ordinary `SetProjectID`/`SetEnvironmentID` and `SetEnvironmentIPAllowList` stay single-attempt.
- **t004:** ADR032 and ADR018 record the behavior and the preserved preserve-and-detach divergence. Render retry/timing was not measured.
- **t006:** `TestPlacementConditionalClearRecoversFromLockConflicts` (both kinds × project/environment) covers one unrelated update recovered in two patches with the write kept; persistent updates giving the coded conflict with placement intact; replaced UID; and cancellation. `TestPlacementConcurrentReassignmentWinsOverStalePatch` now expects a no-op. `TestProjectDeleteSurvivesDatastoreLockConflicts` runs REST/GraphQL/MCP project delete with real resourceVersion bumps: transient contention succeeds and detaches both datastores; persistent contention returns the coded conflict, keeps the project, then the same delete succeeds once contention ends. Both new tests fail with a single attempt. Full backend suite (real Postgres, `-p 1`), lint and deadcode pass. **Not exercised:** a live multi-writer cluster and the 390px native replay (t007).
