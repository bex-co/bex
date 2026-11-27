# w3 · m704 — Every URL Render's dashboard builds has a row in `dashboard-routes.md`, and a weekly job finds new ones

**Worker:** worker3 **Goal:** `docs/render-artifacts/dashboard-routes.md` lists every path Render's dashboard bundle builds or routes, each with what bex does about it, and the weekly `render-dashboard-ops` job fails when Render adds a URL with no row or drops one a row names; the two rows known to be wrong today are corrected. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The crawler lists every dashboard URL pattern in Render's bundle: the `urls` chunk's builders and every route table's `path:` | 1h30m | — |
| [t002](t002.md) | `dashboard-routes.md` gets one row per pattern with its disposition and pointer, and the env-group row is corrected | 1h | t001 |
| [t003](t003.md) | `render-dashboard-ops.sh check` also checks the URL ledger, and its synthetic-bundle test covers it | 1h | t002 |
| [t004](t004.md) | `dashboard-operations.md`'s `resourceTypeBy*` rows name Render's `/resource/<id>` redirector, not Get service | 20m | t002 |
| [t005](t005.md) | Simplify | 15m | t003, t004 |
| [t006](t006.md) | Test coverage | 30m | t003, t004 |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

Total: **7 tasks, about 4h45m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-26 without sign-in: 459 chunks from the entry `index-D5V4ZgbE.js`; the URL builders live in `urls-ubVUii_6.js`. Hashes change with each Render deploy, so the crawler finds chunks by family (`urls-*.js`) and code by shape, as `scripts/lib/render_dashboard_ops.py` already does for operations and flags.
- **Two sources of URLs.** The `urls` chunk exports one builder per page (`/env-group/${e.id}`, `/organization/${e}/members`, `/sandbox-group/${e}/sandboxes`, `/resource/${e}`, `/billing/update-plan?plan=professional&next=${window.location.pathname}`, …). The route tables (`{path:…, Component:…}` objects, plus each resource layout's `hrefPatterns`) add the paths only a router knows: `/env-group/:id`, `/t/:id/settings/*`, `/new/:serviceTypeSlug`, `/organization/:id/billing`, `/network-access/*`, `/billing/update-plan-preview`, `/billing/update-plan-confirm`. Many `path:` values are constants or helper calls (`path:ri`, `na(':serviceTypeSlug')`), resolved where the constant is a literal in the same chunk.
- **What the table shows today.** Render's env-group detail is `/env-group/:id` (singular), where `dashboard-routes.md:24` records `/env-groups/{evg-id}` as "detail inferred … already matches". The bundle also routes `/sandbox-group/*`, `/organization/*`, `/resource/*`, `/new/*` slugs, `/t/:id/settings/*` and `/network-access/*`, none of which bex routes (w3/m705 owns them).

## bex today

Verified 2026-11-26 on main `76dfb1e0e2`.

- `docs/render-artifacts/dashboard-routes.md` (w5/m39, w1/m45, 2026-07-16) was written from Render's CLI source, its docs and live probes: § Resource → path map, § Service sub-tab map, § Workspace / user / create scheme, § Sidebar navigation. Nothing checks it against Render since.
- `scripts/render-dashboard-ops.sh` with `scripts/lib/render_dashboard_ops.py` (w1/m645) already downloads every chunk weekly in `.github/workflows/render-schema-drift.yml` (job `render-dashboard-ops`, whose checkout skips `.pm/`, w6/m157) and checks operations and flags against `dashboard-operations.md`. `scripts/render-dashboard-ops.test.sh` runs it on synthetic bundles in `.github/workflows/scripts.yml`.
- `dashboard-operations.md` lines 116, 302, 382, 410, 473, 557 and 579–581 mark `resourceTypeByServiceId`, `…EnvGroupId`, `…DatabaseId`, `…RedisId`, `…BlueprintId`, `…WebhookId`, `…SandboxGroupId`, `…SandboxId` and `…SandboxSnapshotId` `matched` to the resource's CRUD rows. They are read only by Render's `ResourceRedirector` chunk, which serves `/resource/<id>`; bex has no such route.

## Definition of done

1. `dashboard-routes.md` has one URL table row for every path pattern the 2026-11-26 bundle builds or routes, using `dashboard-operations.md`'s six dispositions: `matched` names the bex route or alias, `gap` and `partial` name the owning board item, `non-goal` cites `DO_NOT_DO.md` or an ADR018 `—` row, `n/a` says why.
2. The env-group row names Render's `/env-group/:id`; no row in the file says "inferred" for a path the bundle shows.
3. `scripts/render-dashboard-ops.sh check` fails, naming the pattern, when the bundle has a URL with no row or a row's URL left the bundle, and when a row is malformed; it passes on the current bundle. `list` prints the URL inventory with the operations and flags.
4. `scripts/render-dashboard-ops.test.sh` covers a new URL, a removed URL, a constant-resolved `path:` and a malformed row, and runs in `scripts.yml`.
5. The nine `resourceTypeBy*` rows say they serve `/resource/<id>` and point at the board item that adds it.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), item 1, verified against Render's bundle the same day.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Render-shaped dashboard links are a contract: Render's CLI builds them, Render's API returns them as `dashboardUrl`, and people paste them. ADR018 records the surface in `docs/ADR018/deployment-sources-iac.md` (the w5/m39 and w1/m45 paragraph).
- **Expected outcome:** each URL Render's dashboard can produce has a written answer in bex, and a URL Render adds is found within a week instead of by a user's 404.
- **Why now:** it is the inventory w3/m705 (resource and create URLs) and w3/m707 (service sub-pages) work from, and it corrects two records that currently claim parity that does not exist. The crawler already holds every chunk, so the cost is the parser and the rows.
- **Render parity task:** omitted. The milestone changes a document and a CI check; no REST, GraphQL, MCP or dashboard surface changes.
