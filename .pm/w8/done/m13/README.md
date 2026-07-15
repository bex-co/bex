# w8 · m13 — Datastore list pagination: Postgres + Key Value

**Worker:** worker8 **Goal:** `GET /v1/postgres` and `GET /v1/key-value` page with Render's cursor/limit envelope exactly like the services list already does, removing the last list-endpoint asymmetry. **Status:** done

## Tasks (in order)

| id   | title                                              | est | depends_on |
| ---- | --------------------------------------------------- | --- | ---------- |
| t001 | REST paging on both datastore lists                  | 40m | —          | — **DONE** |
| t002 | GraphQL envelope parity                               | 30m | t001       | — **DONE** |
| t003 | MCP args checked against Render's tools               | 30m | t001       | — **DONE** |
| t004 | Render parity                                        | 30m | t002, t003 | — **DONE** |
| t005 | Simplify                                             | 30m | t004       | — **DONE** |
| t006 | Test coverage                                        | 40m | t004       | — **DONE** |
| t007 | Closeout                                             | 15m | t006       | — **DONE** |

## Definition of done

Both datastore list endpoints accept `cursor`/`limit` and return the `{object, cursor}` per-item envelope with the same semantics as `GET /v1/services` (`core.PageParams`/`core.Page`); omitted params keep today's full-list behavior byte-identical; GraphQL and MCP match their Render counterparts' paging vocabulary; paging visits every instance exactly once.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 3, 2026-07-14 (item 3); code fact: `core.PageParams` appears in `apps/rest.go:309` but nowhere in `postgres/rest.go` or `keyvalue/rest.go` — Render's `GET /postgres` and `GET /key-value` both page.
- **Goal linkage:** Render parity (API contract consistency across resource lists).
- **Expected outcome:** a client paging services can page datastores with the same code; the asymmetry disappears from the ledger's datastore rows.
- **Why now:** mechanical while the `core.Page` helper's conventions are established and fresh; w8 datastore-family placement per the m12 precedent. Render parity task included — API surface change (no UI change expected: the dashboard lists are workspace-scoped and small; note if that assumption breaks).
