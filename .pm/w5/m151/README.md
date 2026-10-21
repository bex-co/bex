# w5 · m151 — Render edge caching: `serviceDetails.cache.profile`

**Worker:** worker5 **Goal:** a web service's edge cache profile works as on Render, replacing the `—` row w5/m146 recorded without user sign-off. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Accept and read back serviceDetails.cache.profile on every surface | 1h30m | — |
| t002 | Design the edge cache: profile semantics, scope and purge | 1h | — |
| t003 | Implement the edge cache behind the profile | 2h | t002 |
| t004 | Dashboard control and the ADR018 row | 45m | t001, t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

Total: **8 tasks, about 7h**.

## Definition of done

- An `origin-controlled` service serves its second request for a `Cache-Control: max-age` asset from the edge, a deploy purges it, and a `no-cache` service is never cached.
- The profile round-trips on REST, GraphQL, MCP and the dashboard.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); w5/m146 refused `serviceDetails.cache` and added an ADR018 `—` row without sign-off.
- **Goal linkage:** ADR008 Render parity first; ADR006/ADR018.
- **Expected outcome:** tenants get Render's edge caching, and the unsigned divergence is gone.
- **Why now:** the unsigned `—` row breaks the new rule today.
- **Render parity:** included: new field on REST/GraphQL/MCP and the dashboard.
