# Account Settings ▸ Appearance — Render's dashboard bundle, 2026-11-20

## Sources

Read without signing in, on 2026-11-20, from Render's public dashboard bundle (`https://dashboard.render.com/` → `assets/index-bOVEr6Yr.js`) and the chunks it imports, for w3/m531. The hashes change with every Render deploy, so find the chunks by the strings quoted below:

- `UserSettings-BJ_9xILH.js` (loaded through `AsyncComponents-BcVUJzxn.js`): the account settings page and its **Appearance** block. w3/m528 read the same component as `UserSettings-3jrn6Tc2.js`.
- `RowInput-1UX5LqZi.js`: the editable row the two theme choices use.
- `Button-C53p1QQg.js`: the theme provider (`currentThemeSettingOptions`, `effectiveLogViewerTheme`), the `Theme` wrapper, and the tooltip theme.
- `theme-utils-BCizDnCG.js`: the two enums and the mapping between a setting, a theme and its high-contrast twin.
- `user-DPFug-8R.js`: the GraphQL documents. `updateThemeSetting-BxAhuW7U.js`: the account menu's writer.
- `localStorage-CPWZs5nN.js`: the browser copy.
- `Logs-kRP_JevQ.js`, `Dropdown-D1gIrxMU.js`, `OverflowMenuItem-BCKAanA3.js`: how the log explorer, its dropdowns and its menus take a theme.

## Render's behaviour

### The account's two settings

- **GraphQL.** `fragment userFields on User { id … themeSetting logThemeSetting … }` is read by `query user { user { ...userFields } }`. The writes are `mutation updateUserThemeSetting($id: String!, $themeSetting: ThemeSetting!) { updateUserThemeSetting(id: $id, themeSetting: $themeSetting) { id themeSetting } }` and `mutation updateUserLogThemeSetting($id: String!, $logThemeSetting: LogThemeSetting!) { updateUserLogThemeSetting(id: $id, logThemeSetting: $logThemeSetting) { id logThemeSetting } }`. High contrast has no field of its own.
- **`ThemeSetting`**: `SYSTEM`, `LIGHT`, `DARK`, `LIGHT_HIGH_CONTRAST`, `DARK_HIGH_CONTRAST`, `SYSTEM_HIGH_CONTRAST`.
- **`LogThemeSetting`**: `MATCH_APP`, `ALWAYS_DARK`, `ALWAYS_LIGHT`.
- **Defaults.** The signed-in shell reads `user.themeSetting ?? SYSTEM` and `user.logThemeSetting ?? MATCH_APP`, and Appearance's Log Explorer Theme starts at `user.logThemeSetting ?? MATCH_APP`.
- **Render's public REST API and MCP server have no theme operation.** The settings live only on the dashboard's GraphQL.

### Appearance, in order

The account settings list Profile, **Appearance**, Account Security, Stripe, CLI Tokens, API Keys, Connections, SSH Public Keys, PR Requests and Delete Account. Appearance is a block titled **Appearance** with three rows:

1. **Dashboard Theme**: an editable row (`RowInput`: the label, the current value and a ghost **Edit** button; editing shows a select with **Cancel** and **Save changes**, which reads **Saving** while it sends). The options are **System (Default)**, **Light** and **Dark**: the provider's six options filtered to the three high-contrast twins while High Contrast Mode is on and to the three bases while it is off, each twin labelled like its base (`System`, `Light`, `Dark`), the system option suffixed **(Default)**. Changing the select previews the theme on the page at once; Cancel, or leaving the page without saving, puts the stored theme back. Save sends `updateUserThemeSetting` and then commits the value the server returns to the provider and the cached user.
2. **Log Explorer Theme**: the same editable row with **Match Dashboard (Default)** (`MATCH_APP`), **Light** (`ALWAYS_LIGHT`) and **Dark** (`ALWAYS_DARK`). Save sends `updateUserLogThemeSetting` and stores the returned value on the cached user.
3. **High Contrast Mode**: a row headed **High Contrast Mode** with the description _"Increase the visibility of interactive elements."_ and a toggle labelled **Enabled** or **Disabled** after its state. Turning it on writes the current setting's twin, off writes its base, both through the account menu's writer (`updateUserThemeSetting`). The toggle is disabled while the Dashboard Theme row is being edited, and while it is off on a theme with no twin (`currentThemeSupportsHighContrast` is false; every theme Render offers has one).

