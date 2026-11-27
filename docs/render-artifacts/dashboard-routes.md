# Render dashboard URL scheme (route compatibility)

**Captured:** 2026-07-16 · **Method:** primary-source code reading of Render's official open-source CLI ([`render-oss/cli`](https://github.com/render-oss/cli), `pkg/dashboard/dashboard.go` — the switch that constructs every "open in dashboard" URL), corroborated by live `dashboard.render.com` URLs already captured in this repo's earlier authenticated walks (w3/m3, w5/m13, w5/m32, w9/m1) and public web evidence. The design source for bex's Render-shaped dashboard route aliases and for bex-api's `dashboardUrl` metadata shape (`w5/m39`). Since w3/m704, [Render's URL inventory](#renders-url-inventory) below lists every path Render's dashboard bundle builds or routes, and a weekly job checks it against the bundle.

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
| Env group | `/env-group/{evg-id}` (the list is `/env-groups`) | verified 2026-11-27 in Render's dashboard bundle (`index-D5V4ZgbE.js` routes `/env-group/:id` and `/env-group/*`; the `urls` chunk builds `` `/env-group/${e.id}` ``); list page `dashboard.render.com/env-groups` (live link, 2026-07-16) | `/env-groups/$groupId` | alias → redirect (w3/m705; query and hash kept). Until 2026-11-27 this row read the plural path, inferred, as already matching |
| Blueprint | `/blueprint/{exs-id}`, with `/resources`, `/syncs`, `/sync/{syncId}` and `/settings` | verified 2026-11-26 in Render's dashboard bundle (`index-D5V4ZgbE.js`, the `iac-source-app` routes; [blueprint-pages.md](blueprint-pages.md)); list `dashboard.render.com/blueprints` + `blueprint/new` (create) are live | `/blueprints/$blueprintId` | alias → redirect (w3/m677; sub-path, query and hash kept; `/blueprint/new` stays the create page); a sync's own page `/blueprints/$blueprintId/sync/$syncId` (w3/m678) |
| Workflow | `/wf/{id}` | CLI switch | — | non-goal (workflows off-roadmap, `.pm/DO_NOT_DO.md`) |

## Service sub-tab map

Render's service route table (its `v0` in `index-D5V4ZgbE.js`, read 2026-11-27) serves every service segment it routes, `/web`, `/pserv`, `/worker` and `/static`; a cron job has a table of its own ([cron-dashboard.md](cron-dashboard.md)). Each Render sub-path and the bex page it opens (w3/m707; earlier rows from the w3/m3, w5/m13 and w9/m1 captures and the m32 walk, [dashboard-walk/services.md](dashboard-walk/services.md)):

| Render sub-path | Render's page | bex | Note |
| --- | --- | --- | --- |
| (root) | Deploys | `services.$serviceId.index.tsx` → Deploys | match (w5/m36); a cron job's Runs, a static site's Events |
| `*` (any other) | the root, or Settings for a member who may not view events | `services.$serviceId.$` → the type's primary tab | match (w3/m707) |
| `events` | Events | `services.$serviceId.events.tsx` | match; `?et=` [below](#renders-deep-link-queries) |
| `logs` | Logs | `services.$serviceId.logs.tsx` | match; `?t=`, `?q=`, `?l=`, `?r=` [below](#renders-deep-link-queries) |
| `env` | Environment | `services.$serviceId.env.tsx` | match |
| `headers`, `redirects` | a static site's Headers and Redirects/Rewrites | `…headers.tsx`, `…redirects.tsx` | match |
| `shell` | Shell | `services.$serviceId.shell.tsx` | match (w2/m55) |
| `shell/:instanceId` | Shell on that instance, named by its id's last segment | `/shell?instance=<service id>-<suffix>` | match (w3/m707): bex's `?instance=` takes the whole id |
| `metrics` | Metrics | `services.$serviceId.metrics.tsx` | match; `?t=` [below](#renders-deep-link-queries) |
| `metrics/:period` | Metrics, its path rewritten to `?t=<range>` from the period table below | `/metrics?range=<range>` | match (w3/m707) |
| `settings` | Settings | `services.$serviceId.settings.tsx` | match |
| `builds/:buildId` | the failed deploy of that build, else the root with `?et=DeployEnded` | `services.$serviceId.builds.$.tsx`: a cron job's deploy, another type's primary tab | match (w3/m554) |
| `deploys` | Deploys | `services.$serviceId.deploys.index.tsx` | match |
| `deploys/:deployId/*` | the deploy's page; `rollback` its Confirm Rollback | `…deploys.$deployId.tsx`, its `rollback` page; another sub-path the deploy's page | match (w1/m646, w3/m707) |
| `disks` | Disk | `services.$serviceId.disk.tsx` | match (w3/m707): bex's path is singular |
| `plan` | the plan chooser | `services.$serviceId.plan.tsx` → Compute; a cron job's own plan page | match (w6/m483, w3/105) |
| `scaling` | Scaling | `services.$serviceId.scaling.tsx` → `/compute#manual-scaling` | match (w6/m483) |
| `compute` | Compute | `services.$serviceId.compute.tsx` | match ([compute-page.md](compute-page.md)) |
| `jobs` | One-Off Jobs | the service's root | non-goal (DO_NOT_DO: one-off jobs) |
| `pulls`, `previews` | Previews | the service's root | non-goal (DO_NOT_DO: PR preview environments) |
| `sharing` | Render's deprecated per-resource sharing | the service's root | flagged (`grandfather-owners-into-sharing`) |

The aliases copy the sub-path, `/web/{srv-id}/<rest>` (and `worker`, `pserv`, `cron`) → `/services/{srv-id}/<rest>` with the query and hash, and the canonical tree maps the names that are not bex's: `services.$serviceId.$` and `static.$serviceId.$` land Render's `disks`, `metrics/:period`, `shell/:instanceId` and a deploy's sub-paths on bex's pages (`serviceSubpathLanding`), as a datastore's tree does (w3/m641, w3/m649). Until w3/m707 the alias relied on the names agreeing, and those four reached bex's not-found page. Render serves the same table under `/static/<id>`; bex's static-site tree has Events, Deploys, Environment, Headers, Redirects, Metrics and Settings, so `static.$serviceId.$` opens `metrics/:period` and a deploy's sub-paths there and lands any other page on the static site's Events, while a service of another type under `/static/` goes to its page under `/services`.

### Render's legacy Metrics periods

Render's `/metrics/:period` predates its `?t=` window. Its `xSe` replaces the period in the path with `?t=` from this table (`eC`, in its `service-metrics-utils` chunk), and drops a period the table lacks, which opens the default window (Last 12 hours). bex opens the same window on every Metrics page that has the route: a service's, a static site's, a Postgres database's and a Key Value instance's.

| Period             | Window |
| ------------------ | ------ |
| `5m`, `15m`, `30m` | `30m`  |
| `1h`               | `1h`   |
| `4h`               | `4h`   |
| `12h`              | `12h`  |
| `24h`              | `24h`  |
| `2d`               | `2d`   |
| `1w`               | `7d`   |

### Render's deep-link queries

Render keeps a page's filters in its URL, under short keys (its `log-filters`, `src` and `serviceEventsPicker` chunks), and its own cards link them. bex reads them on entry; its own keys win when a link carries both, and Render's leave the URL at the page's first write (w7/m42, w3/m707).

| Page | Render key | Value | bex |
| --- | --- | --- | --- |
| Logs | `t` | `all`, `app`, `request` | `type` (w7/m42) |
| Logs | `q` | the search text | `text` (w3/m707) |
| Logs | `l` | JSON `[[label, value], …]`, labels `INSTANCE`, `LEVEL`, `METHOD`, `STATUS_CODE`, `HOST`, `PATH` | `instance`, `level`, `method`, `statusCode`, `host`, `path`: each label's first value; another label is dropped (w3/m707) |
| Logs | `r` | `live`, a preset, or `YYYY-MM-DD@HH:mm:ss~YYYY-MM-DD@HH:mm:ss` in UTC | `range`: Live tail (w3/m537), a preset (w7/m42), or the custom window (w3/m707) |
| Logs | `m`, `c`, `p` | full-screen mode, and View in context's line id and time | not read: bex's View in context writes its own `contextLine` |
| Metrics | `t` | a preset, or `<startMs>,<endMs>` | `range`, or the custom window, on every Metrics page (w3/m707) |
| Metrics | `i`, `u`, `a`, `r`, `l`, `s`, `h`, `p`, `m` | instances, unit, aggregations, status codes, hosts, paths, selected metrics | not read: w3/161 |
| Events | `et` | comma-separated GraphQL type names, `DeployEnded,ServerFailed` | the filter opens showing those types (`eventTypesHiddenBy`; Render's `ExtraInstancesChanged` is bex's `instance_count_changed`); bex does not write it back: w3/160 |

Render's own links into them: an Instance failed event's Debug menu links `…/logs?l=[["INSTANCE","<instance id>"]]&r=<an hour before>~<a minute after>`, `…/metrics?i=<slug>-<suffix>&t=<startMs>,<endMs>` and `…/shell/<suffix>` (bex has no such menu: w3/162); a cron run links `…/logs?r=<its window>`, and Run now opens `…/logs?r=live`; Metrics' Open in logs carries its window into Logs' `r`. Postgres and Key Value Logs take bex's keys only: w3/163.

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
| `/web/new`, `/static/new`, `/pserv/new`, `/cron/new`, `/worker/new` | New-menu service creates live under the type segment | served at these paths (2026-11-22; one `_serviceCreate` layout, so a type switch keeps the draft); the old `/services/new?type=…` redirects here | match |
| `/d/new` | New menu's "Postgres" entry | **broken**: the `d.$` alias redirected it to nonexistent `/databases/new` → 404 catch-all | special-cased to the database create landing |
| `/new/database`, `/new/redis`, `/new/project` | New-menu datastore/project creates live under `/new/` | 404 | `/new/redis` and `/new/database` are the create pages (2026-11-22; the old `/keyvalue/new` and `/?new=database` redirect to them); project → the URL-owned create dialog on `/` (`?new=project`) |
| `/workflow/new`, `/wf/{id}` | New menu / CLI switch | 404 | non-goal (workflows off-roadmap) |
| `/login`, `/register` | auth pages | `/auth/login`, `/auth/sign-up` | thin redirects (query/hash preserved — an `?invite=` token must survive) |
| `/observability`, `/private-links`, `/dedicated-ips`, `/invites`, `/documents` | sidebar/footer pages | 404 | non-goals (drains, managed-infra, referral program, compliance docs — `.pm/DO_NOT_DO.md`) |

## Sidebar navigation (2026-07-16 live probes, w1/m45)

**Global sidebar** — identical on `/`, workspace settings, and account settings: ungrouped **Projects** (`/`), **Blueprints**, **Environment Groups**; group **Integrations** — Observability, Webhooks, Notifications; group **Networking** — Private Links, Dedicated IPs; group **Workspace** — Billing, Settings; footer — Changelog, Invite a friend, Contact support, Render Status. bex mapping (m45): keep the ungrouped trio; Integrations carries Webhooks + Notifications only (Observability = drains non-goal); the Networking group is omitted entirely (both entries non-goals); Workspace carries Usage (Billing counterpart) + Settings; the footer is out of scope (not capability navigation).

**Service pages** — Render **replaces** the global sidebar with a resource-scoped one: a "Dashboard" back link (`/`), then **Events**, **Settings**, group **Monitor** — Logs, Metrics; group **Manage** — Environment, Shell, Scaling, Previews, Disk, One-Off Jobs. Notably **no Deploys entry**: the service root (`/{seg}/{id}`) _is_ the deploy history. bex mapping (m45 + running-instance SSH follow-up): the same sidebar-swap pattern (precedent: the project sidebar), with Events / Settings, Monitor{Logs, Metrics}, Manage{Environment, Shell, Scaling, Plan (bex-only)}. bex's Shell route presents its copy-ready running-instance SSH command and SSH-key setup; it does not implement Render's browser-hosted terminal or ephemeral cron shell. Previews, Disk, and One-Off Jobs remain non-goals.

**Datastore pages** — a Postgres database's sidebar is Info / Monitor: Logs, Metrics / Manage: Plan, Apps, Recovery, each page at its own `/d/<id>/<page>` ([postgres-dashboard.md](postgres-dashboard.md)). bex matches it since w3/m641: the same sidebar and pages at `/databases/<id>/<page>`, the bare URL and `/info` on Info, and the old `?tab=logs` and section anchors landing on their page. The m32 walk had ruled bex's consolidated `?tab=` page an IA-only difference ("not a gap"); that ruling predated the parity-first rule, was never signed off in ADR018, and no longer stands. A Key Value instance's sidebar is Info / Monitor: Logs, Metrics, each page at its own `/r/<id>/<page>`, with `/plan` a full-page plan chooser ([key-value.md § Pages](key-value.md#pages-sidebar-routes-and-info-w3m649-2026-11-25)). bex matches it since w3/m649: the same sidebar and pages at `/keyvalue/<id>/<page>`, the bare URL and `/info` on Info, `/plan` on Info's plan card, and the old `?tab=logs`, `?tab=metrics` and section anchors landing on their page.

**Blueprint pages** — a Blueprint's sidebar is Resources, Syncs and Settings in one group with no label, under a **Blueprints** back link, each page at its own `/blueprint/<id>/<page>`, with Resources also the bare URL's page and Syncs also open on a sync's own `/sync/<syncId>` ([blueprint-pages.md](blueprint-pages.md)). bex matches it since w3/m677: the same sidebar and pages at `/blueprints/<id>/<page>`, Render's `/blueprint/<id>/…` redirecting there, a sync's own page at `/sync/<id>` with Syncs open in the sidebar (w3/m678), and the header's one action, Manual sync.

## Render's URL inventory

Every path Render's dashboard bundle builds or routes, with what bex does about each (w3/m704). Read 2026-11-27 from the bundle whose entry is `index-D5V4ZgbE.js`: 459 chunks and 385 URL patterns.

`scripts/render-dashboard-ops.sh list` collects them from four places, crawling the bundle as [dashboard-operations.md](dashboard-operations.md) describes. It reads the chunks as text and never runs them.

- **builder**: each export of the `urls-*.js` chunk, Render's one builder per page (`` e=>`/env-group/${e.id}` ``), called with run-time arguments. 159 of its 185 declarations build a path; the rest are helpers (URL checks, a query-string builder) and bare segments (`rollback`, `team-invite`).
- **route**: each route object's `path` in Render's router tables, a child joined to its parent as React Router matches it. A path is often a builder call or a constant imported from the `urls` chunk under a local name (`path:na(':serviceTypeSlug')`, a TypeScript enum for the service types), and the parser resolves both.
- **link**: a `path` elsewhere that evaluates to a dashboard path, such as the New menu's entries (`{title:'Web Service', path:na(al.web)}`). The key also holds form-field paths and log filters, so nothing else under it counts.
- **nav**: side navigation's `hrefPatterns`, the paths that select a sidebar entry.

Patterns are normalized. A segment known only at run time, or a router's `:name`, reads `:param`, and a trailing `*` stays. The query is split off, and `list` records its parameter names (`plan` on `/billing/update-plan`, `type` on `/select-repo`). A hash stays in the pattern, because Render links to sections of a page (`/billing#free-usage`). Assets, API paths and external URLs are not rows.

`list` also prints, under `unresolved_urls`, each route or navigation expression it could not read: route objects built by a call (the workflow routes' factory), descendant `<Routes>` inside a component (a deploy's page under `/deploys/:id/*`), and sidebar patterns built from a local variable. The weekly job prints each one as a notice and never fails on it, because none is a URL it can compare.

The `render-dashboard-ops` job in `.github/workflows/render-schema-drift.yml` fails when the bundle has a URL with no row below, when a row's URL leaves the bundle, or when a row is malformed. Rows use the [dispositions of dashboard-operations.md](dashboard-operations.md#dispositions). A `matched` or `partial` row names the bex route in its bex column, and `—` means bex has no route for the URL. The job never writes this table.

| Render URL | bex | Disposition | Pointer |
| --- | --- | --- | --- |
| `/` | `/` | matched | the projects home, `routes/index.tsx` |
| `/*` | `/$` → bex's not-found page | gap | Render draws its home page for any path it does not route: `w3/146` |
| `/:param/:param` | the service aliases → `/services/$serviceId` | matched | w5/m39 aliases for `web`, `worker`, `pserv`, `static`, `cron` (`common/lib/render-alias.ts`) |
| `/:param/:param/definitions/*` | — | non-goal | a workflow's sidebar pattern; DO_NOT_DO: workflows & tasks |
| `/:param/:param/deploys` | the service aliases → `/services/$serviceId/deploys` | matched | w5/m39 aliases (`common/lib/render-alias.ts`) |
| `/:param/:param/deploys/:param` | the service aliases → `/services/$serviceId/deploys/$deployId` | matched | w5/m39 aliases (`common/lib/render-alias.ts`) |
| `/:param/:param/deploys/:param/*` | the service aliases → `/services/$serviceId/deploys/$deployId/…` | matched | w5/m39 aliases (`common/lib/render-alias.ts`); a sub-path other than `rollback` opens the deploy's page, as Render's `deploys/:deployId/*` does (`serviceSubpathLanding`, w3/m707) |
| `/:param/:param/deploys/:param/rollback` | the service aliases → `/services/$serviceId/deploys/$deployId/rollback` | matched | w5/m39 aliases (`common/lib/render-alias.ts`) |
| `/:param/:param/logs` | the service aliases → `/services/$serviceId/logs` | matched | w5/m39 aliases (`common/lib/render-alias.ts`) |
| `/:param/:param/runs/*` | — | non-goal | a workflow's sidebar pattern; DO_NOT_DO: workflows & tasks |
| `/:param/:param/tasks/*` | — | non-goal | a workflow's sidebar pattern; DO_NOT_DO: workflows & tasks |
| `/:param/:param/versions/*` | — | non-goal | a workflow's sidebar pattern; DO_NOT_DO: workflows & tasks |
| `/:param/new` | `/web/new`, `/worker/new`, `/pserv/new`, `/static/new`, `/cron/new`; any other name `$serviceTypeSlug.new` → its create page | matched | `routes/_serviceCreate.*.new.tsx` (2026-11-22); Render's create slug table (`/postgres/new`, `/kv/new`, `/background-worker/new`, …) read case-insensitively, anything else → `/new`'s landing (`routes/$serviceTypeSlug.new.tsx`, w3/m705); `/workflow/new` has its own row |
| `/billing` | `/billing` | matched | bex's Billing page (w5/m70) |
| `/billing#credit-balance` | `/billing#credit-balance` | matched | `features/usage/components/usage-page.tsx` |
| `/billing#free-usage` | `/billing#free-usage` → the Included usage card | matched | `usage-page.tsx` (w3/m707): no card on Render's own page carries the id (its Monthly Included Usage card is `#included-usage`), so bex scrolls to the card the link names |
| `/billing#invoice-history` | `/billing#invoice-history` | matched | `features/usage/components/usage-page.tsx` |
| `/billing#payment-method` | `/billing#payment-method` | matched | `features/usage/components/usage-page.tsx` |
| `/billing#unbilled-charges` | `/billing#unbilled-charges` → the Charges card | matched | `usage-page.tsx` (w3/m707): Render's Unbilled Charges card is bex's Charges |
| `/billing/*` | `billing_.$first.$` → `/billing` | matched | w1/m45 |
| `/billing/update-plan` | `billing_.$first.$` → the change-plan dialog | partial | Render's plan page: `w3/m708` |
| `/billing/update-plan-confirm` | `billing_.$first.$` → `/billing` | gap | `w3/m708` |
| `/billing/update-plan-preview` | `billing_.$first.$` → `/billing` | gap | `w3/m708` |
| `/bitbucket/exists` | — | non-goal | DO_NOT_DO: Bitbucket git-provider integration |
| `/bitbucket/login-exists` | — | non-goal | DO_NOT_DO: Bitbucket git-provider integration |
| `/blueprint/*` | `blueprint.$` → `/blueprints/…` | matched | w3/m677 |
| `/blueprint/:param` | `blueprint.$` → `/blueprints/$blueprintId` | matched | w3/m677 |
| `/blueprint/:param/*` | `blueprint.$` → `/blueprints/$blueprintId/…` | matched | w3/m677 |
| `/blueprint/:param/resources` | `blueprint.$` → `/blueprints/$blueprintId/resources` | matched | w3/m677 |
| `/blueprint/:param/settings` | `blueprint.$` → `/blueprints/$blueprintId/settings` | matched | w3/m677 |
| `/blueprint/:param/sync` | `blueprint.$` → `/blueprints/$blueprintId/sync` | matched | w3/m677; Syncs open in the sidebar (w3/m678) |
| `/blueprint/:param/sync/:param` | `blueprint.$` → `/blueprints/$blueprintId/sync/$syncId` | matched | a sync's own page (w3/m678) |
| `/blueprint/:param/syncs` | `blueprint.$` → `/blueprints/$blueprintId/syncs` | matched | w3/m677 |
| `/blueprint/generate` | `/blueprint/generate` | matched | `routes/blueprint.generate.tsx` (w3/m681) |
| `/blueprint/new` | `/blueprint/new` | matched | `routes/blueprint.new.tsx` (w3/m681) |
| `/blueprints` | `/blueprints` | matched | `routes/blueprints.tsx` |
| `/blueprints/new` | `/blueprints/new` | matched | `routes/blueprints.new.tsx` |
| `/build-sources` | `/build-sources` | matched | `routes/build-sources.tsx` |
| `/build-sources/:param` | `/build-sources/$buildSourceId` | matched | `routes/build-sources_.$buildSourceId*.tsx` |
| `/build-sources/:param/*` | `/build-sources/$buildSourceId/…` | matched | `routes/build-sources_.$buildSourceId*.tsx` |
| `/build-sources/:param/builds` | `build-sources_.$buildSourceId.builds.index` → `/build-sources/$buildSourceId`, `?build=<id>` → `…/builds/$buildId` | matched | the builds list is the page's index and a build has its own page; the route came with w3/m705, whose router test found this row answered by bex's not-found page |
| `/build-sources/:param/environment` | `/build-sources/$buildSourceId/environment` | matched | `routes/build-sources_.$buildSourceId*.tsx` |
| `/build-sources/:param/services` | `/build-sources/$buildSourceId/services` | matched | `routes/build-sources_.$buildSourceId*.tsx` |
| `/build-sources/:param/settings` | `/build-sources/$buildSourceId/settings` | matched | `routes/build-sources_.$buildSourceId*.tsx` |
| `/build-sources/new` | `/build-sources/new` | matched | `routes/build-sources_.new.tsx` |
| `/cron/:param` | `cron.$` → `/services/$serviceId` | matched | w5/m39 alias; `services.$serviceId.tsx` |
| `/cron/:param/*` | `cron.$` → `/services/$serviceId/$rest` | matched | w5/m39 alias keeps the sub-path (`common/lib/render-alias.ts`) |
| `/cron/:param/builds` | `cron.$` → `/services/$serviceId/builds` | matched | `services.$serviceId.builds.$.tsx` lands on Deploys, as Render's does (w3/m554) |
| `/cron/:param/builds/*` | `cron.$` → `/services/$serviceId/builds/$` | matched | `services.$serviceId.builds.$.tsx` (w3/m554) |
| `/cron/:param/builds/:param` | `cron.$` → `/services/$serviceId/builds/$` | matched | `services.$serviceId.builds.$.tsx` (w3/m554) |
| `/cron/:param/deploys` | `cron.$` → `/services/$serviceId/deploys` | matched | `services.$serviceId.deploys.index.tsx` |
| `/cron/:param/deploys/*` | `cron.$` → `/services/$serviceId/deploys/…` | matched | `services.$serviceId.deploys.index.tsx` and `…deploys.$deployId.tsx` |
| `/cron/:param/deploys/:param` | `cron.$` → `/services/$serviceId/deploys/$deployId` | matched | `services.$serviceId.deploys.$deployId.tsx` |
| `/cron/:param/env` | `cron.$` → `/services/$serviceId/env` | matched | `services.$serviceId.env.tsx` |
| `/cron/:param/events` | `cron.$` → `/services/$serviceId/events` | matched | `services.$serviceId.events.tsx` |
| `/cron/:param/jobs` | `cron.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: one-off jobs (ADR018 parity non-goals) |
| `/cron/:param/logs` | `cron.$` → `/services/$serviceId/logs` | matched | `services.$serviceId.logs.tsx` |
| `/cron/:param/metrics` | `cron.$` → `/services/$serviceId/metrics` | matched | `services.$serviceId.metrics.tsx` |
| `/cron/:param/metrics/:param` | `cron.$` → `services.$serviceId.$` → `/services/$serviceId/metrics?range=…` | matched | `serviceSubpathLanding` (w3/m707): the window Render's period table maps the period to |
| `/cron/:param/plan` | `cron.$` → `/services/$serviceId/plan` | matched | `services.$serviceId.plan.tsx` lands on Compute (w6/m483) |
| `/cron/:param/runs` | `cron.$` → `/services/$serviceId/runs` | matched | `services.$serviceId.runs.tsx` |
| `/cron/:param/settings` | `cron.$` → `/services/$serviceId/settings` | matched | `services.$serviceId.settings.tsx` |
| `/cron/:param/sharing` | `cron.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | flagged | Render's deprecated per-resource sharing, `grandfather-owners-into-sharing` |
| `/cron/:param/shell` | — | non-goal | DO_NOT_DO: cron and ephemeral shells (ADR018 parity non-goals) |
| `/cron/new` | `/cron/new` | matched | `routes/_serviceCreate.cron.new.tsx` (w1/m45) |
| `/d/:param` | `d.$` → `/databases/$databaseId` | matched | w5/m39 alias; `databases.$databaseId.tsx` |
| `/d/:param#delete-database` | — | gap | Info has no `delete-database` anchor: `w3/147` |
| `/d/:param/*` | `d.$` → `/databases/$databaseId/…` | matched | w5/m39 alias keeps the sub-path |
| `/d/:param/apps` | `d.$` → `/databases/$databaseId/apps` | matched | w3/m641 |
| `/d/:param/high-availability-upgrade` | `d.$` → `/databases/$databaseId/plan#high-availability` | matched | `databaseSubpathLanding` (w3/m641) |
| `/d/:param/info` | `d.$` → `/databases/$databaseId/info` | matched | w3/m641 |
| `/d/:param/logs` | `d.$` → `/databases/$databaseId/logs` | matched | w3/m641 |
| `/d/:param/metrics` | `d.$` → `/databases/$databaseId/metrics` | matched | w3/m641 |
| `/d/:param/metrics/:param` | `d.$` → `/databases/$databaseId/metrics?range=…` | matched | `databaseSubpathLanding` (w3/m641), on the window Render's period table maps the period to (w3/m707) |
| `/d/:param/plan` | `d.$` → `/databases/$databaseId/plan` | matched | w3/m641 |
| `/d/:param/recovery` | `d.$` → `/databases/$databaseId/recovery` | matched | w3/m641 |
| `/d/:param/recovery/new/database` | — | gap | Render's restore-to-a-new-database form; bex lands on Info: `w3/148` |
| `/d/:param/sharing` | — | flagged | Render's grandfathered database sharing, `share-postgres` with `grandfather-owners-into-sharing` |
| `/d/:param/upgrade` | `d.$` → `/databases/$databaseId/plan` | matched | `databaseSubpathLanding` (w3/m641) |
| `/d/:param/version-upgrade` | `d.$` → `/databases/$databaseId/version-upgrade` | matched | `databases.$databaseId.version-upgrade.tsx` (w3/m676) |
| `/dedicated-ips` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/dedicated-ips/*` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/dedicated-ips/:param` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/dedicated-ips/:param/*` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/dedicated-ips/:param/environments` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/dedicated-ips/:param/settings` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/dedicated-ips/new` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/device-authorization/:param` | — | n/a | the pinned Render CLI opens the `verification_uri_complete` bex-api returns (`/auth/device`), never a Render-shaped page (`pkg/client/oauth/oauth.go`) |
| `/documents` | — | n/a | Render's Document Center (signed NDAs and compliance reports): Render's legal paperwork |
| `/documents/*` | — | n/a | Render's Document Center (signed NDAs and compliance reports): Render's legal paperwork |
| `/email-confirm` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/email-confirm-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/email-reset/confirm` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/env-group/*` | `env-group.$` → `/env-groups/$groupId` | matched | w3/m705: Render's group has one page, so a sub-path lands on it (`routes/env-group.$.tsx`) |
| `/env-group/:param` | `env-group.$` → `/env-groups/$groupId` | matched | w3/m705 (`routes/env-group.$.tsx`); query and hash kept |
| `/env-groups` | `/env-groups` | matched | `routes/env-groups.tsx` |
| `/github/authorization-expired` | — | n/a | an error page of Render's own GitHub sign-in and connect callbacks; bex's are `/auth/error` and its GitHub App flow (ADR026) |
| `/github/connect-forbidden` | — | n/a | an error page of Render's own GitHub sign-in and connect callbacks; bex's are `/auth/error` and its GitHub App flow (ADR026) |
| `/github/different-account` | — | n/a | an error page of Render's own GitHub sign-in and connect callbacks; bex's are `/auth/error` and its GitHub App flow (ADR026) |
| `/github/exists` | — | n/a | an error page of Render's own GitHub sign-in and connect callbacks; bex's are `/auth/error` and its GitHub App flow (ADR026) |
| `/github/login-exists` | — | n/a | an error page of Render's own GitHub sign-in and connect callbacks; bex's are `/auth/error` and its GitHub App flow (ADR026) |
| `/gitlab/exists` | — | non-goal | DO_NOT_DO: GitLab git-provider integration |
| `/gitlab/login-exists` | — | non-goal | DO_NOT_DO: GitLab git-provider integration |
| `/i/*` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/i/:param/view` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/i/:param/view/*` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/integrations/*` | — | non-goal | DO_NOT_DO: external log and metric drains |
| `/invites` | — | n/a | Render's referral program (Invite a friend): growth marketing, not a product surface |
| `/jules` | — | gap | Render sends it to `/u/settings#api-keys`: `w3/146` |
| `/login` | `/login` → `/auth/login` | matched | w1/m45; query and hash kept |
| `/login-again` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/login-as` | — | n/a | Render's staff impersonation |
| `/login-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/login-token` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/login/oauth/authorize` | — | n/a | an OAuth authorization endpoint clients read from the server's metadata; bex publishes Hydra's (ADR012) |
| `/login/sso` | — | non-goal | DO_NOT_DO: SSO / SAML (ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—)) |
| `/login/sso-next` | — | non-goal | DO_NOT_DO: SSO / SAML (ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—)) |
| `/network-access` | `network-access.$` → `/workspace/settings#inbound-ip-rules` | partial | w3/m705: Render's page holds one panel, Inbound IP Restrictions, which is bex's Inbound IP Rules card; a page of its own under a Networking group: `w3/155` |
| `/networking/*` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/new` | `new.$` → `/web/new` | partial | w3/m705: New Web Service, whose type switcher lists every type; Render's "Create a new Service" chooser page: `w3/154` |
| `/new/:param` | `new.$` → the slug's create page | matched | w3/m705: Render's create slug table read case-insensitively (`newPathLanding`), `team` → `/new/workspace`, anything else → `/new`'s landing; on Render a breadcrumb pattern of the New Service page, whose slug route is `/:param/new` |
| `/new/database` | `/new/database` | matched | `routes/new.database.tsx` (2026-11-22) |
| `/new/env-group` | `/new/env-group` → `/env-groups?new=env-group`, the list's New Environment Group dialog | partial | w3/m705 (`routes/new.env-group.tsx`); Render's full page: `w1/m684` (t003) |
| `/new/project` | `/new/project` | matched | `routes/new.project.tsx` |
| `/new/redis` | `/new/redis` | matched | `routes/new.redis.tsx` (2026-11-22) |
| `/new/team` | `new.$` → `/new/workspace` | matched | w3/m705: Render's `NewTeam` page serves `/new/team` and `/new/workspace` |
| `/new/workspace` | `/new/workspace` | matched | `routes/new.workspace.tsx` |
| `/notifications` | `/notifications` | matched | `routes/notifications.tsx` |
| `/oauth` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/oauth/*` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/oauth/consent` | — | n/a | the consent page Render's OAuth server redirects to; bex's MCP clients reach Hydra's through `/auth/consent` (ADR012) |
| `/oauth/cursor-origin` | — | flagged | Cursor Origin as a Git provider, `cursor_origin` |
| `/observability` | — | non-goal | DO_NOT_DO: external log and metric drains |
| `/observability#log-streams` | — | non-goal | DO_NOT_DO: external log and metric drains |
| `/observability#metrics-stream` | — | non-goal | DO_NOT_DO: external log and metric drains |
| `/observability/*` | — | non-goal | DO_NOT_DO: external log and metric drains |
| `/org-invite/:param` | — | n/a | the link in Render's own organization invitation email; bex's mail bex's own (ADR115) |
| `/organization/:param` | `organization.$` → `/org/$orgId/workspaces` | matched | w3/m705: Workspaces is Render's page for the org; `/org/<id>` refuses an org the caller is not in |
| `/organization/:param/*` | `organization.$` → `/org/$orgId/workspaces` | matched | w3/m705: Render's `/organization/:id/*` route draws Workspaces |
| `/organization/:param/billing` | `organization.$` → the org's primary workspace's `/billing` | partial | w3/m705: through `/w/<primary>/billing`, which checks the caller is in it; Render's org Billing (a default payment method and billing email for the org's workspaces) is behind `org_default_payment_method`, and bex bills each org workspace on its own payment method, ADR018 § Billing & invoicing: Who sees and manages billing (ADR115 D2) |
| `/organization/:param/members` | `organization.$` → `/org/$orgId/members` | matched | w3/m705 |
| `/organization/:param/new/workspace` | `organization.$` → `/org/$orgId/workspaces?new=workspace`, the New Workspace dialog | partial | w3/m705; Render's New Workspace page with the org chosen: `w3/157` |
| `/organization/:param/settings` | `organization.$` → `/org/$orgId/settings` | matched | w3/m705 |
| `/password-reset` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/password-reset-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/password-reset/confirm` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/password-reset/confirm-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/pending-invoice/*` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/pending-invoice/:param/:param/view` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/pending-invoice/:param/:param/view/*` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/private-link/:param` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/private-link/:param/*` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/private-link/:param/environments` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/private-link/:param/settings` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/private-links` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/private-links/*` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/private-links/new` | — | gap | Render's private links have no disposition yet: `w7/071` |
| `/project/:param` | `/project/$projectId` | matched | w1/m682 |
| `/project/:param/*` | `/project/$projectId/…` | matched | bex's project pages, `routes/project.$projectId*.tsx` |
| `/project/:param/:param/new` | — | gap | Render's project-scoped create URLs: `w3/148` |
| `/project/:param/environment/:param` | `/project/$projectId/environment/$environmentId` | matched | the project page with the section outlined (w1/m682) |
| `/project/:param/environment/:param/*` | `/project/$projectId/environment/$environmentId/…` | matched | w1/m682, w1/m683 |
| `/project/:param/environment/:param/:param` | `…/environment/$environmentId/$page` → the project page | matched | Render's `:environmentPage` route (w1/m683) |
| `/project/:param/environment/:param/:param/new` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/new` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/new/:param` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/new/database` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/new/env-group` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/new/redis` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/sandbox-group/new` | — | gap | Render's environment create URLs: `w1/m684` |
| `/project/:param/environment/:param/select-repo` | — | gap | `w3/148` |
| `/project/:param/environment/:param/settings` | `/project/$projectId/environment/$environmentId/settings` | matched | w1/m683 |
| `/project/:param/environment/:param/settings#dedicated-ips` | — | non-goal | DO_NOT_DO: dedicated outbound IPs (ADR018 parity non-goals) |
| `/project/:param/environment/:param/settings#networking` | — | gap | the environment settings page has no `networking` anchor: `w3/147` |
| `/project/:param/new/database` | — | gap | Render's project-scoped create URLs: `w3/148` |
| `/project/:param/new/redis` | — | gap | Render's project-scoped create URLs: `w3/148` |
| `/project/:param/observability/*` | `/project/$projectId/observability/…` | matched | w1/m682: the project's logs and metrics pages |
| `/project/:param/observability/logs/application` | `/project/$projectId/observability/logs/application` | matched | `routes/project.$projectId.observability.logs.application.tsx` |
| `/project/:param/observability/logs/request` | `/project/$projectId/observability/logs/request` | matched | `routes/project.$projectId.observability.logs.request.tsx` |
| `/project/:param/observability/metrics` | `/project/$projectId/observability/metrics` | matched | `routes/project.$projectId.observability.metrics.tsx` |
| `/project/:param/observability/workflow-runs` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/project/:param/sandbox-group/new` | — | gap | Render's project-scoped create URLs: `w3/148` |
| `/project/:param/settings` | `/project/$projectId/settings` | matched | `routes/project.$projectId.settings.tsx` |
| `/pserv/:param/*` | `pserv.$` → `/services/$serviceId/$rest` | matched | w5/m39 alias keeps the sub-path (`common/lib/render-alias.ts`) |
| `/pserv/:param/builds/:param` | `pserv.$` → `/services/$serviceId/builds/$` | matched | `services.$serviceId.builds.$.tsx` (w3/m554) |
| `/pserv/:param/compute` | `pserv.$` → `/services/$serviceId/compute` | matched | `services.$serviceId.compute.tsx` (w6/m483) |
| `/pserv/:param/deploys` | `pserv.$` → `/services/$serviceId/deploys` | matched | `services.$serviceId.deploys.index.tsx` |
| `/pserv/:param/deploys/:param/*` | `pserv.$` → `/services/$serviceId/deploys/$deployId/…` | matched | `services.$serviceId.deploys.$deployId.tsx` and its `rollback` page; another sub-path opens the deploy's page (`serviceSubpathLanding`, w3/m707) |
| `/pserv/:param/disks` | `pserv.$` → `services.$serviceId.$` → `/services/$serviceId/disk` | matched | `serviceSubpathLanding` (w3/m707), Render's `disks` is bex's Disk page |
| `/pserv/:param/env` | `pserv.$` → `/services/$serviceId/env` | matched | `services.$serviceId.env.tsx` |
| `/pserv/:param/events` | `pserv.$` → `/services/$serviceId/events` | matched | `services.$serviceId.events.tsx` |
| `/pserv/:param/headers` | `pserv.$` → `/services/$serviceId/headers` | matched | `services.$serviceId.headers.tsx` |
| `/pserv/:param/jobs` | `pserv.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: one-off jobs (ADR018 parity non-goals) |
| `/pserv/:param/logs` | `pserv.$` → `/services/$serviceId/logs` | matched | `services.$serviceId.logs.tsx` |
| `/pserv/:param/metrics` | `pserv.$` → `/services/$serviceId/metrics` | matched | `services.$serviceId.metrics.tsx` |
| `/pserv/:param/metrics/:param` | `pserv.$` → `services.$serviceId.$` → `/services/$serviceId/metrics?range=…` | matched | `serviceSubpathLanding` (w3/m707): the window [Render's period table](#renders-legacy-metrics-periods) maps the period to |
| `/pserv/:param/plan` | `pserv.$` → `/services/$serviceId/plan` | matched | `services.$serviceId.plan.tsx` lands on Compute (w6/m483) |
| `/pserv/:param/previews` | `pserv.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/pserv/:param/pulls` | `pserv.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/pserv/:param/redirects` | `pserv.$` → `/services/$serviceId/redirects` | matched | `services.$serviceId.redirects.tsx` |
| `/pserv/:param/scaling` | `pserv.$` → `/services/$serviceId/scaling` | matched | `services.$serviceId.scaling.tsx` lands on Compute (w6/m483) |
| `/pserv/:param/settings` | `pserv.$` → `/services/$serviceId/settings` | matched | `services.$serviceId.settings.tsx` |
| `/pserv/:param/sharing` | `pserv.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | flagged | Render's deprecated per-resource sharing, `grandfather-owners-into-sharing` |
| `/pserv/:param/shell` | `pserv.$` → `/services/$serviceId/shell` | matched | `services.$serviceId.shell.tsx` (w2/m55) |
| `/pserv/:param/shell/:param` | `pserv.$` → `services.$serviceId.$` → `/services/$serviceId/shell?instance=<service id>-<suffix>` | matched | `serviceSubpathLanding` (w3/m707): Render names the instance by its id's last segment |
| `/pserv/new` | `/pserv/new` | matched | `routes/_serviceCreate.pserv.new.tsx` (w1/m45) |
| `/r/:param` | `r.$` → `/keyvalue/$keyValueId` | matched | w5/m39 alias; w3/m649 |
| `/r/:param/*` | `r.$` → `/keyvalue/$keyValueId/…` | matched | w5/m39 alias; w3/m649 |
| `/r/:param/info` | `r.$` → `/keyvalue/$keyValueId/info` | matched | w5/m39 alias; w3/m649 |
| `/r/:param/logs` | `r.$` → `/keyvalue/$keyValueId/logs` | matched | w5/m39 alias; w3/m649 |
| `/r/:param/metrics` | `r.$` → `/keyvalue/$keyValueId/metrics` | matched | w5/m39 alias; w3/m649 |
| `/r/:param/metrics/:param` | `r.$` → `/keyvalue/$keyValueId/metrics?range=…` | matched | w5/m39 alias; w3/m649; on the window Render's period table maps the period to (w3/m707) |
| `/r/:param/plan` | `r.$` → Info's plan card | partial | Render's full-page plan chooser: `w3/127` |
| `/register` | `/register` → `/auth/sign-up` | matched | w1/m45; query and hash kept |
| `/register-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/register/bitbucket` | — | non-goal | DO_NOT_DO: Bitbucket git-provider integration |
| `/register/bitbucket-next` | — | non-goal | DO_NOT_DO: Bitbucket git-provider integration |
| `/register/github` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/register/github-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/register/gitlab` | — | non-goal | DO_NOT_DO: GitLab git-provider integration |
| `/register/gitlab-next` | — | non-goal | DO_NOT_DO: GitLab git-provider integration |
| `/register/google` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/register/google-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/resource/:param` | `resource.$` → the page of the id's kind | matched | w3/m705 (`resourceAliasPath`): the kind read from the id's prefix, as Render's `ResourceRedirector` reads it; a `prj-` id, an unknown prefix and a short id go home, as on Render |
| `/resource/:param/*` | `resource.$` → the page of the id's kind, sub-path kept | matched | w3/m705; a sandbox's or a snapshot's opens its drawer as it opens |
| `/sandbox-group/:param` | `sandbox-group.$` → `/sandbox-groups/$sandboxGroupId` | matched | w3/m705 (`sandboxGroupAliasPath`); query and hash kept |
| `/sandbox-group/:param/*` | `sandbox-group.$` → `/sandbox-groups/$sandboxGroupId` | matched | w3/m705: a sub-path bex has no page for lands on Sandboxes |
| `/sandbox-group/:param/builds/*` | `sandbox-group.$` → `/sandbox-groups/$sandboxGroupId/images` | matched | w3/m705: Render's Custom images |
| `/sandbox-group/:param/builds/:param` | `sandbox-group.$` → `…/images?image=<name>` | matched | w3/m705: the image's drawer, on Versions |
| `/sandbox-group/:param/sandboxes/*` | `sandbox-group.$` → `/sandbox-groups/$sandboxGroupId` | matched | w3/m705: Sandboxes is the group's index |
| `/sandbox-group/:param/sandboxes/:param` | `sandbox-group.$` → `…?sandbox=<sbx-id>` | matched | w3/m705: the sandbox's drawer; Render's `configurations` and `snapshots` tabs open Details and Snapshots |
| `/sandbox-group/:param/settings` | `sandbox-group.$` → `/sandbox-groups/$sandboxGroupId/settings` | matched | w3/m705 |
| `/sandbox-group/:param/snapshots/*` | `sandbox-group.$` → `/sandbox-groups/$sandboxGroupId/snapshots` | matched | w3/m705 |
| `/sandbox-group/:param/snapshots/:param` | `sandbox-group.$` → `…/snapshots?snapshot=<snp-id>` | matched | w3/m705: the snapshot's drawer |
| `/sandbox-group/new` | `sandbox-group.$` → `/?new=sandbox-group`, the create dialog | partial | w3/m705: the dialog bex's New menu opens; Render's New Sandbox Group page: `w3/156` |
| `/select-plan` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/select-repo` | — | gap | Render's repository picker that starts a create: `w3/148` |
| `/services` | — | gap | Render draws its home page at `/services`: `w3/146` |
| `/settings` | `/settings` | matched | w1/m45 |
| `/settings/spend-limit` | `settings_.spend-limit` → `/workspace/settings#build-pipeline` | matched | w3/m705: Render's `AccountSpendLimitRedirect` sends a team's owner to its workspace settings' `#build-pipeline` |
| `/signup-survey` | — | flagged | Render's sign-up survey, `signup_survey_oct_2026` |
| `/sso` | — | non-goal | DO_NOT_DO: SSO / SAML (ADR018 § Identity, workspaces & account: SSO / SAML · SCIM (—)) |
| `/start` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/static/:param/*` | `/static/$serviceId/…` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/builds/:param` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/compute` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/deploys` | `/static/$serviceId/deploys` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/deploys/:param/*` | `/static/$serviceId/deploys/$deployId/…` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/disks` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/env` | `/static/$serviceId/env` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/events` | `/static/$serviceId/events` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/headers` | `/static/$serviceId/headers` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/jobs` | `static.$serviceId.$` → the static site's Events, as Render's `*` route | non-goal | DO_NOT_DO: one-off jobs (ADR018 parity non-goals) |
| `/static/:param/logs` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/metrics` | `/static/$serviceId/metrics` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/metrics/:param` | `static.$serviceId.$` → `/static/$serviceId/metrics?range=…` | matched | `serviceSubpathLanding` (w3/m707): the window Render's period table maps the period to |
| `/static/:param/plan` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/previews` | `static.$serviceId.$` → the static site's Events, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/static/:param/pulls` | `static.$serviceId.$` → the static site's Events, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/static/:param/redirects` | `/static/$serviceId/redirects` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/scaling` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/settings` | `/static/$serviceId/settings` | matched | bex's static-site pages, `routes/static.$serviceId*.tsx` (w5/m39) |
| `/static/:param/sharing` | `static.$serviceId.$` → the static site's Events, as Render's `*` route | flagged | Render's deprecated per-resource sharing, `grandfather-owners-into-sharing` |
| `/static/:param/shell` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/:param/shell/:param` | `static.$serviceId.$` → a static site's Events; another type's page under `/services` | matched | Render serves every type's sub-paths under `/static/<id>`; a static site has no such page in Render's sidebar or bex's, so `redirectServiceSubpath` (w3/m707) lands it on its root, as Render's `*` route does |
| `/static/new` | `/static/new` | matched | `routes/_serviceCreate.static.new.tsx` (w1/m45) |
| `/stripe-agentic-link` | — | n/a | Render's Stripe partner integrations: landing pages Stripe's own flows open |
| `/stripe/connect/*` | — | n/a | Render's Stripe partner integrations: landing pages Stripe's own flows open |
| `/stripe/new/database/*` | — | n/a | Render's Stripe partner integrations: landing pages Stripe's own flows open |
| `/stripe/new/database/confirmation/*` | — | n/a | Render's Stripe partner integrations: landing pages Stripe's own flows open |
| `/success/workspace/:param` | — | n/a | the return page of Render's own workspace checkout; bex's checkout returns to its own page |
| `/success/workspace/:param/*` | — | n/a | the return page of Render's own workspace checkout; bex's checkout returns to its own page |
| `/suspended` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/suspended-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/t/:param/settings` | `t.$` → `/w/$` → `/workspace/settings` | matched | w3/m705: Render's older team scheme, the same page as `/w/:id/settings` |
| `/t/:param/settings/*` | `t.$` → `/w/$` → `/workspace/settings` | matched | w3/m705 |
| `/team-invite/:param` | — | n/a | the link in Render's own invitation email; bex's invitations mail bex's `/invite` link (ADR024) |
| `/test-error` | — | n/a | Render's error-boundary test page |
| `/u/:param` | `u.$` → `/settings` | matched | w1/m45 |
| `/u/:param/*` | `u.$` → `/settings` | matched | w1/m45 |
| `/u/:param/settings` | `u.$` → `/settings` | matched | w1/m45 |
| `/u/:param/settings#account-security` | `u.$` → `/settings#account-security` → the Account section | matched | `settings-page/index.tsx` (w3/m707): Render's Account Security (password, sign-in methods, two-factor) is the Kratos flow's cards under bex's Account section |
| `/u/:param/settings#build-pipeline` | `u.$` → `/settings` | gap | bex's build pipeline is a workspace setting: `w3/147` |
| `/u/:param/settings#cli-tokens` | `u.$` → `/settings#cli-tokens` | matched | w1/m45; the hash is kept |
| `/u/:param/settings/*` | `u.$` → `/settings` | matched | w1/m45 |
| `/u/settings` | `u.$` → `/settings` | matched | w1/m45 |
| `/u/settings#account-security` | `u.$` → `/settings#account-security` → the Account section | matched | `settings-page/index.tsx` (w3/m707): Render's Account Security (password, sign-in methods, two-factor) is the Kratos flow's cards under bex's Account section |
| `/u/settings#build-pipeline` | `u.$` → `/settings` | gap | bex's build pipeline is a workspace setting: `w3/147` |
| `/u/settings#cli-tokens` | `u.$` → `/settings#cli-tokens` | matched | w1/m45; the hash is kept |
| `/u/settings/*` | `u.$` → `/settings` | matched | w1/m45 |
| `/usage/*` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/usage/:param/view` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/usage/:param/view/*` | — | gap | Render's invoice, usage and upcoming-invoice pages: `w3/149` |
| `/verify-email` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/verify-email-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/verify-otp` | — | n/a | reached only from Render's own sign-in flow and account emails; bex's are its own (`/auth/*`, ADR012) |
| `/verify-otp-next` | — | n/a | Render's second variant of its sign-in pages (`auth_variant: next`), an experiment beside the classic path's row |
| `/w/:param` | `w.$` → `/` | matched | w1/m45: the workspace is selected, membership checked |
| `/w/:param/*` | `w.$` → `/workspace/settings` | matched | w1/m45 |
| `/w/:param/billing` | `w.$` → `/billing` | matched | w1/m45; bex's Billing page since w5/m70 |
| `/w/:param/billing/*` | `w.$` → `/billing` | matched | w1/m45 |
| `/w/:param/billing/update-plan` | `w.$` → `/billing` | gap | Render's plan change pages: `w3/m708` |
| `/w/:param/billing/update-plan-confirm` | `w.$` → `/billing` | gap | Render's plan change pages: `w3/m708` |
| `/w/:param/billing/update-plan-preview` | `w.$` → `/billing` | gap | Render's plan change pages: `w3/m708` |
| `/w/:param/settings` | `w.$` → `/workspace/settings` | matched | w1/m45 |
| `/w/:param/settings#audit-logs` | `w.$` → `/workspace/settings`, anchor dropped | partial | the section exists; `w.$` drops the hash on a workspace id: `w3/147` |
| `/w/:param/settings#build-pipeline` | `w.$` → `/workspace/settings`, anchor dropped | partial | the section exists; `w.$` drops the hash on a workspace id: `w3/147` |
| `/w/:param/settings#compliance` | — | gap | Render's HIPAA-enabled workspaces: `w7/071` |
| `/w/:param/settings#deploy-policy` | `w.$` → `/workspace/settings` | gap | no section under Render's anchor, and `w.$` drops the hash: `w3/147` |
| `/w/:param/settings#hipaa-compliance` | — | gap | Render's HIPAA-enabled workspaces: `w7/071` |
| `/w/:param/settings#invite-team-members` | `w.$` → `/workspace/settings` | gap | no section under Render's anchor, and `w.$` drops the hash: `w3/147` |
| `/w/:param/settings#registry-credentials` | `w.$` → `/workspace/settings`, anchor dropped | partial | the section exists; `w.$` drops the hash on a workspace id: `w3/147` |
| `/w/:param/settings#security` | `w.$` → `/workspace/settings` | gap | no section under Render's anchor, and `w.$` drops the hash: `w3/147` |
| `/w/:param/settings#team-members` | `w.$` → `/workspace/settings` | gap | no section under Render's anchor, and `w.$` drops the hash: `w3/147` |
| `/w/:param/settings/*` | `w.$` → `/workspace/settings` | matched | w1/m45 |
| `/w/settings` | `w.$` → `/workspace/settings` | matched | w1/m45 |
| `/w/settings/*` | `w.$` → `/workspace/settings` | matched | w1/m45 |
| `/web/:param` | `web.$` → `/services/$serviceId` | matched | w5/m39 alias; `services.$serviceId.tsx` |
| `/web/:param/*` | `web.$` → `/services/$serviceId/$rest` | matched | w5/m39 alias keeps the sub-path (`common/lib/render-alias.ts`) |
| `/web/:param/builds/:param` | `web.$` → `/services/$serviceId/builds/$` | matched | `services.$serviceId.builds.$.tsx` (w3/m554) |
| `/web/:param/compute` | `web.$` → `/services/$serviceId/compute` | matched | `services.$serviceId.compute.tsx` (w6/m483) |
| `/web/:param/deploys` | `web.$` → `/services/$serviceId/deploys` | matched | `services.$serviceId.deploys.index.tsx` |
| `/web/:param/deploys/:param/*` | `web.$` → `/services/$serviceId/deploys/$deployId/…` | matched | `services.$serviceId.deploys.$deployId.tsx` and its `rollback` page; another sub-path opens the deploy's page (`serviceSubpathLanding`, w3/m707) |
| `/web/:param/disks` | `web.$` → `services.$serviceId.$` → `/services/$serviceId/disk` | matched | `serviceSubpathLanding` (w3/m707), Render's `disks` is bex's Disk page |
| `/web/:param/env` | `web.$` → `/services/$serviceId/env` | matched | `services.$serviceId.env.tsx` |
| `/web/:param/events` | `web.$` → `/services/$serviceId/events` | matched | `services.$serviceId.events.tsx` |
| `/web/:param/headers` | `web.$` → `/services/$serviceId/headers` | matched | `services.$serviceId.headers.tsx` |
| `/web/:param/jobs` | `web.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: one-off jobs (ADR018 parity non-goals) |
| `/web/:param/logs` | `web.$` → `/services/$serviceId/logs` | matched | `services.$serviceId.logs.tsx` |
| `/web/:param/metrics` | `web.$` → `/services/$serviceId/metrics` | matched | `services.$serviceId.metrics.tsx` |
| `/web/:param/metrics/:param` | `web.$` → `services.$serviceId.$` → `/services/$serviceId/metrics?range=…` | matched | `serviceSubpathLanding` (w3/m707): the window [Render's period table](#renders-legacy-metrics-periods) maps the period to |
| `/web/:param/plan` | `web.$` → `/services/$serviceId/plan` | matched | `services.$serviceId.plan.tsx` lands on Compute (w6/m483) |
| `/web/:param/previews` | `web.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/web/:param/pulls` | `web.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/web/:param/redirects` | `web.$` → `/services/$serviceId/redirects` | matched | `services.$serviceId.redirects.tsx` |
| `/web/:param/scaling` | `web.$` → `/services/$serviceId/scaling` | matched | `services.$serviceId.scaling.tsx` lands on Compute (w6/m483) |
| `/web/:param/settings` | `web.$` → `/services/$serviceId/settings` | matched | `services.$serviceId.settings.tsx` |
| `/web/:param/sharing` | `web.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | flagged | Render's deprecated per-resource sharing, `grandfather-owners-into-sharing` |
| `/web/:param/shell` | `web.$` → `/services/$serviceId/shell` | matched | `services.$serviceId.shell.tsx` (w2/m55) |
| `/web/:param/shell/:param` | `web.$` → `services.$serviceId.$` → `/services/$serviceId/shell?instance=<service id>-<suffix>` | matched | `serviceSubpathLanding` (w3/m707): Render names the instance by its id's last segment |
| `/web/new` | `/web/new` | matched | `routes/_serviceCreate.web.new.tsx` (w1/m45) |
| `/webhook/:param` | `/webhook/$webhookId` | matched | `routes/webhook.$webhookId.tsx` |
| `/webhook/:param/*` | `/webhook/$webhookId/…` | matched | `routes/webhook.$webhookId*.tsx` |
| `/webhook/:param/activity` | `/webhook/$webhookId/activity` | matched | `routes/webhook.$webhookId.activity.tsx` (w3/m707): Activity at Render's path, its tab linking there; the bare URL opens it too |
| `/webhook/:param/settings` | `/webhook/$webhookId/settings` | matched | `routes/webhook.$webhookId.settings.tsx` |
| `/webhooks` | `/webhooks` | matched | `routes/webhooks.tsx` |
| `/webhooks/new` | `/webhooks/new` | matched | `routes/webhooks_.new.tsx` |
| `/wf/:param` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/*` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/definitions` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/definitions/:param` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/definitions/:param/run/:param` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/env` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/events` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/runs` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/runs/:param` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/settings` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/tasks` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/tasks/:param` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/versions/:param` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/wf/:param/versions/:param/*` | — | non-goal | DO_NOT_DO: workflows & tasks |
| `/worker/:param/*` | `worker.$` → `/services/$serviceId/$rest` | matched | w5/m39 alias keeps the sub-path (`common/lib/render-alias.ts`) |
| `/worker/:param/builds/:param` | `worker.$` → `/services/$serviceId/builds/$` | matched | `services.$serviceId.builds.$.tsx` (w3/m554) |
| `/worker/:param/compute` | `worker.$` → `/services/$serviceId/compute` | matched | `services.$serviceId.compute.tsx` (w6/m483) |
| `/worker/:param/deploys` | `worker.$` → `/services/$serviceId/deploys` | matched | `services.$serviceId.deploys.index.tsx` |
| `/worker/:param/deploys/:param/*` | `worker.$` → `/services/$serviceId/deploys/$deployId/…` | matched | `services.$serviceId.deploys.$deployId.tsx` and its `rollback` page; another sub-path opens the deploy's page (`serviceSubpathLanding`, w3/m707) |
| `/worker/:param/disks` | `worker.$` → `services.$serviceId.$` → `/services/$serviceId/disk` | matched | `serviceSubpathLanding` (w3/m707), Render's `disks` is bex's Disk page |
| `/worker/:param/env` | `worker.$` → `/services/$serviceId/env` | matched | `services.$serviceId.env.tsx` |
| `/worker/:param/events` | `worker.$` → `/services/$serviceId/events` | matched | `services.$serviceId.events.tsx` |
| `/worker/:param/headers` | `worker.$` → `/services/$serviceId/headers` | matched | `services.$serviceId.headers.tsx` |
| `/worker/:param/jobs` | `worker.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: one-off jobs (ADR018 parity non-goals) |
| `/worker/:param/logs` | `worker.$` → `/services/$serviceId/logs` | matched | `services.$serviceId.logs.tsx` |
| `/worker/:param/metrics` | `worker.$` → `/services/$serviceId/metrics` | matched | `services.$serviceId.metrics.tsx` |
| `/worker/:param/metrics/:param` | `worker.$` → `services.$serviceId.$` → `/services/$serviceId/metrics?range=…` | matched | `serviceSubpathLanding` (w3/m707): the window [Render's period table](#renders-legacy-metrics-periods) maps the period to |
| `/worker/:param/plan` | `worker.$` → `/services/$serviceId/plan` | matched | `services.$serviceId.plan.tsx` lands on Compute (w6/m483) |
| `/worker/:param/previews` | `worker.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/worker/:param/pulls` | `worker.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | non-goal | DO_NOT_DO: PR preview environments |
| `/worker/:param/redirects` | `worker.$` → `/services/$serviceId/redirects` | matched | `services.$serviceId.redirects.tsx` |
| `/worker/:param/scaling` | `worker.$` → `/services/$serviceId/scaling` | matched | `services.$serviceId.scaling.tsx` lands on Compute (w6/m483) |
| `/worker/:param/settings` | `worker.$` → `/services/$serviceId/settings` | matched | `services.$serviceId.settings.tsx` |
| `/worker/:param/sharing` | `worker.$` → `services.$serviceId.$` → the service's root, as Render's `*` route | flagged | Render's deprecated per-resource sharing, `grandfather-owners-into-sharing` |
| `/worker/:param/shell` | `worker.$` → `/services/$serviceId/shell` | matched | `services.$serviceId.shell.tsx` (w2/m55) |
| `/worker/:param/shell/:param` | `worker.$` → `services.$serviceId.$` → `/services/$serviceId/shell?instance=<service id>-<suffix>` | matched | `serviceSubpathLanding` (w3/m707): Render names the instance by its id's last segment |
| `/worker/new` | `/worker/new` | matched | `routes/_serviceCreate.worker.new.tsx` (w1/m45) |
| `/workflow/new` | — | non-goal | DO_NOT_DO: workflows & tasks |
