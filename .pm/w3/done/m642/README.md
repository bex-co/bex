# w3 · m642 — Keyboard focus is drawn at the contrast the theme test promises, on every control

**Worker:** worker3 **Goal:** every focus indicator the dashboard draws reaches 3:1 against the surface behind it in both themes (4.5:1 in the high-contrast twins), and `theme-contrast.test.ts` checks the ring at the strength the components apply, not the token alone. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The shared controls and the global outline draw the ring at a strength that passes — **DONE** | 45m | — |
| [t002](done/t002.md) | Sweep the pages in both themes and both high-contrast twins for the indicators the shared controls do not own — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Render parity — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 15m | t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 30m | t003 |
| [t006](done/t006.md) | Closeout — **DONE** | 10m | t004, t005 |

Total: **6 tasks, about 2h45m**.

## What is drawn today

Verified 2026-11-23 on main `33d658b497`.

- **The token passes, the ring does not.** `dashboard/src/test/__tests__/theme-contrast.test.ts` asserts `tokenPair("ring", "background", 3)` and `tokenPair("ring", "card", 3)`. At full strength `--ring` measures 4.90:1 on `--background` and 5.19:1 on `--card` in the light theme, 9.20:1 and 8.80:1 in the dark. The components apply it at half strength, `focus-visible:ring-ring/50`, and the test never reads a component.
- **At the strength applied** (the ring blended over the surface in sRGB, from the stylesheet's tokens):

  | theme | surface | 100% | 50% | 40% |
  | --- | --- | --- | --- | --- |
  | light | `--background` | 4.90 | 2.06 | 1.76 |
  | light | `--card` | 5.19 | 2.10 | 1.79 |
  | dark | `--background` | 9.20 | 2.93 | 2.25 |
  | dark | `--card` | 8.80 | 2.96 | 2.30 |

- **In a browser** (Chromium, the tree's dashboard against `yarn local-bex`, a service's Settings page, 40 Tab presses a theme, the painted ring colour against the nearest surface behind the control): the section rail's links carry a 2 px ring at 50% that measures 2.05:1 in light and 2.95:1 in dark; plain links carry the global 1 px outline at 50%, 2.0–2.05:1 and 2.95:1. The sidebar's buttons and the skip link carry a full-strength ring: 4.6–4.9:1 and 9.0–9.2:1.
- **Which controls depend on the ring alone.** A control with a border also swaps that border to the ring colour at full strength (`focus-visible:border-ring`, on eight of the nine shared controls): inputs, textareas, selects, checkboxes, outline buttons, badges and the pill tabs keep a 1 px line at 4.9:1 inside the weak halo. A control with no border has only the halo: default, ghost and link buttons, links, the section rail and log lines.
- **Where the strength is set.**
  - `ring-ring/50`, 12 uses in 11 files: `dashboard/src/common/components/ui/` `button.tsx`, `input.tsx`, `textarea.tsx`, `auto-textarea.tsx`, `select.tsx`, `checkbox.tsx`, `tabs.tsx`, `badge.tsx`, `scroll-area.tsx`; `common/components/section-navigation.tsx`; `features/logs/components/log-line-list.tsx`.
  - `dashboard/src/style.css`: `* { @apply border-border outline-ring/50; }` in the base layer, and the Ory form rule `box-shadow: 0 0 0 3px color-mix(in oklab, var(--ring) 50%, transparent)`.
  - `ring-ring/40`, 3 uses: the composers in `features/agent-sessions/components/steering-composer.tsx` and `new-session-composer.tsx` (focus-within and drag).
  - The destructive button's own ring is `focus-visible:ring-destructive/20` in light and `/40` in dark (`button.tsx`). Its contrast was not measured.
- **The local patches.** Fifteen files apply the ring at full strength themselves (`focus-visible:ring-ring`): among them `ui/dialog.tsx`, `ui/switch.tsx`, `ui/slider.tsx`, `instant-tooltip.tsx`, `sidebar-brand.tsx`, `service-detail-header.tsx`, `service-source-card.tsx`, `service-source-picker.tsx`, `git-credentials-menu.tsx`, `chart-instance-menu.tsx`, `event-timeline.tsx`. The Update Source rework (`a6076387b5`) put it on four of them, one review finding at a time; whether each of the others is a correction or the control's own default is for t001 to read.

## What is drawn now

Shipped 2026-11-25 (t001, t002).

- **The treatment** (t001): a 2 px ring at full strength, `focus-visible:ring-2 focus-visible:ring-ring`, on every shared control; the bordered ones keep `focus-visible:border-ring`, so a field reads as a 3 px band. It replaced the 3 px ring at 50% in `button`, `input`, `textarea`, `auto-textarea`, `select`, `checkbox`, `tabs`, `badge` and `scroll-area`, and the 2 px ring at 50% in `section-navigation.tsx` and the log viewport. The pill and underline tabs lost their `ring-offset-2`, whose offset was painted white whatever the surface.
- **The rest at the same bar.** The base layer's outline is `outline-ring` (the plain links' 1 px auto outline, which Chrome paints 2 px). The Ory form rule and the GitHub sign-in button (`sso-buttons.tsx`, a third `color-mix` at 50% the filing did not list) draw `0 0 0 2px var(--ring)`. The destructive button, the destructive badge and the destructive confirm (`confirm-dialog.tsx`) draw `ring-destructive` at full strength in both themes, and an invalid field's ring is `aria-invalid:ring-destructive` (it was `/20` and `/40`, and it is the ring a focused invalid field shows). The two agent composers draw `ring-2 ring-ring` on focus and while a file is dragged over them. Menu and select items, which marked keyboard focus with `bg-accent` alone (1.2:1), add `focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset`.
- **The light high-contrast twin's `--ring`** is `oklch(0.4 0.14 138.3)`, its `--primary`: the normal ring is 4.35:1 on `--muted` and `--accent` (a tab list, a highlighted menu item), under the twin's 4.5:1.
- **The local patches removed** (t001 step 4): `focus-visible:ring-ring` on the Update Source footer buttons and Branch field (`service-source-card.tsx`), the repository list's actions and search field and the source tabs' 3 px override (`service-source-picker.tsx`), and the credentials menu's buttons (`git-credentials-menu.tsx`). The others stay: each is a raw element with no shared ring (`dialog.tsx`'s close, `switch`, `slider`, `sheet`, `instant-tooltip`, `service-detail-header`, `event-timeline`, the chart menus, the repository and deploy rows' inset rings). `sidebar-brand.tsx`'s ring offset is painted `--sidebar` now, the surface it sits on (it was `--background`, 1.06:1 against the sidebar).
- **Computed ratios** of `--ring` at full strength (`dashboard/src/style.css`, WCAG relative luminance):

  | theme | `--background` | `--card`, `--popover` | `--muted`, `--accent` | `--sidebar` |
  | --- | --- | --- | --- | --- |
  | light | 4.90 | 5.19 | 4.35 | 4.62 |
  | dark | 9.20 | 8.80 | 8.21, 7.85 | 9.03 |
  | light high contrast | 8.23 | 8.72 | 7.31 | 7.76 |
  | dark high contrast | 9.20 | 8.80 | 8.21, 7.85 | 9.03 |

  `--destructive` at full strength: 4.93, 5.22, 4.38 and 4.65 in light; 5.65, 5.40, 5.04 / 4.82 and 5.54 in dark; 6.76 or more in both twins. `--sidebar-ring` on `--sidebar`: 4.62 light (and its twin), 9.03 dark.

