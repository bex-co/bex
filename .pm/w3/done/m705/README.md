# w3 · m705 — Render's resource and create URLs that bex answers with 404 open their bex pages

**Worker:** worker3 **Goal:** every Render dashboard URL for an environment group, a sandbox group, an organization, a create page, a legacy team settings page, Network Access, the spend limit or `/resource/<id>` opens the bex page it names, with the query and hash kept, where today each falls through to `routes/$.tsx` and draws `NotFoundPage`. **Status:** done (t001–t011; walked locally and on production pin `7d1dce861173`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `/env-group/<id>` opens the environment group, and `/new/env-group` opens New Environment Group — **DONE** | 40m | — |
| [t002](done/t002.md) | `/sandbox-group/<id>/…` opens the same sandbox group page, and `/sandbox-group/new` opens its create dialog — **DONE** | 45m | — |
| [t003](done/t003.md) | `/organization/<id>/…` opens the organization's page, membership-checked — **DONE** | 45m | — |
| [t004](done/t004.md) | `/new`, `/new/<slug>` by Render's slug table, `/new/team` and `/t/<id>/settings/*` open their bex pages — **DONE** | 40m | — |
| [t005](done/t005.md) | `/network-access` opens Inbound IP Rules, and `/settings/spend-limit` opens the Build Pipeline card — **DONE** | 25m | — |
| [t006](done/t006.md) | `/resource/<id>` opens the resource the id names, by its prefix — **DONE** | 45m | — |
| [t007](done/t007.md) | Walk every row in a real browser at 1440 and 390, locally and on production — **DONE** | 40m | t001, t002, t003, t004, t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 25m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](done/t011.md) | Closeout — **DONE** | 10m | t009, t010 |

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
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). Render's CLI builds these links, its API returns them, and people paste them; ADR018 records the dashboard URL surface in `docs/ADR018/deployment-sources-iac.md` (w5/m39, w1/m45).
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

Follow-ups filed by t008: w3/154 (Render's `/new` chooser page), w3/155 (Network Access as a page under a Networking group), w3/156 (New Sandbox Group page), w3/157 (an org's New Workspace page). Render's New Environment Group page stays w1/m684 t003. Follow-ups filed by t007's production walk: w3/167 (an org the caller is not in draws the read-failure card), w3/168 (the environment group page's frame), w3/169 (the org billing alias frame's top bar).

## Walk

**Locally, 2026-11-27** (t007 step 1): dev-3 rebuilt from the tree (`bash scripts/dev-env.sh 3 up`), the dashboard on `:50030`, signed in as a dev-3 user created for the walk in a disposable dev-3 workspace, headless Chromium at 1440×900 and 390×844. Every URL of the definition of done landed where the router tests say, query and hash kept: the environment group dialog open from `/env-group/new`, `/new/env-group?from=cli` and `/env-groups/new`; the sandbox group pages and drawers (`?sandbox=…&tab=details#b`, `/images?image=agent+runtime&version=v2`, `/snapshots?snapshot=…`, `/settings`); `/sandbox-group/new` with New Sandbox Group open; an org the user is not in refused on `/org/<id>/workspaces`, `/members` and `/settings`, its `billing` sent to the org's page; `/new`, `/new/WEB?x=1`, `/new/nope` on New Web Service, `/new/postgresql` and `/postgres/new?x=1` on New Postgres, `/new/kv` on New Key Value, `/new/blueprints` on New Blueprint Instance, `/new/team` on New Workspace, `/background-worker/new` on New Background Worker; `/t/<own tea-id>/settings` on Workspace Settings (its query and hash dropped by `w.$`, w3/147); `/network-access?a=1` and `/settings/spend-limit` with the Inbound IP Rules and Build Pipeline cards scrolled into view; `/resource/<dpg-id>?x=1#h`, `/resource/<red-id>/metrics` and `/resource/<dpg-id>/logs` on a Postgres and a Key Value created for the walk (disposable, deleted afterwards); `/resource/<prj-id>` and `/resource/nonsense` home; `/resource/<wfl-id>` and `/env-group` on the not-found page. dev-3 serves no sandbox groups and has no secret store, so the sandbox group and environment group pages themselves were not drawn with data there; production's walk covers them.

**Production, 2026-11-28** (t007 step 2). Pin `7d1dce861173` (`207a050b22`, carries `2a51a0a13a`): the `dashboard` pod ran `bex-dashboard@sha256:8ad155ac3cb1…` and both `bex-api` pods `bex-operator@sha256:d2f83bec3be6…`, the pin's digests, read through the admin kubeconfig before the walk. Walked 00:58–01:12Z as the QA user (`scripts/qa-login.sh`) in `bex-canary` (`tea-daif693dqjvc73e7as3g`) with a run-minted API key, in a headless Chromium of its own through `playwright-core`, `en-US`, at 1440×900 and 390×900: each URL a full load with GraphQL held 2.5 s for the pending frame, then released for the ready page. Created for the walk through REST (disposable): environment group `qa-20261128-m705-evg` (`evg-dc52ektisb3c73bls4pg`, one variable), sandbox group `qa-20261128-m705-sbg` (`sbg-dc52ekr7laqs73baffbg`) with sandbox `qa-20261128-m705-sbx` (`sbx-dc52epj7laqs73baffdg`) and its filesystem snapshot `qa-20261128-m705-snp` (`snp-dc52erdisb3c73bls4ug`), and Postgres `qa-20261128-m705-pg` (`dpg-dc52el37laqs73baffc0`, Basic 256MB, since the free slot was taken). Every URL landed the same at both widths, with no sideways scroll, no page error, and no console error but the two not-found documents (`/env-group`, `/resource/wfl-…`) and the analytics beacon the CSP refuses:

