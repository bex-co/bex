# w5 · m153 — Services, Postgres and Key Value created through the API can have a free-form name

**Worker:** worker5 **Goal:** a Render client creating `My API` or `Prod DB` on any surface gets the resource Render gives, instead of RESOURCE_NAME_INVALID. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Service create maps a free-form name onto a slug and keeps it as the display name — **DONE** | 1h | — |
| t002 | Datastore names are free-form labels on every surface — **DONE** | 1h15m | — |
| t003 | Blueprint datastore names keep their original label — **DONE** | 30m | t002 |
| t004 | Dashboard create forms accept free-form names — **DONE** | 45m | t001, t002 |
| t005 | Render parity — **DONE** | 30m | t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

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

## Closeout (2026-10-21)

- **Services.** `Service.Create`, which REST, GraphQL `createService` and the MCP create tools share, maps a free-form name onto its slug and keeps the original as the display name. A dry run previews the label. `TestCreateMapsAFreeFormNameOntoASlugAndKeepsItAsTheLabel` covers it, and `TestAFreeFormNameIsAcceptedOnEverySurface` replaces w5/093's refusal test.
- **Datastores.** The Postgres and Key Value `name` is a free-form label (`core.CheckLabel` plus the resource-id-shape refusal) on create and rename, and the CRD pattern is relaxed (MaxLength 100). Service display names share the same label check. `TestDatastoreNamesPassOneRule` is updated.
- **Blueprints.** Datastore names keep their original label; only services are slugged. The corpus still passes at 90 of 170 valid.
- **Dashboard.** The service create form takes free-form names with a "Resource name: …" slug preview, and its availability check uses the slug. The Postgres and Key Value create and rename forms take labels. Lint passes, and all 5487 tests pass.
- **Not covered.** Read-replica and workspace names keep the slug rule; Render's workspace names are free-form, but neither was in this milestone's scope.
