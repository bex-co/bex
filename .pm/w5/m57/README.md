# w5 · m57 — Static-site page Render-parity polish (URL + landing IA)

**Worker:** worker5 **Goal:** close the residual static-site dashboard divergences w5/m48 left open — a canonical `/static/<id>` URL, an Events-first sidebar/landing (no Deploys tab), and a Region-free Settings page — so bex's static-site page matches Render's `/static/<id>` page. **Status:** in progress (t001–t003 + t006 done; t004 live-dashboard walk, t005 simplify, t007 closeout open)

## Tasks (in order)

| id   | title                                                       | est   | depends_on       | status       |
| ---- | ----------------------------------------------------------- | ----- | ---------------- | ------------ |
| t001 | Canonical `/static/<id>` route for static sites             | 1h30m | —                | — **DONE**   |
| t002 | Static-site sidebar/landing — Events first, drop Deploys tab | 45m   | t001             | — **DONE**   |
| t003 | Hide read-only Region row on static-site Settings           | 20m   | —                | — **DONE**   |
| t004 | Render parity — live static-site page walk vs render.com    | 45m   | t001, t002, t003 | todo         |
| t005 | Simplify — reuse/simplification pass over the changes       | 30m   | t004             | todo         |
| t006 | Test coverage — routing, landing, nav, settings gating      | 1h    | t004             | — **DONE**   |
| t007 | Closeout — land the milestone                               | 10m   | t006             | todo         |

**Implementation note (2026-07-29):** shipped as a full parallel `/static/$serviceId` route tree that reuses the shared `ServiceDetailLayout` + tab pages; the base rides a `ServiceBaseContext` the layout provides, and a shared detail loader (`service-detail-loader.ts`) canonicalizes the base (a static site under `/services` bounces to `/static`, and vice-versa — loop-free). Entry links stay bare `/services/$serviceId` and self-canonicalize via the index redirect. Verified with `yarn typecheck && lint && test` (1684 tests). The "Redirects/Rewrites" rename was already shipped (not a task). t004 needs a running/logged-in dashboard (dev-5 or prod) for the live side-by-side, which wasn't available in the implementing session.

## Definition of done

- A static-typed service's canonical dashboard URL is `/static/<serviceId>` (and its sub-routes `/static/<serviceId>/{events,settings,metrics,env,redirects,headers}`); `/services/<serviceId>` for a static service still resolves (redirects to the `/static` equivalent), and non-static types are unchanged on `/services/`.
- Landing on a static service (services list, project page, or bare URL) shows **Events**, not Deploys; the static sidebar has Events at top-level and no Deploys tab; web/worker/pserv/cron keep their Deploys-first IA.
- The static-site Settings page no longer renders the read-only Region row (non-static types keep it).
- A live side-by-side walk vs `dashboard.render.com/static/<id>` shows the URL scheme, sidebar (Events/Settings · Metrics · Environment/Redirects-Rewrites/Headers), landing tab, and Settings sections match, with any residual drift logged. Previews / PR Previews remain excluded (DO_NOT_DO #27).
- `yarn typecheck && yarn lint && yarn test` green.

## Source + Goal linkage

- **Source:** user request 2026-07-28 — "learn from dashboard.render.com/static/srv-… and make dashboard.bex.co/services/srv-… consistent for static sites"; live Render capture (`.playwright-mcp/`, 2026-07-28) + a dashboard code diff. Gap-closing follow-on to **w5/m48** (Static-site parity, done 2026-07-18), which type-gated the sidebar/header/settings/metrics and shipped the Redirects/Rewrites + Headers pages but deliberately kept `/static/*` **redirecting to** the canonical `/services/`, and kept the shared Deploys-first landing.
- **Gap analysis (avoid duplication):** the proposed "Redirects" → "Redirects/Rewrites" rename was found **already shipped** — the `services.navRedirects` label is `Redirects/Rewrites` (en) / `重定向/重写` (zh), tagged "Render's label" in the locale files — so it is **not** a task here. Region row was added platform-wide by w5/m52; this milestone only scopes it out for static.
- **Goal linkage:** Render parity / dashboard fidelity (docs/ADR018-render-parity.md, docs/ADR029-static-sites.md) — the dashboard should read as a drop-in Render replacement for static sites.
- **Expected outcome:** the static-site page's URL, sidebar, landing, and Settings match Render's `/static/<id>` page, scoped narrowly to static-typed services so other types are untouched.
- **Why now:** direct user request; small, well-scoped follow-on to the already-shipped m48 IA work while that context is fresh.
- **Why Render parity task included:** the milestone changes the dashboard UI (a user-facing surface). It is UI-routing-only with no REST/GraphQL/MCP change, so the parity task (t004) is a final live-Render UI walk rather than a cross-adapter check.
- **Out of scope (user decision + DO_NOT_DO #27):** Previews tab, PR Previews / preview environments.
