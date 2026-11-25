# w3 · m642 — Keyboard focus is drawn at the contrast the theme test promises, on every control

**Worker:** worker3 **Goal:** every focus indicator the dashboard draws reaches 3:1 against the surface behind it in both themes (4.5:1 in the high-contrast twins), and `theme-contrast.test.ts` checks the ring at the strength the components apply, not the token alone. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The shared controls and the global outline draw the ring at a strength that passes | 45m | — |
| [t002](t002.md) | Sweep the pages in both themes and both high-contrast twins for the indicators the shared controls do not own | 45m | t001 |
| [t003](t003.md) | Render parity | 20m | t002 |
| [t004](t004.md) | Simplify | 15m | t003 |
| [t005](t005.md) | Test coverage | 30m | t003 |
| [t006](t006.md) | Closeout | 10m | t004, t005 |

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

## What Render does

Seen 2026-11-23 on Render's New Web Service page (signed in, read only): the focused repository search field is drawn with a solid blue 2 px border, at full strength. Its colour and ratio were not measured; t003 measures it.

## Definition of done

1. In a real browser, in the light and the dark theme, on the pages t002 lists, every control reached with Tab shows a focus indicator whose painted colour measures at least 3:1 against the surface behind it. In the two high-contrast twins it measures at least 4.5:1 (the bar w3/m531 set). The measured ratios are recorded in this README as a table.
2. `rg "(ring|outline)-ring/[0-9]" dashboard/src` finds no use whose blended contrast is under the bar on a surface it is drawn on. The Ory form rule and the destructive button's ring meet the same bar.
3. `theme-contrast.test.ts` fails when a component applies the ring token below the bar: putting `/50` back on `button.tsx` turns it red.
4. A local `focus-visible:ring-ring` that only restored full strength over a shared control is gone where that control now provides it.

## Source + Goal linkage

- **Source:** the Update Source dialog review of 2026-11-23 (its judge sampled the dialog's close ring at 2.9:1 and found the other stops fixed only locally), verified the same day by the token arithmetic and the browser measurement above. Builds on [w3/m531](../done/m531/README.md), whose t005 set the 3:1 and 4.5:1 bars in `theme-contrast.test.ts`.
- **Goal linkage:** the dashboard is the product's main surface, and its own test file states the accessibility bar it holds (WCAG 2.1 AA: 3:1 for the part of a control that identifies its state, 1.4.11). Keyboard users cannot see where they are on the borderless controls today.
- **Expected outcome:** tabbing through any page shows a clearly visible focus indicator on every control in both themes, and the contrast test protects what is painted.
- **Why now:** the test passes at 4.9:1 while the screen shows 2.05:1, so it gives false assurance; every new component inherits the weak ring from the shared controls; and fifteen files have already patched it locally, one review at a time. One change in the shared controls replaces those patches.
- **Render parity:** included (t003). The change is to a user-facing surface, and Render draws its focus indicator at full strength.
