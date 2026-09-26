# w4 · m140 — A 7-day log search on a busy service dies at 30s as an edge 502, and the Logs tab spins ~90s before "Failed to fetch"

**Worker:** worker4 **Goal:** a log search the server cannot finish in time comes back as a named, CORS-carrying API error that tells the user to narrow the range, and the Logs tab shows it promptly instead of retrying a doomed request for a minute and a half **Status:** todo

## Tasks (in order)

| id   | title                                                                                           | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Put the GraphQL execution deadline strictly inside the server WriteTimeout and name the timeout | 40m | —          |
| t002 | Establish why a no-match 7-day text search needs >30s, and bound it at the source               | 45m | —          |
| t003 | Logs tab: no automatic retry of a search timeout; say "narrow the range" instead of "Failed to fetch" | 30m | t001   |
| t004 | Render parity across REST / GraphQL / MCP / UI                                                 | 20m | t001, t002, t003 |
| t005 | Simplify                                                                                        | 15m | t004       |
| t006 | Test coverage                                                                                   | 30m | t004       |
| t007 | Closeout                                                                                        | 10m | t006       |

## Definition of done

Probes were run read-only at filing (pass 170, 2026-09-25) against the production service `beancount-cms-v2` (`srv-d9bj8s3eg85c7390eb9g`). The API was probed through Playwright's request context (not subject to browser CORS) after a 60s idle, to rule out the `BEX_RATE_LIMIT` trap:

```graphql
query ($r: String!, $t: String, $s: String, $e: String) {
  logs(resource: $r, text: $t, startTime: $s, endTime: $e, limit: 100) { hasMore logs { timestamp } }
}
```

| window | `text`                 | result at filing                                                                                             |
| ------ | ---------------------- | ------------------------------------------------------------------------------------------------------------ |
| 1h     | `GET`                  | 200 in 314 ms, `hasMore:true`, `access-control-allow-origin: https://dashboard.bex.co`                       |
| 1h     | `zzqqxx-no-such-token` | 200 in 411 ms, `{hasMore:false, logs:[]}`                                                                    |
| 7d     | `zzqqxx-no-such-token` | **502 after 30 218 ms**, Cloudflare HTML error page, `retry-after: 60`, **no** `access-control-allow-origin` |

- **The API answers within its own budget.** The 7-day no-match search above returns a GraphQL response (200 with `data`, or a GraphQL error with a stable code such as `LOG_QUERY_TIMEOUT` and a message naming the window) that carries CORS headers, never an edge 502. Better still, t002 makes it return `{hasMore:false, logs:[]}` or a partial-page envelope.
- **The Logs tab tells the truth fast.** At `…/logs?range=7d&text=zzqqxx-no-such-token&live=0`, the pane settles within about the server budget into either "No matching logs" or a timeout state that suggests a narrower range. At filing it showed `status "Loading logs…"` for **~85–90s** (three attempts through `common/apollo/retry-link.ts`: `attempts.max 3`, retrying network failures and 5xx), then "Couldn't load logs — Failed to fetch". Evidence (local): `.playwright-mcp/qa-logs-search-7d.png`.
- **Control stays correct:** the 1h searches above keep their 200s, and the no-match 1h search still renders "No matching logs".

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` pass 170, 2026-09-25 (w4-targeted, `muse.env` credentials), journey 7 (Logs), read-only. The same pass re-verified `w4/m107` live: on `…/logs?range=7d&live=0` the notice "Showing the newest 100 matching lines in this range — scroll up for older history." renders, and each scroll-to-top loads an older page (pane `scrollHeight` 32 745 → 57 628 → 75 720 px). Searching `GET` narrows to matching rows. m107 is intact; this is a different failure.
- **Mechanism (partly verified):**
  - bex-api's `http.Server` has `WriteTimeout: 30 * time.Second` (`lego/backend/cmd/api/main.go:1389`).
  - The GraphQL handler bounds execution with the *same* 30s (`gqlExecTimeout`, `lego/backend/internal/api/graphql_cost.go:30`, applied at `internal/api/server.go:1494`). When the resolver runs to the deadline, the write deadline expires at the same moment, the connection is cut before the handler can serialize the deadline error, and Cloudflare substitutes its 502. That explains the missing CORS header (the browser therefore reports only "Failed to fetch"), and the measured 30 218 ms matches it exactly.
  - The REST log *stream* already clears its write deadline for this reason (`internal/logs/rest.go:383-386`), which shows the hazard is known on one surface.
  - This is the same timeout-inversion class as `w3/011` (a 30s client under a 60s server wait).
- **Why the query is slow is unverified** (t002): a line-filter over 7 days of a busy service's Loki streams with zero matches must scan everything. Whether bex-api issues one 7-day LogQL range or splits it, and whether Loki's own limits apply, was not read from production.
- **Goal linkage:** ADR010 (observability), ADR006 (one contract across surfaces), `w4/m107` (history reachability).
- **Expected outcome:** a user searching a week of logs gets either results or a clear "too broad, narrow the range" within the server's budget, never a 90-second spinner that ends in a transport error.
- **Why now:** `w4/m107` made 7-day history reachable, which makes 7-day *search* the natural next click, and it currently fails in the least informative way.
- **Render parity task included:** API error shape and UI behavior change. t004 also checks REST `GET /v1/logs?text=…` and MCP `list_logs` over the same window, since they share the core query but not the GraphQL deadline.

## Unverified

- Whether Loki alone would finish the 7-day no-match query given more than 30s (t002).
- The REST and MCP behavior for the same 7-day search (t004).
- The exact retry count behind the ~90s spinner. Three attempts × ~30s is inferred from `retry-link.ts`, not traced request by request.
