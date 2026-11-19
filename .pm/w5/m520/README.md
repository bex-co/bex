# w5 · m520 — A text search over a day of a busy service answers in a few seconds: Loki drops the lines that cannot match before bex-api reads them

**Worker:** worker5 **Goal:** a `text` search over a day of the busiest service answers in a few seconds with its whole answer, instead of reading every raw line of the window into bex-api and stopping at the 18 s scan budget with a partial page. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Time candidate prefilters against production Loki and record where the busiest service's lines are | 1h | — |
| [t002](t002.md) | A proven-superset Loki prefilter for lines whose message is the stored line or its prefix (app, build, Postgres, Key Value) | 1h30m | t001 |
| [t003](t003.md) | Request lines: a prefilter that is a superset of `requestLine`'s rendered message, or the request streams read raw with the cost recorded | 1h30m | t002 |
| [t004](t004.md) | Re-time the shapes against production Loki and record before → after | 30m | t003 |
| [t005](t005.md) | Render parity | 15m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout (24 h production reading) | 15m | t007 |

Total: **8 tasks, about 6h**.

## Definition of done

1. A no-match `text` search over a day of the busiest service answers under 5 s against production Loki (t004's timing), and over 24 hours of production traffic after bex-api rolls onto the pin carrying the fix, `bex_api_logs_page_duration_seconds{filter="text"}` p95 is under 5 s (panel 41/42 on `bex-api-gateways`).
2. The prefiltered read returns exactly the matches the Go matcher finds today: tests cover Unicode case folding (U+0130 and U+212A lowercase to ASCII `i` and `k` in Go), invalid UTF-8, escapes, a line over the 64 KiB cap whose message ends in ` …[truncated]`, `textMode=display` over ANSI text, wildcards and `/…/` regexes, and a rendered request line (`responseTimeMS=12` from a stored `Duration` in nanoseconds), on the fake frontend and the pinned Loki (`BEX_TEST_LOKI_URL`).
3. REST `GET /v1/logs`, GraphQL and MCP `list_logs` still answer the same bytes for the same query (w4/m378), and `hasMore`, `nextStartTime` and `nextEndTime` keep their meaning.
4. `BexApiLogsReadSlow` stays quiet over the same day of production traffic.
5. The Findings below record each timed shape before and after, and which term shapes still read raw lines and why.

## Source + Goal linkage

- **Source:** inbox note [w5/198](../done/198.md), promoted 2026-11-19 by `/loopx w5`. It came from w5/m503 t002's Findings ([m503](../blocked/m503/README.md), cause 3): after m503 a `text` search reads every raw line once, about 0.4 to 0.5 s of Loki time and 12 to 14 MB to decode per hour of the busiest service (about 20,000 request lines an hour), so a day's or a week's search covers about four hours before the budget and answers a partial page with `hasMore`.
- **Code:** `lego/backend/internal/logs/loki.go` (`NewLokiSource`, `lokiPipelineFor`, `scanLokiWindow`, `textScanPage`, `parseLokiStreams`, `requestLine`), `service.go` (`localTextSearch`, `matchesText`, `normalizeLogText`, `capLogMessage`), `filterpattern.go` (`textMatcher`), `display.go` (`displayText`); timing harness `loki_timing_test.go` (build tag `lokitiming`, targets from the environment, never committed).
- **Goal linkage:** hosting observability on Render's API ([ADR018](../../../docs/ADR018-render-parity.md) Logs rows, [ADR010](../../../docs/ADR010-observability.md)): Render's `text` filter answers a search over the window the caller names.
- **Expected outcome:** a search for a term over a day of a busy service comes back whole in a few seconds on REST, GraphQL, MCP, the CLI and the dashboard, instead of an 18 s partial page the caller has to page through.
- **Why now:** it is the shape m503 left above 5 s, and m503's 24 h reading (t007) names it. Since w5/197 (`e0dc599d3`) a search with no `startTime` covers one hour (about 1 s), so the slow reads are the ones that name a day or more.
- **Why the match stays in Go:** Loki's case-insensitive regexp simplifier is not a safe Unicode superset of the API's literal match (w4/m189, w8/m58), a request line's message is rendered from its stored JSON, and display mode matches text with ANSI escapes removed. A prefilter only drops lines that provably cannot match; Go's matcher still decides every line Loki returns.
- **Render parity:** included (t005). `text` is a Render filter on REST, GraphQL and MCP, and the CLI and dashboard search through them.

## Findings

_None yet._
