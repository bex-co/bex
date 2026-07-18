# Render dashboard document-head audit — 2026-07-17

## Scope and method

This audit answers the head behavior needed by bex, not the unrelated content SEO of `render.com`. It inspected the live `dashboard.render.com` HTML shell and its deployed JavaScript route bundles on 2026-07-17, then cross-checked the page families against Render's official [dashboard](https://render.com/docs/render-dashboard), [deploy](https://render.com/docs/deploys), [logging](https://render.com/docs/logging), and [metrics](https://render.com/docs/service-metrics) documentation. The representative authenticated service capture already preserved by `w5/done/m42` corroborates the compiled service rule across tabs.

Bundle evidence was read from the deployed hashes `index-C2WBuoWA.js` and `NavigateContainer-C37ZQFFU.js` plus their route chunks. Those hashes are evidence coordinates, not stable API contracts; t007 re-checks the live behavior before closeout.

## Render's current contract

### Global shell metadata

Render sends the same generic marketing metadata before its authenticated SPA resolves a route:

| head field | observed behavior |
| --- | --- |
| fallback title | `Render · The Easiest Cloud For All Your Apps` |
| description | One global Render platform description |
| Open Graph | Static marketing title/description/image plus `website`, `en`, and site name |
| Twitter | Static `summary_large_image`, `@render`, marketing title/description/image |
| other shell head | `google=notranslate`, viewport with `shrink-to-fit=no`, manifest, theme-aware favicon |
| absent | No canonical link and no robots meta in the inspected shell |

The important privacy property is that resource/page titles do **not** flow into Open Graph or Twitter tags. A shared private service URL therefore does not put its tenant resource name in the shell's social preview metadata.

### Route title formatter

The deployed route code defines one brand suffix equivalent to:

```text
title(page) = page + " ・ Render Dashboard"
```

Shared page containers call that formatter, and resource layouts supply the human-readable prefix. The route families reduce to these rules:

| route family | Render title shape | deployed examples/evidence |
| --- | --- | --- |
| Standard list/settings page | `<page> ・ Render Dashboard` | Environment Groups, Webhooks, Notifications, Workspaces |
| Create flow | `New <resource> ・ Render Dashboard` | New Postgres, New Key Value, New Blueprint |
| Service and every service tab | `<name> ・ <user-facing service type> ・ Render Dashboard` | `backend-v2 ・ Web Service ・ Render Dashboard` |
| Postgres detail | `<name> ・ Database ・ Render Dashboard` | resource layout title |
| Key Value detail | `<name> ・ Key Value ・ Render Dashboard` | resource layout title |
| Project detail | `<project name> ・ Render Dashboard` | project route chunk |
| Project/environment settings | `<project> / [<environment> /] Settings ・ Render Dashboard` | settings layout title |
| Other named resource details | `<name> ・ <resource type> ・ Render Dashboard` | Blueprint, Webhook, Workflow, Sandbox Group, Private Link |
| Nested tab/detail | Parent resource title; no tab name appended | service Overview/Logs/Metrics/Settings/Deploys stay identical |

Render uses the Japanese middle dot `・` (U+30FB) with spaces in the current formatter. It does not use bex's current `·` (U+00B7) or the auth routes' em dash. Standard page containers also keep the visible page heading and document-title prefix aligned.

## Planning-time bex inventory

There are 58 non-test route definitions under `dashboard/src/routes/`; 30 contain a `head` declaration. Missing `head` is not automatically a bug: two routes are non-HTML server APIs, Render-shaped aliases and legacy auth/create routes redirect before rendering, project/service child routes inherit a layout, and service tabs intentionally share one parent title.

The complete planning-time classification is:

