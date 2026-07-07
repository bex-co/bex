# w5 · m5 — Service overview page + Render-style service IA (Overview / Logs)

**Worker:** worker5 **Goal:** A per-service page at `/services/$serviceId` backed by bex-api's `server(id)` query, laid out as Render's service-detail page (overview panel + a service-scoped nav mirroring Render's service sidebar) that gives every later per-service page (logs, env) a home. **Status:** done

## Tasks (in order)

| id   | title                                                                                             | est | depends_on         | status     |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ------------------ | ---------- |
| t001 | Capture Render's service-detail IA via Playwright (service side-nav + overview panel) as the design source | 30m | —                  | — **DONE** |
| t002 | `server(id)` query + codegen; `/services/$serviceId` overview route (phase, url, revision, replicas, createdAt, suspended) | 50m | w5/m5/t001         | — **DONE** |
| t003 | Service-scoped nav chrome (Overview / Logs) mirroring Render's service sidebar                     | 30m | w5/m5/t002         | — **DONE** |
| t004 | Lifecycle actions on the overview header (reuse m4 mutations); link services-list rows to the overview | 40m | w5/m5/t002         | — **DONE** |
| t005 | Simplify — `/simplify` over the code this milestone changed                                         | 30m | w5/m5/t003, w5/m5/t004 | — **DONE** |
| t006 | Test coverage — meaningful tests for `server(id)` mapping + nav routing + header actions            | 30m | w5/m5/t003, w5/m5/t004 | — **DONE** |

## Render reference (captured 2026-07-06 from `dashboard.render.com/web/srv-…`)

Screenshots: `.playwright-mcp/render-service-logs.png`, `.playwright-mcp/render-service-overview.png` (gitignored). Verified live against Render's `cuckoo-backend` web service.

**Service nav is a left sidebar, not a top tab bar.** Render's service-detail page is a full-height left service sidebar + a main content pane. The sidebar:

- Header: a back link ("← Environment") and the service name with a type icon (globe = web service).
- Ungrouped items: **Events**, **Settings**.
- **MONITOR** group: **Logs**, **Metrics**.
- **MANAGE** group: Environment, Shell, Scaling, Previews, Disk, One-Off Jobs.
- There is **no "Overview" item** — the service root redirects into Events/deploys. bex adds an **Overview** landing (the `server(id)` panel) as its own first item; that is a deliberate bex addition, not a Render mirror.

**bex's subset for m5:** a service-scoped left nav with **Overview** (bex landing) and a **MONITOR › Logs** item. Metrics, Events, Settings, Environment, and the rest are later/out of scope. Route each item under a shared `services.$serviceId` layout that renders the nav + `<Outlet/>`.

**Service root / overview landing** (`render-service-overview.png`): the root shows a header — `WEB SERVICE` type label, the name with runtime (`Node`) + plan (`Starter`) badges, **Connect ▾** and **Manual Deploy ▾** actions, then **Service ID**, the **GitHub repo + branch**, and the **live URL** (each copy-able) — above the deploys/events timeline. Field mapping onto bex-api's `server(id)` and the gaps flagged:

| Render overview field   | bex `server(id)`        | Notes                                                        |
| ----------------------- | ----------------------- | ----------------------------------------------------------- |
| name                    | `name`                  | ✓                                                           |
| type (`WEB SERVICE`)    | `type` (`web_service`)  | ✓ (bex is always a web service today)                       |
| live URL                | `url`                   | ✓ (flat `url`; REST nests under `serviceDetails.url`)       |
| Service ID              | `id`                    | bex's id **is** the App name (no opaque `srv-…`)            |
| runtime / plan badges   | —                       | **gap:** bex-api exposes neither runtime nor a plan concept |
| GitHub repo + branch    | —                       | **gap:** not on `server(id)` (a build-source field, later)  |
| —                       | `phase`/`replicas`/`revision` | bex-native operational extras Render's header omits   |

bex's Overview panel therefore renders a bex-shaped superset — Status/Phase/URL/Instances/Revision/Created/Suspended (the operational fields bex has), not Render's repo/plan/runtime (which `server(id)` does not expose). Header actions map to bex's lifecycle verbs (suspend/resume/restart) rather than Render's Manual Deploy/Connect.

**Logs page (reference for the Logs nav item; content ships in m6):**

- URL param contract: `/web/srv-…/logs?t=app&r=14d` — `t` = log source (`app` = application logs; also build/system), `r` = time range (`14d`). Mirror these as query params on bex's logs route so deep-links carry source + range.
- Toolbar (left→right): **[Application logs ▾]** source filter · **[🔍 Search logs]** searchbox · **[🕐 Last 14 days ▾]** time-range picker · **[⤢ Maximize]** · **[⋯ Options]**.
- Row anatomy: a **level icon** (red ✕ for errors — error rows get a pink row background), a **timestamp** (`12:07:54 PM`), a clickable **instance/deploy tag** (`[fhgtt]`), then the **message** (URLs auto-linked). Date separators (`Jun 26`) break the stream by day.
- Live-tail affordance: a floating **"Return to Bottom"** button when scrolled up. Maps to bex's live-tail Logs API (`docs/observability.md`).

## Definition of done

- `/services/$serviceId` renders live `server(id)` data — phase, `serviceDetails.url`, revision, replicas, createdAt, string `suspended` enum — in an overview panel laid out per the Render reference above.
- A service-scoped left nav (Overview / Logs) mirrors Render's service sidebar structure (grouped, with Logs under a Monitor-style group).
- The services list (m4) links each row to its overview page; lifecycle actions (suspend/resume/restart) work from the overview header, reusing m4's mutations + poll-to-converge.
- The Logs nav item is present as a nav target (its content ships in m6) — an unbuilt item is a labeled placeholder, not a broken route.
- `yarn lint && yarn typecheck && yarn test && yarn build` all pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5 to work on dashboard` (2026-07-06) + user directive "all apis and uis should be consistent with render.com". `server(id)` query per `docs/bex-api.md` (mirrors Render's dashboard GraphQL `server(id)`). Service IA + logs contract captured live from Render (see the Render reference above).
- **Goal linkage:** `docs/vision.md` dashboard pillar + pillar-1 API-first (`server(id)` already exposed). Establishes the per-service IA — matching Render's service-detail shape — that logs (m6) and later pages slot into.
- **Expected outcome:** the Render service-detail experience — a real drill-down from the services list into an overview, controllable from the header, with a Render-shaped service nav that logs slots into.
- **Why now:** once the list is real (m4), the drill-down is the natural sequel; building the nav shell now, before logs (m6), means logs lands as a nav item rather than another bare route.
