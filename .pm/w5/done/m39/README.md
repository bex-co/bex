# w5 · m39 — Render dashboard-route compatibility (`/web/srv-…` deep links)

**Worker:** worker5 **Goal:** Render-shaped dashboard URLs resolve on bex's dashboard, and bex-api's `dashboardUrl` metadata emits those Render-shaped URLs — so links minted by Render-trained habits, agents, and the unmodified official Render CLI (bex's fifth surface) land on the right page instead of 404ing. **Status:** done (2026-07-16)

## Research findings (2026-07-16)

- **The gap is path segments only.** bex already mints Render's exact id prefixes (`lego/backend/internal/id/id.go`: `srv-`, `dpg-`, `red-`, `prj-`, `evg-`, `dep-`, `blp-`…), so `https://<dashboard>/web/srv-d9bkcspg9s7c73d0n8ug` fails only because bex's route is `/services/$serviceId`, not `/web/$serviceId`.
- **Render's scheme (primary source: `render-oss/cli`, `pkg/dashboard/dashboard.go`)** — the official CLI hardcodes `<dashboard>/{segment}/{id}` with segment by resource type: `web` (web service), `worker` (background worker), `pserv` (private service), `static` (static site), `cron` (cron job), `r` (Key Value), `d` (Postgres), `wf` (workflows — bex non-goal); deploys are `/{segment}/{id}/deploys/{deployId}`. Unknown types fall back to `web`. Confirmed in the wild: `dashboard.render.com/web/srv-ck99r5ldrqvc73bkdu6g`.
- **Concrete driver:** the Render CLI constructs these URLs client-side from its configured dashboard URL — pointed at bex (docs/cli-compatibility-checklist.md, w9/m2), every "open in dashboard" affordance today produces a bex 404.
- **Already-matching routes (no work):** `/project/$projectId` and `/env-groups/$groupId` match Render's shapes (repo comments in `dashboard/src/routes/project.$projectId.index.tsx` cite `dashboard.render.com/project/{id}`).
- **bex-api today** builds `dashboardUrl` via `resourcemeta.Config.DashboardURL(route, id)` (`lego/backend/internal/resourcemeta/metadata.go:96`) with bex-shaped routes `services`/`databases`/`keyvalue` (callers: `internal/apps/render.go:281`, `internal/apps/service.go:736`, `internal/postgres/rest.go:69`, `internal/keyvalue/rest.go:106`). Render's API returns type-aware `/web/srv-…`-shaped `dashboardUrl` values.
- **Decision: alias + redirect, don't flip canonical.** bex's `/services/...` routes stay canonical (every internal `<Link>`, test, and doc keeps working); new Render-shaped alias routes redirect to them, preserving sub-paths (`/web/srv-x/deploys/dep-x` → `/services/srv-x/deploys/dep-x`). bex-api's emitted `dashboardUrl` flips to the Render shape (wire-format parity); the redirect makes it land correctly. All five service segments accept any `srv-` id (Render's CLI falls back to `web` for unknown types, so strict type↔segment validation would break its links).

## Tasks (in order)

| id   | title                                                                                     | est | depends_on   |
| ---- | ----------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Capture Render dashboard URL-scheme evidence → docs/render-artifacts/dashboard-routes.md — **DONE** | 30m | —            |
| t002 | Dashboard alias routes: `/web` `/worker` `/pserv` `/static` `/cron` → `/services` (+sub-paths) — **DONE** | 45m | t001         |
| t003 | Dashboard alias routes: `/d/$id` → `/databases`, `/r/$id` → `/keyvalue` — **DONE**                   | 30m | t001         |
| t004 | bex-api `dashboardUrl` emits Render-shaped, type-aware paths — **DONE**                              | 45m | t002, t003   |
| t005 | Update docs/cli-compatibility-checklist.md + ADR018 parity-ledger evidence — **DONE**                | 30m | t004         |
| t006 | Render parity (standing) — **DONE**                                                                  | 30m | t004, t005   |
| t007 | Simplify (standing) — **DONE**                                                                       | 30m | t006         |
| t008 | Test coverage (standing) — **DONE**                                                                  | 30m | t006         |
| t009 | Closeout (standing) — **DONE**                                                                       | 15m | t007, t008   |

## Definition of done

On a running dashboard, `GET /web/<srv-id>`, `/worker/<srv-id>`, `/pserv/<srv-id>`, `/static/<srv-id>`, `/cron/<srv-id>` (and `/web/<srv-id>/deploys/<dep-id>` sub-paths), `/d/<dpg-id>`, and `/r/<red-id>` all land on the correct bex detail pages (redirect to the canonical routes), and `GET /v1/services` / `/v1/postgres` / `/v1/key-value` return Render-shaped `dashboardUrl` values (type-aware segment for services), with tests covering the alias redirects and the URL construction, and the CLI checklist + ADR018 rows updated with evidence.

## Source + Goal linkage

- **Source:** user request 2026-07-16 (`/pm research how to achieve route compatibility with render.com for bex.co — like https://oauth.bex.co/web/srv-d9bkcspg9s7c73d0n8ug`); research above (Render CLI source `render-oss/cli` `pkg/dashboard/dashboard.go`, live `dashboard.render.com/web/srv-…` evidence).
- **Goal linkage:** Render compatibility (docs/ADR018-render-parity.md; docs/ADR006-bex-api.md wire-format parity) and the fifth surface — the unmodified official Render CLI against bex-api (docs/cli-compatibility-checklist.md, DO_NOT_DO "no first-party CLI").
- **Expected outcome:** any Render-shaped dashboard deep link (pasted by a user, minted by an agent, or constructed by the Render CLI's open-in-dashboard paths) resolves on bex; bex-api's `dashboardUrl` metadata is byte-shape-compatible with Render's API responses.
- **Why now:** the CLI-compatibility checklist (w9/m2) already runs the official CLI against bex — its dashboard-URL construction 404s today; ids were deliberately minted Render-shaped (ADR020) precisely so this class of compatibility stays cheap, and the remaining gap is a thin routing layer.
- **Render parity task included:** yes — this touches REST metadata (`dashboardUrl` on services/postgres/key-value across REST/GraphQL/MCP) and the dashboard UI routes.
