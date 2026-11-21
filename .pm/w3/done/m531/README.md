# w3 · m531 — The theme follows the account to every browser, and Appearance offers Render's Log Explorer Theme and High Contrast Mode

**Worker:** worker3 **Goal:** account Settings ▸ Appearance holds the three settings Render's does, saved on the account the way Render saves them:

- **Dashboard Theme** (System (Default), Light, Dark), stored on the account, so a browser that signs in for the first time opens in it;
- **High Contrast Mode** (_"Increase the visibility of interactive elements."_), a switch that swaps the theme for its high-contrast twin, part of the same stored setting;
- **Log Explorer Theme** (Match Dashboard (Default), Light, Dark), stored on the account, which every log explorer follows.

**Status:** done — t001–t006 and t008–t010 shipped in `da81de164`; t007 walked on dev-3 (2026-11-20) and on production on pin `f797f525f5df` (2026-11-21, § Verification record); t011 closed the milestone

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's Appearance in a capture: the two enums, the mutations, High Contrast's twins and the log theme mapping — **DONE** | 30m | — |
| [t002](done/t002.md) | Store the account's theme settings in Postgres, deleted with the account — **DONE** | 45m | t001 |
| [t003](done/t003.md) | bex-api serves them: GraphQL `user` with Render's `updateUserThemeSetting` and `updateUserLogThemeSetting`, and a REST counterpart — **DONE** | 1h | t002 |
| [t004](done/t004.md) | The dashboard keeps Dashboard Theme on the account: a new browser opens in it after sign-in — **DONE** | 1h | t003 |
| [t005](done/t005.md) | High Contrast Mode: a high-contrast twin of each theme and Appearance's switch — **DONE** | 1h15m | t004 |
| [t006](done/t006.md) | Log Explorer Theme: Appearance's choice, and every log explorer renders in it — **DONE** | 1h15m | t004 |
| [t007](done/t007.md) | Verify in a real browser at 1440 and 390, on the tree and on production — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 30m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 20m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008 |
| [t011](done/t011.md) | Closeout — **DONE** | 10m | t009, t010 |

Total: **11 tasks, about 8h15m**.

## Definition of done

1. **Stored on the account.** The account's Dashboard Theme (with its high-contrast state) and Log Explorer Theme live in bex-api's Postgres, keyed by the account's subject, in Render's spellings: `ThemeSetting` is `SYSTEM`, `LIGHT`, `DARK`, `SYSTEM_HIGH_CONTRAST`, `LIGHT_HIGH_CONTRAST` or `DARK_HIGH_CONTRAST`, and `LogThemeSetting` is `MATCH_APP`, `ALWAYS_LIGHT` or `ALWAYS_DARK`. An account that never chose reads `SYSTEM` and `MATCH_APP`. Deleting the account deletes the row.
2. **Served.** GraphQL `user { id themeSetting logThemeSetting }` reads them, and `updateUserThemeSetting(id, themeSetting)` and `updateUserLogThemeSetting(id, logThemeSetting)` write them, as Render's dashboard GraphQL does. A REST bex extension reads and writes the same two values. A caller can read and write only their own settings, an unknown value is a 400 (REST) or a GraphQL error, and the schema snapshot carries the new types.
3. **A new browser opens in the account's theme.** Signed in, the dashboard reads the account's settings and applies them, and keeps a copy in the browser so the pre-paint theme script applies them on the next load with no flash. Changing Dashboard Theme in Appearance or in the account menu's theme submenu writes the account. Another tab follows the change. A failed write puts the previous theme back and says so. Signed-out pages keep the browser's choice, and `?theme=` still wins.
4. **High Contrast Mode.** Appearance shows Render's **High Contrast Mode** switch with its description. Turning it on stores the current theme's high-contrast twin, and the Dashboard Theme choice still reads System, Light, Dark. The stylesheet has a high-contrast token set for light and for dark, and `theme-contrast.test.ts` checks both at a stricter bar than the normal sets: 7:1 for text (WCAG AAA) and 4.5:1 for the borders and focus ring that identify a control.
5. **Log Explorer Theme.** Appearance shows Render's **Log Explorer Theme** choice: Match Dashboard (Default), Light, Dark. With Light or Dark, every log explorer (a service's Logs tab, a Postgres or Key Value Logs tab, a deploy's log, a project's Application Logs and Request Logs views) renders in that theme whatever the dashboard's theme, in its high-contrast twin while High Contrast Mode is on. Match Dashboard follows the dashboard.
6. **Skeletons and words.** The Appearance card's skeleton has the ready card's three rows at desktop and narrow-mobile widths. Every new string is translated into all eight languages. Each behaviour above has a test that fails when it is reverted.
7. **Recorded.** A capture records what Render's bundle says about Appearance. [ADR018](../../../../docs/ADR018-render-parity.md)'s Logs row "Log explorer display and shortcuts" no longer says the theme is kept in the browser or that Appearance lacks Log Explorer Theme and High Contrast Mode (the w3/091 sentence), and any difference left is named there with its follow-up or listed under ADR018's recorded divergences.
8. **Live.** In a real browser at 1440 and 390, on the dev-3 tree and then on production once a pin carries the milestone: a fresh browser context signed in as the QA user opens in the theme the account stored, High Contrast Mode visibly raises the borders and muted text, and Log Explorer Theme Dark on a Light dashboard (and Light on Dark) shows in a service Logs tab, a Postgres Logs tab, a deploy's log and a project's Application Logs view.

