# w5 · m520 — A text search over a day of a busy service answers in a few seconds: Loki drops the lines that cannot match before bex-api reads them

**Worker:** worker5 **Goal:** a `text` search over a day of the busiest service answers in a few seconds with its whole answer, instead of reading every raw line of the window into bex-api and stopping at the 18 s scan budget with a partial page. **Status:** blocked (t001 to t007 done and shipped in `c3d4c075e`; t008 reads 24 h of production text searches once bex-api runs it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Time candidate prefilters against production Loki and record where the busiest service's lines are — **DONE** | 1h | — |
| [t002](done/t002.md) | A proven-superset Loki prefilter for lines whose message is the stored line or its prefix (app, build, Postgres, Key Value) — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Request lines: a prefilter that is a superset of `requestLine`'s rendered message, or the request streams read raw with the cost recorded — **DONE** | 1h30m | t002 |
| [t004](done/t004.md) | Re-time the shapes against production Loki and record before → after — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](t008.md) | Closeout (24 h production reading) | 15m | t007 |

Total: **8 tasks, about 6h**.

## Definition of done

1. A no-match `text` search over a day of the busiest service answers under 5 s against production Loki (t004's timing), and over 24 hours of production traffic after bex-api rolls onto the pin carrying the fix, `bex_api_logs_page_duration_seconds{filter="text"}` p95 is under 5 s (panel 41/42 on `bex-api-gateways`).
2. The prefiltered read returns exactly the matches the Go matcher finds today: tests cover Unicode case folding (U+0130 and U+212A lowercase to ASCII `i` and `k` in Go), invalid UTF-8, escapes, a line over the 64 KiB cap whose message ends in ` …[truncated]`, `textMode=display` over ANSI text, wildcards and `/…/` regexes, and a rendered request line (`responseTimeMS=12` from a stored `Duration` in nanoseconds), on the fake frontend and the pinned Loki (`BEX_TEST_LOKI_URL`).
3. REST `GET /v1/logs`, GraphQL and MCP `list_logs` still answer the same bytes for the same query (w4/m378), and `hasMore`, `nextStartTime` and `nextEndTime` keep their meaning.
4. `BexApiLogsReadSlow` stays quiet over the same day of production traffic.
5. The Findings below record each timed shape before and after, and which term shapes still read raw lines and why.

## Source + Goal linkage

- **Source:** inbox note [w5/198](../../done/198.md), promoted 2026-11-19 by `/loopx w5`. It came from w5/m503 t002's Findings ([m503](../../done/m503/README.md), cause 3): after m503 a `text` search reads every raw line once, about 0.4 to 0.5 s of Loki time and 12 to 14 MB to decode per hour of the busiest service (about 20,000 request lines an hour), so a day's or a week's search covers about four hours before the budget and answers a partial page with `hasMore`.
- **Code:** `lego/backend/internal/logs/loki.go` (`NewLokiSource`, `lokiPipelineFor`, `scanLokiWindow`, `textScanPage`, `parseLokiStreams`, `requestLine`), `service.go` (`localTextSearch`, `matchesText`, `normalizeLogText`, `capLogMessage`), `filterpattern.go` (`textMatcher`), `display.go` (`displayText`); timing harness `loki_timing_test.go` (build tag `lokitiming`, targets from the environment, never committed).
- **Goal linkage:** hosting observability on Render's API ([ADR018](../../../../docs/ADR018-render-parity.md) Logs rows, [ADR010](../../../../docs/ADR010-observability.md)): Render's `text` filter answers a search over the window the caller names.
- **Expected outcome:** a search for a term over a day of a busy service comes back whole in a few seconds on REST, GraphQL, MCP, the CLI and the dashboard, instead of an 18 s partial page the caller has to page through.
- **Why now:** it is the shape m503 left above 5 s, and m503's 24 h reading (t007) names it. Since w5/197 (`e0dc599d3`) a search with no `startTime` covers one hour (about 1 s), so the slow reads are the ones that name a day or more.
- **Why the match stays in Go:** Loki's case-insensitive regexp simplifier is not a safe Unicode superset of the API's literal match (w4/m189, w8/m58), a request line's message is rendered from its stored JSON, and display mode matches text with ANSI escapes removed. A prefilter only drops lines that provably cannot match; Go's matcher still decides every line Loki returns.
- **Render parity:** included (t005). `text` is a Render filter on REST, GraphQL and MCP, and the CLI and dashboard search through them.

## Findings

### 2026-11-19 — t001: where the lines are, and what a filter costs (loopx w5, read-only)

Timed against production Loki (3.6.7, the same build as the pinned test image) through `kubectl port-forward`, with `TestLokiTimingPrefilter` in `lego/backend/internal/logs/loki_timing_test.go` (build tag `lokitiming`; targets from the environment, never committed). Compare requests, Loki's own `execTime` and MB: the port-forward carries 3–5 MB/s, so its wall time overstates the cluster's.

**The busiest service's day is request lines.** `count_over_time` and `bytes_over_time` over 24 hours: 387,535 request lines and 306.5 MB (about 790 bytes each, Traefik's JSON record with 17 fields), against 657 app lines and 1.2 MB. Its selector holds no build, Postgres or Key Value streams. Reading every line of the day in Loki alone is about 2 s of Loki time (a `count_over_time` over the day took 2.07 s).

