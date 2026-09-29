# w1 · m168 — Honor multiple workspace IDs in REST resource lists

**Worker:** worker1 **Goal:** Render-compatible automation can list matching resources across explicitly selected authorized workspaces. **Status:** done 2026-09-29

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Apply owner arrays and authorize each workspace — **DONE** | 40m | — |
| t002 | Combine service inventories before filtering and pagination — **DONE** | 45m | t001 |
| t003 | Apply multi-owner listing to Postgres and Key Value — **DONE** | 50m | t001 |
| t004 | Stabilize combined ordering deduplication and cursors — **DONE** | 35m | t002, t003 |
| t005 | Render parity — **DONE** | 25m | t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 35m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

## Definition of done

- Services, Postgres and Key Value REST lists honor repeated and comma-separated ownerId arrays, returning the authorized union without duplicate resources.
- Every requested workspace is authorized before any resource is returned; forbidden mixtures and machine-token scope cannot disclose resources.
- Resource filters run before one combined pagination pass. Stable cursor traversal includes equal display names across workspaces and produces no omissions or duplicates.
- Existing omitted-owner behavior and documented single-workspace GraphQL/MCP contracts remain intact; uncertainty in the scoped REST contract follows Render.
- Meaningful handler/authz regressions pass, relevant backend checks are recorded, and ADR018 accurately describes the verified behavior.

## Source + Goal linkage

- **Source:** user-approved w1 brainstorm item 2, 2026-09-28. `apps/rest.go:675`, `postgres/rest.go:163`, and `keyvalue/rest.go:229` read singular `q.Get("ownerId")`; env groups already use `core.QueryList` for owner arrays. Render documents arrays for [services](https://api-docs.render.com/reference/list-services), [Postgres](https://api-docs.render.com/reference/list-postgres), and [Key Value](https://api-docs.render.com/reference/list-key-value).
- **Goal linkage:** ADR008 agent-operated hosting and ADR006 Render-compatible API contracts.
- **Expected outcome:** one explicitly multi-workspace request returns the complete permitted inventory with correct filters and pagination.
- **Why now:** ADR018 List services, List Postgres and List Key Value rows currently claim completion while these handlers consume only one owner value. This is distinct from the completed Postgres suspension-array repair.
- **Parity:** included for a tenant-facing REST fix; audit GraphQL/MCP/dashboard consumers without inventing new workspace dialects. No deliberate non-goal is reopened.
- **Size:** 2h50m implementation; 4h15m including standing closing tasks. New work; no existing milestone is duplicated.