| category | count | route files |
| --- | --: | --- |
| Root/fallback | 2 | `__root.tsx`, `$.tsx` |
| Non-HTML server API | 2 | `api.connected-agents.tsx`, `api.sessions.tsx` |
| Rendered auth/OAuth/device | 9 | `auth.consent`, `auth.device`, `auth.device.success`, `auth.forgot-password`, `auth.login`, `auth.logout`, `auth.reset-password`, `auth.sign-up`, `auth.verification` |
| Rendered workspace/static/list/create | 11 | `index`, `settings`, `workspace.settings`, `notifications`, `usage`, `webhooks`, `blueprints`, `env-groups`, `new.workspace`, `services.new`, `keyvalue.new` |
| Project layout/content | 3 | `project.$projectId`, `project.$projectId.index`, `project.$projectId.settings` |
| Service layout/inherited children | 12 | `services.$serviceId` plus Overview, Logs, Metrics, Deploys list/detail, Events, Environment, Shell, Scaling, Plan, and Settings children |
| Other dynamic resource content | 4 | `databases.$databaseId`, `keyvalue.$keyValueId`, `env-groups_.$groupId`, `blueprints.$blueprintId` |
| Redirect-only compatibility aliases | 15 | `billing.$`, `cron.$`, `d.$`, `login`, `new.database`, `new.project`, `new.redis`, `pserv.$`, `r.$`, `register`, `static.$`, `u.$`, `w.$`, `web.$`, `worker.$` |

The short names above omit only the shared `.tsx` suffix. t006 turns this planning inventory into an executable route manifest so generated routes cannot drift silently.

The actual inconsistencies are:

| bex family | current behavior | required behavior |
| --- | --- | --- |
| Root shell | charset, viewport, `bex dashboard`, favicon only | Generic bex description + OG/Twitter shell; one title formatter |
| Static/auth/list/create pages | Mix of `— bex`, `· bex dashboard`, hard-coded English, and generic root fallback | Localized page prefix + one ` ・ bex Dashboard` suffix |
| Project overview/settings | Static SSR title, then client `document.title`; Settings puts `Settings` before the project name | SSR human name; Render's name-first Settings hierarchy |
| Service and nested tabs/deploy | SSR id fallback; post-load client effect from w5/m42 supplies name/type | SSR name/type where authenticated data exists; same title on tabs |
| Database / Key Value | Opaque id remains in the settled title | Loaded name + `Database` / `Key Value` |
| Environment group / Blueprint | Opaque id remains in the settled title | Loaded name + resource type |
| Error/not-found/loading | Root and route fallbacks exist, but no central stale-title policy | Deterministic state title; never retain the previous page title |
| Redirect aliases / server APIs | Usually no head, correctly | Explicitly classified and guarded, not given duplicate metadata |

The only direct `document.title` writes are the service layout and two project pages. The other dynamic resource pages have no post-load correction. Existing route tests pin only the service client effect; there is no global route-head completeness/privacy test.

## Bex contract handed to w2

1. Use one formatter and one branded suffix: `<prefix> ・ bex Dashboard`.
2. Keep the resource hierarchy Render-compatible, but translate visible page/resource labels through bex i18n.
3. Resolve dynamic names/types in authenticated route loaders or an equivalent SSR-safe, cache-sharing seam; avoid duplicate GraphQL requests. An opaque id is acceptable only during loading or in a not-found diagnostic.
4. Keep description/Open Graph/Twitter values generic and bex-branded. Never place workspace/project/resource names, ids, deploy ids, or user data in social tags.
5. Derive absolute metadata asset URLs from the request/configured dashboard origin. Self-hosted bex must not emit `dashboard.bex.co` by default.
6. Do not invent route-specific canonical URLs, robots policy, or social images that the Render dashboard does not use. If a security/indexing policy is added for independent reasons, document it as deliberate divergence.
7. Maintain an exhaustive route classification so redirect-only, inherited-title, non-HTML, and rendered-content routes cannot silently change category.

## Known overlap

`w5/done/m42/t005` already made every service tab settle on `<name> · <service type> · bex dashboard` after Apollo data loads. m54 reuses its service-type derivation and tests, but replaces the ad hoc suffix/client-only fallback with the shared, SSR-aware contract. It must not regress the tab-invariance that m42 proved.

