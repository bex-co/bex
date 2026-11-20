# w3 · m509 — Every log explorer reads and drives like Render's: level gutter, copy, order, expand, fullscreen and keyboard shortcuts

**Worker:** worker3 **Goal:** bex's shared log explorer (service, datastore, deploy and project views) gains what Render's has and bex lacks: the level gutter with tinted warning and error rows, full-timestamp hover, date separators, the Options menu (Copy logs, Order by, Expand everything), Maximize and Render's keyboard shortcuts. **Status:** blocked — t001–t005 and t007–t009 done and shipped in `849cc4c9b`; t006 is verified on the tree at 1440 and 390, and its production walk waits for a production pin carrying `849cc4c9b` (§ Gate)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Inventory: what each bex explorer already has against Render's list — **DONE** | 30m | — |
| [t002](done/t002.md) | Level gutter and tinted rows; request status chips — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Full-timestamp hover and date separators — **DONE** | 45m | t001 |
| [t004](done/t004.md) | The Options menu: Copy logs, Order by, Expand everything — **DONE** | 1h | t001 |
| [t005](done/t005.md) | Maximize and the keyboard shortcuts — **DONE** | 1h | t004 |
| [t006](t006.md) | Verify at 1440 and 390, and on production | 30m | t002, t003, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

## Definition of done

- Every bex log explorer shows the level gutter with tinted warning and error rows, request status chips, full-timestamp hover and date separators.
- Options offers Copy logs (`⌘/Ctrl ⇧ C`), Order by and Expand everything; Maximize and the shortcuts `/`, `M`, `Home`/`End`, `Shift+L` work and never fire while typing.
- Verified at 1440 and 390 on production.

## Inventory (t001, 2026-11-19)

