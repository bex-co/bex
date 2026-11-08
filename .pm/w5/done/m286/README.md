# w5 · m286 — REST and MCP requests have a deadline instead of a 502 that keeps running

**Worker:** worker5 **Goal:** every REST `/v1/*` and `/mcp` request either finishes or returns a coded timeout error before the server's 30 s write timeout, and its handler context is cancelled. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A request-deadline middleware with a coded timeout error — **DONE** | 1h30m | — |
| t002 | Mount it on REST and MCP with a guarded exemption list — **DONE** | 45m | t001 |
| t003 | Mutations stop at the deadline instead of finishing after the client gave up — **DONE** | 1h | t002 |
| t004 | Prove it on dev-5 with a slow handler — **DONE** | 30m | t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 5h30m**.

## Definition of done

- A REST or MCP request that would run past the budget returns a coded timeout error (with CORS headers) before 30 s, and its handler context is cancelled; no 502 from the write timeout on these routes.
- Streaming, upload, log-tail routes and `POST /v1/webhooks/git` are exempt through an explicit list guarded by a test.
- The main mutation paths stop on cancellation or are idempotent on retry.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 4 (2026-11-07); w3/m248 findings (handlers running 3 600 s, ending 502 at the 30 s write timeout).
- **Goal linkage:** basic obs and reliable operation (V0 roadmap 2); Render CLI/API clients (ADR006).
- **Expected outcome:** slow REST/MCP calls fail fast with a clear, retryable error instead of a 502 while the change keeps running and may apply twice on retry.
- **Why now:** w3/m140 and m211 fixed this for GraphQL only; REST and MCP still have no deadline. The git webhook stays with w3/m248, which owns its acknowledgement.
- **Render parity:** included — the change alters REST/MCP error responses, which must keep Render's error envelope.

## Findings and closeout (2026-11-08, loopx w5)

- **t001 budget:** `core.RequestBudget` is 28 s, the shared bound. A test holds it below `api.HTTPWriteTimeout` (30 s) and above `wait_agent_sessions`' 25 s wait, so that wait still gets to answer `timedOut` itself. It is 3 s longer than GraphQL's 25 s `gqlExecTimeout` for that reason.
  - `core.WriteErr` maps any non-coded `context.DeadlineExceeded` to 503 `REQUEST_TIMEOUT` in Render's `{id, message}` envelope (plus bex's `error` and `code`), with a fixed message: "the request did not finish in time and was stopped; a change it was making may still have been applied, so check before retrying". `core.MCPError` does the same. The dependency's wrapped text, which can name internal hosts, is logged and never shown.
  - A timeout that is a coded refusal keeps its own code.
- **t002 mount:**
  - **REST:** `Server.withRequestBudget` wraps the REST validator innermost, so the time spent in auth is not counted. It exempts, through an explicit list, the routes that already extend their own write deadline or stream: `GET /v1/logs/subscribe`, `/v1/sandboxes/` (exec streams, long resume waits) and `/v1/agent-attachments`. The platform rebuild hook is mounted outside `/v1`.
  - **MCP:** `mcputil.AddTool` bounds every tool call, which covers `AddExactJSONTool` too.
  - **Git webhook:** `POST /v1/webhooks/git` mounts outside the REST chain and stays with `w3/blocked/m248`.
- **t003 mutations:** every write path runs on the request context (pgx, controller-runtime), so the budget stops it. The create paths' own rollback and cleanup deliberately use `context.WithoutCancel` (`apps/service.go:2668-2973`), so a timed-out create still undoes its partial resources. The timeout message tells the caller that a commit may have landed before the stop.
- **t004 proof:** proven in-process rather than on dev-5 (the CAPD cluster's certificate mismatch is noted in m283). `TestRequestBudgetStopsASlowRESTHandler` shows a handler blocked on its dependency stopped at the budget with a coded 503 and no leaked host. `TestAddToolBoundsTheCallWithTheRequestBudget` covers the same for an MCP tool. A production slow path is not induced on purpose.
- **t005 Render parity:** the timeout keeps Render's error envelope on REST and the MCP tool-error shape. GraphQL already had its own budget.
- **t006 Simplify:** a small diff with one shared constant, one `timedOut` predicate shared by REST and MCP, and one exemption table. Nothing to reduce.
- **t007 tests:** in `api` (budget stop, exemption table, constant ordering), `core` (`TestMCPErrorReportsATimeout`) and `mcputil`.
- **DoD:** all three bullets hold, by the tests above.
