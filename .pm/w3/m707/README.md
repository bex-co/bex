# w3 · m707 — A service's Render sub-page links open the same page and state

**Worker:** worker3 **Goal:** every sub-path and deep-link query Render's dashboard builds for a service, a webhook and the account and billing pages opens the same bex page in the same state (the disk, the metrics window, the instance's shell, a webhook's activity, filtered logs, a billing card), where today `redirectRenderAlias` copies the sub-path verbatim onto `/services/<id>/…` and several reach `routes/$.tsx`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A service's Render sub-paths map onto bex's: `disks` to `disk`, `metrics/<period>` to Render's time range, `shell/<instanceId>` to `?instance=` | 45m | — |
| [t002](t002.md) | `/webhook/<id>/activity` opens the webhook's Activity page | 15m | — |
| [t003](t003.md) | Render's deep-link queries on Logs, Metrics and Events open the same filters | 1h15m | — |
| [t004](t004.md) | `/settings#account-security`, `/billing#free-usage` and `/billing#unbilled-charges` land on their cards | 20m | — |
| [t005](t005.md) | The URL ledger records every service sub-path, with Render's non-goal pages marked | 15m | t001 |
| [t006](t006.md) | Walk every sub-path and query in a real browser at 1440 and 390, locally and on production | 30m | t001, t002, t003, t004 |
| [t007](t007.md) | Render parity | 20m | t005, t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 40m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 4h45m**.

## What Render does

- **Evidence:** Render's public dashboard bundle, read 2026-11-26 without sign-in (`index-D5V4ZgbE.js`, `urls-ubVUii_6.js`); find the code by the strings quoted here.
- **Service routes** (the `v0` table: `*`, `events`, `jobs`, `logs`, `env`, `headers`, `pulls`, `previews`, `shell`, `shell/:instanceId`, `sharing`, `metrics`, `metrics/:period`, `redirects`, `settings`, …, plus `disks`, `builds/:buildId`, `deploys/:deployId`, `plan`). The cron job table has `metrics/:period` too; Postgres and Key Value tables have it and bex maps those already (w3/m641, m649).
- **`metrics/:period`:** `xSe` reads the legacy period, looks it up in a table (`eC`) and rewrites the URL in place to `?timeRange=<value>` (default window `12h`).
- **`shell/:instanceId`:** the Shell page opens on that instance.
- **Webhooks:** routes `/webhook/:webhookId/activity` and `/settings` (the `ORe` table).
- **Deep-link queries:** log links carry an instance filter (`l=[["INSTANCE","<suffix>"]]`) and a window (`r=<start>,<end>`); event filters take `eventTypes=` (`DeployEnded`, …); metrics take `timeRange=`.
- **Anchors:** `/settings#api-keys`, `#cli-tokens`, `#account-security`; `/billing#free-usage`, `#unbilled-charges` (the "remaining free usage" link on a free service's Settings goes to the free-usage anchor).

## bex today

Verified 2026-11-26 on main `76dfb1e0e2`.

- `dashboard/src/common/lib/render-alias.ts` `redirectRenderAlias`: `rest === "new"` goes to the create landing; any other `rest` is appended verbatim to `/services/<id>/` (or `/databases`, `/keyvalue`), so `/web/<srv>/disks` lands on `/services/<srv>/disks`, which has no route (bex's is `services.$serviceId.disk.tsx`), and `/web/<srv>/shell/<instance>` and `/metrics/<period>` likewise.
- `services.$serviceId.shell.tsx` takes `?instance=` (w3/m528); `services.$serviceId.metrics.tsx` persists its window as `range` / `rangeStart` / `rangeEnd` (w6/065); `features/logs/lib/log-search.ts` `parseLogSearch` already accepts Render's `r` and `t` aliases.
- `webhook.$webhookId.index.tsx` is the Activity page ("Render's Recent deliveries"); there is no `activity` child route.
- Anchors present: `api-keys`, `cli-tokens` on `/settings`; `payment-method`, `included-usage`, `charges`, `credit-balance`, `invoice-history` on `/billing` (`features/usage/components/usage-page.tsx`). Missing: `account-security`, `free-usage`, `unbilled-charges`.

## Definition of done

1. `/web/<srv>/disks` (and each service segment's) opens the Disk page; `…/metrics/<legacy period>` opens Metrics on the window Render's table maps it to; `…/shell/<instanceId>` opens Shell on that instance.
2. `/webhook/<id>/activity` opens the webhook's Activity page.
3. A Render log link with `l=[["INSTANCE","<suffix>"]]` and `r=<start>,<end>` opens Logs filtered to that instance and window; `eventTypes=` filters Events; `timeRange=` sets Metrics' window; values bex cannot honour are ignored, not errors.
4. The three missing anchors scroll to their cards at 1440 and 390.
5. `dashboard-routes.md` § Service sub-tab map and the URL table (w3/m704) record every service sub-path, with `previews`, `pulls` (DO_NOT_DO: PR previews), `jobs` (DO_NOT_DO: one-off jobs) and `sharing` marked with their disposition.
6. Walked locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` on a service created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), item 4, verified against Render's bundle the same day.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). `dashboard-routes.md` § Service sub-tab map promises that the alias keeps a Render sub-path, which holds only where the names match.
- **Expected outcome:** a link from Render's event cards, CLI or docs into a service's disk, metrics window, instance shell or filtered logs opens that exact view.
- **Why now:** the datastore pages already map their sub-paths (w3/m641, m649), so services are the outlier; best after w3/m704, whose URL table t005 fills.
- **Render parity task:** included (t007): the milestone changes which dashboard URLs and queries open which views.
