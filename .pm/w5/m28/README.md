# w5 · m28 — Dashboard SQL console for managed Postgres

**Worker:** worker5 **Goal:** let a dashboard user run SQL queries against a managed Postgres from its detail page, matching the MCP `query_render_postgres` tool's capability **Status:** todo

## Tasks (in order)

| id   | title                                                                                                | est  | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Backend: scoped read/write SQL-execution endpoint for a named Database (reuse the connection-resolution logic behind `query_render_postgres`, gated by the same authz as the Databases page) | 1.5h | —          |
| t002 | Dashboard: SQL console page/panel on the Database detail view (editor + run button + results table)      | 2h   | t001       |
| t003 | Query history / result pagination for large result sets                                                  | 1h   | t002       |
| t004 | Guard rails: statement timeout, row-limit cap, and a confirmation step for non-SELECT statements          | 1h   | t001       |
| t005 | Render parity: verify query semantics/limits consistent with the MCP tool                                | 30m  | t003, t004 |
| t006 | Simplify                                                                                                   | 30m  | t005       |
| t007 | Test coverage                                                                                              | 1h   | t005       |
| t008 | Closeout                                                                                                   | 15m  | t007       |

## Definition of done

From a Database's dashboard page, a user can write and run a SQL query against it and see tabular results, with the same connection scoping/authz as the existing Databases page.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 (second pass) — `docs/ADR018-render-parity.md` "Read-only SQL query" row, UI cell (`✖`, "Dashboard SQL console: none (low)").
- **Goal linkage:** closes the dashboard-only gap on an already-MCP-complete capability.
- **Expected outcome:** users who don't use an MCP client (Claude Code/Cursor) get the same query capability from the dashboard.
- **Why now:** self-contained, no backend architecture changes — reuses existing connection/authz plumbing. Render parity included — dashboard should match the MCP tool's query semantics/limits.
