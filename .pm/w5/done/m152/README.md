# w5 · m152 — Changing a service's runtime works (PATCH `serviceDetails.runtime`)

**Worker:** worker5 **Goal:** changing a service's runtime rebuilds and serves it as on Render, instead of the named refusal w5/m146 added. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Map Render's allowed runtime transitions and their side effects — **DONE** | 45m | — |
| t002 | Update sets the runtime on REST, GraphQL and MCP and rebuilds — **DONE** | 2h | t001 |
| t003 | Dashboard Settings runtime selector — **DONE** | 1h | t002 |
| t004 | Transition tests: node to python and node to docker rebuild and serve — **DONE** | 1h | t002 |
| t005 | Render parity — **DONE** | 30m | t003, t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

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

## Closeout (2026-10-21)

- **Transitions.** They are Render's `serviceRuntime` enum under create's own rules (`resolveBuildStrategy`), recorded in ADR018's Runtime row. Repo-backed services move between the native runtimes and `docker` and rebuild with the builder the runtime selects. Native runtimes need build and start commands, which the same PATCH may carry, because the settings table orders commands first. Image services stay `image`, and static sites have no runtime.
- **Surfaces.** REST `PATCH serviceDetails.runtime` replaces w5/m146's named refusal. GraphQL has `setRuntime`, and the dashboard Settings has a **Runtime** selector with confirmation. Its skeleton matches at 390/640/768/1024/1280/1440/1920px, and the Edge Caching skeleton was corrected at the same widths. MCP has no runtime argument, like Render's MCP.
- **Tests.** `runtime_change_test.go` covers node→python (native), node→docker (dockerfile, rebuilt), and docker→node refused without commands but accepted with them in the same PATCH. An image service moving to docker is refused. Fold, settings-order, verb-count, scope-matrix and events guards are updated, and the dashboard component, hook and page tests pass (5427). The backend suite is full and lint is clean.
- **Deviation.** "node to docker rebuilds and serves" is proven at the spec level (runtime, builder, a rebuild bump), not by a live dev-5 build; dev-5 reuses the shared operator.
