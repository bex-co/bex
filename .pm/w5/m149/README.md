# w5 · m149 — `PORT` is user-settable like on Render

**Worker:** worker5 **Goal:** a service's own `PORT` sets the port it serves and is probed on, as on Render, instead of being refused. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Amend ADR018 row 138 and ADR004: PORT is user-settable as on Render | 30m | — |
| t002 | Drop the reserved-PORT refusal on every write path | 1h | t001 |
| t003 | The operator serves, routes and probes on the effective PORT | 1h45m | t001 |
| t004 | Dashboard editors accept PORT; envtests for own, group and precedence | 1h | t002, t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

Total: **8 tasks, about 6h**.

## Definition of done

- Setting `PORT=8080` on a service rolls it out serving and passing health checks on 8080.
- A linked group's `PORT` behaves as on Render, and every surface accepts the key.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); ADR018 row 138 "Deliberate divergence (w2/m95)".
- **Goal linkage:** ADR008 Render parity first; ADR006/ADR018.
- **Expected outcome:** apps that set their own PORT migrate without edits.
- **Why now:** `2163b27c7` lets a divergence stand only with recorded user sign-off, and this is the most visible unsigned one.
- **Render parity:** included: removes refusals on REST/GraphQL/MCP and the dashboard.