Each Render behavior from the capture ([project-observability.md](../../../../docs/render-artifacts/project-observability.md) § The view page) and [render.com/docs/logging](https://render.com/docs/logging), per bex explorer before this milestone. Service: a service's Logs tab (`log-viewer.tsx` + `log-filter-bar.tsx`). Datastore: a Postgres or Key Value Logs tab (`datastore-log-viewer.tsx`). Deploy: a deploy's log (`deploys/components/deploy-log-panel.tsx`). Project: a project's Application Logs and Request Logs views (`project-log-viewer.tsx`). All four draw their lines with `log-line-list.tsx`.

| Render behavior | Service | Datastore | Deploy | Project |
| --- | --- | --- | --- | --- |
| Level icon gutter, `info` shown on hover, level as text on hover | missing | missing | missing | missing |
| Warning and error rows tinted | partial: `warn`/`warning`/`error`/`fatal` only, not `critical`/`alert`/`emergency`, nor a request line without a `level` label | partial, as Service | partial, as Service | partial, as Service |
| Request status as a coloured chip | present | n/a (no request lines) | n/a | present (Request Logs) |
| Full local, UTC and Unix timestamp on hover | partial: the raw RFC 3339 stamp as a `title` | partial, as Service | partial, as Service | partial, as Service |
| Date separator rows | missing | missing | missing | missing |
| Coloured dot and `[service]` per line | n/a (one resource) | n/a | n/a | present (w3/m505) |
| Short instance `[rstgn]` | present | present | present | present |
| Options ▸ Copy logs (`⌘/Ctrl ⇧ C`) | missing | missing | missing | missing |
| Options ▸ Order by | missing | missing | missing | missing |
| Options ▸ Display ▸ Expand everything | missing | missing | missing (has Wrap lines) | missing |
| Maximize | missing | missing | present (Esc leaves) | missing |
| `/` focuses search, `M` toggles full screen | missing | missing | missing | missing |
| `Home`/`End` | partial: native scroll while the list has focus | partial | partial | partial |
| `↑`/`↓`, `Page Up`/`Page Down` | present (focused list) | present | present | present |
| `Shift+L` clears a live tail | missing | missing | n/a (no live toggle) | missing |
| Return to Bottom while scrolled away | present ("Jump to latest") | present | present | present |
| Row hover highlight, row menu (…) | highlight only | highlight only | highlight only | highlight only |

## Verification record (t006)

### Local, on the tree (2026-11-19)

The dashboard from this tree (`vite dev`) against the repository's offline bex-api stub (`scripts/local-bex.mjs`, which streams app lines with `info`, `warn` and `error` levels and serves a deploy and a Postgres), in Playwright at 1440×900 and 390×844. The stub has no saved searches, so the project views were not walked here; their wiring is covered by `project-log-viewer.test.tsx`, and the production walk below covers them.

- **Service Logs tab, 1440:** warning rows carried the triangle and amber tint, error rows the ✕ and red tint, an info row's mark appeared on hover only. Hovering a time opened Local, UTC and Timestamp. Options listed Copy logs `⌘⇧C`, Order by and Display ▸ Expand everything; Descending put the newest line at the top, pinned there. `M` maximized and `Esc` left; `End` (descending) reached the bottom with "Jump to latest" at the top; `/` focused the search and a typed `m` went into the box without maximizing. `Ctrl+Shift+C` copied 59 lines while 35 rows were in the DOM, and toasted "Logs copied". `Shift+L` with Live on showed "Logs cleared", and new lines appeared under it within seconds.
- **After the Simplify pass:** no row's time carried tooltip state until hovered; hovering opened it, and it closed as the pointer left.
- **Service Logs tab, 390:** the bar read [All logs | Search logs | ⋯] over [Last hour | Filters | Live]; Options held Maximize, which filled the screen.
- **Deploy page, 1440:** the build log's Options held Time range, Wrap lines and Show timestamps, then Copy logs, Order by and Expand everything; Maximize stayed beside it.
- **Postgres Logs tab, 1440 and 390:** Maximize and Options at the toolbar's end; at 390 they sit at the end of the range's row. `M` maximized.
- **Pending and ready (the route's GraphQL read held, then released), measured in CSS px:**
  - Service Logs tab, 1440: every control at the same box in both states (`[280,269,440,301]` … `[1344,269,1416,301]`, Filters and Live within 1 px of text width), the panel's top at 317 in both.
  - Service Logs tab, 390: the same boxes in both states (`[16,56,165,100]`, `[173,56,322,100]`, `[330,56,374,100]`, `[16,108,165,152]`, `[173,108,374,152]`), the panel's top at 176 in both.
  - Postgres Logs tab, 1440 and 390: every control the same width and height in both states, all 4 px lower while pending, as is the panel (232 against 228, 376 against 372). The offset comes from the page's header skeleton above the tab, which this milestone does not touch; filed as [w3/082](../../082.md). The skeleton used to draw a 128 px range, no Level select and no Live switch; it now draws each control.

### Production (open)

The definition of done asks for 1440 and 390 on production, which needs a deploy carrying `849cc4c9b` (§ Gate).

## Render parity (t007, 2026-11-19)

Compared with the capture ([project-observability.md](../../../../docs/render-artifacts/project-observability.md)) and [render.com/docs/logging](https://render.com/docs/logging). A dashboard-only change: Render's public REST API and MCP have no log explorer, so neither does bex's. The four explorers draw one line list (`LogLineList`) and one set of controls (`MaximizeButton`, `LogOptionsMenu`, `useLogExplorer`), so they cannot drift from each other. The deploy page keeps its own range, Wrap lines and Show timestamps options above the shared ones, and has no `Shift+L` because it has no live tail toggle.

| Behavior | Render | bex (every explorer) | Verdict |
| --- | --- | --- | --- |
| Level gutter | an icon per level; `info` hidden until hover; the level as text on hover | the same; `warning` a triangle, the error levels a ✕ (the w5/m5 capture's "red ✕"), `debug` a bug, `notice` an info mark in blue; the line's own word as the hover title and accessible name | matched; the `debug` and `notice` icons unverified |
| Tinted rows | a warning icon and tinted row | `warning` amber, `error`/`critical`/`alert`/`emergency` red, with a rule at the edge | matched |
| Request status chip | `200` green, `404` orange | by class: 2xx green, 3xx blue, 4xx amber, 5xx red; a request line without a `level` label takes its class's level, as Render labels it (`request-logs.md`) | matched |
| Timestamp hover | "full timestamp in local, UTC, and Unix formats" | the shared `InstantTooltip` (Local, UTC, Timestamp), opened by hover or the keyboard's row cursor | matched; the Unix precision unverified (bex: whole seconds) |
| Date separators | `Nov 19` row | a row over each day of a list spanning more than one day, in both orders; the year when it is not this year's | matched; whether Render also heads a one-day list unverified |
| Copy logs | `⌘ ⇧ C`, "Copy all currently displayed logs" | every row shown (not only the rows in the DOM), as it reads, ANSI stripped, the whole message even when clamped; `⌘` or `Ctrl` | matched; the text format unverified |
| Order by | Ascending / Descending | the same; descending pins the newest line at the top and loads older pages at the bottom | matched; persistence across visits unverified (bex: per visit) |
| Expand everything | under Display | a long message shows its first 10 lines or 1,000 characters with Show more, until Expand everything | matched; Render's clamp unverified |
| Maximize | button; `M`, and `M` or `Esc` to leave | the same, in every explorer; on a phone the service Logs tab offers it inside Options | matched |
| `/` | focuses search | the same, without typing the slash | matched |
| `Home` / `End` | jump to top / bottom | the same, from anywhere on the page but a field or a widget that moves its own focus | matched |
| `↑`/`↓`, `Page Up`/`Page Down` | scroll | the focused list scrolls (↑/↓ move its row cursor) | matched |
| `Shift+L` | clears logs, live tail only | clears while Live is on (service, datastore, project); Show earlier lines, pausing or new filters bring them back | matched; Render's way back unverified |
| Not while typing | — (docs silent) | no shortcut fires in a field; `Esc` still leaves full screen | unverified (Render does not document it); the milestone's own requirement |
| Row menu (…), New logs toast, "Return to Bottom" wording, Theme Settings link | shown in the capture | not built | drift, filed as [w3/081](../../081.md) |

ADR018's Logs table gained the row "Log explorer display and shortcuts" with the same unverified points and the w3/081 drift.

## Simplify (t008, 2026-11-19)

`/simplify` ran three reviews (reuse, quality, efficiency) over the diff. Applied, each behavior-preserving and covered by the suites below:

- **Shared pieces:** one `LogExplorerActions` (Maximize and Options, with the phone's Maximize-in-Options set by one prop), one `LogClearedState`, and the layout constants `LOG_ACTIONS_CLASS`, `LOG_MAXIMIZED_BODY_CLASS` and `LOG_MAXIMIZED_LIST_CLASS`, which the skeletons use too. Each viewer computes `tailing` once, for its tail and its clear, and `useClearedLines` offers `clear` only while live.
- **Hooks:** the shortcut listener is a `useEffectEvent`, and returns at once for any key that is not a shortcut. The topbar search and Copy logs share a new `useIsMac`.
- **The line list:** a row mounts its timestamp tooltip only when hovered or under the row cursor, since a Radix tooltip in every row cost each appended or scrolled-in row its mount. `logListRows` compares only the first and last days before walking the lines. `clampMessage` returns early on the raw message's length and line count. A clear slices the lines after the cut from the end. Each row clamps once, and `jump` and the newest-edge scroll share `scrollToEdge`.

Skipped: moving `data-index` off the memoized row, so that a descending append re-renders fewer rows. Ascending order already re-rendered every row once the buffer started dropping lines, and the existing reflow tests measure the row element itself. Also skipped: a shared day-label helper with the service events page, which picks between different formats.

## Test coverage (t009, 2026-11-19)

| Definition of done | Test |
| --- | --- |
| Level gutter, tinted warning and error rows, a 404 request line's warning mark | `lib/__tests__/log-levels.test.ts`; `log-line-list-explorer.test.tsx` § level gutter |
| Request status chips by class | `log-line-list.test.tsx` (existing), `theme-contrast.test.ts` (the chip and level-mark colours) |
| Full-timestamp hover, by pointer and by the keyboard's row cursor | `log-line-list-explorer.test.tsx` § timestamp hover |
| Date separators in both orders; the row cursor passes over them | `lib/__tests__/log-rows.test.ts`; `log-line-list-explorer.test.tsx` § date separators |
| Copy logs (every row shown, not only the DOM's), `⌘⇧C` and `Ctrl+Shift+C`, not plain `⌘C` | `log-rows.test.ts` § logRowsText; `log-line-list-explorer.test.tsx` § handle; `log-explorer.test.tsx` |
| Order by: newest first, pinned at the top, older pages loaded at the bottom | `log-line-list-explorer.test.tsx` § order |
| Expand everything and a row's Show more / Show less | `log-rows.test.ts` § clampMessage; `log-line-list-explorer.test.tsx` § long messages |
| Maximize, `M`, `Esc`, `/`, `Home`/`End` | `log-explorer.test.tsx`; each viewer's test (`log-viewer`, `project-log-viewer`, `datastore-log-viewer`, `deploy-log-panel`) |
| `Shift+L` only while live; restore, pause and new filters bring the lines back | `log-explorer.test.tsx` § useClearedLines; `log-viewer.test.tsx`, `datastore-log-viewer.test.tsx` |
| No shortcut while typing or in an open dialog; `Esc` still leaves full screen | `log-explorer.test.tsx` |
| Pending frames hold the new buttons at the ready sizes | `log-viewer.test.tsx` (service), `project-log-viewer.test.tsx` (project) |

Mutation spot-checks, each run against these suites and reverted: dropping the typing guard, the clear's reset, the paused-tail guard on `clear`, the status-class level, descending order, Expand everything, the line-count clamp and the service skeleton's Maximize each fail at least one test.

## Gate (2026-11-19)

**Clears by itself; no decision needed.** t006's production half needs a production deploy carrying `849cc4c9b` (`feat(dashboard): every log explorer … (w3/m509 t001-t005, t007-t009)`). Check: the newest `chore(deploy): pin platform images to <sha>` on `main` passes `git merge-base --is-ancestor 849cc4c9b <sha>`, and production's `dashboard` Deployment runs the digest that pin names.

**Next (t006, then t010):** sign in as the QA user (`bash scripts/qa-login.sh`). In `bex-canary`, use or create a project with a web service that logs at more than one level and serves a 404, a Postgres, and a finished deploy. Walk, at 1440 and 390: the service's Logs tab, the Postgres Logs tab, the deploy's page, and the project's Application Logs and Request Logs views. In each, check the gutter and the tints (a 404 request line's warning mark), the timestamp hover, Options (Copy logs and its shortcut, Order by, Expand everything), Maximize with `M` and `Esc`, `/`, `Home`/`End`, and `Shift+L` with Live on. Typing `m` in a search box must not maximize. Pick a range across midnight for the date rows. Compare pending and ready for the service and project views. Record the evidence here, delete any fixture with by-id `GET` 404 evidence, log out, and close the milestone.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18/19 (topic: one aspect of Render parity, project and environment observability and the log explorer), filed by `/pmx them all to w3` 2026-11-19 after a read-only capture of Render's live project Observability views in the user's Render workspace (`docs/render-artifacts/project-observability.md`).
- **Goal linkage:** ADR008 § Render parity first: [render.com/docs/logging](https://render.com/docs/logging) documents the line metadata and shortcuts, and the capture shows the Options menu, Maximize and the gutter.
- **Expected outcome:** reading logs on bex works the way Render users expect: levels visible at a glance, copy and fullscreen at hand.
- **Why now:** w3/m505 adds the project views on the same shared components; adding these to the shared line list and toolbar now gives every explorer them once instead of retrofitting four.
- **Render parity included:** dashboard-only change; the parity task compares each explorer with Render's.
