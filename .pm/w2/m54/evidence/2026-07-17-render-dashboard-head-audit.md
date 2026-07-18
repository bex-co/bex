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

## Current bex inventory

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
