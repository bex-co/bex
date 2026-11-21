# w3 · m537 — Every log explorer's range picker starts with Live tail, and Live is a range, not a switch, as on Render

**Worker:** worker3 **Goal:** every bex log explorer with a live tail (a service's Logs tab, a Postgres or Key Value Logs tab, a project's Application Logs and Request Logs views) has one control for when, as Render's explorer does: its time range picker, whose first entry is **Live tail**. Picking Live tail starts the tail, and picking any other range pauses it. The Live switch beside the range is gone, so a range and a Live state can no longer disagree (a custom range with Live on).

**Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Read Render's Live tail range from its dashboard bundle and record how it maps onto bex | 30m | — |
| [t002](t002.md) | A log range that can be Live tail: the picker's first entry and the URL's `live` range, with `live=0` links still read | 1h | t001 |
| [t003](t003.md) | The service Logs tab: Live tail is picked in the range, the switch is gone, and New logs in Live Tail and the empty state select it | 1h | t002 |
| [t004](t004.md) | The Postgres and Key Value Logs tabs: Live tail in the range picker, the switch gone, the tab still opening on its one-shot range | 45m | t002 |
| [t005](t005.md) | A project's Application Logs and Request Logs views: Live tail in the range picker, the switch gone | 45m | t002 |
| [t006](t006.md) | Verify in a real browser at 1440 and 390, on the tree and on production | 45m | t003, t004, t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

Total: **10 tasks, about 6h30m**.

## Definition of done

1. **Live tail is a range.** In the service Logs tab, a Postgres or Key Value Logs tab and a project's Application Logs and Request Logs views, the time range picker lists **Live tail** first, then the presets (Last 30 minutes … Last 30 days, disabled past the plan's window as today), then Custom. Picking Live tail starts the tail. Picking a preset or a custom range stops it, as Render's `liveMode` is `timeRange === "live"`: a custom range never tails, open or closed. Where bex's tail cannot run (a filter it cannot follow, a Hobby request view), Live tail is disabled in the picker, the reason is shown as it is today, and an explorer that would open on it opens on the range t001 records instead.
2. **The switch is gone.** No explorer toolbar renders `LiveToggle`, and no skeleton reserves its slot. Each route's pending toolbar matches its ready toolbar at 1440 and 390 (the root `AGENTS.md` skeleton rule): `ServiceLogsBodySkeleton`, `DatastoreLogsSkeleton` and `ProjectLogViewPageSkeleton` in `dashboard/src/common/components/route-skeletons.tsx`.
3. **The URL names it as a range.** The service tab reads and writes Live tail as a `range` value, a datastore tab the same, and a project view as its `r` value (spelling from t001, Render's when Render's URL carries one). A link with `live=0` still opens, paused on the range it names, and `live` leaves the URL on the first write (the tombstone pattern of `CLEARED_LOG_SEARCH`). Metrics' range keys (`parseRangeSearch` in `features/metrics/lib/range.ts`) do not accept the Live tail value, and Metrics' picker does not list it.
4. **Defaults.** The service Logs tab opens on Live tail, as it opens with Live on today. A Postgres or Key Value Logs tab still opens on its one-shot range (`DEFAULT_DATASTORE_LOG_RANGE`) with no tail. A project view opens on the time range its saved search names, else on the default t001 records from Render.
5. **What selects Live tail.** **New logs in Live Tail** selects Live tail in the picker. The empty state's action that turns Live on today selects Live tail. `Shift+L` and Options ▸ Clear logs are offered exactly while Live tail is the range (w3/092).
6. **View in context is unchanged.** A View in context link (w3/m528) still opens a new tab on a custom range 10 minutes either side of its line, paused, with no New logs in Live Tail. The marked line still loads older pages until it has half a panel of lines on its older side, then sits centred (`93051820b`, `71d3dc5e5`). Every w3/m528 test of the mark and its paging still passes unchanged.
7. **Tests and words.** Each behaviour above has a test that fails when it is reverted. Every new string is translated into all eight languages, and strings left unused (`logs.live`, and any other the switch alone used) are removed.
8. **Recorded.** The [capture](../../../docs/render-artifacts/project-observability.md#the-view-page) § The view page records what t001 read about Render's Live tail. [ADR018](../../../docs/ADR018-render-parity.md)'s Logs row "Log explorer display and shortcuts" says Live tail is the picker's first range, and its "Built differently" sentence about the Live switch (w3/093) is gone. Any difference left is named there with its follow-up item or listed under ADR018's recorded divergences.
9. **Live.** In a real browser at 1440 and 390, on the dev-3 tree and then on production once a pin carries the milestone, in a service Logs tab, a Postgres Logs tab and a project's Application Logs view: Live tail is the picker's first entry and starts the tail; a preset pauses it, and a new line brings up New logs in Live Tail, whose click selects Live tail; a `live=0` link opens paused; View in context opens on its line, paused and centred.

## Source + Goal linkage

- **Source:** promoted from [w3/093](../done/093.md) on 2026-11-20 by `/loopx w3`. The note was filed by [w3/m528](../blocked/m528/README.md) t010's parity check: Render's range picker starts with Live tail, and w3/m528 t001 mapped Render's relative range onto bex's Live off on a preset range and kept the switch.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). A Render user looks for Live tail in the range list, and finds a switch beside it instead.
- **Expected outcome:** one control decides what an explorer shows, as on Render, and a shared link names Live tail the way it names a range.
- **Why now:** this is the last open follow-up from w3/m528. The explorer's Options menu, Display, Order by, Copy logs and Clear logs (w3/088–092) have shipped, so the range picker is the one part of the toolbar left that is shaped differently from Render's. w3/m528 itself waits only for a production re-walk of View in context, which this milestone must leave as it is.
- **Render parity task included:** the milestone changes the dashboard's log explorers and their URLs. No REST, GraphQL or MCP surface changes. A Time Range in a saved view that names Live tail would reach bex-api's saved-search validation; t001 records whether Render's Edit view offers one, and files it separately if it does.