### High Contrast's twins

- A setting is a twin when it ends in `_HIGH_CONTRAST`; `highContrastThemeEnabled` is exactly that.
- A setting resolves to a theme: `SYSTEM` to the system preference (`light` or `dark`), `SYSTEM_HIGH_CONTRAST` to the preference plus `_high_contrast`, any other setting to its lowercase (`DARK_HIGH_CONTRAST` → `dark_high_contrast`). The `<html>` element's class is `theme <theme>`.
- **On** (`currentHighContrastThemeSetting`): `SYSTEM` → `SYSTEM_HIGH_CONTRAST`; `LIGHT` → `LIGHT_HIGH_CONTRAST`; `DARK` → `DARK_HIGH_CONTRAST`.
- **Off** (`currentNonHighContrastThemeSetting`): `SYSTEM_HIGH_CONTRAST` → `SYSTEM`; `LIGHT_HIGH_CONTRAST` → `LIGHT`; `DARK_HIGH_CONTRAST` → `DARK`.
- Picking System, Light or Dark while the mode is on picks among the twins, so the mode survives a theme change. The account menu offers the same filtered three.

### Log Explorer Theme

- `effectiveLogViewerTheme` is nothing for `MATCH_APP`, so the explorer follows the dashboard. For `ALWAYS_DARK` it is `dark`, and for `ALWAYS_LIGHT` it is `light`, each as its `_high_contrast` twin while High Contrast Mode is on.
- The explorer is wrapped in Render's `Theme` component: a `<div class="theme <theme>">` that also provides the theme to everything inside it through a React context (`Theme`'s value falls back to the enclosing one when it gets none).
- **Menus, dropdowns and tooltips opened inside a themed explorer follow the explorer's theme, not the dashboard's.** Each floating panel reads the nearest theme from that context and puts `theme <theme>` on its own portalled element (`Dropdown`: `className: ["theme", nearestTheme]`; the overflow menu the same; the default tooltip takes the nearest theme's light or dark classification).

### The browser copy and other tabs

- `localStorage["render-user-settings"]` holds a JSON copy, `{themeSetting, logThemeSetting}`, and `localStorage["render-last-known-theme-preference"]` the bare theme setting. Signed-out pages (sign-in, sign-up) read that copy, falling back to `SYSTEM` and `MATCH_APP`.
- The provider listens to `storage` events on `render-user-settings`. When another tab writes it, the provider applies the new `themeSetting` and `logThemeSetting`, so every open tab follows a change.
- The provider sets `<meta name="theme-color">` from the current theme's page colour.

## What bex had before w3/m531

- Appearance (w3/m528, `dashboard/src/features/auth/pages/settings-page/appearance-card.tsx`) held only Dashboard Theme (System (Default), Light, Dark), an instant select with no Edit, Cancel or Save step.
- The theme was kept in the browser (`localStorage["vite-ui-theme"]` and a cookie of the same name), read by the pre-paint script `theme-script.js.txt`; `?theme=light|dark` won over it. The account menu's theme submenu set the same value. Nothing was stored on the account, and bex-api had no `user` query.
- There was no high-contrast token set, no Log Explorer Theme, and no way to force one explorer into a theme other than the page's: the stylesheet's dark variant was `&:is(.dark *)`, so a light subtree inside a dark page still matched it.

## What bex has since w3/m531

All three settings, stored on the account in Render's enums and served by Render's GraphQL documents, with a REST extension; the account's settings follow it to every browser, High Contrast Mode is a token twin of each theme, and Log Explorer Theme reaches every explorer and what it opens. The behaviour and what is still built differently are recorded in [ADR018](../ADR018-render-parity.md)'s Identity section, row _Account Appearance_.
