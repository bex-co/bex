# w5 · m286 — REST and MCP requests have a deadline instead of a 502 that keeps running

**Worker:** worker5 **Goal:** every REST `/v1/*` and `/mcp` request either finishes or returns a coded timeout error before the server's 30 s write timeout, and its handler context is cancelled. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A request-deadline middleware with a coded timeout error | 1h30m | — |
| t002 | Mount it on REST and MCP with a guarded exemption list | 45m | t001 |
| t003 | Mutations stop at the deadline instead of finishing after the client gave up | 1h | t002 |
| t004 | Prove it on dev-5 with a slow handler | 30m | t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

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
