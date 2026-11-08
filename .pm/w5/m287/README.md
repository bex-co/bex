# w5 · m287 — Sandboxes have names (`--name`), as in Render CLI v2.36

**Worker:** worker5 **Goal:** `render sandboxes create --name`, the NAME column and `list --name` work against bex, with Render's `invalid_sandbox_name` error. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Store and validate a sandbox name | 1h | — |
| t002 | REST, GraphQL and MCP return the name and filter by it | 1h | t001 |
| t003 | CLI checklist, OpenAPI pin and CLI v2.36 walk | 45m | t002 |
| t004 | Render parity | 30m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

Total: **7 tasks, about 4h30m**.

## Definition of done

- With Render CLI v2.36 against bex, `sandboxes create --name foo` succeeds, `sandboxes list` shows a NAME column, and `list --name foo` returns only that sandbox.
- An invalid name answers `invalid_sandbox_name`; REST, GraphQL and MCP all show the name (bex has no Render-sandbox dashboard list; t003 checks whether Render has one).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 5 (2026-11-07); Render CLI v2.36.0 (2026-11-06).
- **Goal linkage:** Render parity first (ADR008); the Render CLI is bex's CLI (DO_NOT_DO: no first-party CLI); pillar 5 reopened 2026-07-27.
- **Expected outcome:** users who upgrade the Render CLI can name sandboxes on bex.
- **Why now:** CLI v2.36 shipped yesterday, and `--name` fails today with `unknown field "name"`.
- **Render parity:** included. This surface is Render's `/sandboxes` API driven by Render's CLI, so render.com is the reference; agent-session sandboxes (Devin reference) are not touched.