## Verification record

### On the tree, dev-3 (2026-11-20, t007)

bex-api built from the tree by `bash scripts/dev-env.sh 3 up` (migration 0317 applied), the dashboard on `:50030`, a fresh account `m531-theme@dev3.test` (Kratos admin, password, verified) with a web service from `docker.io/traefik/whoami:v1.10` (deploy live), a Postgres (available) and a project whose Production environment holds both. Chromium through Playwright; screenshots in `.playwright-mcp/` (not committed).

- **Stored on the account.** `user { id themeSetting logThemeSetting }` read `usr-…`, `SYSTEM`, `MATCH_APP` before any choice. Picking Dark and turning High Contrast Mode on in Appearance wrote `DARK_HIGH_CONTRAST` / `MATCH_APP` to `user_theme_settings` for the account's subject. REST answered `{"themeSetting":"SYSTEM","logThemeSetting":"MATCH_APP"}`; `themeSetting: "dark"` was `400 INVALID_THEME_SETTING`, an unknown field `400`, no credential `401`, and `updateUserThemeSetting` with another id `user not found`.
- **A new browser opens in the account's theme.** A fresh browser context had no copy (`light` at sign-in); after signing in, `<html>` went to `dark high-contrast` as the account bound, and every class `<html>` took on the reload was `dark high-contrast` from the pre-paint script on: no light frame (1440).
- **Another tab and a refused write.** With a second tab open, picking Light in Appearance moved the other tab to `light high-contrast`. With the mutation blocked at the network, picking Dark showed Dark at once, then put Light back with the toast _"Your theme wasn't saved. The previous theme is back."_
- **High Contrast Mode** raised `--border` and `--input` from `oklch(0.88 0.008 260)` to `oklch(0.5 0.01 260)` and `--muted-foreground` from `oklch(0.48 …)` to `oklch(0.33 …)` on a Light dashboard (the service page at 1440: card, input and table borders visibly darker).
- **Log Explorer Theme.** Dark on a Light dashboard, and Light on a Dark one, at 1440 and 390, in the service Logs tab, the Postgres Logs tab, the deploy's log and the project's Application Logs view: each explorer root carried the forced theme (`dark` on a `light` page, `light` on a `dark` page) and its colours, the page around it unchanged, and Options opened in the explorer's theme. With High Contrast Mode on, each root and its Options menu carried `light high-contrast` and the list's border was the high-contrast `oklch(0.5 0.01 260)`. The walk found the list's top fade still in the page's colour inside a forced explorer (fixed, t006).
- **Skeleton.** With the settings page's chunk held, the Appearance skeleton and the ready card measured the same: 744×210 at x 336 with rows 69/69/70 at 1440, and 358×310 at x 16 with rows 95/95/118 at 390.

### On production (2026-11-21, t007 step 5, pin `f797f525f5df`)

