# w3 · m537 — Every log explorer's range picker starts with Live tail, and Live is a range, not a switch, as on Render

**Worker:** worker3 **Goal:** every bex log explorer with a live tail (a service's Logs tab, a Postgres or Key Value Logs tab, a project's Application Logs and Request Logs views) has one control for when, as Render's explorer does: its time range picker, whose first entry is **Live tail**. Picking Live tail starts the tail, and picking any other range pauses it. The Live switch beside the range is gone, so a range and a Live state can no longer disagree (a custom range with Live on).

**Status:** blocked (t001–t005, t007–t009 done in `781b1d7c9`; t006's tree walk done; its production walk waits for a pin carrying `781b1d7c9`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Read Render's Live tail range from its dashboard bundle and record how it maps onto bex — **DONE** | 30m | — |
| [t002](done/t002.md) | A log range that can be Live tail: the picker's first entry and the URL's `live` range, with `live=0` links still read — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The service Logs tab: Live tail is picked in the range, the switch is gone, and New logs in Live Tail and the empty state select it — **DONE** | 1h | t002 |
| [t004](done/t004.md) | The Postgres and Key Value Logs tabs: Live tail in the range picker, the switch gone, the tab still opening on its one-shot range — **DONE** | 45m | t002 |
| [t005](done/t005.md) | A project's Application Logs and Request Logs views: Live tail in the range picker, the switch gone — **DONE** | 45m | t002 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, on the tree and on production | 45m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h30m**.

## Definition of done

1. **Live tail is a range.** In the service Logs tab, a Postgres or Key Value Logs tab and a project's Application Logs and Request Logs views, the time range picker lists **Live tail** first, then the presets (Last 30 minutes … Last 30 days, disabled past the plan's window as today), then Custom. Picking Live tail starts the tail. Picking a preset or a custom range stops it, as Render's `liveMode` is `timeRange === "live"`: a custom range never tails, open or closed. Where bex's tail cannot run (a filter it cannot follow, a Hobby request view), Live tail is disabled in the picker, the reason is shown as it is today, and an explorer that would open on it opens on the range t001 records instead.
2. **The switch is gone.** No explorer toolbar renders `LiveToggle`, and no skeleton reserves its slot. Each route's pending toolbar matches its ready toolbar at 1440 and 390 (the root `AGENTS.md` skeleton rule): `ServiceLogsBodySkeleton`, `DatastoreLogsSkeleton` and `ProjectLogViewPageSkeleton` in `dashboard/src/common/components/route-skeletons.tsx`.
3. **The URL names it as a range.** The service tab reads and writes Live tail as a `range` value, a datastore tab the same, and a project view as its `r` value (spelling from t001, Render's when Render's URL carries one). A link with `live=0` still opens, paused on the range it names, and `live` leaves the URL on the first write (the tombstone pattern of `CLEARED_LOG_SEARCH`). Metrics' range keys (`parseRangeSearch` in `features/metrics/lib/range.ts`) do not accept the Live tail value, and Metrics' picker does not list it.
4. **Defaults.** Every explorer opens on **Last hour** when its URL names no range, as Render's runtime explorers do (t001): the service Logs tab (Live on until now), a Postgres or Key Value Logs tab on its one-shot range (`DEFAULT_DATASTORE_LOG_RANGE`) with no tail, and a project view on the time range its saved search names, else Last hour. _Amended 2026-11-20 by t001: this point first said the service Logs tab opens on Live tail, but Render's opens on Last hour (its bundle's `Ie` falls back to `1h` for runtime logs, and render.com/docs/logging says "The default displayed range is Last hour"), so under Render parity first bex does too._
5. **What selects Live tail.** **New logs in Live Tail** selects Live tail in the picker. The empty state's action that turns Live on today selects Live tail. `Shift+L` and Options ▸ Clear logs are offered exactly while Live tail is the range (w3/092).
6. **View in context is unchanged.** A View in context link (w3/m528) still opens a new tab on a custom range 10 minutes either side of its line, paused, with no New logs in Live Tail. The marked line still loads older pages until it has half a panel of lines on its older side, then sits centred (`93051820b`, `71d3dc5e5`). Every w3/m528 test of the mark and its paging still passes unchanged.
7. **Tests and words.** Each behaviour above has a test that fails when it is reverted. Every new string is translated into all eight languages, and strings left unused (`logs.live`, and any other the switch alone used) are removed.
8. **Recorded.** The [capture](../../../../docs/render-artifacts/project-observability.md#the-view-page) § The view page records what t001 read about Render's Live tail. [ADR018](../../../../docs/ADR018-render-parity.md)'s Logs row "Log explorer display and shortcuts" says Live tail is the picker's first range, and its "Built differently" sentence about the Live switch (w3/093) is gone. Any difference left is named there with its follow-up item or listed under ADR018's recorded divergences.
9. **Live.** In a real browser at 1440 and 390, on the dev-3 tree and then on production once a pin carries the milestone, in a service Logs tab, a Postgres Logs tab and a project's Application Logs view: Live tail is the picker's first entry and starts the tail; a preset pauses it, and a new line brings up New logs in Live Tail, whose click selects Live tail; a `live=0` link opens paused; View in context opens on its line, paused and centred.

## Mapping onto bex (t001, 2026-11-20)

Render's bundle and docs were read on 2026-11-20 and the findings are in the [capture](../../../../docs/render-artifacts/project-observability.md#the-view-page) § The view page (**Live tail is a range**). t002–t005 build this:

- **URL.** Render spells Live tail `live`. The service and datastore tabs write it as `range=live` (bex's canonical key; the service tab also reads Render's `r=live`, as it reads its other `r` values), and a project view as `r=live`. A preset and a custom range keep their spellings. A `live=0` link opens paused on the range it names, which every range but Live tail now is, and `live` is a tombstone beside `t` and `r`, leaving the URL on the first write. Metrics' `parseRangeSearch`, a datastore's Metrics tab (`metricsRangeSearch`) and a project's Service Metrics view (`parseProjectMetricsSearch`) refuse `live`, and Metrics' picker does not list it.
- **What Live tail reads.** The last 4 hours, as Render's (`LIVE_TAIL_HISTORY`), sliding with the clock as Live on did, with bex's own paging (100 lines a page, Load older and scrolling up); the tail follows from that window's start. Every other range is a frozen snapshot (w4/399).
- **Defaults.** Last hour (`DEFAULT_LOG_RANGE`) for every explorer when its URL names no range: a service's Logs tab (no longer Live on), a Postgres or Key Value Logs tab (`DEFAULT_DATASTORE_LOG_RANGE`, as before) and a project view without a saved time range. A project view with one opens on it, as Render's.
- **Where bex's tail cannot run.** A filter the tail cannot follow (the app type with a request filter) disables Live tail in the picker, described by the note under the bar (`logs.liveUnsupported`), and an explorer on Live tail reads Last hour, Render's runtime default, until it can tail again. A Hobby request view disables Live tail too, its plan notice saying why, and its picker shows Last hour; its read keeps its window, so the refusal cannot move it. Render's tail has no such cases.
- **A custom range never tails**, ended or not, as Render's `liveMode` is `timeRange === "live"`. The window-closed note (`logs.liveWindowClosed`) and `useCustomWindow` went with the switch.
- **Edit view.** Render's Time Range field lists the relative ranges only, so a saved view never names Live tail and there is nothing to file for bex-api's saved-search validation.

## Live walk

**On the tree (t006, 2026-11-20).** Chromium through Playwright against the dashboard dev server (`vite dev`, this checkout) and the `local-bex` stub (`dashboard/scripts/local-bex.mjs`), as w3/m528's tree walk ran. The stub serves the service `eden-cms-v2` and the Postgres `dpg-c185th5c2rvvnhbfiltg` with a synthetic tail, but no project or saved search, so a project view waits for production. Pending toolbars were held by stalling the stub's GraphQL replies until they were measured.

| Point | Seen | Verdict |
| --- | --- | --- |
| Service Logs opens on Last hour, no switch | `/services/eden-cms-v2/logs` read Last hour, no `role=switch`, no stream line; New logs in Live Tail rose within seconds | holds |
| Live tail first; it tails | the list read Live tail, Last 30 minutes … Last 30 days, Custom…; Live tail wrote `?range=live`, showed "Live — streaming new lines", and a new last line arrived within 6 s | holds |
| A preset pauses it; New logs in Live Tail picks Live tail | Last 4 hours wrote `?range=4h`, the stream line went, the last line held, New logs in Live Tail rose; its click wrote `?range=live` and the tail resumed | holds |
| A `live=0` link opens paused | `?range=4h&live=0` opened on Last 4 hours with no tail, and the URL read `?range=4h` | holds |
| View in context | under the search `slow query` the row menu's link was `range=custom` ±10 minutes with `contextLine` and no `live`, `target=_blank`; it opened on the custom window, no tail, no New logs in Live Tail. The mark itself cannot be checked on the stub, which mints new line keys on every read (as w3/m528 found); its tests pass unchanged | holds, mark on production |
| Service pending against ready | 1440: [type 280, search 452, range 1075/1076, Filters 1247/1248, actions 1344] at y 269, all 32 px tall, within 1 px. 390: [type 16, search 173, Options 330] at y 56 over [range 16, Filters 173 at 201 px] at y 108, all 44 px tall. The first pass found the pending Filters slot 84 px wide at 390 against the button's 201; `LOG_FILTERS_BUTTON_CLASS` gained `max-sm:w-auto`, and a test pins it | holds after the fix |
| Postgres Logs | `?tab=logs` opened on Last hour, no switch; the list started Live tail, Last 30 minutes, Last hour; Live tail wrote `range=live` and streamed; Last 4 hours wrote `range=4h`, stopped it and raised New logs in Live Tail, whose click wrote `range=live` again. Pending and ready toolbars equal at 1440 (range 336, instance 508, level 744, search 916, actions 1288) and 390 | holds |

**Next:** on production as the QA user in `bex-canary` once a pin carries this milestone's commit, at 1440 and 390, with a short-lived web service logging about a line a second and a free Postgres: the service Logs tab, the Postgres Logs tab and a project's Application Logs view (Live tail first, it tails, a preset pauses it, New logs in Live Tail picks it, a `live=0` link opens paused, View in context opens on its line paused and centred), then each view's pending toolbar against its ready one. Delete the fixtures, log out, record it here, and close with t010.

## Source + Goal linkage

- **Source:** promoted from [w3/093](../../done/093.md) on 2026-11-20 by `/loopx w3`. The note was filed by [w3/m528](../../done/m528/README.md) t010's parity check: Render's range picker starts with Live tail, and w3/m528 t001 mapped Render's relative range onto bex's Live off on a preset range and kept the switch.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). A Render user looks for Live tail in the range list, and finds a switch beside it instead.
- **Expected outcome:** one control decides what an explorer shows, as on Render, and a shared link names Live tail the way it names a range.
- **Why now:** this is the last open follow-up from w3/m528. The explorer's Options menu, Display, Order by, Copy logs and Clear logs (w3/088–092) have shipped, so the range picker is the one part of the toolbar left that is shaped differently from Render's. w3/m528 itself waits only for a production re-walk of View in context, which this milestone must leave as it is.
- **Render parity task included:** the milestone changes the dashboard's log explorers and their URLs. No REST, GraphQL or MCP surface changes. A Time Range in a saved view that names Live tail would reach bex-api's saved-search validation; t001 records whether Render's Edit view offers one, and files it separately if it does.
