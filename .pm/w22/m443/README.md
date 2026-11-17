# w22 · m443 — Sandbox API rate limits match Render's per-operation table

**Worker:** worker22 **Goal:** a user's sandbox calls are rate limited per operation class at Render's documented rates (create 100, terminate 1,000, exec 1,000, file transfer 1,000, reads 400, group writes 100, snapshot capture 10 per minute), per user and shared across that user's API keys, on REST, GraphQL and MCP, instead of sharing bex-api's general 500 per minute. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                   | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Design record: Render's rate table in ADR104 D7 and ADR018, which route belongs to which class, and what the bucket key is              | 40m | —                |
| t002 | A per-user, per-class limiter on `core.KeyedRateLimiter`; sandbox routes stop spending the general 500 per minute                       | 60m | t001             |
| t003 | REST: the sandbox and sandbox-group routes and the run and file connect-token mints take their class's bucket                           | 50m | t002             |
| t004 | GraphQL and MCP sandbox operations use the same buckets; the 429 body and `Retry-After` match the existing limiter's                    | 40m | t002             |
| t005 | The snapshot-capture class (10 per minute) lands as a constant with tests, ready for w32/m285's capture route                           | 20m | t002             |
| t006 | Production check: a matrix leg where the 101st create in a minute answers 429 and 600 execs in a minute are all accepted                | 45m | t003, t004       |
| t007 | Render parity                                                                                                                           | 20m | t003, t004, t005 |
| t008 | Simplify                                                                                                                                | 20m | t007             |
| t009 | Test coverage                                                                                                                           | 30m | t007             |
| t010 | Closeout                                                                                                                                | 10m | t006, t008, t009 |

## Definition of done

- One user, using two API keys they created, gets 429 on the 101st sandbox create inside a minute; the first 100 succeed (subject to the group's concurrency limit, which is a different 429).
- The same user runs 600 execs inside a minute and none is refused by the rate limiter. Today the 501st request of any kind is refused (`BEX_RATE_LIMIT` default `500`).
- Each class in Render's table has its own bucket at Render's rate, and a call spends exactly one class. REST, GraphQL and MCP calls for the same operation spend the same bucket.
- A sandbox call no longer spends the general per-subject budget, and a non-sandbox call never spends a sandbox bucket.
- The 429 a sandbox bucket answers has the same body shape and `Retry-After` header as the existing limiter's 429.
- ADR104 D7 and ADR018 each carry the row, citing render.com/docs/sandboxes-limits with the read date; the ADR018 cells are ✅ once t006 passes.
- Every class has an automated test, and the production check is recorded in this README with the observed codes.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w22` (2026-11-16), item 1, filed by `/pmx 1,3,4,5 to w22`.
  - **Render** ([render.com/docs/sandboxes-limits](https://render.com/docs/sandboxes-limits), read 2026-11-16, public beta, "limits are subject to change"): "These limits apply per user, shared across that user's API keys. Requests over a limit fail with HTTP `429`." Requests per minute: create a sandbox 100; terminate a sandbox 1,000; execute a command 1,000; upload or download a file or directory 1,000; read sandbox and sandbox group resources 400; write sandbox group resources 100; capture a snapshot 10. Single-vendor documentation; bex has no Render account to measure it.
  - **bex today:** one general limiter for every operation. `lego/backend/cmd/api/config.go` (~line 770) reads `BEX_RATE_LIMIT` with default `500` per minute; `lego/backend/internal/api/ratelimit.go` keys a bucket by the authenticated subject and, when wired, a second aggregate bucket by workspace; `lego/backend/cmd/api/main.go` (~line 1882) builds it. `lego/backend/internal/sandbox/` has no per-operation limit. So exec, file transfer and terminate are stricter than Render (500 against 1,000) and create and group writes are looser (500 against 100).
  - Neither ADR104 nor ADR018 has a row for this table.
- **Goal linkage:** ADR008 § Render parity first; ADR104 D7 (limits) and D11 (abuse bounds).
- **Expected outcome:** an agent workload sized to Render's documented rates is not refused early on bex, and a create storm is refused where Render refuses it.
- **Why now:** w32/m285 (snapshots) is about to start and ADR104 D4 requires a capture rate limit, which t005 supplies. bex-api's limiter is in-process and single-replica today, so adding classes is simplest before it scales out.
- **Open recommendations, not user decisions** (the tasks implement them under the parity rule; t001 records them in ADR104 as recommendations):
  - Sandbox routes are counted only against Render's per-user table and leave the general 500 per minute, because otherwise 1,000 execs a minute cannot be reached.
  - The existing workspace aggregate bucket stays as an ADR104 D11 abuse bound for sandbox calls, sized above the sum a single user can reach under Render's table so it never trips first. It is a bex-owned control, not a Render fact.
  - "Per user" maps to the subject that owns the API key, so every key a user created shares the bucket. t001 confirms how the existing limiter resolves an API key to its creator.
- **Render parity included:** the limits are customer-facing on REST, GraphQL and MCP, so t007 compares each surface with Render's table.
