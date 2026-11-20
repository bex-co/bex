# w3 · m528 — Every log explorer carries Render's row menu, its new-log buttons, Return to Bottom and a Theme Settings link

**Worker:** worker3 **Goal:** every bex log explorer (a service's Logs tab, a Postgres or Key Value Logs tab, a deploy's log, a project's Application Logs and Request Logs views) has the parts of Render's explorer that w3/m509 left out, built the way Render's dashboard bundle builds them:

- a row menu (…) on hover: Copy log line, View in context, SSH into instance;
- an icon-only Return to Bottom (Return to Top when descending) at the panel's right edge;
- a scrolled-away live tail that holds new lines behind a "New logs" / "N+ new logs" button;
- a paused view that offers "New logs in Live Tail" when new lines arrive;
- an Options menu that ends with a Theme Settings link to an Appearance section in account Settings.

**Status:** todo (t001–t008 and t010–t012 done; t009's walk on the tree done, its production walk waits for a pin carrying the milestone)

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
| [t009](t009.md) | Verify in a real browser at 1440 and 390, on the tree and on production | 45m | t003, t004, t006, t007, t008 |
| [t010](done/t010.md) | Render parity — **DONE** | 30m | t009 |
| [t011](done/t011.md) | Simplify — **DONE** | 20m | t010 |
| [t012](done/t012.md) | Test coverage — **DONE** | 45m | t010 |
| [t013](t013.md) | Closeout | 10m | t011, t012 |

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
7. **Recorded.** The [capture](../../../docs/render-artifacts/project-observability.md) § The view page and the [ADR018](../../../docs/ADR018-render-parity.md) Logs row "Log explorer display and shortcuts" describe what Render does and what bex does. Every remaining difference is either named with its follow-up item or listed under ADR018's recorded divergences. The ADR018 line "Not built, filed as w3/081" is gone.
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

The bundle was re-read on 2026-11-20 (`Logs-DOHYAXbe.js`, `UserSettings-3jrn6Tc2.js`, unchanged since the triage) and the findings are in the [capture](../../../docs/render-artifacts/project-observability.md#the-view-page). Render makes Live tail one of its time ranges; bex keeps a Live switch beside the range, so:

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

**On production:** waits for a pin carrying the milestone's commit (t009 steps 3–4).

## Also read from the bundle, outside this milestone's scope

t010 folds these into ADR018 and files each difference as its own item:

- **Display** is a three-way choice, not a checkbox: `Expand everything`, `Expand errors` (lines at `error`, `critical`, `alert` or `emergency` wrap, the rest collapse) and `Collapse everything` (one line each, ellipsized). It defaults to `expand-all` and persists in `localStorage`. bex has one "Expand everything" checkbox that unclamps a 10-line / 1,000-character clamp.
- **Order by persists** in `localStorage`. ADR018 lists this as unverified, and bex keeps it per visit.
- **Copy logs** writes `timestamp [label] METHOD STATUS host+path text` per line (`Dn`). ADR018 lists the format as unverified.
- **Log Explorer Theme** and **High Contrast Mode** are Appearance settings bex does not have.

## Source + Goal linkage

- **Source:** promoted from [w3/081](../done/081.md) 2026-11-20 by `/loopx w3`. That note was w3/m509's parity check (2026-11-19) against [project-observability.md](../../../docs/render-artifacts/project-observability.md) § The view page and the [ADR018](../../../docs/ADR018-render-parity.md) Logs row "Log explorer display and shortcuts".
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). The log explorer is the surface every bex customer reads most, and these are the pieces of Render's that bex's still lacks.
- **Expected outcome:** a Render user's habits carry over to bex's explorer. They can copy one line, jump from a filtered search to its surrounding lines, open a Shell on the instance that wrote a line, read a busy tail without it moving under them, and see that a paused view has new lines.
- **Why now:** w3/m505–m509 just rebuilt every explorer on one line list and one options menu (`LogLineList`, `LogOptionsMenu`), so each piece lands once for all five explorers. The triage read Render's bundle, so nothing waits on a Render workspace.
- **Render parity task included:** this changes a customer-facing dashboard surface. Render's public API and MCP have no log explorer, so the check is dashboard-only.
