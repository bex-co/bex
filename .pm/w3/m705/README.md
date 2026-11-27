# w3 · m705 — Render's resource and create URLs that bex answers with 404 open their bex pages

**Worker:** worker3 **Goal:** every Render dashboard URL for an environment group, a sandbox group, an organization, a create page, a legacy team settings page, Network Access, the spend limit or `/resource/<id>` opens the bex page it names, with the query and hash kept, where today each falls through to `routes/$.tsx` and draws `NotFoundPage`. **Status:** in progress (t001–t006 and t008–t010 done; t007 walked locally, its production walk waits for a pin carrying the change)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `/env-group/<id>` opens the environment group, and `/new/env-group` opens New Environment Group — **DONE** | 40m | — |
| [t002](done/t002.md) | `/sandbox-group/<id>/…` opens the same sandbox group page, and `/sandbox-group/new` opens its create dialog — **DONE** | 45m | — |
| [t003](done/t003.md) | `/organization/<id>/…` opens the organization's page, membership-checked — **DONE** | 45m | — |
| [t004](done/t004.md) | `/new`, `/new/<slug>` by Render's slug table, `/new/team` and `/t/<id>/settings/*` open their bex pages — **DONE** | 40m | — |
| [t005](done/t005.md) | `/network-access` opens Inbound IP Rules, and `/settings/spend-limit` opens the Build Pipeline card — **DONE** | 25m | — |
| [t006](done/t006.md) | `/resource/<id>` opens the resource the id names, by its prefix — **DONE** | 45m | — |
| [t007](t007.md) | Walk every row in a real browser at 1440 and 390, locally and on production | 40m | t001, t002, t003, t004, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 25m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

