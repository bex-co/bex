# w5 · m61 — Datastore Logs panels (Postgres + Key Value)

**Worker:** worker5 **Goal:** the Databases and Key Value detail pages gain a Logs panel consuming the existing, tested but dashboard-unconsumed `databaseLogs` / `keyValueLogs` GraphQL reads — matching Render's datastore Logs tabs — with the honest store-unavailable state when the log store is unconfigured. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Read-shape confirm (filters/paging/store-gating of the two reads) + panel design pick | 30m | —          |
| t002 | Shared datastore-logs panel component (query, range, store-unavailable state, en/zh)  | 75m | t001       |
| t003 | Wire onto the Databases detail page + the Key Value detail page                       | 45m | t002       |
| t004 | Live dev-5 proof (real CNPG/Valkey log lines) + Render-capture comparison evidence    | 45m | t003       |
| t005 | Render parity: panel vs Render's datastore Logs tabs + cross-surface consistency     | 30m | t004       |
| t006 | Simplify (`/simplify` over the milestone's diff)                                      | 20m | t005       |
| t007 | Test coverage: rendering, range, store-unavailable, error and empty states            | 45m | t005       |
| t008 | Closeout                                                                              | 15m | t007       |

## Definition of done

On a live bex dashboard, the Databases detail page and the Key Value detail page each show a Logs panel rendering real instance log lines (CNPG Postgres / Valkey) over a selectable time range, sourced from `databaseLogs` / `keyValueLogs`. A deployment without the log store shows an explicit store-unavailable state (the Logs-tab 503 pattern) — never a silent blank. The panel's layout and affordances are compared against Render's datastore Logs tabs with drift recorded, and the UI's semantics match the reads' REST/MCP counterparts where those exist. `cd dashboard && yarn typecheck && yarn lint && yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w5` round 2, 2026-07-30 (proposal 3; renumbered from the proposal's m62 to next-free m61 — the sandboxes proposal was not materialized). Found by the same capability-diff scan as m60: `databaseLogs` and `keyValueLogs` are schema root reads implemented and tested in `lego/backend/internal/postgres/graphql.go` (+ `logs_test.go`) and `lego/backend/internal/keyvalue/graphql.go` (+ `logs_test.go`) with zero dashboard consumers.
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md`) — Render's Postgres and Key Value pages both carry a Logs tab; bex's datastore pages ship metrics, SQL console, recovery, and networking but no logs.
- **Expected outcome:** datastore debugging (connection errors, slow queries surfaced in server logs, Valkey warnings) works from the dashboard, matching Render, without kubectl.
- **Why now:** the backend half already exists and is tested — this is pure consumption; shipping it now also pre-empts a future datastore-family walk re-discovering the gap at higher total cost.
- **Render parity:** included (t005) — a tenant-facing UI surface with existing machine-surface counterparts to stay consistent with.
