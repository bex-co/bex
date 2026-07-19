# w5 · m48 — Static-site parity: type-gated IA + always-on site URL

**Worker:** worker5 **Goal:** a `static_site` App's dashboard experience matches Render's static-site product — the sidebar/header/settings stop pretending a static site has a runtime instance, redirects/rewrites + headers get their Render-shaped dedicated pages, and every static site shows its platform URL from the moment it's created (not only after the first successful publish). **Status:** done (2026-07-18 — all ten tasks; live dev-5 URL/branch proof + stub browser walk + full suites green)

## Tasks (in order)

| id   | title                                                                                          | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Type-gate the static-site sidebar + header meta (hide Shell/Scaling/Plan, Instances; Logs call) — **DONE** | 45m | —                |
| t002 | Dedicated Redirects/Rewrites + Headers pages under Manage (move editors out of Settings) — **DONE** | 45m | t001             |
| t003 | Platform URL from creation: derive + surface `https://<slug>.<base-domain>` before first publish — **DONE** | 60m | —                |
| t004 | Settings static gaps: no Instance Type row, name-not-id value, root-dir prefix, hook URL check — **DONE** | 45m | —                |
| t005 | Editable Branch in Build settings (backend update path + searchable picker) — **DONE** | 60m | —                |
| t006 | Static Metrics content: requests/bandwidth panels, no pod CPU/mem for static sites — **DONE** | 45m | —                |
| t007 | Render parity — REST/GraphQL/MCP/UI consistency for every surface this milestone touched — **DONE** | 30m | t001, t002, t003, t004, t005, t006 |
| t008 | Simplify — `/simplify` over the changed code — **DONE** | 30m | t007             |
| t009 | Test coverage — meaningful tests for type-gating, URL derivation, branch update — **DONE** | 45m | t007             |
| t010 | Closeout — verify DoD, mark done, move milestone to done/ — **DONE** | 15m | t009             |

## Definition of done

Against a live cluster (dev-5 or prod) with a `static_site` App that has **never successfully deployed**:

- The service sidebar shows Deploys, Settings, Events, Metrics, Environment, Redirects/Rewrites, Headers — and does **not** show Shell, Scaling, or Plan (Logs per the t001 decision, recorded in the task).
- The header shows the site's `https://<slug>.<base-domain>` URL with a copy button immediately after creation, and no "Instances" meta; REST `serviceDetails.url`, GraphQL, and MCP return the same URL.
- Settings for the static site shows no Instance Type row, the Service Name row prefills the actual name (never the `srv-…` id), and Build Command / Publish directory inputs carry the root-directory prefix affordance when a root dir is set.
- Branch is editable from Build & Deploy settings and a change round-trips through the backend (REST/GraphQL) and triggers the documented redeploy behavior.
- The static site's Metrics tab renders request/bandwidth-oriented panels, not empty pod CPU/memory charts.
- Dashboard test suite green; `docs/ADR018-render-parity.md` static-site rows updated with evidence if any cell changes.

## Source + Goal linkage

- **Source:** user research request 2026-07-18 — live Playwright comparison of `dashboard.bex.co/services/srv-d9e5sbd5qe4s73b1mjq0/settings` (bex static site) vs `dashboard.render.com/static/srv-d2rlto3ipnbc73d9fulg/settings` (Render static site) and `dashboard.render.com/web/srv-caiibsqrrk01jaluq6o0/settings` (Render web service, to isolate type-gating); evidence in `.playwright-mcp/bex-settings.png`, `.playwright-mcp/render-settings.png`, `.playwright-mcp/render-web-settings.png`. User explicitly scoped to static sites, their sidebar IA, and the site URL.
- **Goal linkage:** Render parity (`docs/ADR018-render-parity.md` static-site rows; `docs/ADR029-static-sites.md` is the shipped mechanism) — the dashboard is bex's human surface and must not present runtime affordances a static site doesn't have.
- **Expected outcome:** a static site in bex reads like a static site on Render: correct IA, correct settings, and a copyable site URL from creation — the strongest single visual signal the product "understands" the service type.
- **Why now:** static-site mechanism (w9/m44) and route/header editors (w1/m21) already shipped; the remaining gap is presentation-layer type-gating plus one URL-derivation change, all unblocked. The user hit the missing-URL gap live today on a fresh static site whose build failed.
- **Render parity closing task included** — this milestone changes UI + the URL surfaces in REST/GraphQL/MCP.
- **Anti-goals respected:** no PR Previews tab (rejected non-goal), no CDN cache purge, no log streams, no Git Credentials row (GitHub App owns credentials).
