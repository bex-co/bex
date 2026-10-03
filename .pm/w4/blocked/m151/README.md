# w4 · m151 — Preserve loaded log history across relative-range refreshes

**Worker:** worker4 **Goal:** a service operator reading older logs keeps the loaded, still-in-window rows and reading position when the selected relative range refreshes. **Status:** blocked — t001/t002/t004/t005 done; t003/t006 await dashboard release and deployed acceptance.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Preserve the log reader through automatic range refresh | 45m | — |
| t002 — **DONE** | Audit shared paging callers and explicit reset boundaries | 35m | w4/m151/t001 |
| t003 | Render parity across log history and live controls | 20m | w4/m151/t001, w4/m151/t002 |
| t004 — **DONE** | Simplify the milestone changes | 20m | w4/m151/t003 |
| t005 — **DONE** | Test clock ticks with real paging and pending states | 35m | w4/m151/t003, w4/m151/t004 |
| t006 | Closeout after live acceptance | 10m | w4/m151/t005 |

Total: **6 tasks, about 2h 45m**. This filing schedules the fix; it does not implement it.

## Definition of done

Recreate the free example service and numbered-request fixture in [finding.md](finding.md). These targets come from probes run in sweep 12; the unprobed sibling cases are assigned to t002/t005.

- Open Application logs with text `/qa-r12/alpha/`, Last hour and Live off. Load the earlier 40 behind the first 100, scroll to an older marker, and leave the page untouched across two 30-second ticks. All 140 still-in-window lines remain reachable exactly once, the reading anchor stays stable, and no first-load skeleton or renewed “newest 100” notice replaces the complete view.
- Repeat on a fresh page. The new first-page time bounds may advance, but current-reader content remains visible while the request is pending and after it settles. Initial loading and a deliberate filter/resource/range change still use the correct state and reject stale responses.
- Select the recorded Custom interval containing the same 140 lines, wait for it to settle, page back and read the same marker for at least a minute. Absolute bounds and the reading position stay fixed. Last 7 days still permits paging to alpha/001.
- Narrow text to `/qa-r12/alpha/00`: exactly nine rows and no truncation notice. A nonsense term produces the honest no-matches state. Reload retains Application logs, Custom bounds, text and Live off.
- Replay the finding's GraphQL 100+40 and REST/MCP 5+4 cursor controls. The complete row sets, cursors, default order and existing 100-row server cap remain unchanged; REST's forward and backward chains reach each sample row once. Live on still appends a newly generated marker.
- On the finished >100-line build, Load older still reaches `==> Build queued` and retains it through same-window first-page polls. Complete the explicitly unverified datastore, live/paging and sibling-route controls in t002/t005 before closing.
- Delete only the verification fixtures, verify API/list/public-route removal and exact-identity resource cleanup, and revoke only that QA session.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, production sweep 12 on 2026-10-02, using `muse.env` and targeting w4 by user request. [Finding](finding.md) contains two fresh-page reproductions, the fixed-range control, dependency-source evidence, full prior-DoD disposition and complete API probes.
- **Goal linkage:** ADR008 usable Render-compatible hosting; [ADR010](../../../../docs/ADR010-observability.md) durable, bounded observability; [ADR018](../../../../docs/ADR018-render-parity.md) log history; [ADR006](../../../../docs/ADR006-bex-api.md) unchanged shared API semantics.
- **Expected outcome:** routine background refresh does not interrupt an operator investigating older retained lines or make them repeatedly reload the same cursor page.
- **Why now:** the default Last hour selection reproduces this every 30 seconds even with Live off. The same data remains stable under Custom bounds, identifying the reset interaction precisely.
- **Render parity included:** tenant-visible log-reader behavior changes. [Render logging docs](https://render.com/docs/logging) describe range/custom/live controls; its authenticated idle-clock implementation remains unverified. API shapes are passing controls, not proposed changes.
- **Dedupe:** residual gap already present in w4/m107; w4/m136 preserved same-variable first-page polls but not clock-driven new bounds. Open w4/m147 is Events freshness through a different hook. No open equivalent, anti-goal match or undeployed main fix found.
- **Limits:** web/Application logs was reproduced live. Cron/worker/private, both datastores, static deploy route, mobile, expiry/error races and simultaneous live/paging require verification. Cleanup and session revocation for this hunt are complete.

## Implementation verdict — 2026-10-02

Reader continuity, bounded paging, stable viewport anchors and local regression gates are complete. [Verification and remaining release/QA gate](verification.md). No hosted fixtures or sessions were created.
