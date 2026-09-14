# w4 · m107 — Log history past the newest 100 lines is unreachable on GraphQL and MCP

**Worker:** worker4 **Goal:** a user who selects "Last 7 days" on a busy service can actually reach seven days of logs, and is told when a view is truncated **Status:** todo

## Tasks (in order)

| id   | title                                                                | est | depends_on         |
| ---- | -------------------------------------------------------------------- | --- | ------------------ |
| t001 | Return the Render paging envelope from the GraphQL `logs` field       | 40m | t008               |
| t002 | Return the same envelope from MCP `list_logs`                          | 25m | t001               |
| t003 | Page backward in the log viewer, and say when a view is truncated      | 50m | t001               |
| t008 | REST's next-page cursors follow Render's contract: feeding `nextStartTime`/`nextEndTime` back fetches the next page | 30m | — |
| t004 | Render parity across REST / GraphQL / MCP / UI                          | 30m | t002, t003, t008   |
| t005 | Simplify                                                               | 20m | t004               |
| t006 | Test coverage                                                          | 40m | t004               |
| t007 | Closeout                                                               | 10m | t006               |

## Definition of done

Each bullet is a command or a click the next person can repeat against production.

- **A busy service's log viewer reaches past the newest 100 lines.** On `https://dashboard.bex.co/services/srv-d9bj8s3eg85c7390eb9g/logs` with range `Last 7 days`, scrolling the log pane to its top loads older entries instead of stopping. Today the pane stops at its oldest loaded line — measured 2026-09-14: scroll-to-top moved the oldest visible entry from `08:06:24 AM` to `08:05:39 AM` and went no further, and the accessibility tree contains no `load`/`older`/`more` control.
- **The viewer says when it is showing less than you asked for.** With a range whose result is capped, the page states that the view is truncated. The target wording must distinguish "this is everything in the range" from "this is the newest 100 of more" — an empty or silent state satisfies neither.
- **REST's cursors page, as Render documents.**
  1. Call `GET /v1/logs?resource=<srv>&startTime=<t0>&endTime=<t1>&limit=5`, with each `direction`.
  2. Repeat it with `startTime=<nextStartTime>&endTime=<nextEndTime>` from the response.

  The repeat returns the next page, and the chain reaches `hasMore:false` with every line of the window exactly once. At filing (2026-09-14, w1 `/qa-find-bugs` pass 27, `srv-dak4bta6m8ac739r63j0`), page 2 was `400 bad request: startTime must be before endTime` in both directions (t008).
- **GraphQL `logs` returns the envelope REST already returns, with t008's cursors.** `{ logs(...) { ... } }` answers with `hasMore`, `nextStartTime` and `nextEndTime` alongside the entries, matching `logs/render.go:63-66`'s `{hasMore,nextStartTime,nextEndTime,logs}` — the shape `w4/m96/t003` records as the unchanged Render wire contract. Today the field's type is a bare `[LogEntry]` (verified by introspection, 2026-09-14).
- **MCP `list_logs` returns the same envelope.** Today `logs/mcp.go:83` is `Logs []LogEntry` with no `hasMore`.
- **The 100-row cap itself is unchanged.** `maxLogLimit = 100` (`logs/service.go:106`) is deliberate Render parity — "Render defaults the logs `limit` to 20 and caps it at 100; bex matches". This milestone must not raise it; reaching history is paging's job, not the cap's.
- **Live tail still appends forward.** The `Live` switch continues to append arriving lines, and paging backward does not disable or fight it.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 37, 2026-09-14, on `beancount-cms-v2` (read-only; nothing created or changed). Probe, re-runnable from an authenticated dashboard page — a 1-hour window and a 7-day window return **identical** results:

  ```graphql
  { logs(resource:"srv-d9bj8s3eg85c7390eb9g", startTime:"<now-1h>", endTime:"<now>", limit:200, direction:"backward"){ timestamp } }
  ```

  Both answered with exactly **100** entries spanning ~14 seconds (`limit:200` requested, 100 returned — the server cap). So selecting "Last 7 days" on this service shows roughly fifteen seconds of history, silently.

