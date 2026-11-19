# w5 · m503 — GET /v1/logs answers in a few seconds, not 10–30 s for one read in four

**Worker:** worker5 **Goal:** reading a service's logs over the API takes a few seconds for the query shapes callers actually send, instead of 10–30 s for a quarter of reads and a timeout for the slowest. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Find the slow query shapes: time the LogQL the handler builds against production Loki, per window, type, filter and direction | 1h30m | — |
| [t002](t002.md) | Fix the dominant causes | 2h | t001 |
| [t003](t003.md) | Finer latency buckets, query-shape labels, and a latency alert with its panel | 1h | t002 |
| [t004](t004.md) | Render parity | 15m | t002, t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 15m | t006 |

Total: **7 tasks, about 5h45m**.

## Definition of done

1. The README's Findings record each measured query shape's time against production Loki and the cause of the 10–30 s requests.
2. Over 24 hours of production traffic after the fix deploys, `GET /v1/logs` p95 is under 5 s, measured with buckets at 15, 20 and 25 s, and no request reaches the 28 s REST deadline for the shapes in the Findings (or the remaining shapes are named with their cause and a follow-up item).
3. MCP `list_logs` and REST `GET /v1/logs` still answer the same bytes for the same query (w4/m378), and Render's paging fields (`hasMore`, `nextStartTime`, `nextEndTime`) keep their meaning.
4. A latency alert for the route has a panel, and `bash scripts/obs-coverage-check.sh` passes.
5. Regression tests cover the fix.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5`, round 10 item 3 (2026-11-18), selected with `/pmx them all to w5`.
- **Evidence (production Prometheus, 7 days to 2026-11-19):** 9,096 `GET /v1/logs` requests; 3,916 finished under 1 s and 2,262 took between 10 and 30 s. In the last 24 hours, 20 answered 500. The p99 (28.7 s) sits at the 28 s REST deadline w5/m286 set. The histogram has no bucket between 10 and 30 s, so its p90 and p95 (about 17 s and 24 s) are interpolated. Traffic is bursty (1,037 requests in the three hours from 2026-11-18 15:00Z). CLI telemetry shows `bex logs` runs at p95 1.3 s over 862 runs, so the slow shapes come from other API callers: the mobile app's REST transport (`mobile/src/features/logs/rest-transport.ts`) and verification scripts (`scripts/logs-verify.sh`, `scripts/cli-logs-verify.sh`) use this route.
- **Code:** `lego/backend/internal/logs/loki.go`, `loki_streams.go` (one `query_range` per read), `rest.go`, `render.go` (paging).
- **Goal linkage:** hosting observability on Render's API ([ADR018](../../../docs/ADR018-render-parity.md) Logs rows, [ADR010](../../../docs/ADR010-observability.md)).
- **Expected outcome:** logs reads stop timing out on wide windows, for the mobile app and API clients alike.
- **Why now:** the slowest reads fail at the deadline instead of finishing, and one read in four takes 10–30 s.
- **Render parity:** included (t004). REST, GraphQL and MCP share the logs service, so a change to how it queries Loki must leave all three identical and Render-shaped.
