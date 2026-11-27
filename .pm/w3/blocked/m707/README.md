# w3 · m707 — A service's Render sub-page links open the same page and state

**Worker:** worker3 **Goal:** every sub-path and deep-link query Render's dashboard builds for a service, a webhook and the account and billing pages opens the same bex page in the same state (the disk, the metrics window, the instance's shell, a webhook's activity, filtered logs, a billing card), where today `redirectRenderAlias` copies the sub-path verbatim onto `/services/<id>/…` and several reach `routes/$.tsx`. **Status:** blocked (pin must carry `7d1dce8611`; t001–t005 and t007–t009 done, t006 walked locally, its production walk and t010 left)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A service's Render sub-paths map onto bex's: `disks` to `disk`, `metrics/<period>` to Render's time range, `shell/<instanceId>` to `?instance=` — **DONE** | 45m | — |
| [t002](done/t002.md) | `/webhook/<id>/activity` opens the webhook's Activity page — **DONE** | 15m | — |
| [t003](done/t003.md) | Render's deep-link queries on Logs, Metrics and Events open the same filters — **DONE** | 1h15m | — |
| [t004](done/t004.md) | `/settings#account-security`, `/billing#free-usage` and `/billing#unbilled-charges` land on their cards — **DONE** | 20m | — |
| [t005](done/t005.md) | The URL ledger records every service sub-path, with Render's non-goal pages marked — **DONE** | 15m | t001 |
| [t006](t006.md) | Walk every sub-path and query in a real browser at 1440 and 390, locally and on production | 30m | t001, t002, t003, t004 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t005, t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 40m | t007 |
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

## What shipped

