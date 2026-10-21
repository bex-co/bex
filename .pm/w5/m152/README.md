# w5 · m152 — Changing a service's runtime works (PATCH `serviceDetails.runtime`)

**Worker:** worker5 **Goal:** changing a service's runtime rebuilds and serves it as on Render, instead of the named refusal w5/m146 added. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Map Render's allowed runtime transitions and their side effects | 45m | — |
| t002 | Update sets the runtime on REST, GraphQL and MCP and rebuilds | 2h | t001 |
| t003 | Dashboard Settings runtime selector | 1h | t002 |
| t004 | Transition tests: node to python and node to docker rebuild and serve | 1h | t002 |
| t005 | Render parity | 30m | t003, t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

Total: **8 tasks, about 6h30m**.

## Definition of done

- PATCHing a node service to `docker` rebuilds it from its Dockerfile and serves it.
- The change is available on REST, GraphQL, MCP and the dashboard.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); w5/m146's named refusal of `serviceDetails.runtime`; no update path sets `Spec.Runtime`.
- **Goal linkage:** ADR008 Render parity first; ADR006/ADR018.
- **Expected outcome:** tenants switch runtimes in place, as on Render.
- **Why now:** the last unsigned refusal from w5/m146.
- **Render parity:** included: REST/GraphQL/MCP and the dashboard.
