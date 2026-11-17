# w3 · m441 — Agent sessions, automations and sandboxes record which surface created them (ADR113 D7 server facts)

**Worker:** worker3 **Goal:** creation-surface facts for the extended CLI's products, answered on the server instead of by collecting more on the client. **Status:** done (2026-11-17; shipped in `ce5d4aa47`, verified on production)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Widen product activity resource types for agent sessions, automations and sandboxes — **DONE** | 30m | — |
| [t002](done/t002.md) | Record created facts for agent sessions, automations and sandboxes — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Automation-fired sessions are attributed to the automation surface — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Product adoption panel and SQL tests — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Docs and production check — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

- Creating an agent session, an automation or a sandbox records one `created` product activity row with the surface that created it; an automation-fired session records `automation`; a recorder failure never fails a create.
- The Product adoption board shows "Agent sessions, automations and sandboxes by surface", executed by `scripts/test_product_analytics.py`.
- On production, a sandbox created with the `bex` CLI in `bex-canary` has a row with surface `cli`.

## Source + Goal linkage

- **Source:** [ADR113](../../../../docs/ADR113-cli-usage-telemetry.md) D7 (server facts). `/pm-brainstorm` for ADR113's implementation, 2026-11-16, item 5; filed by `/pmx adr113's implementation to w3`. Re-checked 2026-11-16 against `origin/main` `9a66e98b5`.
- **bex today:** product activity records creations only for services, datastores, workspaces and domains (`core.ObserveProductActivity` call sites in `apps/` and `core/audit.go`). Agent sessions, automations and sandboxes, the extended CLI's main products, have no surface record, so "does `bex agent run` start real work?" has no answer without collecting arguments on the client.
- **Goal linkage:** ADR008's agent-native thesis (how much work agents and the CLI drive) and ADR113's rule to answer such questions from server facts, never from more client telemetry.
- **Expected outcome:** the Product adoption board shows which surface starts agent sessions, automations and sandboxes, and automation-driven sessions are counted as such.
- **Why now:** Cloud Agents and their automations shipped (ADR101, ADR106) and are the fastest-growing CLI area; the recorder and surface rules already exist, so the cost is small.
- **Recommendation recorded (not a user decision):** a new closed surface value `automation`, following the Blueprint precedent, instead of counting scheduler-started sessions as `unknown`.
- **Render parity omitted:** internal analytics only. No REST, GraphQL, MCP or dashboard response changes.