- **Environment groups:** `/env-group/<evg>?x=1#h` on the group (`/env-groups/<evg>?x=1#h`) with its variable, `/env-group/<evg>/settings?x=1` on the same page; `/new/env-group?from=cli#h`, `/env-group/new`, `/env-groups/new?x=1` and `/new/env-groups` on the list with Create environment group open (`?new=env-group&from=cli#h` …); `/env-group` on the not-found page.
- **Sandbox groups:** `/sandbox-group/<sbg>?x=1#h` and `/sandboxes` on Sandboxes with the sandbox listed; `/sandboxes/<sbx>?x=1#h` on `?sandbox=<sbx>&x=1#h` with its drawer on Events, `/configurations` on its Details tab and `/snapshots` on its Snapshots tab; `/snapshots?x=1` on Snapshots with the snapshot listed and `/snapshots/<snp>?x=1#h` with its drawer; `/builds?x=1` on Custom images and `/builds/agent%20runtime?version=v2` on `/images?image=agent+runtime&version=v2` with Image not found (the group has no such image); `/settings#h` on Settings; `/sandbox-group/new?x=1` on `/?new=sandbox-group&x=1` with New Sandbox Group open.
- **Organizations:** `/organization/<bex-canary's org>?x=1#h` on its Workspaces (`?x=1#h`), `/members` and `/settings?x=1` on theirs, `/new/workspace` with New workspace in bex-canary open, `/billing?x=1` on `bex-canary`'s Billing (`/billing`, the query dropped by `w.$`, w3/147). An org the QA user is not in (`org-d0000000000000000000`) was refused on `/organization/<id>`, `/members`, `/settings` and `/billing`, as `/org/<id>` refuses it, though every org page draws that refusal as its read-failure card, "Couldn't load the organization", where its "Organization not found" state never draws: w3/167.
- **Create:** `/new?x=1#h` on New Web Service (`/web/new?x=1#h`); `/new/web`, `/new/WEB` and `/new/nope` on New Web Service; `/new/worker` and `/new/background-worker(s)` on New Background Worker; `/new/pserv`, `/new/static` and `/new/cron` on theirs; `/new/blueprint(s)` on New Blueprint Instance; `/new/database`, `/new/postgres` and `/new/postgresql#h` on New Postgres; `/new/kv`, `/new/key-value` and `/new/redis` on New Key Value; `/new/team?x=1` and `/new/workspace` on Create a workspace; `/postgres/new?x=1`, `/kv/new`, `/key-value/new`, `/background-worker/new`, `/blueprints/new`, `/env-groups/new` and `/nope/new` by the same table; `/WEB/new` on New Web Service at its own URL (the router matches `/web/new` case-insensitively).
- **Legacy and workspace pages:** `/t/<bex-canary>/settings?x=1#h` and `/settings/members` on Workspace Settings, as `/w/<bex-canary>/settings…` (its query and hash dropped by `w.$`, w3/147); `/t/tea-d0000000000000000000/settings` on Access denied, as `/w/` with that id; `/network-access?a=1` and `/network-access/inbound` on `/workspace/settings?a=1#inbound-ip-rules` scrolled to Inbound IP Rules; `/settings/spend-limit?x=1` on `/workspace/settings?x=1#build-pipeline`.
- **`/resource/<id>`:** the Postgres's id with `?x=1#h` on its Info (`/databases/<dpg>?x=1#h`) and with `/logs` on its Logs; the group's on the group; the sandbox group's with `/settings` on its Settings; the sandbox's on its group with its drawer open; the snapshot's on Snapshots with its drawer open; `srv-d0000000000000000000/deploys` on `/services/…/deploys`, Service not found (w3/m706's state); `prj-d0000000000000000000`, `dpg-123` and `nonsense` home; `wfl-d0000000000000000000` on the not-found page.
- **Build sources:** `/build-sources/bds-d0000000000000000000/builds?x=1` on `/build-sources/<bds>?x=1` and `…/builds?build=bld-d0000000000000000000` on `…/builds/bld-d0000000000000000000`, each drawing the build source page's "doesn't exist" (`bex-canary` has no build source, so the redirect is what was walked).

Each pending frame stood where its ready page does at both widths (header, tabs, cards, table rows, drawers, dialogs), but for two frames these aliases land on, filed: the environment group page's own frame on a fresh load (a five-row metadata list and no Secret Files card; w3/168), and the top bar of the `/organization/<id>/billing` frame, which reads "bex" over the Billing skeleton (w3/169).

**Cleanup.** The snapshot was deleted, the sandbox terminated, and the sandbox group, environment group and Postgres deleted through REST (204 each, then 404); `bex-canary` lists none of them. The run key was revoked (it then answered 401) and the QA session logged out (`qa-login.sh --logout`).

## Unblock record

- **2026-11-28, `/loopx w3` (verified evidence, no user decision):** the gate, pin must carry `2a51a0a13a`, holds. The newest pin on `origin/main` is `207a050b22` `chore(deploy): pin platform images to 7d1dce861173`, `git merge-base --is-ancestor 2a51a0a13a 7d1dce861173` exits 0, and production's `dashboard` and `bex-api` run that pin's digests. Affects t007 step 2, which then held at 1440 and 390 (§ Walk), so t011 closed the milestone: the definition of done's router test (`routes/__tests__/render-url-ledger-routes.test.ts`) and its `matched` rows in `dashboard-routes.md` and `dashboard-operations.md` were already on main.
