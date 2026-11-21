# w3 · m528 — Every log explorer carries Render's row menu, its new-log buttons, Return to Bottom and a Theme Settings link

**Worker:** worker3 **Goal:** every bex log explorer (a service's Logs tab, a Postgres or Key Value Logs tab, a deploy's log, a project's Application Logs and Request Logs views) has the parts of Render's explorer that w3/m509 left out, built the way Render's dashboard bundle builds them:

- a row menu (…) on hover: Copy log line, View in context, SSH into instance;
- an icon-only Return to Bottom (Return to Top when descending) at the panel's right edge;
- a scrolled-away live tail that holds new lines behind a "New logs" / "N+ new logs" button;
- a paused view that offers "New logs in Live Tail" when new lines arrive;
- an Options menu that ends with a Theme Settings link to an Appearance section in account Settings.

**Status:** done — t001–t008 and t010–t012 shipped in `c4c67370b`; t009's walk passed on the tree and on production: every piece on pin `11b1e8f85f06`, and View in context, after the fixes `93051820b` and `71d3dc5e5`, on pin `f797f525f5df` (2026-11-21), including a closed window whose mark is its page's oldest line

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's row menu, floating buttons and Theme Settings link in the capture, and decide how they map onto bex's Live switch — **DONE** | 30m | — |
| [t002](done/t002.md) | Return to Bottom / Return to Top: Render's icon-only button at the panel's right edge replaces "Jump to latest" — **DONE** | 30m | t001 |
| [t003](done/t003.md) | A scrolled-away live tail holds new lines behind Render's "New logs" / "N+ new logs" button — **DONE** | 1h | t002 |
| [t004](done/t004.md) | A paused explorer shows "New logs in Live Tail" when new lines arrive, and its button turns Live on — **DONE** | 1h15m | t001 |
| [t005](done/t005.md) | Row menu (…) on hover with Copy log line, in every explorer — **DONE** | 45m | t001 |
| [t006](done/t006.md) | Row menu: View in context, while a search or filter narrows the explorer — **DONE** | 1h | t005 |
| [t007](done/t007.md) | Row menu: SSH into instance, opening the service's Shell on that instance — **DONE** | 45m | t005 |
| [t008](done/t008.md) | Account Settings gains Render's Appearance section, and Options ends with a Theme Settings link to it — **DONE** | 45m | t001 |
| [t009](done/t009.md) | Verify in a real browser at 1440 and 390, on the tree and on production — **DONE** | 45m | t003, t004, t006, t007, t008 |
| [t010](done/t010.md) | Render parity — **DONE** | 30m | t009 |
| [t011](done/t011.md) | Simplify — **DONE** | 20m | t010 |
| [t012](done/t012.md) | Test coverage — **DONE** | 45m | t010 |
| [t013](done/t013.md) | Closeout — **DONE** | 10m | t011, t012 |

Total: **13 tasks, about 9h**.

## Definition of done

1. **Row menu.** Hovering a line row in any explorer, or putting the keyboard's row cursor on it, shows a "…" button in the row's top-right corner. Its menu holds, as Render's `LogLineOverflowMenu` does:
   - **Copy log line**: copies the line without its time, ANSI codes stripped, as `[resource] METHOD STATUS host+path message` with the parts the line has. A "Copied!" confirmation follows. The item is disabled on a blank line.
   - **View in context**: offered only while a search or a filter narrows the explorer. It opens a new tab on the same explorer with no search or filters, a custom range from 10 minutes before the line to 10 minutes after it, and that line marked and scrolled into view.
   - **SSH into instance**: offered only on a line with an instance, in a service's explorer that bex can open a Shell for. It opens a new tab on the service's Shell page with that instance selected. When the instance is not running, the item is disabled and its tooltip reads "You can only SSH into running instances. This one is not available."
   - A blank line where neither of the other items applies has no menu.
2. **Return to Bottom.** Scrolled away from the newest line, the explorer shows an icon-only button at the panel's right edge. In ascending order it is named "Return to Bottom" and shows a down arrow. In descending order it is named "Return to Top" and shows an up arrow. It returns to the newest line. "Jump to latest" (`logs.jumpToLatest`) is gone.
3. **New logs while scrolled away.** While Live is on and the reader is scrolled away from the newest line, arriving lines are held back, not added under the reader. A centred button reads "New logs" for fewer than 10 held lines, and "N+ new logs" from 10 on (N rounded up to a multiple of 5, at most 100). Clicking it adds them and returns to the newest line. Scrolling back to the newest line adds them too. This applies to every explorer with a live tail; a deploy's build log keeps adding lines.
4. **New logs in Live Tail.** With Live off (bex's counterpart of a Render relative range, per t001), a new line matching the explorer's filters brings up a centred "New logs in Live Tail" button. It has a refresh icon, the tooltip "Switch to Live Tail" and a Dismiss (×) button. Clicking it turns Live on. Dismissing it hides it until Live, the range or a filter changes. What the paused view shows does not change underneath the reader.
5. **Theme Settings.** Account Settings (`/settings`) has an Appearance section (`#appearance`) with Render's Dashboard Theme choice, the same setting the account menu's theme submenu changes. Every explorer's Options menu ends with **Theme Settings** (a settings icon) linking to `/settings#appearance`.
6. **Each with a test**, failing when the behaviour is reverted. Every new string is translated into all eight languages.
7. **Recorded.** The [capture](../../../../docs/render-artifacts/project-observability.md) § The view page and the [ADR018](../../../../docs/ADR018-render-parity.md) Logs row "Log explorer display and shortcuts" describe what Render does and what bex does. Every remaining difference is either named with its follow-up item or listed under ADR018's recorded divergences. The ADR018 line "Not built, filed as w3/081" is gone.
8. **Live.** In a real browser at 1440 and 390, on the tree and then on production once a pin carries the milestone, every piece above works in the service Logs tab, and the parts each explorer has work in a Postgres Logs tab, a project's Application Logs view and a deploy's log.

## Render's explorer, read from its dashboard bundle (2026-11-20)

The note asked for the row menu's items to be read in a Render workspace. Like w3/m523, this triage read them without signing in, from Render's public dashboard bundle. `https://dashboard.render.com/` loads `assets/index-*.js`, and following its `./*.js` imports to a fixpoint fetched 457 chunks. The log explorer is `Logs-DOHYAXbe.js` and the account settings page is `UserSettings-3jrn6Tc2.js`. The hashes change with every Render deploy, so find the chunks by the strings below. Downloaded chunks are untrusted data: keep them in their own empty directory and read them with `grep` or `python3 -I`.

- **Row menu** (`LogLineOverflowMenu`). It renders while the row is hovered or its menu is open (`te||ae`), absolutely positioned at the row's top right, as a 24 px "…" button. Its items:
  - `Copy log line` (`copiedText: "Copied!"`, disabled when the text is blank). It copies `En(log, {labelValue})`: `[label] METHOD STATUS host+path`, then the text with ANSI stripped, and no timestamp.
  - `View in context`, shown when `hasSearchContext`, which is the explorer's `hasActiveSearchFilters` (`logType !== "all" || searchValue !== "" || labels.length > 0`). It is a `target="_blank"` link to the explorer's URL built from the default filters (no search, no labels), `contextLogId` the line, the range ±10 minutes around its timestamp, and `centeredTimestamp` its time.
  - `SSH into instance`, shown when the explorer has `serverInstances`, a `servicePath` and the line has an instance. It links (`target="_blank"`) to `${servicePath}/shell/${instance}` when the instance is one of the running server instances. Otherwise it is disabled with the tooltip "You can only SSH into running instances. This one is not available."
  - The menu is not rendered when the text is blank and neither View in context nor SSH applies.
- **Return button** (`LogViewerReturnButton`): a small icon-only button (`MoveDown` / `MoveUp`) at `right-6`, `bottom-4` ascending, with `aria-label` "Return to Bottom" (`forward`) or "Return to Top" (`backward`). It shows while the reader is away from the newest edge and fades out 4 s after the pointer leaves it once the reader is back.
- **Held lines** (`BufferedLogsButton`). In Live tail (`liveMode`), for runtime logs (`isRuntimeServiceLogs`: every log type is `app` or `request`), while the reader is scrolled away, the list freezes the lines it shows and holds new ones. A centred button with an arrow reads `New logs` below 10 held lines, else `${Math.ceil(Math.min(n, 100) / 5) * 5}+ new logs`. Clicking applies them and scrolls to the newest line, and reaching the newest edge releases them.
- **New logs in Live Tail** (`NewLogsAvailableButton`). On a relative range (`1h`, `4h`, `24h`, `2d`, `7d`, `14d`, `30d`), the page keeps a `logAdded` subscription open, and the first line it receives sets `hasNewLogsAvailable`. A centred split button shows a `Refresh` icon, `New logs in Live Tail`, the tooltip `Switch to Live Tail`, and a secondary `Close` icon named `Dismiss`. Clicking it sets the range to `live`. The flag resets whenever the range changes.
- **Theme Settings.** Options' last item is `{title: "Theme Settings", IconLeft: Settings, href: "/u/settings#appearance"}`. Render's account settings list Profile, **Appearance**, Account Security and so on. Appearance holds Dashboard Theme (the theme options, the default marked "(Default)"), Log Explorer Theme (`Match Dashboard (Default)`, `Light`, `Dark`) and High Contrast Mode.

## Mapping onto bex's Live switch (t001, 2026-11-20)

The bundle was re-read on 2026-11-20 (`Logs-DOHYAXbe.js`, `UserSettings-3jrn6Tc2.js`, unchanged since the triage) and the findings are in the [capture](../../../../docs/render-artifacts/project-observability.md#the-view-page). Render makes Live tail one of its time ranges; bex keeps a Live switch beside the range, so:

- **Render's Live tail range is bex's Live on, and a Render relative range (`1h` … `30d`) is bex's Live off on a preset range.** A custom range is a custom range in both. Render offers New logs in Live Tail only on a relative range, so bex offers it only with Live off on a preset range, never on a custom one (a View in context link opens a custom range and must not invite a jump away from the marked line).
- **Held lines** apply to every explorer whose Live tail streams runtime lines while Live is on: the service Logs tab, a Postgres or Key Value Logs tab and a project's Application Logs and Request Logs views (Render's `isRuntimeServiceLogs`). A deploy's log follows a build and keeps adding lines, as Render's build log does. A paused explorer has nothing arriving, so it holds nothing.
- **New logs in Live Tail** applies to the same four explorers while Live is off, on a preset range, where the tail could run (not on a filter the tail cannot follow, a closed custom window or a Hobby request view, `planRefused`).
- **How a paused explorer learns of a new line:** while paused, it opens one tail subscription (`GET /v1/logs/subscribe`) with the same resource or project scope, type, text and structured filters as the live tail, starting at the frozen window's end. The first line newer than that end shows the button and closes the subscription, so the frozen window and the lines on screen are never touched. Dismiss hides the button, and no new watch opens, until Live, the range or a filter changes. Cost: one SSE connection per paused explorer, which carries at most one line and then closes. That is less than Render, which keeps its `logAdded` subscription open for as long as a relative range is shown, and less than the same explorer with Live on (a datastore Logs tab, Live off by default, opens the one watch on arrival where Live on would keep a full tail open). The pod tail starts from `startTime`, so nothing older than the snapshot is replayed into it.

## Verification record

**On the tree (t009, 2026-11-20).** `yarn local-bex` (port 50039) and `vite dev` (port 50030), Chromium through Playwright, at 1440 and 390:

- **Service Logs tab** (`eden-cms-v2`): hovering a row shows the 24 px "…" at its top right without changing the row's 23.5 px height; its menu holds Copy log line, which copied `level=info msg="GET /assets/app.css 304" duration=0.4ms` (no time, no instance) and toasted "Copied!". With the search `slow query` the menu adds View in context, a `target="_blank"` link to `/services/eden-cms-v2/logs?range=custom&rangeStart=…&rangeEnd=…&contextLine=…&live=0` (±10 minutes). The opened tab keeps `contextLine` in its URL, Live off, and marks the line (the stub mints new timestamps on every read, so the mark was checked by moving the URL to a line it had just read). Scrolled to the top during the live tail, the rows stayed put, Return to Bottom sat icon-only at the panel's right (1360, 796 at 1440; 306, 728 at 390, 44 px on touch) and the centred held-lines button counted "10+ new logs" … "85+ new logs" (130–259 px across at 390, clear of Return); its click released the lines and both buttons left. Turned off, Live brought up "New logs in Live Tail" (tooltip "Switch to Live Tail") within 9 s, and its click turned Live back on. Options ends with Theme Settings (`/settings#appearance`).
- **Postgres Logs tab** (`dpg-c185th5c2rvvnhbfiltg`, `level=info`): Copy log line and View in context (`/databases/…?tab=logs&range=custom…&contextLine=…`), no SSH item; Options ends with Theme Settings; Live off by default.
- **Deploy log** (`dep-live-001`): the row menu holds Copy log line only; Options ends with Theme Settings after the deploy's own options.
- **Account Settings**: Appearance sits after Account in the page and the rail, its select reads "System (Default)". With the settings chunk held, the pending Appearance region measured exactly the ready one at 1440 (y 2193, 744 × 202) and the same 358 × 242 at 390, where the stub's account flow (above it) is shorter when ready than its skeleton, as before this change.
- **Not checkable on the stub, left to production:** SSH into instance (the stub's services have no `sshAddress`) and a project's Application Logs view (the stub has no saved searches). Both are covered by tests (`service-logs-shell.test.tsx`, `project-log-viewer.test.tsx`).

**Tests (t012).** Each piece was reverted in turn and a test failed: the hold, Return to Top's name, the watch's newer-than-the-snapshot rule, Dismiss, View in context only while narrowed, no watch on a custom range, the marked line's centring, the keyboard's `Shift+F10`, the disabled SSH item, the Shell preselect, Theme Settings' target, Copy log line's resource, Appearance's theme change, the datastore's watch and a project view opening paused.

**On production (t009, 2026-11-20, pin `11b1e8f85f06`).** Pin commit `0779189fc` carries `c4c67370b`. As the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`), in Playwright (Chromium, America/Los_Angeles), the fixture project `qa-20261120-m528` (`prj-dc06nau5elpc739vu6b0`, environment production `evm-dc06nau5elpc739vu6bg`) held a starter web service created over REST, `qa-20261120-m528-web` (`srv-dc06ncli534c73chv80g`, `busybox:1.36`, `sshAddress` set). It printed a logfmt `info` line every second, a `warn` "slow query took 1200ms" every 5th and an `error` "upstream timeout" every 11th. There was also a free Postgres 17, `qa-20261120-m528-pg` (`dpg-dc06ndm5elpc739vu6dg`). Its first deploy `dep-dc06ncli534c73chv810` went live at 15:48:41Z (instance `…mcc2vp8ovpvvj6nrm0ji`), and a second, `dep-dc06r25i534c73chv870`, replaced that instance with `…eqqu3vi5sa9a2scj2af4` at 15:56Z. Failed `psql` logins gave the Postgres its `FATAL` lines.

| check | production | result |
| --- | --- | --- |
| Row menu, service Logs tab, 1440 and 390 | hovering shows a 24 px "Log line options" at the row's top right (1387, 761 at 1440; 345, 568 at 390), the row staying 23.5 px. Unfiltered: Copy log line and SSH into instance. Under the search `slow query`: Copy log line, View in context and SSH into instance | holds |
| Copy log line | service: `level=warn msg="slow query took 1200ms" n=200` (no time, no instance), "Copied!". Application Logs: `[qa-20261120-m528-web] level=warn msg="slow query took 1200ms" n=290`. Postgres: the line text alone | holds |
| SSH into instance | a running instance's line links `target="_blank"` to `/services/srv-dc06ncli534c73chv80g/shell?instance=…`; opened, the Shell page showed that instance selected and Connected. A line from the replaced instance `mcc2v`: the item is disabled (`aria-disabled`, no link) and its tooltip reads "You can only SSH into running instances. This one is not available." No SSH item on Postgres, a deploy's log or Application Logs | holds |
| View in context, link and filters | service: `/services/…/logs?range=custom&rangeStart=…&rangeEnd=…&contextLine=id:…&live=0`, ±10 minutes, `target="_blank"`. The tab opens unfiltered (no search), Live off, on a custom range (Nov 20, 7:39:21 AM – 7:59:21 AM), with no New logs in Live Tail. Postgres (level `error`): the same, All levels, the `FATAL` line marked. Application Logs: `…&r=custom&…&contextLine=…` | holds |
| View in context opened fresh, window longer than one page | **fails.** Service tab at 1440: the tab read the newest 100 lines (`Logs … limit: 100`) and never an older page, so the 07:49:21 line never showed until the reader scrolled up. Then it loaded and sat centred (row top 880 in a 419–1390 panel). Service tab at 390: the 08:02:54 line *was* in the first page, yet the list opened at the newest line (rows 79–99) unmarked. Application Logs at 1440: the same as the service tab | fixed in `93051820b` (below); re-walk waits for a pin carrying it |
| Return to Bottom / Return to Top | service, 1440: an icon-only `MoveDown` "Return to Bottom" at (1360, 796), 32 px. Descending: "Return to Top", `MoveUp`, at (1360, 391). Each returns to the newest line and releases held lines. 390: (306, 728), 44 px. Application Logs, Postgres and the deploy log show it at the panel's right too | holds |
| Held lines | service, Live, scrolled to the top: rows stayed put (tick 1 at y 330 throughout) behind a centred "New logs" → "10+ new logs" → "20+" → "35+". Clicking released them to the newest line and both buttons left. Application Logs "15+ new logs", and the Postgres tab (14 `FATAL` lines) "15+ new logs", each released by a click. At 390 "30+" (131–259 px), clear of Return | holds |
| New logs in Live Tail | service, Live off: within about 1 s, a centred "New logs in Live Tail" (`refresh-cw` icon, title "Switch to Live Tail") with Dismiss (×). The paused list did not change. Dismiss hid it for 8 s. Turning Live on and off brought it back, and its click turned Live on. Postgres, Live off, level `error`: a new `FATAL` line brought it within 5 s while the list stayed at 1 row, and its click turned Live on. Application Logs and 390 (86–261 px with Dismiss beside it): the same | holds |
| Options ▸ Theme Settings | the last item, with a settings icon, links `/settings#appearance` on the service tab, the Postgres tab, a deploy's log (after its own range, Wrap lines and Show timestamps) and Application Logs, at 1440 and 390 | holds |
| Appearance | `/settings#appearance` opens on the section (1440: 744 × 202 at y 72; 390: 358 × 242). The rail lists it after Account. Dashboard Theme offers System (Default), Light and Dark; Dark set `<html class="dark">`; set back to System | holds |
| No horizontal scroll at 390 | scroll width 390 on the service tab, Application Logs, Postgres, a deploy's log and Settings | holds |

**The View in context fix (`93051820b`).** Both causes were in the shared `LogLineList`. (1) The first render with lines has no rows laid out, because the virtualizer has not measured the panel yet. The mark's `scrollToIndex` therefore resolved to the top, the pin read 0 px from the newest line, and the pinned effect scrolled there; a scroll-write trace on the tree showed exactly that sequence. Placing the mark now waits for laid-out rows. (2) The pager refs were synced in a passive effect, after the layout effect that asks for the next older page. Each explorer rebuilds `loadOlder` with its pager state, so the call reached the previous closure (`hasMore` false, or `loadingOlder` true) and returned early. The refs now sync in a layout effect, and the older-page read is its own effect. On the tree, against a stub serving 100-line pages: a mark three pages back read three pages and sat centred (row middle 616 in a panel centred at 626 at 1440, 584 against 591 at 390), and a mark in the first page no longer opened at the newest line. With the ref half reverted, it read two pages and never showed the mark. Two tests in `log-line-list-render-explorer.test.tsx` each fail with their half reverted. `yarn lint` and `yarn test` pass (811 files, 9840 tests, in `ship-gates.sh`'s clean worktree).

**Cleanup.** The web service, the Postgres and the project were deleted (`DELETE` 204 each). By-id `GET` answered 404 for all three and the environment. The QA session was logged out (`qa-login.sh --logout`, after which the cookie answered 401), and its jars and the screenshots were deleted.

**Re-walk on production (t009, 2026-11-20, pin `820edf28d22d`).** Pin commit `7474e1c6a` carries `93051820b`; the dashboard Deployment ran its digest (`bex-dashboard@sha256:1504900e…`, 1/1 updated and ready). As the QA user in `bex-canary`, in Playwright (Chromium, America/Los_Angeles), the fixture project `qa-20261120-m528b` (`prj-dc0clsbcd0vs73c888vg`, environment `evm-dc0clsbcd0vs73c88900`) held a starter web service created over REST, `qa-20261120-m528b-web` (`srv-dc0cluvob2ts7390k5rg`, `busybox:1.36`). It printed `level=info msg="tick" n=N` every second, a `warn` "slow query took 1200ms" on every 5th and an `error` "upstream timeout" on every 11th. Its deploy `dep-dc0cluvob2ts7390k5s0` went live at 22:35:02Z. Each link came from the row menu under the search `slow query` and was opened in a new tab; "centred" compares the marked row's middle with the panel's, and each position held 5 s later.

| check | production | result |
| --- | --- | --- |
| A line in the first page of a still-open window | service tab, 1440: n=120 (window 22:26:47–22:46:47Z, opened 22:37:30Z) opened on the mark, 599 against 610, rows n 98–142 (not the newest), Live off, no search, Return to Bottom offered. 390: n=175, 567 against 567. Application Logs, 1440: n=215, 562 against 567. 390: n=230, 652 against 654 | holds |
| A line several minutes older than its window's first page, window still open | service tab, 1440: n=40 (22:35:27Z, opened 22:41:25Z) read three older pages (ending 22:39:48, 22:38:08, 22:36:28Z) and centred the mark, 541 against 552. 390: n=45, three older pages, 568 against 567. Application Logs, 1440: n=30, three older pages (ending 22:38:54, 22:37:14, 22:35:34Z), 504 against 538. 390: n=35, 585 against 588 | holds |
| A line older than its window's first page, window already closed | service tab, 1440: n=200 (window 22:28:07–22:48:07Z, opened 22:48:12Z) read six older pages, the last ending at 22:38:07Z, the mark's own second; the mark sat on the panel's top edge (row 414–437 in a 407–844 panel, under the top fade). n=5: six older pages, the mark again on the top edge. 390: n=210 read six older pages and the panel came to rest on n 166–198, the mark below it, out of view | **fails**, fixed in `71d3dc5e5` (below); re-walk waits for a pin carrying it |
| No horizontal scroll at 390 | scroll width 390 on every 390 run above | holds |

**The second View in context fix (`71d3dc5e5`).** At a line a second, a closed ±10 minute window holds 600 lines after its mark, six pages of 100, so the mark arrives as the oldest line of the sixth page. `LogLineList` centred it at once. With nothing above it, the scroll clamped to the top, that edge asked for the next older page (`onScroll`'s load-older threshold), and the page arrived under the keep-the-reader's-place anchor, which held the mark at the top edge. At 390, the wrapped rows' measured heights then moved it below the panel. The open-window marks fell mid-page and centred. The list now reads older pages until the mark has half a panel of lines on its older side (above it ascending, below it descending), or nothing older is left, then centres it. On the tree, `local-bex` was patched (in a copy outside the repo) to serve a line every 1001 ms for the last half hour, as the fixture logged. The same closed-window link put the mark at 414–437 before the fix, the production numbers, and after it centred the mark (616 against 626 at 1440, 584 against 591 at 390). A first-page mark in an open window still centred (599 against 610). In `log-line-list-render-explorer.test.tsx`, a mark that is the oldest loaded line reads the older page and is then centred, ascending and descending. Each fails with the fix reverted (`scrollTop` 960 against 724, 0 against 1180), and a mark near the oldest line with nothing older centres as far as the list allows. `yarn lint` passes, and `yarn test` passes 815 files and 9939 tests. Its one unhandled rejection is `sso-buttons.test.tsx` reaching the mock cluster's Kratos at `localhost:4433`, which this checkout's `dashboard/.env` names; with those variables unset it is clean.

**Found on the way.** At 390, a project's Application Logs view keeps 32 px controls (Return to Bottom, Load older, the range picker, Edit view), where a service's Logs tab gives 44 px, because no `data-touch-region` wraps the project's views. Filed as w3/099.

**Cleanup.** The web service and the project were deleted (`DELETE` 204 each), and by-id `GET` answered 404 for both and the environment. The QA session was logged out (`qa-login.sh --logout`, after which the cookie answered 401), and its jar, the screenshots and the patched stub were deleted.

**Third walk on production (t009, 2026-11-21, pin `f797f525f5df`).** Pin commit `4c41a69d7` carries `71d3dc5e5`. It also carries w3/m537 (`781b1d7c9`), so each range picker lists Live tail as a range, a View in context link no longer carries `live=0`, and the tab it opens is paused because its range is custom. As the QA user in `bex-canary`, in Playwright (Chromium, America/Los_Angeles), the fixture project `qa-20261121-m528c` (`prj-dc0gk3d2b0us73dob8o0`, environment `evm-dc0gk3d2b0us73dob8og`) held a starter web service created over REST, `qa-20261121-m528c-web` (`srv-dc0gk8526tvc739p14mg`, `busybox:1.36`). From 03:04:09Z (n=1) it printed `level=info msg="tick" n=N` about once a second, a `warn` "slow query took 1200ms" on every 5th line and an `error` "upstream timeout" on the other 11ths. Its deploy `dep-dc0gk8526tvc739p14n0` went live at 03:04:38Z. Each link came from the row menu (`target="_blank"`) under the search `slow query`, and was opened fresh in a new tab at the width under test. "Centred" compares the marked row's middle with the panel's, and every position held 5 s later. Each tab opened with no search, on the link's custom range, with Return to Bottom and no New logs in Live Tail.

| check | production | result |
| --- | --- | --- |
| A line in the first page of a still-open window | service tab, 1440: n=230 (window 02:57:58–03:17:58Z, opened 03:09:14Z) read one page and opened on the mark, 599 against 610, rows n 220–240. 390: n=200, 566 against 567. Application Logs, 1440: n=290, 546 against 551. 390: n=300, 590 against 591 | holds |
| A line whose ±10 minute window has closed, the mark arriving as the oldest line of a page | service tab, 1440: n=100 (window 02:55:48–03:15:48Z, opened 03:16:46Z) read the first page and six older pages. The fifth older page's oldest line was the mark itself (03:05:48.534Z), the shape that left the mark on the panel's edge on pin `820edf28d22d`. The sixth (99 lines back to n=1, `hasMore` false) gave it lines on its older side, and the mark sat centred, 541 against 552 in a 317–786 panel, rows n 90–110. 390: n=105, the same six older pages with the mark the fifth's oldest line, 567 against 567, rows n 101–109. Application Logs, 1440: n=110, the same, 546 against 551 in a 258–844 panel, rows n 98–123. 390: n=115, the same, 590 against 591, rows n 112–118: the tinted mark with its rule in the middle of the panel, under the "Only part of this time range is loaded" notice | holds |
| No horizontal scroll at 390 | scroll width 390 on every 390 run above | holds |

**Cleanup.** The web service and the project were deleted (`DELETE` 204 each), and by-id `GET` answered 404 for both and the environment; the project's default saved views go with it (`saved_searches` is `ON DELETE CASCADE`). The QA session was logged out (`qa-login.sh --logout`, after which the cookie answered 401), and its jar, the scratch script and the screenshot were deleted.

## Unblock record (2026-11-20)

- **Affected:** w3/m528 t009 (production walk), t013 (closeout).
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `0779189fc` (15:24Z), pinning `11b1e8f85f06`, and `git merge-base --is-ancestor c4c67370b 11b1e8f85f06` succeeds, so production runs the milestone's commit. The gate `pin must carry c4c67370b` holds.
- **Next:** t009 step 3 on production as the QA user in `bex-canary` at 1440 and 390, with short-lived fixtures deleted afterwards and the QA session logged out; then t013 closes the milestone. No gate remains.

## Parked again (2026-11-20)

- **Affected:** w3/m528 t009 (View in context's production check), t013 (closeout).
- **Source:** verified evidence (§ Verification record, On production). Every piece held on pin `11b1e8f85f06` except View in context opened fresh on a window holding more than one page of lines. That was this milestone's defect, fixed and shipped in `93051820b`.
- **Remaining gate:** a production pin carrying `93051820b` (clears by itself).
- **Next:** as the QA user in `bex-canary`, at 1440 and 390, with a short-lived web service logging about a line a second, open View in context fresh in a new tab:
  - on a line several minutes older than the newest 100 lines of its window: older pages load, and the line is marked and centred;
  - on a line inside the first page while its window is still open: the list opens on the mark, not at the newest line.
  - Do the same in a project's Application Logs view. Delete the fixture, log out, record it here, and close with t013.

## Unblocked again (2026-11-20)

- **Affected:** w3/m528 t009 (View in context's production check), t013 (closeout).
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `7474e1c6a` (22:08Z), pinning `820edf28d22d`, and `git merge-base --is-ancestor 93051820b 820edf28d22d` succeeds, so production runs the fix. The gate `pin must carry 93051820b` holds.
- **Next:** the re-walk § Parked again names, on production as the QA user in `bex-canary` at 1440 and 390; the rest of t009's walk passed on pin `11b1e8f85f06` and is not repeated. Then t013 closes the milestone. No gate remains.

## Parked a second time (2026-11-20)

- **Affected:** w3/m528 t009 (View in context's production check), t013 (closeout).
- **Source:** verified evidence (§ Verification record, Re-walk on production). On pin `820edf28d22d`, both checks § Parked again names held, in the service tab and Application Logs at 1440 and 390. A closed window whose mark was the oldest line of its page left the mark on the panel's edge at 1440 and off it at 390. That was this milestone's defect, fixed and shipped in `71d3dc5e5`.
- **Remaining gate:** a production pin carrying `71d3dc5e5` (clears by itself).
- **Next:** as the QA user in `bex-canary`, with a short-lived web service logging about a line a second, wait until a `slow query` line's ±10 minute window has closed. Then open its View in context link fresh, at 1440 and 390, in a service Logs tab and a project's Application Logs view. Six older pages load, and the line is marked and centred, not on the panel's edge. Also open one line in the first page of a still-open window, to see it still opens on the mark. Delete the fixture, log out, record it here, and close with t013.

## Unblocked a third time (2026-11-21)

- **Affected:** w3/m528 t009 (View in context's production check on a closed window), t013 (closeout).
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `4c41a69d7` (02:20Z), pinning `f797f525f5df`, and `git merge-base --is-ancestor 71d3dc5e5 f797f525f5df` succeeds, so production runs the fix. The gate `pin must carry 71d3dc5e5` holds. The same pin carries w3/m537 (`781b1d7c9`), so each explorer's range picker now lists Live tail as a range instead of a Live switch.
- **Next:** the re-walk § Parked a second time names, on production as the QA user in `bex-canary` at 1440 and 390; the rest of t009's walk passed on earlier pins and is not repeated. Then t013 closes the milestone. No gate remains.

## Also read from the bundle, outside this milestone's scope

t010 folds these into ADR018 and files each difference as its own item:

- **Display** is a three-way choice, not a checkbox: `Expand everything`, `Expand errors` (lines at `error`, `critical`, `alert` or `emergency` wrap, the rest collapse) and `Collapse everything` (one line each, ellipsized). It defaults to `expand-all` and persists in `localStorage`. bex has one "Expand everything" checkbox that unclamps a 10-line / 1,000-character clamp.
- **Order by persists** in `localStorage`. ADR018 lists this as unverified, and bex keeps it per visit.
- **Copy logs** writes `timestamp [label] METHOD STATUS host+path text` per line (`Dn`). ADR018 lists the format as unverified.
- **Log Explorer Theme** and **High Contrast Mode** are Appearance settings bex does not have.

## Source + Goal linkage

- **Source:** promoted from [w3/081](../../done/081.md) 2026-11-20 by `/loopx w3`. That note was w3/m509's parity check (2026-11-19) against [project-observability.md](../../../../docs/render-artifacts/project-observability.md) § The view page and the [ADR018](../../../../docs/ADR018-render-parity.md) Logs row "Log explorer display and shortcuts".
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)). The log explorer is the surface every bex customer reads most, and these are the pieces of Render's that bex's still lacks.
- **Expected outcome:** a Render user's habits carry over to bex's explorer. They can copy one line, jump from a filtered search to its surrounding lines, open a Shell on the instance that wrote a line, read a busy tail without it moving under them, and see that a paused view has new lines.
- **Why now:** w3/m505–m509 just rebuilt every explorer on one line list and one options menu (`LogLineList`, `LogOptionsMenu`), so each piece lands once for all five explorers. The triage read Render's bundle, so nothing waits on a Render workspace.
- **Render parity task included:** this changes a customer-facing dashboard surface. Render's public API and MCP have no log explorer, so the check is dashboard-only.