## Closeout verification

### Render re-check

The public Render dashboard shell and the representative official dashboard, deploy, logging, and metrics documentation were re-fetched immediately before closeout. The shell still referenced `index-C2WBuoWA.js` and `NavigateContainer-C37ZQFFU.js`. The deployed formatter remained `prefix ・ Render Dashboard` (U+30FB), and the inspected resource chunks still used the following hierarchy:

- project: `<name>`;
- project settings: `<name> / Settings`;
- service: `<name> ・ <user-facing type>`;
- Postgres: `<name> ・ Database`;
- Key Value: `<name> ・ Key Value`;
- Blueprint: `<name-or-repository> ・ Blueprint`.

The public shell still supplied generic description/Open Graph/Twitter metadata and no canonical or robots tag. Bex follows that structure but deliberately uses bex copy, assets, translations, and installation origin; no Render-owned marketing copy or image was copied.

### Implemented bex contract

The planning inventory is now executable in `dashboard/src/common/lib/document-head/route-inventory.ts`. It initially classified all 58 route definitions as 27 content routes, 12 inherited-title routes, 15 redirects, two non-HTML APIs, and two fallbacks. A pre-ship rebase added Webhook create/detail/Activity/Settings routes upstream; the final manifest classifies all 62 routes as 29 content routes, 14 inherited-title routes, 15 redirects, two non-HTML APIs, and two fallbacks. The Webhook parent now resolves its human name (or endpoint URL when unnamed) through the same authenticated SSR/cache seam, while both child tabs inherit that title. The guard fails when a route is added, removed, reclassified, given a competing head, or starts writing `document.title` directly without updating the contract.

One module owns the title formatter, generic metadata, resource states, and self-host origin normalization. Dynamic parent loaders use Apollo `network-only` reads so authenticated SSR receives the current human name; page hooks then use the normalized Apollo cache rather than issuing a title-only duplicate query. Project and service client effects were removed. Rename/sync callbacks invalidate the router so the same loader/head path refreshes the title.

The settled title rules are:

| bex family | settled contract |
| --- | --- |
| Static/auth/list/create/settings | `<localized page> ・ bex Dashboard` |
| Project | `<project name> ・ bex Dashboard` |
| Project settings | `<project name> / <localized Settings> ・ bex Dashboard` |
| Service and all tabs/deploy pages | `<service name> ・ <localized service type> ・ bex Dashboard` |
| Postgres / Key Value | `<name> ・ <localized resource type> ・ bex Dashboard` |
| Environment Group / Blueprint | `<name> ・ <localized resource type> ・ bex Dashboard` |
| Webhook and Activity/Settings tabs | `<name-or-endpoint URL> ・ Webhook ・ bex Dashboard` |
| Loading / missing / failed resource | Localized deterministic generic state title |
| Redirect / API | No competing head |
| Not found / root error | Localized fallback title component |

Global description, Open Graph, and Twitter fields remain generic. Only `<title>` receives a route or resource prefix. Absolute `og:url`, Open Graph image, and Twitter image values derive from the active request origin. When no valid HTTP(S) origin exists, those absolute fields are omitted instead of falling back to a hosted bex domain. No canonical or robots policy was added.

### Live dev-2 matrix

The dev-2 stack ran the dashboard at `http://localhost:50020`, Kratos at `:51020`, and bex-api at `:54020`. Verification registered an isolated synthetic identity and created uniquely named, workspace-scoped fixtures. The record below intentionally keeps only title shapes and resource classes; it contains no identity, cookie, token, workspace id, resource id, or resource name.

