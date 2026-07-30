# Static-site page walk — bex vs Render `/static/<id>` (w5/m57/t004)

Live side-by-side of bex's static-site dashboard page against Render's `dashboard.render.com/static/<id>`, confirming the m57 parity work (canonical `/static/<id>` URL, Events-first IA, Region-free Settings). bex side captured 2026-07-30 driving the offline dashboard (`yarn dev:local`, the `docs-site` `static_site` fixture in `scripts/local-bex.mjs`, which carries `region: "fsn1"` so the Region-gating is actually exercised). Render side from the 2026-07-28 live capture referenced by the milestone + `docs/render-artifacts/dashboard-routes.md`. Screenshot: `.playwright-mcp/m57-static-events-landing.png`.

## Surfaces

| Surface | Render `/static/<id>` | bex `/static/<id>` | Verdict |
| --- | --- | --- | --- |
| URL scheme | `/static/<id>` + `/{events,settings,metrics,env,...}` | `/static/docs-site/{events,settings,metrics,env,redirects,headers}` | match |
| Reverse alias | n/a (Render has one scheme) | `/services/<static-id>` → `/static/<id>/events` (loop-free canonicalize) | match¹ |
| Bare-URL land | Events | `/static/docs-site` → `/static/docs-site/events` | match |
| Sidebar top | Events, Settings | Events, Settings | match |
| Sidebar Monitor | Metrics (no Logs — static has no runtime) | Metrics | match |
| Sidebar Manage | Environment, Redirects/Rewrites, Headers | Environment, Redirects/Rewrites, Headers | match |
| No Deploys tab | Deploy history/detail reached from the Events feed | No Deploys nav item; `/static/<id>/deploys/<dep>` routes exist for deep links | match |
| No runtime nav | no Shell / Scaling | no Shell / Scaling / Plan (Plan is bex-only, gated out for static) | match |
| Settings | General (name), Build, Deploy, Static (publish + edge rules), Custom Domains, Networking, Notifications, Suspend, Danger | same sections; **no Region row**, **no Instance Type row** | match |
| Header | type eyebrow + name + status; no instance/plan chip | "Static Site" eyebrow + name + Running; Manual Deploy; **no Connect button** | match² |

¹ bex extension over Render — bex keeps `/services/<id>` resolving for every type so old links/bookmarks land; a static id bounces to its canonical `/static` base.

² **Drift found + fixed in this milestone (t005):** the shared service header rendered a "Free" instance-type/plan chip linking to `/services/<id>/plan`. For a static service that link canonicalizes to `/static/<id>/plan`, which has no route → 404 ("Page not found"). Render's static header has no plan/instance concept, and `service-detail-header.tsx`'s own comment already said the chip should render "only for compute types" — the `!isStaticSite` gate was simply missing. Gated in t005.

## Intentional exclusions (not drift)

- **Previews / PR Previews** — DO_NOT_DO #27; absent by design on both the sidebar and Settings.
- **Plan tab** — bex-only (Render folds instance type into scaling/settings); gated out of the static sidebar and, now, the static header chip.

## Console noise (not a defect)

The offline walk logs Apollo `Missing field 'internalAddress'/'displayName'/…` cache warnings and TanStack router-devtools errors — both are `local-bex` fixture / devtools artifacts, not app behavior. A hard load of any real route (Settings, Redirects, Headers, Metrics) renders 0 app errors.
