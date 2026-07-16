# w3 · m30 — Datastore logs completion: Key Value coverage + Postgres-logs surface consistency

**Worker:** worker3 **Goal:** Key Value (Valkey) stores get the same logs story Postgres got in m28 — durable, queryable over REST/GraphQL/MCP, visible in a dashboard Logs tab — and the two verified consistency escapes in the day-old Postgres-logs surface (GraphQL dropping `labels`/`instance`; stale "no durable store" MCP description) are fixed. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Capture Render's Key Value logs contract into `docs/render-artifacts/`                                            | 30m | —                |
| t002 | Ship Valkey pod streams via Alloy under immutable keyvalue labels (CNPG marker pattern)                            | 45m | t001             |
| t003 | Backend core: route KeyValue resources through the shared durable log query with `AuthorizeKeyValue`              | 60m | t002             |
| t004 | REST `resource=` support + GraphQL query + MCP tool, with the honest store-off fallback                            | 45m | t003             |
| t005 | Dashboard Logs tab on keyvalue detail, cloned from `postgres-log-viewer.tsx`                                       | 45m | t004             |
| t006 | Postgres-logs consistency: `instance`/`type` on `databaseLogs` GraphQL; refresh stale "live only" descriptions     | 30m | —                |
| t007 | Render parity — same fields/semantics/errors across REST/GraphQL/MCP/UI vs Render's Key Value logs                 | 30m | t004, t005, t006 |
| t008 | Simplify — `/simplify` over the milestone's diff                                                                   | 20m | t007             |
| t009 | Test coverage — meaningful tests for the shipped behavior                                                          | 30m | t007             |
| t010 | Closeout — move to `done/` when the DoD holds                                                                      | 15m | t009             |

## Definition of done

A live Valkey store's logs are queryable over REST (`resource=` its typed id), GraphQL, and MCP, and visible in the dashboard keyvalue Logs tab with range/instance/search controls and honest empty/403/503 states; with Loki wired, lines survive a pod restart; with Loki unset, the fallback is honestly degraded (never silently empty). `databaseLogs` GraphQL exposes `instance`/`type` derived from labels (matching REST/MCP), and no shipped description or comment still claims Postgres logs are live-only.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 21, 2026-07-15 — dashboard-gap mine G1 (Render's List-logs API covers Key Value per `docs/render-artifacts/postgres-logs.md`, bex has no Valkey log path at all: no backend route, no shipper stream, no UI) + shipped-diff mine findings 2–3 over `4627caaf..65a979b6` (`postgres/graphql.go` log-entry type drops the `labels`/`instance` REST and MCP return — the `instance` filter arg exists but a consumer can't see which CNPG pod produced a line; `postgres/mcp.go` description still claims "lines do not survive restarts" after `6f1bbaa7` shipped durability).
- **Goal linkage:** `GOAL.md` #2 (observability) + the Render parity ledger's logs coverage (`docs/ADR018-render-parity.md`); ADR010's one-core-three-adapters rule.
- **Expected outcome:** the datastore-logs story is symmetric across Postgres and Key Value, matching Render's documented coverage; no cross-surface field drift in the day-old Postgres surface.
- **Why now:** the entire Postgres-logs mechanism (w3/m28 — shared durable core, Alloy shipping pattern, viewer component) shipped yesterday; cloning it for Valkey is the cheapest it will ever be, and w3's queue is empty.
- **Render parity:** included (t007) — feature dev touching REST/GraphQL/MCP/UI.
