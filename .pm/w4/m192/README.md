# w4 · m192 — Keep paused logs visible and merge live rows chronologically

**Worker:** worker4 **Goal:** operators can pause to inspect rows they just saw and read live/history logs in timestamp order. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Preserve current-reader rows when the user pauses Live | 45m | — |
| t002 | Merge history and live replay into chronological display order | 35m | — |
| t003 | Audit both live callers and shared virtualized reader controls | 30m | w4/m192/t001, w4/m192/t002 |
| t004 | Check Render parity and unchanged log API contracts | 20m | w4/m192/t001, w4/m192/t002, w4/m192/t003 |
| t005 | Simplify the milestone changes | 15m | w4/m192/t004 |
| t006 | Test retention, replay ordering and pending-reader boundaries | 40m | w4/m192/t004, w4/m192/t005 |
| t007 | Closeout after observable live acceptance | 10m | w4/m192/t006 |

Total: **7 tasks, 195m**. This ships the researched filing, not product fixes.

## Definition of done

Use the Free image fixture and exact probes in [finding.md](finding.md). These repeat observed journeys; unprobed siblings are verification work in t003/t006.

- From a fresh Application logs / Last hour page, enable Live and wait for two numbered rows beyond the historical head. Turn Live off and scroll to the bottom: both already visible matching rows remain accessible exactly once, the transport is closed and the footer says paused. Resume preserves them and deduplicates replay.
- Repeat on another fresh page after >100 lines. Enable Live and scroll to the historical/replay boundary: older START/n=1 precede later history; no later timestamp is followed by an earlier valid timestamp. Newly emitted matching rows are reachable at the bottom without discarding older in-window replay.
- Repeat the exact GraphQL, REST and MCP n=150-equivalent marker probes against the new verification service. They return the same message/precise timestamp as the displayed row; the API envelopes, limits and cursor semantics remain intact.
- Keep the Logs document open across Manual Deploy → Deploy latest image. Old/new-instance markers arrive without reload or a false disconnect; the actual deploy becomes Live and the public endpoint responds 200.
- Delete the owned fixture, verify service/public 404 and exact resource inventory restoration, and revoke only the verification session. Complete t003/t006 shared-callers, paging, pending and geometry checks before closeout.

## Source + Goal linkage

- **Source:** continuous user-requested `qa-find-bugs w4`, cycle34, 2026-10-10 UTC, private muse.env login. [Finding](finding.md) carries repeated live observations, complete API probes, cause, dependency/source census, dedupe and cleanup.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) usable hosting, [ADR010](../../../docs/ADR010-observability.md) trustworthy logs, [ADR018](../../../docs/ADR018-render-parity.md) dashboard parity and [ADR006](../../../docs/ADR006-bex-api.md) unchanged API contract.
- **Expected outcome:** user pause retains current-reader visible rows; historical and streamed rows read chronologically with stable dedupe and viewport behavior.
- **Why now:** both defects reproduce on the ordinary default Last hour reader; the API retains the missing row, and service redeploy delivery passed, isolating display fixes.
- **Sizing:** 80m for two independent behavioral changes plus 30m for shared caller/reset/virtualized checks; standing verification/closing tasks make 195m. This exceeds a sub-hour inbox item without broadening the feature.
- **Render parity included:** tenant-facing reader behavior changes; official logging docs support runtime search/ranges/live/instance controls, while exact authenticated Render pause behavior is unverified.
- **Limits:** web was probed live; seven-family scope, build completion, pending races, mobile and busy-buffer cases are explicitly verification work. Both findings are minor; no backend data loss is claimed.
- **Cleanup:** owned Free service deleted, service/public 404, all six identity sets restored, isolated session revoked and cookie/state files removed.