- **Measured in a browser** (t002, 2026-11-25): Chromium 1440×900, the tree's dashboard (`vite dev`) against `yarn local-bex`, the stub's web service `eden-cms-v2`; the sign-in form served Kratos's production login flow shape through intercepted requests, since the stub has no login flow. A script pressed Tab (arrow keys inside the menus) and read, at each stop, the focused element's ring, outline and pseudo-element bar, blended each over the backgrounds behind it, and computed the ratio; transitions were off so no stop was read half-drawn. Lowest ratio per page:

  | page (stops) | light | dark | light high contrast | dark high contrast |
  | --- | --- | --- | --- | --- |
  | dashboard home (28) | 4.61 sidebar link | 9.04 | 4.61 workspace button | 9.04 |
  | service Settings (88) | 4.33 section rail, current item (inset on `--accent`) | 7.85 | 4.61 workspace button | 7.85 |
  | service Logs (40) | 4.61 | 9.04 | 4.61 | 9.04 |
  | New Web Service (27) | 4.33 repository row (inset on its focus tint) | 8.21 | 4.61 | 8.21 |
  | Update Source dialog (14) | 4.27 repository row | 6.31 | 7.01 | 6.31 |
  | Regenerate Hook confirm (3) | 4.88 | 5.43 destructive action | 7.62 | 8.53 |
  | delete domain confirm (3) | 4.88 | 5.43 Delete | 7.62 | 8.53 |
  | account menu (5) | 4.30 Log out (inset) | 7.18 | 7.21 | 6.34 |
  | Manual Deploy menu (5) | 4.88 | 8.82 | 8.27 | 8.82 |
  | sign-in form (9) | 4.88 | 8.57 | 8.27 | 5.91 |

  Every stop has an indicator; none is under 3:1 in light or dark or under 4.5:1 in a twin. At 390×844 (home, Settings, Update Source and sign-in) the lowest are 4.88, 4.33, 4.27 and 4.88 in light and 9.19, 7.85, 6.31 and 8.57 in dark. Sampled pixels agree with the computed colours: a focused project card's outline and the New button's ring paint `rgb(48, 125, 3)` on `rgb(248, 248, 250)`, 4.88:1. One thing the sweep showed that no colour fixes: Tab scrolls a control to the edge of the page's scroll container, which cuts the 2 px of ring on that side until the page scrolls on; it is in [w3/114](../114.md).