Total: **11 tasks, about 6h15m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`, `urls-ubVUii_6.js`, `ResourceRedirector-DwoXTM_f.js`); find the code by the strings quoted here, since hashes change with each deploy.
- **Environment groups:** routes `/env-group/:id` and `/env-group/*`; the builder is `` `/env-group/${e.id}` ``; create is `/new/env-group`.
- **Sandbox groups:** `` `/sandbox-group/${e}` ``, `` `/sandbox-group/${e}/sandboxes` ``, `` `/sandbox-group/${e}/*` ``, `/sandbox-group/new` (behind Render's `sandboxes` gate, as `dashboard-operations.md` records).
- **Organizations:** routes `/organization/:id/*`, `/organization/:id/members`, `/organization/:id/settings`, `/organization/:id/billing`; builders add `/organization/${e}/new/workspace`.
- **Create:** `/new` is Render's chooser; `/new/:serviceTypeSlug` runs a loader (`Ez`/`Tz`): a service-type slug goes to `/<type>/new` with the query kept; `background-worker(s)` to the worker create; `blueprint(s)` to `/blueprint/new`; `database`, `postgres`, `postgresql` to `/new/database`; `env-group(s)` to `/new/env-group`; `kv`, `key-value`, `redis` to `/new/redis`; anything else to `/new`. `/new/team` and `/new/workspace` both exist.
- **Legacy and workspace pages:** `/t/:id/settings` and `/t/:id/settings/*` (Render's older team scheme); `/network-access/*` is Render's Network Access page ("Inbound IP Restrictions"); `/settings/spend-limit` is the spend limit.
- **`/resource/<id>`:** the `ResourceRedirector` chunk asks `resourceTypeBy{Service,EnvGroup,Database,Redis,Blueprint,Webhook,SandboxGroup,Sandbox,SandboxSnapshot,FlowRun,Workflow}Id` which kind the id is and redirects to that resource's page, keeping the rest of the path.

## bex today

Verified 2026-11-26 on main `76dfb1e0e2`.

- `dashboard/src/routes/` has none of `env-group.$`, `sandbox-group.$`, `organization.$`, a `new` index or `new.$`, `new.env-group`, `new.team`, `t.$`, `resource.$`, `network-access` or `settings.spend-limit`; each URL above reaches `routes/$.tsx`.
- Canonical bex pages: `env-groups_.$groupId.tsx` (and the New Environment Group dialog on `env-groups.tsx`), `sandbox-groups_.$sandboxGroupId.{index,images,settings,snapshots}.tsx` with drawers as search params (`?sandbox=`, `?tab=`; w1/m658, `docs/render-artifacts/sandbox-groups.md`), `org.$orgId.{members,settings,workspaces}.tsx` (the org's plan and billing sit on its primary workspace), `new.{database,redis,project,workspace}.tsx`, `_serviceCreate.{web,worker,pserv,static,cron}.new.tsx`, `services.new.tsx`, `blueprint.new.tsx`, and `/workspace/settings` with `#build-pipeline` and `#inbound-ip-rules`.
- `common/lib/render-alias.ts` holds `redirectRenderAlias` and `redirectPreservingSuffix` (query and hash kept); `routes/w.$.tsx` checks membership and switches the workspace for `/w/<tea-id>/…` and is the model for the organization alias.
- Ids: `lego/backend/internal/id/id.go` mints Render's prefixes (`srv`, `dpg`, `red`, `evg`, `prj`, `evm`, `whk`, `sbg`, `sbx`, `snp`, `dep`, …); Blueprints link by their `exs-` id with `blp-` kept for old bookmarks (w2/m269).

## Definition of done

1. Each Render URL listed under § What Render does opens the bex page it names, with the query and hash kept, at 1440 and 390: `/env-group/<id>` the group; `/new/env-group` the group list with New Environment Group open; `/sandbox-group/<id>`, `/sandboxes[/<sbx>]`, `/snapshots[/<snp>]`, `/builds` and `/settings` the matching sandbox group page or drawer; `/sandbox-group/new` the create dialog; `/organization/<id>`, `/members`, `/settings`, `/new/workspace` the org's pages, `/billing` the primary workspace's Billing; `/new` and every `Tz` slug their create page; `/new/team` `/new/workspace`; `/t/<id>/settings[/*]` what `/w/<id>/settings[/*]` opens; `/network-access` Inbound IP Rules; `/settings/spend-limit` the Build Pipeline card; `/resource/<id>[/*]` the resource's page.
2. An organization or workspace the user is not in is refused as `/w/<id>` refuses it today; an id `/resource/` does not recognize reaches the not-found handling.
3. A router test walks every alias row of `dashboard-routes.md`'s URL table (w3/m704) and asserts none reaches `routes/$.tsx`.
4. `dashboard-routes.md` and `dashboard-operations.md` (the nine `resourceTypeBy*` rows) say `matched`; ADR018's dashboard URL paragraph names the new aliases.
5. Walked locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` (resources created for the walk are disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), item 2, verified against Render's bundle the same day.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Render's CLI builds these links, its API returns them, and people paste them; ADR018 records the dashboard URL surface in `docs/ADR018/deployment-sources-iac.md` (w5/m39, w1/m45).
- **Expected outcome:** a Render-shaped link to an env group, sandbox group, organization, create page or bare resource id opens the page instead of a 404.
- **Why now:** the sandbox group pages (w1/m658) and organization pages (w7/m464) shipped without Render's paths, and the env-group row was recorded as matching when it does not. Best after w3/m704, whose URL table this milestone turns from `gap` to `matched`; it does not need it to start.
- **Out of scope (owned elsewhere):** `/blueprint/*` (w3/m677, m678, m681), `/project/<id>/environment/*` (w1/m683, m684), `/i/<id>/view` (w6/m625), `/private-links` (w7/071), `/integrations` and `/observability` (DO_NOT_DO: drains), service sub-pages (w3/m707), `/billing/update-plan*` (w3/m708).
- **Render parity task:** included (t008): the milestone changes which dashboard URLs open which pages.

## Implementation (2026-11-27)

One mapping per URL family in `dashboard/src/common/lib/render-alias.ts` (`envGroupAliasPath`, `sandboxGroupAliasPath`, `organizationAliasPath`, `renderCreateSlugLanding`, `newPathLanding`, `resourceAliasPath`), each served by a thin route: `env-group.$`, `env-groups_.new`, `new.env-group`, `sandbox-group.$`, `organization.$`, `new.$`, `$serviceTypeSlug.new`, `t.$`, `network-access.$`, `settings_.spend-limit` and `resource.$`. `redirectPreservingSuffix` keeps a landing's own section (`#build-pipeline`) after the incoming query, and finds the incoming suffix at the href's first `?` or `#` (slicing at the decoded pathname's length cut `…/agent%20runtime?version=v2` into `me%3Fversion…`). The environment group list (`?new=env-group`) and an org's Workspaces page (`?new=workspace`) let the URL own their create dialogs.

Where Render's bundle, read again on 2026-11-27, differs from this README's premises, bex follows the bundle:

- **The slug table's route is `/:serviceTypeSlug/new`.** Render's loader (`Ez` over `Tz`) sits on `/<slug>/new` and rewrites `/<x>/new`; `/new/:serviceTypeSlug` is only the New Service breadcrumb's pattern. Both are served from the one table: `/postgres/new`, `/kv/new`, `/WEB/new` and `/new/postgresql`, `/new/WEB` alike.
- **`/resource/` sends what it cannot place home.** Render's `ResourceRedirector` redirects to `/` for an id without a known prefix, one shorter than 24 characters, and a `prj-` id (`prj: null` in its kind table), so bex does too. A known prefix whose resource does not exist meets the destination page's not-found handling, and a workflow id (`wfl`, `tsk`, `trn`, `flw`, `frn`; bex mints none) the not-found page.
- **`/new` has no bex counterpart page.** It opens New Web Service, whose type switcher lists every type; Render's chooser page is w3/154.
- **Back does not close the env group dialog.** Opening and closing replace the history entry, bex's `?new=` idiom (`routes/index.tsx`), as t001's acceptance had it otherwise.
- **The router test found a `matched` row bex answered with its 404.** Render's build source Builds URL, `/build-sources/<bds-id>/builds[?build=<bld-id>]`, now lands on the index or the build's page (`build-sources_.$buildSourceId.builds.index.tsx`).

Follow-ups filed by t008: w3/154 (Render's `/new` chooser page), w3/155 (Network Access as a page under a Networking group), w3/156 (New Sandbox Group page), w3/157 (an org's New Workspace page). Render's New Environment Group page stays w1/m684 t003.

## Walk

**Locally, 2026-11-27** (t007 step 1): dev-3 rebuilt from the tree (`bash scripts/dev-env.sh 3 up`), the dashboard on `:50030`, signed in as a dev-3 user created for the walk in a disposable dev-3 workspace, headless Chromium at 1440×900 and 390×844. Every URL of the definition of done landed where the router tests say, query and hash kept: the environment group dialog open from `/env-group/new`, `/new/env-group?from=cli` and `/env-groups/new`; the sandbox group pages and drawers (`?sandbox=…&tab=details#b`, `/images?image=agent+runtime&version=v2`, `/snapshots?snapshot=…`, `/settings`); `/sandbox-group/new` with New Sandbox Group open; an org the user is not in refused on `/org/<id>/workspaces`, `/members` and `/settings`, its `billing` sent to the org's page; `/new`, `/new/WEB?x=1`, `/new/nope` on New Web Service, `/new/postgresql` and `/postgres/new?x=1` on New Postgres, `/new/kv` on New Key Value, `/new/blueprints` on New Blueprint Instance, `/new/team` on New Workspace, `/background-worker/new` on New Background Worker; `/t/<own tea-id>/settings` on Workspace Settings (its query and hash dropped by `w.$`, w3/147); `/network-access?a=1` and `/settings/spend-limit` with the Inbound IP Rules and Build Pipeline cards scrolled into view; `/resource/<dpg-id>?x=1#h`, `/resource/<red-id>/metrics` and `/resource/<dpg-id>/logs` on a Postgres and a Key Value created for the walk (disposable, deleted afterwards); `/resource/<prj-id>` and `/resource/nonsense` home; `/resource/<wfl-id>` and `/env-group` on the not-found page. dev-3 serves no sandbox groups and has no secret store, so the sandbox group and environment group pages themselves were not drawn with data there; production's walk covers them.

**Production:** waits for a pin carrying the change (t007 step 2).