| live request | English SSR | Chinese SSR | result |
| --- | --- | --- | --- |
| Project overview | `<human project> ・ bex Dashboard` | `<human project> ・ bex Dashboard` | pass |
| Project settings | `<human project> / Settings ・ bex Dashboard` | covered by the localized production-head test | pass |
| Service Logs / Metrics | `<human service> ・ Web Service ・ bex Dashboard` | `<human service> ・ Web 服务 ・ bex Dashboard` | pass |
| Postgres overview | `<human database> ・ Database ・ bex Dashboard` | `<human database> ・ 数据库 ・ bex Dashboard` | pass |
| Key Value overview | `<human key value> ・ Key Value ・ bex Dashboard` | `<human key value> ・ 键值存储 ・ bex Dashboard` | pass |
| Login direct load | `Sign in to your account ・ bex Dashboard` | `登录您的账户 ・ bex Dashboard` | pass |

Every sampled SSR document contained exactly one `<title>`. A headless Chrome run navigated from Login to Sign Up through the running TanStack client router: the title changed from `Sign in to your account ・ bex Dashboard` to `Create your account ・ bex Dashboard`, an in-page marker survived, and the Navigation Timing entry count remained one. This proves the change occurred without a second document load. The route-head test additionally exercises client navigation, rename, private-title replacement, and live locale changes through the production router/head machinery.

The dev-2 environment deliberately has no OpenBao URL, so it cannot create/read a live Environment Group, and Blueprints have no standalone create endpoint because they auto-register from a repo-backed deploy. Their ready/missing/error SSR states are therefore covered by production route-object tests with authenticated Apollo responses rather than fabricated live control-plane state. The same tests cover all five service types and service-tab/deploy inheritance. This is an environment capability limit, not an accepted product divergence.

### Metadata and privacy probe

A Chinese direct SSR response on the self-hosted origin contained one description, one Open Graph title, one Open Graph URL, and one Twitter card. It contained three `http://localhost:50020` absolute-origin references, zero `dashboard.bex.co` references, zero canonical links, and zero robots tags. Dynamic route tests assert that private project/service/datastore/environment-group/Blueprint/Webhook names and opaque ids never enter description/Open Graph/Twitter tags. Missing-origin tests assert that no made-up absolute URL is emitted.

Hydration serialization necessarily carries authenticated route data in executable state for the page itself; the privacy assertion applies to metadata/link-preview tags, not to the authenticated HTML application's hydration payload. The metadata test renders the actual TanStack `HeadContent`, verifies a single title after overrides, and inspects the resulting SSR markup rather than only assigning JSDOM's `document.title`.

### Automated verification

Closeout ran the required gate as one chain from `dashboard/`:

```text
yarn typecheck && yarn lint && yarn test && yarn build
```

All commands passed. The completion audit expanded exact English/Chinese assertions to every static/auth/list/create/settings title state and added regressions for empty dynamic records, stale titles during pending navigation, and single-title error/not-found SSR. The post-rebase integration also covers Webhook create/detail/tab ownership and unnamed-endpoint fallback. Vitest reported 231 passing files and 1,458 passing tests. The production client, SSR, and Nitro builds completed successfully. Focused coverage includes:

- actual SSR `HeadContent`, global metadata, self-host/missing origins, privacy, fallback deduplication, pending replacement, resource states, navigation, rename, and locale;
- exact production route heads for every static/auth/list/create/settings state in English and Chinese, plus authenticated loaders for project/settings, all five service types, Postgres, Key Value, Environment Group, Blueprint, and Webhook;
- exact 62-file inventory/category enforcement and guards against manual title writes or competing inherited/redirect/API heads;
- workspace/language cookie persistence so SSR selection matches the browser and language changes invalidate route heads.

The final changed-path audit contains only dashboard implementation/tests/locales and this milestone's PM artifacts. No `lego/` code or REST, GraphQL, MCP schema/adapter documentation changed, so machine surfaces remain unchanged and no `ADR018` ledger update is warranted.

### Accepted divergence

Bex localizes page and resource-type prefixes; Render's inspected dashboard shell is English. Bex uses the existing bex logo for generic social metadata and omits absolute social URLs when the installation origin is invalid or unavailable. Those are required self-hosting/branding differences. The title hierarchy, middle-dot separator, parent-title inheritance, generic social boundary, and absence of invented canonical/robots behavior match the observed Render structure.
