# Render dashboard URL scheme (route compatibility)

**Captured:** 2026-07-16 · **Method:** primary-source code reading of Render's official open-source CLI ([`render-oss/cli`](https://github.com/render-oss/cli), `pkg/dashboard/dashboard.go` — the switch that constructs every "open in dashboard" URL), corroborated by live `dashboard.render.com` URLs already captured in this repo's earlier authenticated walks (w3/m3, w5/m13, w5/m32, w9/m1) and public web evidence. The design source for bex's Render-shaped dashboard route aliases and for bex-api's `dashboardUrl` metadata shape (`w5/m39`).

## Why this matters

Render's API returns `dashboardUrl` values in these shapes, Render's official CLI **constructs** them client-side from its configured dashboard origin (so the unmodified CLI pointed at bex — bex's fifth surface, [cli-compatibility-checklist.md](../cli-compatibility-checklist.md) — mints them against bex's dashboard), and users/agents paste them from habit. bex already mints Render's exact id prefixes (`lego/backend/internal/id/id.go`: `srv-`, `dpg-`, `red-`, `prj-`, `evg-`, `dep-`, …), so path segments are the entire compatibility surface.

## Resource → path map

`<dashboard>/{segment}/{id}`; deploys nest as `/{segment}/{srv-id}/deploys/{dep-id}`. Unknown service types fall back to `web` in the CLI — so bex must accept **any** `srv-` id under **every** service segment, never validating segment ↔ service type.

| Resource | Render path | Evidence | bex canonical route (pre-m39) | m39 disposition |
| --- | --- | --- | --- | --- |
| Web service | `/web/{srv-id}` | CLI switch; live `dashboard.render.com/web/srv-ck99r5ldrqvc73bkdu6g`; repo captures (w3/m3, w9/m1) | `/services/$serviceId` | alias → redirect |
| Background worker | `/worker/{srv-id}` | CLI switch | `/services/$serviceId` | alias → redirect |
| Private service | `/pserv/{srv-id}` | CLI switch | `/services/$serviceId` | alias → redirect |
| Static site | `/static/{srv-id}` | CLI switch | `/services/$serviceId` | alias → redirect |
| Cron job | `/cron/{srv-id}` | CLI switch | `/services/$serviceId` | alias → redirect |
| Deploy | `/{segment}/{srv-id}/deploys/{dep-id}` | CLI switch; live `…/web/srv-cr1aprdds78s739qrbg0/deploys/dep-d9bb06vlk1mc73fgp9pg` (w9/m1) | `/services/$serviceId/deploys/$deployId` | alias → redirect (sub-path preserved) |
| Postgres | `/d/{dpg-id}` | CLI switch | `/databases/$databaseId` | alias → redirect |
| Key Value | `/r/{red-id}` | CLI switch (legacy Redis segment) | `/keyvalue/$keyValueId` | alias → redirect |
| Project | `/project/{prj-id}` | repo capture (`project.$projectId.index.tsx` cites it) | `/project/$projectId` | already matches |
| Env group | `/env-groups/{evg-id}` | list page `dashboard.render.com/env-groups` (live link, 2026-07-16); detail inferred — Render docs never show a detail URL | `/env-groups/$groupId` | already matches |
| Blueprint | `/blueprints/{id}` (docs-fallback) | list `dashboard.render.com/blueprints` + `blueprint/new` (create) are live; detail path unverified | `/blueprints/$blueprintId` | assumed match; revisit if a live capture disagrees |
| Workflow | `/wf/{id}` | CLI switch | — | non-goal (workflows off-roadmap, `.pm/DO_NOT_DO.md`) |

## Service sub-tab map

From live repo captures (w3/m3 metrics, w5/m13 settings, w9/m1 deploys) plus the m32 page-by-page walk ([dashboard-walk/services.md](dashboard-walk/services.md)):

| Render tab path | Evidence | bex route file | Note |
| --- | --- | --- | --- |
| `/{seg}/{id}` (root = deploy history) | m32 walk | `services.$serviceId.index.tsx` (redirects to Deploys since w5/m36) | match |
| `/deploys`, `/deploys/{dep-id}` | w9/m1 live URL | `services.$serviceId.deploys.index.tsx`, `…deploys.$deployId.tsx` | match |
| `/events` | m32 walk (separate tab) | `services.$serviceId.events.tsx` | match |
| `/logs` | m32 walk | `services.$serviceId.logs.tsx` | match |
| `/env` | m32 walk (Environment tab) | `services.$serviceId.env.tsx` | match |
| `/metrics` | w3/m3 live URL | `services.$serviceId.metrics.tsx` | match |
| `/settings` | w5/m13 live URL | `services.$serviceId.settings.tsx` | match |
| `/scaling` | m32 walk (Render folds scaling into settings/scaling) | `services.$serviceId.scaling.tsx`, `…plan.tsx` | bex splits Plan out; IA-only difference (m32 verdict: not a gap) |

Because tab names already agree, the alias redirect needs **no segment renaming**: `/{web|worker|pserv|static|cron}/{srv-id}/<rest>` → `/services/{srv-id}/<rest>` verbatim (query string preserved). bex-only tabs (`/plan`) simply have no Render inbound shape.

## Decision (w5/m39)

bex's `/services/...`, `/databases/...`, `/keyvalue/...` routes stay **canonical** (internal links, tests, and docs unchanged); Render-shaped paths are thin **redirect aliases**. bex-api's emitted `dashboardUrl` flips to the Render shape — type-aware segment for services (`web`/`worker`/`pserv`/`static`/`cron`, fallback `web`), `/d/` for Postgres, `/r/` for Key Value — so API responses are byte-shape-compatible with Render's and the redirect makes every emitted link land.
