# w3 · m554 — A cron job's sidebar and routes are Render's: Runs, Deploys, Settings, and its bare URL opens Runs

**Worker:** worker3 **Goal:** a cron job's sidebar reads Runs, Deploys, Settings / Monitor: Events, Logs, Metrics / Manage: Environment, Shell, as Render's does; its bare URL opens Runs, `…/builds` lands on `…/deploys`, and its compute plan is Settings' `#compute` section, not a Compute tab. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's cron job navigation from the walk and the bundle, and close cron-dashboard.md's Unverified list | 40m | — |
| [t002](t002.md) | Sidebar: a cron job's top level is Runs, Deploys, Settings, and its Manage group has no Compute | 45m | t001 |
| [t003](t003.md) | Routes: a cron job's bare URL opens Runs, `/builds` lands on `/deploys`, and `/compute` lands on Settings' compute section | 1h | t001 |
| [t004](t004.md) | Settings' compute plan section answers `#compute`, and the changed routes' skeletons match their ready pages | 45m | t002, t003 |
| [t005](t005.md) | Verify in a real browser at 1440 and 390, on dev-3 and on production | 45m | t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 5h35m**.

## What Render does

- Evidence: a signed-in, read-only walk of four cron jobs in a Render workspace on 2026-11-21 (nothing that acts was clicked), plus Render's public dashboard bundle read the same day without sign-in (`CronJob-D3WeWa8V.js` sha256 `23d3ad7d7c97…`, `index-BR2q8yex.js` `f19b12ced9c1…`, `urls-B5ihPFNO.js` `3ab9b8053a93…`, `helpers-C_tFPHBn.js` `302444d539f3…`; the hashes change with every Render deploy, so find the chunks by name, as `metrics-page.md` § Status-code label describes).
- Sidebar: Runs, Deploys, Settings · Monitor: Events, Logs, Metrics · Manage: Environment, Shell, One-Off Jobs (One-Off Jobs stays out: `DO_NOT_DO.md`). No Compute: the bundle's cron title map has no `/compute`, where the Server map has `/compute` and `/scaling`.
- `/cron/<id>` shows Runs; `/cron/<id>/builds` lands on `/cron/<id>/deploys`; a build's detail stays under `/cron/<id>/builds/<id>`.
- Settings has a `#compute` section right after General, and the header's plan badge links to it.
- Render's docs and its 2026-08 changelog screenshot still say Builds; the signed-in dashboard says Deploys.

## bex today

- `serviceNavGroups` (`dashboard/src/features/services/components/service-nav.tsx`): cron top level `[BUILDS, RUNS, SETTINGS]`, Manage `[ENVIRONMENT, SHELL, COMPUTE]`.
- `service-root-redirect.ts`: the cron root and a cron `/deploys` land on `/builds`.
- `services.$serviceId.compute.tsx` serves every runtime type.

## Definition of done

1. A cron job's sidebar is Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell, with Deploys highlighted on the list and on a deploy's detail; no Compute entry for a cron job; every other type's sidebar is unchanged.
2. `/services/<cron>` and `/cron/<id>` open Runs; `…/builds` lands on `…/deploys` with its search; a cron `…/compute` lands on Settings with the plan section (`#compute`) in view; a non-cron `/runs` or `/builds` still lands on its primary tab.
3. Each changed route's pending state matches its ready state at 1440 and 390.
4. `cron-dashboard.md` has no Unverified list, `compute-page.md` says a cron job has no Compute page, and ADR018's Cron job row says what bex does.
5. Walked in a real browser on dev-3 and, once the pin carries it, on production.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w3` (2026-11-21), filed by `/pmx the verified parity to w3`, item 1. Amends [w2/m360](../../w2/done/m360/README.md), which mapped Render's docs ("Builds page") because no Render dashboard was available; the bundle alone disagreed with the docs on the tab's name, and the signed-in walk settled it.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a Render user finds a cron job's runs, deploys and plan where Render puts them, and Render-shaped links such as `/cron/<id>/builds` land.
- **Why now:** w3/m555 and w3/m556 build on these routes, and every week `/builds` stays a cron job's canonical URL adds links and bookmarks to it.
- **Render parity task included:** the dashboard's cron sidebar and routes change. No REST, GraphQL or MCP surface changes.
