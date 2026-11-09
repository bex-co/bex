# w32 · m298 — Sandbox execution history (`/sandboxes/{id}/execs`), as on Render

**Worker:** worker32 **Goal:** every run and file transfer in a sandbox is recorded and listed on Render's `/sandboxes/{id}/execs` paths with Render's fields **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Record executions at token mint | 60m | — |
| t002 | Record completion: exit code and stop time | 60m | t001 |
| t003 | Serve list and get on REST, GraphQL and MCP | 60m | t001 |
| t004 | Retention as on Render | 30m | t002 |
| t005 | Production check and ledger update | 40m | t003, t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
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