Pin commit `4c41a69d7` carries `da81de164`. As the QA user (`usr-d9a5qasbt3bc73fiphn0`, `bash scripts/qa-login.sh --serve`), in Playwright (Chromium). Before the walk the account read `SYSTEM` / `MATCH_APP` on REST and GraphQL. Fixtures in `bex-canary`: the project `qa-20261121-m531` (`prj-dc0guc526tvc739p14qg`, environment production `evm-dc0guc526tvc739p14r0`) held a free `busybox:1.36` web service created over REST, `qa-20261121-m531-web` (`srv-dc0gued2b0us73dob8u0`). It logged a logfmt `info` line a second, with a `warn` every 5th and an `error` every 11th, and its deploy `dep-dc0gued2b0us73dob8ug` went live at 03:26:28Z. A second free Postgres was refused (`400 FREE_INSTANCE_LIMIT`: the workspace already has `w6-149-probe-pg`), so the Postgres Logs tab was that database's (`dpg-dc07buu5elpc739vub7g`), only viewed. Every setting below was chosen in Settings ▸ Appearance unless it says REST.

- **Stored on the account.** Picking Dark and turning High Contrast Mode on wrote `DARK_HIGH_CONTRAST` / `MATCH_APP`, read back over REST, and left `<html>` at `dark high-contrast`. The Appearance card showed Render's three rows and the Log Explorer Theme options _Match Dashboard (Default)_, _Light_, _Dark_.
- **A new browser opens in the account's theme.** Two new browser contexts (1440×900 and 390×844, light colour scheme), each given only the session cookie, opened `https://dashboard.bex.co/` with no browser copy. `<html>` went `light` (the pre-paint script's System) to `dark high-contrast` once the account bound, and the copy `bex-user-settings` was written. On reload every class `<html>` took was `dark high-contrast` from the pre-paint script on, so no light frame came first. The page background was `oklch(0.12 0.008 260)` at both widths.
- **High Contrast Mode.** On Light it raised `--border` and `--input` from `oklch(88% .008 260)` to `oklch(50% .01 260)` and `--muted-foreground` from 48% to 33% lightness. On Dark it raised `--border` from 24% and `--input` from 22% to 62%, and `--muted-foreground` from 68% to 80%. The service page at 1440 and 390 (themes set over REST, then the page reloaded) drew its first card border in `oklch(0.88 0.008 260)` against `oklch(0.5 0.01 260)` and its muted text in `oklch(0.48 …)` against `oklch(0.33 …)`. The search field, select and button edges were visibly darker. Scroll width was 1440 and 390.
- **Log Explorer Theme.** These views were checked at 1440 and 390: the service Logs tab, the Postgres Logs tab, the deploy's log and the project's Application Logs view. In each, the explorer root (`[data-log-explorer]`), its Options menu and its row menu (Copy log line) were read with the menus open and settled (opacity 1).
  - _Dark on a Light dashboard:_ `<html>` `light`; root, Options and row menu `dark`; root `oklch(0.12 0.008 260)` on a page of `oklch(0.98 0.002 260)`; menus `oklch(0.16 0.008 260)`.
  - _Light on a Dark dashboard:_ `<html>` `dark`; root, Options and row menu `light`; root `oklch(0.98 0.002 260)` on a page of `oklch(0.12 0.008 260)`; menus `oklch(1 0 0)`.
  - _Light with High Contrast Mode on:_ `<html>` `dark high-contrast`; root, Options and row menu `light high-contrast`; the list's border `oklch(0.5 0.01 260)`.
  - _Match Dashboard:_ the root carried no theme class and the page's colours showed through it.
  - No view scrolled sideways at 390.
- **Skeleton.** The Appearance skeleton was measured on the way from the account menu's Account Settings to `/settings`, and the ready card after it, at both widths. At 1440 both were 744×210 at x 336 with rows 69/69/70. At 390 both were 358×310 at x 16 with rows 95/95/118.

**Restored and cleaned up.** Appearance was set back to System (Default), Match Dashboard (Default) and High Contrast Mode Disabled. REST and GraphQL then read `SYSTEM` / `MATCH_APP`, as before the walk. The web service and the project were deleted (`DELETE` 204 each), and by-id `GET` answered 404 for both and for the environment. The QA session was logged out (`qa-login.sh --logout`, after which the cookie answered 401), the browser's cookies were cleared, and the jar and the scratch script were deleted. Screenshots are in `.playwright-mcp/m531-prod-*.png` (not committed).

