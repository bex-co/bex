# w8 · m117 — Canceling the newer of two open deploys strands the older

**Worker:** worker8 **Goal:** Canceling one deploy never strands another: when two deploys are open for a service and the newer (latest-pending) one is canceled, the older open deploy is dispatched and reaches a terminal state on its own merits. **Status:** todo

## Tasks (in order)

| id   | title                                                                | est | depends_on     |
| ---- | -------------------------------------------------------------------- | --- | -------------- |
| t001 | Diagnose the stranded-row mechanism and choose the fix             | 45m | —              |
| t002 | Fix cancel/adoption so the surviving open deploy proceeds          | 90m | w8/m117/t001   |
| t003 | Render parity: cancel semantics across REST, GraphQL, MCP, and UI  | 30m | w8/m117/t002   |
| t004 | Simplify the changed code                                          | 20m | w8/m117/t003   |
| t005 | Test coverage: cancel-newer, cancel-older, and supersede interplay | 50m | w8/m117/t003   |
| t006 | Close out the milestone                                            | 10m | w8/m117/t005   |

Six tasks, 245m.

## Definition of done

- Live repro passes: create a free web service (initial deploy A open), trigger deploy B, cancel B via `bex deploys cancel` — A leaves `created`/`queued` within 5 minutes and reaches `live` (or honestly fails its build/rollout); the service URL serves 200. B stays `canceled`.
- The same holds for an established service: canceling the newest pending deploy leaves the previously open deploy (or the live release) unaffected and advancing; no deploy row sits in `created`/`queued` with an untouched `updatedAt` for > 10 minutes while a same-shape control service builds normally.
- Canceling the older deploy still supersedes to the newer exactly as today (sweep-51 behavior preserved), and canceling the only open deploy still settles to the last served release / `PhaseCanceled`.
- Regression tests cover cancel-newer-with-older-open at the store/reconciler level and pass in CI.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweeps 52–53, 2026-10-18 (this loop's ledger `/tmp/bex-qa-cli-loop/ledger.txt` at filing time). Repro ×2 against `https://api.bex.co/v1/` in workspace bex-canary with the pinned-CLI HEAD build: sweep 52 older deploy sat in `created` ~9.5m until the fixture was deleted; sweep 53 (`srv-dba8idfarj3c73bslk6g`, older `dep-dba8idfarj3c73bslk70` created 08:54:13Z, newer `dep-dba8ifnarj3c73bslk8g` canceled 08:54:22Z) sat in `queued` with `updatedAt` untouched since 08:54:17Z for 13m+, while a same-shape control service created 09:05 reached `build_in_progress` in ~80s — so this is not build-capacity queueing.
- **Goal linkage:** deploys are the core hosting primitive (ADR008 Render-alternative core); a cancel — an explicitly per-deploy Render API (`POST /services/{id}/deploys/{deployId}/cancel`) — silently wedging a different deploy breaks the deploy contract and, for a brand-new service, leaves it never-live with no error.
- **Expected outcome:** per-deploy cancel is per-deploy; overlapping-deploy state machine has no stranded-open-row state.
- **Why now:** found live twice in one QA day with a trivial two-command repro any API/CLI/dashboard user can hit (double-trigger then cancel the duplicate). The stranded row only closes via the gate-timeout fallback (~35m), if at all, so users see a deploy that never starts and no failure reason.
- **Render parity included because** cancel is a tenant-facing verb on REST/GraphQL/MCP and the dashboard; the fix must not change cancel's wire semantics (200/409 shapes) on any surface.
