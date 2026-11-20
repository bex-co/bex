# w3 · m528 — Every log explorer carries Render's row menu, its new-log buttons, Return to Bottom and a Theme Settings link

**Worker:** worker3 **Goal:** every bex log explorer (a service's Logs tab, a Postgres or Key Value Logs tab, a deploy's log, a project's Application Logs and Request Logs views) has the parts of Render's explorer that w3/m509 left out, built the way Render's dashboard bundle builds them:

- a row menu (…) on hover: Copy log line, View in context, SSH into instance;
- an icon-only Return to Bottom (Return to Top when descending) at the panel's right edge;
- a scrolled-away live tail that holds new lines behind a "New logs" / "N+ new logs" button;
- a paused view that offers "New logs in Live Tail" when new lines arrive;
- an Options menu that ends with a Theme Settings link to an Appearance section in account Settings.

**Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's row menu, floating buttons and Theme Settings link in the capture, and decide how they map onto bex's Live switch | 30m | — |
| [t002](t002.md) | Return to Bottom / Return to Top: Render's icon-only button at the panel's right edge replaces "Jump to latest" | 30m | t001 |
| [t003](t003.md) | A scrolled-away live tail holds new lines behind Render's "New logs" / "N+ new logs" button | 1h | t002 |
| [t004](t004.md) | A paused explorer shows "New logs in Live Tail" when new lines arrive, and its button turns Live on | 1h15m | t001 |
| [t005](t005.md) | Row menu (…) on hover with Copy log line, in every explorer | 45m | t001 |
| [t006](t006.md) | Row menu: View in context, while a search or filter narrows the explorer | 1h | t005 |
| [t007](t007.md) | Row menu: SSH into instance, opening the service's Shell on that instance | 45m | t005 |
| [t008](t008.md) | Account Settings gains Render's Appearance section, and Options ends with a Theme Settings link to it | 45m | t001 |
| [t009](t009.md) | Verify in a real browser at 1440 and 390, on the tree and on production | 45m | t003, t004, t006, t007, t008 |
| [t010](t010.md) | Render parity | 30m | t009 |
| [t011](t011.md) | Simplify | 20m | t010 |
| [t012](t012.md) | Test coverage | 45m | t010 |
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
