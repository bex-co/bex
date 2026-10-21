# w5 · m153 — Services, Postgres and Key Value created through the API can have a free-form name

**Worker:** worker5 **Goal:** a Render client creating `My API` or `Prod DB` on any surface gets the resource Render gives, instead of RESOURCE_NAME_INVALID. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Service create maps a free-form name onto a slug and keeps it as the display name | 1h | — |
| t002 | Datastore names are free-form labels on every surface | 1h15m | — |
| t003 | Blueprint datastore names keep their original label | 30m | t002 |
| t004 | Dashboard create forms accept free-form names | 45m | t001, t002 |
| t005 | Render parity | 30m | t003, t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

Total: **8 tasks, about 5h15m**.

## Definition of done

- REST, GraphQL, MCP and the dashboard create a service named `My API` (resource `my-api`, labeled `My API`) and a Postgres or Key Value named `Prod DB`.
- Blueprint datastores keep their original names; the corpus still passes.

## Source + Goal linkage

- **Source:** promoted from w5/173 (found by w5/m150), filed under ADR008 parity first.
- **Goal linkage:** ADR006/ADR018 REST compatibility; ADR020 identifiers stay the resource ids.
- **Expected outcome:** Render clients with human names work unchanged.
- **Why now:** m150 made Blueprint names free-form; the single-resource creates are the remaining refusal, and the datastore label is already mutable.
- **Render parity:** included: REST, GraphQL, MCP and dashboard create.