**One `query_range` of a day, limit 1000, backward, by candidate filter** (Loki s; a no-match term and the common `mozilla`):

| Candidate | request, no match | all types, no match | request, common |
| --- | --- | --- | --- |
| `\|= "term"` (case-sensitive, not a superset) | 1.42 | 1.35 | 2.77 (no line: the case differs) |
| `\|~ "(?i)term"`, which Loki answers with its own lowercase compare | 2.11 | 2.29 | 0.70 (1000 lines) |
| `\|~` with `i` and `k` as classes (`[iI\x{0130}]`), which keeps Loki on Go's regexp | 13.24 | 13.74 | 1.21 (1000 lines) |
| `\| json \| line_format` rendering the request line, then `(?i)` | 5.91 | 6.11 | 5.95 (1000 lines) |

Over one hour every candidate but the classes (0.46 s) and the render (0.25 s) is under 0.05 s. Unfiltered, the day is about 290 MB of answers to decode (m503: 12 MB and 0.4 s of Loki time an hour).

**Chosen: `|~ "(?i)<needle>"`.** Loki turns a regex that parses to a literal into its own substring compare (`containsLower` in `pkg/logql/log/filter.go`), six times cheaper than Go's regexp on this volume. That compare lowercases U+0130 and U+212A except as the needle's first character (the pinned Loki misses `\u212aill` for `(?i)kill`), and Go's `(?i)i` never matches U+0130, which `strings.ToLower` turns into `i`. So a needle holds no `i` and does not start with `k`: then it keeps every line Go's matcher can accept under either engine. The LogQL render was dropped: three times the cost and no proof that it equals `requestLine`.

### 2026-11-19 — t002 to t007 (loopx w5)

