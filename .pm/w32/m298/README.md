# w32 · m298 — Sandbox execution history (`/sandboxes/{id}/execs`), as on Render

**Worker:** worker32 **Goal:** every run and file transfer in a sandbox is recorded and listed on Render's `/sandboxes/{id}/execs` paths with Render's fields **Status:** in progress (t001–t004, t006–t008 done; t005, the production isolation-matrix run, follows the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record executions at token mint — **DONE** | 60m | — |
| t002 | Record completion: exit code and stop time — **DONE** | 60m | t001 |
| t003 | Serve list and get on REST, GraphQL and MCP — **DONE** | 60m | t001 |
| t004 | Retention as on Render — **DONE** | 30m | t002 |
| t005 | Production check and ledger update | 40m | t003, t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- After an exec and an upload on production, `GET /v1/sandboxes/{id}/execs` returns both records, newest first, with Render's required fields and the exit code.
- `GET …/execs/{execId}` returns one record.
- Another member's sandbox answers 404.
- The isolation matrix on production is green with the execs probe.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w32`, 2026-11-09, item 2. Render OpenAPI `GET /sandboxes/{sandboxId}/execs` and `/execs/{execId}`; SDK v1.8.0 ships generated `list_sandbox_executions`/`retrieve_sandbox_execution`. bex mints `exe-…` ids but keeps no record. **This reopens execution history from the ADR018 `—` row "Render sandbox groups, snapshots and executions"**, by the user's choice of item 2 on 2026-11-09 (`/pm them all to w32`); groups stay a non-goal (w7/m150), and snapshots are w32/m285.
- **Goal linkage:** Pillar 5 sandboxes, Render parity (ADR008), and an audit trail of what ran in a tenant sandbox (ADR047 operator need).
- **Expected outcome:** Users and the Render SDK can list what ran in a sandbox and with what exit code, as on Render.
- **Why now:** Every execution minted before the table exists is lost for good, so recording sooner keeps more history; the mint points already carry ids.
- **Render parity:** Included: new REST/GraphQL/MCP read surfaces. Dashboard N/A (bex has no sandbox page).

## Progress — 2026-11-09

- **t001/t002:** migration `0217_sandbox_executions` and `internal/store/sandbox_executions.go`. A record is written at every run and file token mint (`ConnectRun`, `ConnectFile`), and also by a tokenless exec (legacy `POST /exec` as `stream`, MCP `sandbox_exec` as `sync`). A run stops with the exit code read from its own SSE stream (`exitSniffer`); a transfer stops with none. Termination stops what is still open. A store failure is logged and never refuses a command.
- **t003:** REST `GET /v1/sandboxes/{id}/execs` (newest first, `cursor`/`limit`, `{execution, cursor}`) and `GET …/execs/{execId}`, both behind the sandbox's own Get (another member gets 404); `userId` is the caller's `usr-` id. GraphQL uses top-level `sandboxExecutions`/`sandboxExecution` queries instead of a `Sandbox.executions` field, since the sandbox type has no service handle. MCP `list_sandbox_executions`. Both Render operations left the omission inventory, and the scope matrix classes them as reads.
- **t004 retention:** Render documents none (render.com sandbox docs and the API reference, checked 2026-11-09). bex keeps a record while its sandbox is readable, since an unreadable sandbox has no readable history, and the idle sweeper prunes rows with the sandbox, keeping rows under ten minutes old, as for names.
- **t006 Render parity:** the field set, ordering and paging match Render's OpenAPI (default 20, max 100). The REST, GraphQL and MCP reads share `ListExecutions`/`GetExecution`. Dashboard N/A (no sandbox page; `schema-types.ts` regenerated).
- **t008:** `executions_test.go` (mint → stream → exit code → REST list/get; foreign 404; buffered sync; store-outage tolerance; file transfer; sniffer chunking and bounds; rune-safe truncation) and `sandbox_executions_pg_test.go` (ordering, paging, single stop, workspace isolation, prune).
- **Also:** removed m297's redundant create-body cap; bex-api's `BEX_MAX_BODY_BYTES` already bounds every body at 2 MiB.
- **t007 Simplify:** applied: keyset paging through the store's `pageKeyset` (so an unbindable cursor is an empty page); a bounded record write in front of mint and stream; a 30-day retention cap on every record, so a long-lived sandbox's history cannot grow without end; history authorization without `Get`'s rules and names reads; one `decodeExitCode` shared by the sniffer and the buffered reader; the byte-based sniffer; `ConnectOperationStream` reused; GraphQL reading views directly; deduplicated user lookups. Kept: the synchronous stop at stream end (one keyed UPDATE, so a listing right after a stream sees it stopped).