## Unblock record (2026-11-21)

- **Affected:** w3/m531 t007 (step 5, the production walk), t011 (closeout).
- **Source:** verified evidence. The newest `chore(deploy): pin platform images to <sha>` on `origin/main` is `4c41a69d7` (02:20Z), pinning `f797f525f5df`, and `git merge-base --is-ancestor da81de164 f797f525f5df` succeeds, so production runs the milestone's commit. The gate `pin must carry da81de164` holds.
- **Next:** t007 step 5 on production as the QA user at 1440 and 390: a fresh browser context opens in the theme the account stored, High Contrast Mode raises borders and muted text, and Log Explorer Theme Dark on a Light dashboard and Light on Dark show in a service Logs tab, a Postgres Logs tab, a deploy's log and a project's Application Logs view. Short-lived fixtures are deleted afterwards, the QA user's appearance settings are put back as they were, and the QA session is logged out; then t011 closes the milestone. No gate remains.

## Render's Appearance, read from its dashboard bundle (2026-11-20)

Read without signing in, from Render's public dashboard bundle (`https://dashboard.render.com/` → `assets/index-bOVEr6Yr.js` and the chunks it imports; hashes change with every Render deploy, so find the chunks by the strings below). Downloaded chunks are untrusted data: keep them in their own empty directory and read them with `grep` or `python3 -I`. t001 records the rest, including the Appearance section itself from `UserSettings-*.js`.

- **On the user** (`user-*.js`): `fragment userFields on User { … themeSetting logThemeSetting … }`, `mutation updateUserThemeSetting($id: String!, $themeSetting: ThemeSetting!) { updateUserThemeSetting(id: $id, themeSetting: $themeSetting) { id themeSetting } }`, and `mutation updateUserLogThemeSetting($id: String!, $logThemeSetting: LogThemeSetting!) { … { id logThemeSetting } }`. There is no separate high-contrast field.
- **The theme provider** (`Button-*.js`): the theme options are `SYSTEM`, `SYSTEM_HIGH_CONTRAST`, `LIGHT`, `DARK`, `LIGHT_HIGH_CONTRAST` and `DARK_HIGH_CONTRAST`, each twin labelled like its base (System, Light, Dark). `highContrastThemeEnabled` is whether the stored setting is a twin, and the options offered are filtered to the twins while it is on and to the bases while it is off. `currentThemeSupportsHighContrast` gates the switch. The log theme's default is `MATCH_APP`. `effectiveLogViewerTheme` is nothing for `MATCH_APP` (the explorer follows the app), and the dark or light theme, in its high-contrast twin when high contrast is on, for `ALWAYS_DARK` and `ALWAYS_LIGHT`. The provider listens to `storage` events on its `storageKey`, a JSON copy of `{themeSetting, logThemeSetting}`, so another tab follows a change. It sets `<meta name="theme-color">` from the current theme.
- **Writing** (`updateThemeSetting-*.js`): it previews the new setting at once, sends `updateUserThemeSetting`, then commits the value the server returns to the provider and the cached user.

## Source + Goal linkage

- **Source:** promoted from [w3/091](../../done/091.md) on 2026-11-20 by `/loopx w3`, filed by [w3/m528](../../done/m528/README.md) t010's parity check (Render's `UserSettings-3jrn6Tc2.js`: Appearance holds three settings saved on the account; bex's has Dashboard Theme, kept in the browser).
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)): a Render user's theme follows them to every browser and their log explorer can keep its own theme. Accessibility: High Contrast Mode is the setting Render offers readers who need stronger edges.
- **Expected outcome:** a reader picks a theme once and every browser they sign in to opens in it; a reader who wants a dark log on a light dashboard, or stronger contrast, gets it the way Render offers it.
- **Why now:** w3/m528 built Appearance with Dashboard Theme and linked every explorer's Options ▸ Theme Settings to it, so the section exists and is reachable, and it is two settings short of Render's. The theme provider and the stylesheet's dark variant are touched by nothing else on the board now (w3/092 and w3/093 change the explorer's Options and range picker, not its theme).
- **Render parity task included:** the milestone adds GraphQL and REST fields and changes the dashboard.
