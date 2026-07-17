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

bex's `/services/...`, `/databases/...`, `/keyvalue/...` routes stay **canonical** (internal links, tests, and docs unchanged); Render-shaped paths are thin **redirect aliases**. Since w1/m46 the `/services/{id}` segment carries the minted `srv-…` id (GraphQL emits it, so the dashboard links by it — matching Render's own id-addressed URLs); the service NAME still resolves as an inbound fallback, so pre-m46 bookmarks keep landing. bex-api's emitted `dashboardUrl` flips to the Render shape — type-aware segment for services (`web`/`worker`/`pserv`/`static`/`cron`, fallback `web`), `/d/` for Postgres, `/r/` for Key Value — so API responses are byte-shape-compatible with Render's and the redirect makes every emitted link land.

## Workspace / user / create scheme (2026-07-16 live probes, w1/m45)

Authenticated probes on `dashboard.render.com`, extending the resource-deep-link capture above with the shapes it never covered:

| Render URL | Live behavior (evidence) | bex pre-m45 | m45 disposition |
| --- | --- | --- | --- |
| `/w/settings` | canonicalizes to `/w/{tea-id}/settings` — Render resolves the caller's current workspace into the URL (header "Workspace settings" nav link mints the id-less form) | 404 | `w.$` alias → `/workspace/settings` |
| `/w/{tea-id}/settings` | workspace settings, workspace named by the URL | 404 | `w.$` alias: select that workspace (membership-checked — a foreign id is refused, never silently shown as the caller's own), then `/workspace/settings` |
| `/w/{tea-id}/billing` | workspace billing (sidebar Workspace → Billing) | 404 | same selection, land on `/usage` (bex's deliberate usage-not-billing counterpart, ADR023) |
| `/billing/update-plan` | plan-change page (upgrade CTAs sitewide) | 404 | alias → `/workspace/settings?plan=change` (the plan dialog); billing proper stays a non-goal |
| `/settings` | canonicalizes to `/u/{usr-id}/settings` — account settings are user-scoped; one long page (Profile / Appearance / Account Security / CLI Tokens / API Keys / SSH Public Keys / PR Requests), no sub-URLs | `/settings` works (matches the inbound shape); `/u/…` 404 | `u.$` alias → `/settings` (page is caller-scoped; per the m39 rule, never validate the id) |
| `/web/new`, `/static/new`, `/pserv/new`, `/cron/new`, `/worker/new` | New-menu service creates live under the type segment | already land on `/services/new` via the m39 aliases | keep (verified) |
| `/d/new` | New menu's "Postgres" entry | **broken**: the `d.$` alias redirected it to nonexistent `/databases/new` → 404 catch-all | special-cased to the database create landing |
| `/new/database`, `/new/redis`, `/new/project` | New-menu datastore/project creates live under `/new/` | 404 | `/new/redis` → `/keyvalue/new`; database/project → URL-owned create dialogs on `/` (`?new=database` / `?new=project`) |
| `/workflow/new`, `/wf/{id}` | New menu / CLI switch | 404 | non-goal (workflows off-roadmap) |
| `/login`, `/register` | auth pages | `/auth/login`, `/auth/sign-up` | thin redirects (query/hash preserved — an `?invite=` token must survive) |
| `/observability`, `/private-links`, `/dedicated-ips`, `/invites`, `/documents` | sidebar/footer pages | 404 | non-goals (drains, managed-infra, referral program, compliance docs — `.pm/DO_NOT_DO.md`) |

## Sidebar navigation (2026-07-16 live probes, w1/m45)

**Global sidebar** — identical on `/`, workspace settings, and account settings: ungrouped **Projects** (`/`), **Blueprints**, **Environment Groups**; group **Integrations** — Observability, Webhooks, Notifications; group **Networking** — Private Links, Dedicated IPs; group **Workspace** — Billing, Settings; footer — Changelog, Invite a friend, Contact support, Render Status. bex mapping (m45): keep the ungrouped trio; Integrations carries Webhooks + Notifications only (Observability = drains non-goal); the Networking group is omitted entirely (both entries non-goals); Workspace carries Usage (Billing counterpart) + Settings; the footer is out of scope (not capability navigation).

**Service pages** — Render **replaces** the global sidebar with a resource-scoped one: a "Dashboard" back link (`/`), then **Events**, **Settings**, group **Monitor** — Logs, Metrics; group **Manage** — Environment, Shell, Scaling, Previews, Disk, One-Off Jobs. Notably **no Deploys entry**: the service root (`/{seg}/{id}`) _is_ the deploy history. bex mapping (m45): the same sidebar-swap pattern (precedent: the project sidebar), with Events / Deploys (bex keeps an explicit entry — its root also redirects to Deploys, but the entry aids discoverability) / Settings, Monitor{Logs, Metrics}, Manage{Environment, Scaling, Plan (bex-only)}; Shell, Previews, Disk, and One-Off Jobs are non-goals.

**Datastore pages** — Render tabs Info / Logs / Metrics / Recovery ([dashboard-walk/datastores.md](dashboard-walk/datastores.md)); bex's consolidated `?tab=` detail page was ruled an IA-only difference ("not a gap") by the m32 walk — that ruling stands.