- **t002 and t003, the prefilter (`lego/backend/internal/logs/textprefilter.go`).** Each term gives one needle, the longest run of it that every line it can match must hold, and the read sends `|~ "(?i)<needle>" or …` ahead of the path and host stages; Go's matcher still decides every line Loki returns, and the raw page, its saturation and edge rules, are unchanged. A run is printable ASCII with no `i`, and loses a leading `k`. A run inside the cap marker (` …[truncated]`) gives no needle, and a trailing space is dropped (it may span the cut). When the read can return request lines, a run also holds no space, `=`, `"`, `\`, `<`, `>` or `&`, is not inside a rendered key name (`clientIP`, `requestID`, `responseTimeMS`, `responseBytes`, `userAgent`) and is not a number, and loses any leading text that can be the tail of an escape `strconv.Quote` writes (`\x07`, `\u00a0`): what is left lies inside one quoted value, which Traefik's Go encoding/json stores as written. `textMode=display` is prefiltered too, keeping every line with an ESC or a CR: `displayText` returns any other message unchanged.
- **What still reads raw lines:** a `/…/` regex term, and a term with no needle (one made of `i`, `k` and characters outside the runs, one inside the cap marker, and, with request lines, a key name, a number, or a run of up to eight hex digits). With several terms, one such term reads the whole window raw.
- **t005, Render parity.** REST, GraphQL and MCP share `queryLogPage` and `NewLokiSource`: one prefiltered search answers REST's bytes on MCP and the same lines, `hasMore` and cursors on GraphQL (`TestAPrefilteredSearchIsOneAnswerOnRESTGraphQLAndMCP`), and a cursor walk pages exactly as an unfiltered one. A search the budget still cuts (a week of the busiest service, below) pages on with `hasMore` and the cursor at the scan edge, as before. No drift found.
- **t006, `/simplify`:** `quoteEscapeTail` branches on the first character instead of taking maxima, and a test holds `requestLineKeys` to the keys `requestLine` writes. Nothing else.
- **t007, tests (`textprefilter_test.go`).** Two fuzz targets (stored lines on app, build, Postgres and Key Value streams; access records and request lines that are not records) check that every line Go's matcher accepts passes the filter under Loki's compare (a port of `containsLower`) and under Go's regexp, with seeds for U+0130 and U+212A, invalid UTF-8, a cut at the cap and the marker, ANSI and CR in display mode, wildcards, regexes, `responseTimeMS=12` from 12,345,678 ns, a response under 1 ms, a user agent with `"`, `\`, NBSP and `&`, a record with no fields and a line that is not one; fuzzed for 3.5 minutes without a failure. On the fake frontend, now able to apply the filter, a cursor walk across split cuts with saturated raw pages gives the same pages, `hasMore` and cursors filtered as unfiltered. On the pinned Loki, 126 searches over that corpus return exactly what Go finds in the unfiltered read. Each rule's removal fails them: a leading `k`, an `i`, a key name, escape tails, request rules off, the marker, a trailing space, display without ESC and CR, and JSON-escaped characters (the number rule is also covered by the escape-tail rule, so no test can tell it is gone).

### 2026-11-19 — t004: before → after on production Loki

`TestLokiTiming`'s text shapes with a term neither service logs, before (this tree with the prefilter switched off) and after, as requests / Loki s / MB / wall s through the port-forward (lines); `partial` is a page the 18 s budget cut, with how far it reached:

| Service | Shape | Before | After |
| --- | --- | --- | --- |
| busy | 1 h, all types, backward | 22 / 0.56 / 17.3 / 8.5 (0) | 2 / 0.09 / 0.0 / 0.8 (0) |
| busy | 1 h, request, forward | 19 / 0.49 / 15.6 / 4.6 (0) | 2 / 0.05 / 0.0 / 0.4 (0) |
| busy | 24 h, all types, backward | 68 / 3.16 / 53.4 / 18 (partial, 3 h 28 m) | 5 / 2.27 / 0.0 / 3.3 (0) |
| busy | 24 h, request, backward | 24 / 0.68 / 18.9 / 18 (partial, 1 h 4 m) | 5 / 2.03 / 0.0 / 3.1 (0) |
| busy | 24 h, all types, forward | 72 / 1.85 / 58.9 / 18 (partial, 20 h 22 m) | 5 / 2.39 / 0.0 / 3.3 (0) |
| busy | 7 days, all types, backward | 68 / 1.86 / 53.6 / 18 (partial, 3 h 30 m) | 8 / 11.67 / 0.0 / 18 (partial, 127 h) |
| idle | 24 h, all types, backward | 77 / 0.77 / 4.8 / 18 (partial, 17 h 55 m) | 5 / 0.33 / 0.0 / 1.3 (0) |
| idle | 24 h, all types, forward | 91 / 0.90 / 6.0 / 18 (partial, 2 h 28 m) | 5 / 0.61 / 0.0 / 1.7 (0) |

A day's no-match search on the busiest service is now 5 requests and 2.0–2.4 s of Loki time with its whole answer, the floor being Loki reading the day's lines at all. A common term (`mozilla`, needle `moz`) still fills the busiest service's page in 2 requests and 1.8 MB, as before (its first raw page already held the page), at 0.1–1.3 s of Loki time; a day of it on the idle service went from 46 requests, 2.8 MB and 9 s through the port-forward to 4 requests, 0.2 MB and 1 s. Left above 5 s: a no-match search over a week of the busiest service, about 2.3 s of Loki time a day, which covers 5 of its 7 days before the budget (it covered 3.5 hours).