- **Goal linkage:** ADR006 (bex-api's three surfaces carry the same contract) and the logs surface of ADR018's parity ledger. It is also the first concrete instance of the hole `w4/086` and `w4/087` named — "bex's MCP is hash-pinned against upstream Render, but bex's own three surfaces are pinned against each other by nothing". Those notes predicted a same-verb-narrower-response divergence existed and had not found one; this is it, in the predicted direction.
- **Expected outcome:** a busy service's operator can read yesterday's logs. Today they cannot, from the dashboard or from an agent over MCP, and neither surface tells them so.
- **Why now:** the dashboard is a GraphQL client (`dashboard/CLAUDE.md`), so it is **structurally** unable to page or to warn — the information it would need is absent from its surface. That makes this a backend-shape fix first and a UI fix second, and it is why no amount of dashboard work alone can close it.
- **Render parity task included:** the change alters response shape on GraphQL and MCP and behavior in the UI. REST is the reference and should not move — `logs/render.go:94-119` already computes `HasMore = limit > 0 && len(entries) >= limit` with both cursors, and Render marks `nextStartTime`/`nextEndTime` REQUIRED.
- **Correction (2026-09-14, w1 `/qa-find-bugs` pass 27):** REST's cursors cannot be followed, so REST does move, in those two fields only.
  - `render.go:105-108` sets them to the page's own bounds (newest and oldest line), not to the next page's window.
  - Render's API docs, `render-mcp-server`'s `list_logs` description, and the official Render CLI's scroll-to-load (`render-oss/cli` `pkg/tui/views/logview.go:196-199`) all feed them straight back as `startTime`/`endTime`, which bex answers with a `400`.
  - t008 fixes the helper first, and t001/t002 reuse it.

## Verified working, and deliberately out of scope

Recorded so the fix is not mistaken for a broader logs overhaul — these were exercised live in the same pass and are correct:

- **Search narrows.** A nonsense term took the pane from 13 visible rows to 0; filter state lands in the URL (`?text=…&live=0`).
- **The empty state is honest** — "No matching logs / No logs match these filters", which correctly distinguishes no-matches from no-logs.
- **The range control reaches the data layer.** Selecting `Last 7 days` updates the URL to `?range=7d`, updates the label, and the API honors `startTime`; the range is not ignored, it is simply unreachable past the cap.
- **Every log control has an accessible name**, including the per-instance filter, which interpolates its instance id (`button "Filter logs by instance srv-…-21s1n5k4mnm3fan0osi5"`). This is the good counterexample to `w4/084`'s bare-verb controls and should be left alone.
- Ascending order (newest at the bottom) is the tail convention, not a defect.

## Unverified

- **Not reproduced on a quiet service.** On a low-traffic service the newest 100 lines may well span the whole selected range, so the truncation is invisible. Whoever picks this up should confirm the fix does not add a "truncated" notice to a view that is actually complete.
- **The 100 entries were counted from the GraphQL response, not from Loki.** Whether Loki itself would return more within the window, and how `direction:"backward"` interacts with `startTime` on a second page, was not tested — t001 needs to establish that before the envelope's cursors can be trusted. **Answered by pass 27 (see t008):** following the returned cursors is a `400` in both directions. A hand-built backward page 2 (original `startTime`, `endTime` set to page 1's oldest line) returned 5 strictly older rows with no overlap.
- **No MCP agent was driven through the truncation.** The `list_logs` gap is read from `logs/mcp.go:83`, not observed by calling the tool.
- The dashboard's logs feature has **no** `direction`, `fetchMore` or `loadMore` today (grepped across `dashboard/src/features/logs/`), so t003 is new behavior rather than wiring an existing path.
- Deploy status, **fifteenth consecutive check**: production still serves the pre-`e6419af0b` build, so `w4/078`'s re-probe, `w4/m105`'s scoped exchange and `w4/m106`'s live refusal test all remain queued.