## What Render does

Measured 2026-11-25 (t003). No signed-in walk: the shared Playwright browser that holds a Render session was in use by another session. Render's dashboard sign-in page (`dashboard.render.com/login-next`) was walked with Tab, and its stylesheet (`index-CKJVak19.css`) and `Tabs` chunk (`Tabs-BccqB0tj.js`) were read; the hashes change with every Render deploy.

| control | Render's indicator | ratio |
| --- | --- | --- |
| text field (Email, Password) | 2 px solid outline, flush outside the 1 px border | 3.71 on white |
| primary button (Sign in), provider buttons | 2 px solid outline, 2 px off the control | 3.71 |
| icon button (show password) | 2 px solid outline, inset (`-2px`) | 3.71 |
| link (Forgot password?, sign up) | 2 px solid outline, 2 px off the text | 3.71 |
| tab (`Tabs` chunk) | `outline-2 outline-offset-2` (or `-offset-1`), not walked | 3.56–3.71 |

The colour is `--outline-focus-action`: `#0088e5` in light and light high contrast (3.71:1 on `#fff`, 3.56:1 on `#fafafa`), `#8ad6ff` in dark and dark high contrast (12.16:1 on `#0d0d0d`). Against bex: both draw 2 px at full strength; bex measures 4.27–4.88 in light where Render measures 3.56–3.71, and 4.61 or more in the light twin, where Render keeps its 3.71. The colour differs (bex's brand green, Render's blue), which is not a gap. One difference remains, filed as [w3/113](../113.md): Render sets buttons, links and tabs 1–2 px off the control, where bex's ring is flush, so bex's ring meets a primary button's fill of the same green.

## Definition of done

1. In a real browser, in the light and the dark theme, on the pages t002 lists, every control reached with Tab shows a focus indicator whose painted colour measures at least 3:1 against the surface behind it. In the two high-contrast twins it measures at least 4.5:1 (the bar w3/m531 set). The measured ratios are recorded in this README as a table.
2. `rg "(ring|outline)-ring/[0-9]" dashboard/src` finds no use whose blended contrast is under the bar on a surface it is drawn on. The Ory form rule and the destructive button's ring meet the same bar.
3. `theme-contrast.test.ts` fails when a component applies the ring token below the bar: putting `/50` back on `button.tsx` turns it red.
4. A local `focus-visible:ring-ring` that only restored full strength over a shared control is gone where that control now provides it.

## Source + Goal linkage

- **Source:** the Update Source dialog review of 2026-11-23 (its judge sampled the dialog's close ring at 2.9:1 and found the other stops fixed only locally), verified the same day by the token arithmetic and the browser measurement above. Builds on [w3/m531](../m531/README.md), whose t005 set the 3:1 and 4.5:1 bars in `theme-contrast.test.ts`.
- **Goal linkage:** the dashboard is the product's main surface, and its own test file states the accessibility bar it holds (WCAG 2.1 AA: 3:1 for the part of a control that identifies its state, 1.4.11). Keyboard users cannot see where they are on the borderless controls today.
- **Expected outcome:** tabbing through any page shows a clearly visible focus indicator on every control in both themes, and the contrast test protects what is painted.
- **Why now:** the test passes at 4.9:1 while the screen shows 2.05:1, so it gives false assurance; every new component inherits the weak ring from the shared controls; and fifteen files have already patched it locally, one review at a time. One change in the shared controls replaces those patches.
- **Render parity:** included (t003). The change is to a user-facing surface, and Render draws its focus indicator at full strength.
