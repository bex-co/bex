# w5 · m5 — Service overview page + Render-style service IA (Overview / Metrics / Logs)

**Worker:** worker5 **Goal:** A per-service page at `/services/$serviceId` backed by bex-api's `server(id)` query, laid out as Render's service-detail page (overview panel + a tab nav mirroring Render's), that folds the existing metrics page in as the Metrics tab and gives every later per-service page (logs, env) a home. **Status:** todo

## Tasks (in order)

| id   | title                                                                                             | est | depends_on         |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ------------------ |
| t001 | Capture Render's service-detail IA via Playwright (tab nav + overview panel) as the design source  | 30m | —                  |
| t002 | `server(id)` query + codegen; `/services/$serviceId` overview route (phase, url, revision, replicas, createdAt, suspended) | 50m | w5/m5/t001         |
| t003 | Service-scoped tab chrome (Overview / Metrics / Logs) mirroring Render's nav; fold the existing metrics route in as the Metrics tab, preserving its deep-link | 45m | w5/m5/t002         |
| t004 | Lifecycle actions on the overview header (reuse m4 mutations); link services-list rows to the overview | 40m | w5/m5/t002         |
| t005 | Simplify — `/simplify` over the code this milestone changed                                         | 30m | w5/m5/t003, w5/m5/t004 |
| t006 | Test coverage — meaningful tests for `server(id)` mapping + tab routing + header actions            | 30m | w5/m5/t003, w5/m5/t004 |

## Definition of done

- `/services/$serviceId` renders live `server(id)` data — phase, `serviceDetails.url`, revision, replicas, createdAt, string `suspended` enum — in an overview panel laid out per the Render reference captured in t001.
- A service-scoped tab nav (Overview / Metrics / Logs) mirrors Render's service-detail nav; the existing metrics page appears as the **Metrics** tab and its old deep-link (`/services/$serviceId/metrics`) still resolves.
- The services list (m4) links each row to its overview page; lifecycle actions (suspend/resume/restart) work from the overview header, reusing m4's mutations + poll-to-converge.
- The Logs tab is present as a nav target (its content ships in m6) — an unbuilt tab is a labeled placeholder, not a broken route.
- `yarn lint && yarn typecheck && yarn test && yarn build` all pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5 to work on dashboard` (2026-07-06) + user directive "all apis and uis should be consistent with render.com". The metrics route (`src/routes/services.$serviceId.metrics.tsx`) currently hangs off a bare deep-link with no parent page. `server(id)` query per `docs/bex-api.md` (mirrors Render's dashboard GraphQL `server(id)`).
- **Goal linkage:** `docs/vision.md` dashboard pillar + pillar-1 API-first (`server(id)` already exposed). Establishes the per-service IA — matching Render's service-detail shape — that logs (m6) and later pages slot into.
- **Expected outcome:** the Render service-detail experience — a real drill-down from the services list into an overview with metrics as one tab among several, controllable from the header.
- **Why now:** once the list is real (m4), the drill-down is the natural sequel and it gives the orphaned metrics page a home; building the tab shell now, before logs (m6), means logs lands as a tab rather than another bare route.
