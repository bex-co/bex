# w4 · m203 — Anchor a newly selected relative range at the current time

**Worker:** worker4 **Goal:** switching to a relative Logs or Metrics range immediately queries a window ending at the selection time. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Resample the relative clock before the first selection query | 45m | — |
| t002 | Verify the three shared callers and unaffected range consumers | 25m | t001 |
| t003 | Render parity for range selection and API bounds | 15m | t001, t002 |
| t004 | Simplify the milestone changes | 10m | t003 |
| t005 | Test transitions, first queries and timer lifecycle | 35m | t003, t004 |
| t006 | Closeout after live acceptance and cleanup | 10m | t005 |

Total: **6 tasks, 140m**. This filing schedules the fix; product code is unchanged.

## Definition of done

Replay the ordinary browser journeys in [finding.md](finding.md) on a newly owned Free image web service producing one plain marker every five seconds.

- On desktop, load the recorded Custom Logs range with Live off, wait at least 90 seconds, and select Last 12 hours. The first actual Logs request ends at the current selection time, rather than the page's earlier mount/tick time. Available markers produced after that old endpoint appear without reload or waiting for the 300-second interval. Its start is exactly 12 hours before its end; the existing limit and pagination envelope remain intact.
- Repeat from a fresh 390px page: switch Last 12 hours to Custom, wait, then select Last 24 hours. Its first request uses the new current endpoint and a 24-hour span. Custom URL bounds clear; `live=0` remains paused. Selecting Last 30 minutes also samples a fresh endpoint, then advances normally at the existing 15-second cadence.
- Freshly load Metrics with Custom bounds, wait at least 90 seconds, then select Last 24 hours. The first actual HTTP_REQUESTS request uses the new current start/end with resolution 720. The observed stale-request defect is the acceptance target; this hunt did not establish wrong Metrics values.
- Reloading the same relative selection remains a passing control; a Custom range remains exactly fixed without an interval. Delete the owned fixture, verify service/public 404 and removal from all seven inventory sets, revoke only that QA session, clear its browser state and remove its isolated session handles.

Caller families, hidden-tab behavior, alias coverage and unprobed API surfaces are verification work in t002/t003/t005, not completed live claims.

## Source + Goal linkage

- **Source:** continuous functional `$qa-find-bugs`, production pass a314, 2026-10-12 local / 2026-10-13 UTC. User requested w4 and credentials through the QA helper using muse.env. [Finding](finding.md) and [complete transport receipts](evidence.json).
- **Goal linkage:** ADR008 usable hosting, ADR010 observability, ADR018 Render-compatible range controls and ADR006 one API core consumed by the dashboard.
- **Expected outcome:** selecting “Last 12/24 hours” includes the latest available history immediately instead of silently ending at an old clock sample.
- **Why now:** desktop omitted 48 available markers; an independent fresh mobile page and Metrics request reproduced the old endpoint. Longer presets wait 12 minutes to six hours for their first timer tick.
- **Render parity included:** tenant-visible range semantics change. Render documents presets, Custom bounds and a separate live feed; exact authenticated Render transition timing was not inspected.
- **Dedupe:** no open equivalent in 90 non-done READMEs or targeted whole-board searches. w5/done/m56 supplies fixed Custom behavior; w4/done/m151 preserves reader continuity on routine ticks; w4/done/m152 updates relative text labels; w4/m175 owns backend bucket clipping. This is an uncovered selection-time clock transition, with no claim that those milestones' full guarantees regressed. No relevant landed fix or anti-goal match.
- **Limits:** live web app logs and HTTP_REQUESTS transport only. Static, cron, worker, private, datastores, REST/MCP, other metric values, hidden tabs and timer races remain unprobed. Cleanup and own-session revocation verified.
