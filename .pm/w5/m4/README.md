# w5 · m4 — Services list + lifecycle actions, Render-consistent, wired to bex-api

**Worker:** worker5 **Goal:** The dashboard home page renders the operator's real Apps from bex-api's `services` GraphQL query — using Render's `Service` field shapes and lifecycle operation names verbatim — and each row can be suspended / resumed / restarted, replacing the hardcoded `sampleServices`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                            | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ------------------ |
| t001 | Capture Render's services-list IA via Playwright as the design source; map its columns to bex-api `services`/`Service` | 30m | —                  |
| t002 | `services.graphql` query + codegen; wire home page to live `services`, drop `sampleServices`, compute stat tiles from real data | 45m | w5/m4/t001         |
| t003 | Loading / error / empty states for the list (skeleton rows, error card, "no services yet")        | 40m | w5/m4/t002         |
| t004 | Lifecycle row actions: `suspendService`/`resumeService`/`restartServer` mutations w/ confirm dialog, in-flight disable, poll `phase`/`suspended` until converged | 60m | w5/m4/t002         |
| t005 | Verify live against prod `api.bex.co` (Kratos session); screenshot to `.playwright-mcp/`          | 30m | w5/m4/t003, w5/m4/t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed                                        | 30m | w5/m4/t005         |
| t007 | Test coverage — meaningful tests for query mapping + list states + lifecycle actions               | 30m | w5/m4/t005         |

## Definition of done

- The home page (`src/routes/index.tsx`) renders the real list of Apps from `https://api.bex.co/graphql` `services` query — **no `sampleServices` array remains in the tree**; the stat tiles (total / running / suspended) are computed from live data.
- The `Service` shape consumed matches Render's dashboard GraphQL verbatim: `id`, `name`, `type: "web_service"`, the **string** `suspended` enum (`"suspended"` / `"not_suspended"`, not a boolean), `dashboardUrl`, `createdAt`, `serviceDetails.url`, plus bex's `phase` / `replicas` / `revision` superset.
- Each row's suspend / resume / restart triggers the matching Render-named mutation (`suspendService` / `resumeService` / `restartServer`) and the row's status badge converges after the operator reconciles (polled `phase`/`suspended`); the action is disabled while in flight.
- Loading shows skeleton rows, a failed query shows an explicit error card, and zero services shows an empty state.
- Any Render list column the UI wants but bex-api does not expose is **flagged as a gap**, not faked with a dashboard-only value.
- `yarn lint && yarn typecheck && yarn test && yarn build` all pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5 to work on dashboard` (2026-07-06) + user directive "all apis and uis should be consistent with render.com". `src/routes/index.tsx:29-33` carries `sampleServices` with a literal "replace with a real Apollo query once wired up" TODO. Wiring path (Apollo + Kratos session + codegen + CORS) proven by w3/m3 (metrics PoC). Render `Service`/operation shapes per `docs/bex-api.md` (verified against Render's OpenAPI spec + dashboard GraphQL).
- **Goal linkage:** `docs/vision.md` human-facing dashboard pillar + pillar-1 API-first — `services` and the lifecycle verbs are already exposed via REST/GraphQL/MCP, so the dashboard is a pure Render-shaped client, never a dashboard-only feature. `GOAL.md` #2 (basic obs for operation).
- **Expected outcome:** an operator opens the dashboard and sees + controls their real running Apps; the home page stops showing fake data and reads as Render's services list.
- **Why now:** the metrics PoC (w3/m3) just proved the Apollo+Kratos+CORS path end-to-end; services is the highest-traffic next page and the one the metrics page already deep-links into (`/services/$serviceId/metrics`). Wiring it before any further page work stabilizes the primary string/data surface once, on Render-consistent shapes.
