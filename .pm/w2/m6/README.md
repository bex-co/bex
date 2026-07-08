# w2 · m6 — MCP `query_render_postgres` (read-only SQL for agents)

**Worker:** worker2 **Goal:** Un-defer the one Render MCP Postgres tool bex omitted — run a read-only SQL query against a managed database from the MCP surface, with hard safety rails (read-only transaction, statement timeout, row cap). **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Query verb in the postgres feature: internal conn string, read-only enforcement, statement timeout, row cap        | 30m | —          |
| t002 | MCP tool under Render's exact name `query_render_postgres` (MCP-only, matching Render — no REST/GraphQL)            | 20m | t001       |
| t003 | Safety acceptance: writes rejected, timeout enforced, unknown db → not-found, values never logged                   | 20m | t001       |
| t004 | Simplify — `/simplify` over the code this milestone changed                                                          | 20m | t002, t003 |
| t005 | Test coverage — meaningful tests for the behavior this milestone shipped                                             | 30m | t002, t003 |

## Definition of done

An MCP agent runs `SELECT`s against a managed database via `query_render_postgres` (name/args identical to Render's official tool) and gets rows back; any write attempt (`INSERT`/`UPDATE`/`DDL`) fails inside a read-only transaction; long queries hit the statement timeout; results are row-capped; query text/values never appear in logs. The tool is MCP-only, exactly like Render.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-08; `docs/bex-api.md` ("Render's `query_render_postgres` … omitted, not faked — a deferred capability"); `render-oss/render-mcp-server`.
- **Goal linkage:** pillar 3 (agents operate bex natively) — inspecting the database is a top agent debugging move.
- **Expected outcome:** the last Render-official Postgres MCP tool exists; Render-trained agents can debug their data on bex without connection-string plumbing.
- **Why now:** the blocker cited at deferral time (live in-cluster connectivity from the API layer) no longer holds — `connection-info` already reads CNPG secrets and bex-api runs in-cluster next to the databases. Small, independent, and closes a named ✖ before w1/m13's parity matrix files it.