Recorded 2026-11-27 (t001–t005, t007–t009). Rows: [dashboard-routes.md § Service sub-tab map](../../../../docs/render-artifacts/dashboard-routes.md#service-sub-tab-map) and the URL inventory; ADR018's paragraph is in [deployment-sources-iac.md](../../../../docs/ADR018/deployment-sources-iac.md).

- **Deviations from the brief, from Render's bundle (re-read 2026-11-27, same entry `index-D5V4ZgbE.js`):** Render's Metrics window key is `t`, not `timeRange` (the latter is the name in its code; its URL key is `t`, a preset or `<startMs>,<endMs>`); its Events filter key is `et`, not `eventTypes`; its custom log window is `r=<start>~<end>` with each instant `YYYY-MM-DD@HH:mm:ss` in UTC, not `<start>,<end>`; its log search text is `q`. Render's Instance failed card links Logs with the whole instance id in `l`, and Shell with the id's last segment. The period table `eC` is `5m`/`15m`/`30m` → `30m`, `1h`, `4h`, `12h`, `24h`, `2d`, `1w` → `7d`. Render's own Billing page has no element with id `free-usage` (its Monthly Included Usage card is `#included-usage`), so bex scrolls to the card the link names.
- **Sub-paths:** the aliases still copy the sub-path; `routes/services.$serviceId.$.tsx` and `routes/static.$serviceId.$.tsx` call `redirectServiceSubpath` (`features/services/lib/service-root-redirect.ts`) over `serviceSubpathLanding` (`features/services/lib/service-subpaths.ts`): `disks` → Disk, `metrics/<period>` → Metrics on the mapped window, `shell/<suffix>` → `shell?instance=<service id>-<suffix>`, `deploys/<id>/<anything>` → the deploy, and anything else (Render's `jobs`, `previews`, `pulls`, `sharing`, an unknown path) → the type's primary tab, as Render's `*` route sends it to the root. A service under the other base goes to its own with the sub-path kept; a static site lands a page it lacks on its Events. The Postgres and Key Value `metrics/<period>` landings take the same window (`metricsPeriodSearch`).
- **Queries:** `parseLogSearch` reads Render's `l`, `q` and custom `r` (bex's keys win; Render's leave the URL at the first write); `parseMetricsRangeSearch` reads `t` on the service, static site, Postgres and Key Value Metrics pages; the Events page opens its filter on `et` (`eventTypesHiddenBy`; Render's `ExtraInstancesChanged` is bex's `instance_count_changed`) and drops `et` at the first change.
- **Webhook and anchors:** `routes/webhook.$webhookId.activity.tsx` renders Activity, and the Activity tab links Render's path while staying selected on the bare URL. `/settings#account-security`, `/billing#free-usage` and `/billing#unbilled-charges` scroll to the Account section, the Included usage card and the Charges card through `useScrollToHashedCard`'s aliases, which now keeps the card in view while cards above it finish loading (at 390 the alias landed 220px past the card before).
- **Surfaces:** bex-api's `dashboardUrl`, REST, GraphQL, MCP and the alert emails emit only `/{segment}/<id>`, `/services/<id>/events|runs` and `/services/<id>/deploys/<id>`, which already open; none emits a Render sub-path this milestone maps.
- **Simplify:** the Events routes share one page that clears `et` itself; Render's `<startMs>,<endMs>` is one regular expression; the Activity tab adds its bare-URL selection to the router's own. **Tests:** sub-path landings and redirects, the routed aliases end to end, the log, metrics and events parsers, the datastore windows, the webhook path and the three anchors, each mutation-checked.
- **Follow-ups:** w3/160 (write `et` back), w3/161 (Metrics' other URL keys), w3/162 (the Instance failed Debug menu), w3/163 (Render's log keys on Postgres and Key Value Logs).

## Walk

**Locally, 2026-11-27** (t006 step 1): dev-3 (`bash scripts/dev-env.sh 3 status` all green), the dashboard from the tree on `:50030`, headless Chromium at 1440×900 and 390×844, signed in as a dev-3 user created for the walk, on a Pro workspace with a web service with a disk and a webhook created for the walk (all disposable, deleted afterwards). Every URL opened its view with no horizontal scroll at both widths: `/web/<srv>/disks`, `/worker/<srv>/disks?x=1#top` (query and hash kept) and `/static/<srv>/disks` (another type under `/static`) on the Disk page; `/web/<srv>/metrics/1w` on Last 7 days, `/pserv/<srv>/metrics/5m` on Last 30 minutes, `/metrics/forever` on Last 12 hours; `/pserv/<srv>/shell/<suffix>` on `shell?instance=<srv>-<suffix>` (dev-3's Shell says Shell access isn't available, so the picker was not drawn); `/web/<srv>/deploys/<dead id>/logs` on that deploy's Deploy not found; `/web/<srv>/jobs` and `/previews` on Deploys; Render's Instance failed Logs link on Logs with the Instance chip and the custom window, its lines filtered to the instance; `?q=hello&l=[["LEVEL","error"]]` with the search text and level; `?t=1h` and `?t=<ms>,<ms>` on Last hour and the custom window; `/events?et=DeployEnded` showing 1 of 2 with Event types: 1; `/webhook/<whk>/activity` and the bare URL on Recent deliveries with Activity selected; `/settings#account-security` and `/u/settings#account-security` with the Account section at the top; `/billing#free-usage` with Included usage at the top and `/billing#unbilled-charges` with Charges in view where bex's own `#charges` lands (the page ends below it). A static site's own pages are covered by the router tests: dev-3 builds no static site without a repository.

**Production:** waits for a pin carrying `7d1dce8611` (t006 step 2). Parked 2026-11-27: then a worker signs in as the QA user in `bex-canary` (`bash scripts/qa-login.sh`), creates a web service with a disk (disposable), opens the URLs above on `dashboard.bex.co` at 1440 and 390, records the pin and the results here, deletes what it created, and runs t010.

## Definition of done

1. `/web/<srv>/disks` (and each service segment's) opens the Disk page; `…/metrics/<legacy period>` opens Metrics on the window Render's table maps it to; `…/shell/<instanceId>` opens Shell on that instance.
2. `/webhook/<id>/activity` opens the webhook's Activity page.
3. A Render log link with `l=[["INSTANCE","<suffix>"]]` and `r=<start>,<end>` opens Logs filtered to that instance and window; `eventTypes=` filters Events; `timeRange=` sets Metrics' window; values bex cannot honour are ignored, not errors.
4. The three missing anchors scroll to their cards at 1440 and 390.
5. `dashboard-routes.md` § Service sub-tab map and the URL table (w3/m704) record every service sub-path, with `previews`, `pulls` (DO_NOT_DO: PR previews), `jobs` (DO_NOT_DO: one-off jobs) and `sharing` marked with their disposition.
6. Walked locally and, once a production pin carries the milestone, on production as the QA user in `bex-canary` on a service created for the walk (disposable).

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w3` (2026-11-26), item 4, verified against Render's bundle the same day.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). `dashboard-routes.md` § Service sub-tab map promises that the alias keeps a Render sub-path, which holds only where the names match.
- **Expected outcome:** a link from Render's event cards, CLI or docs into a service's disk, metrics window, instance shell or filtered logs opens that exact view.
- **Why now:** the datastore pages already map their sub-paths (w3/m641, m649), so services are the outlier; best after w3/m704, whose URL table t005 fills.
- **Render parity task:** included (t007): the milestone changes which dashboard URLs and queries open which views.
