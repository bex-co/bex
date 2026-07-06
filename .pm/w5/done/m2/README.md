# w5 · m2 — Polish dashboard UI: beancount-style layout, remove Ory branding

**Worker:** worker5 **Goal:** Every dashboard page (auth flows + the sample "Services" page) reads as one polished bex product — reusing the visual density/rhythm of the original beancount-dashboard reference (stat-card grids, consistent spacing/typography) — with no visible Ory Elements branding. **Status:** done

## Tasks (in order)

| id   | title                                                             | est | depends_on | |
| ---- | ------------------------------------------------------------------ | --- | ---------- | --- |
| t001 | Capture reference layout pattern from beancount-dashboard           | 30m | —          | **DONE** |
| t002 | Hide Ory Elements branding across all Kratos flow pages              | 30m | t001       | **DONE** |
| t003 | Polish login + register pages                                       | 45m | t002       | **DONE** |
| t004 | Polish forgot-password + settings pages                             | 45m | t002       | **DONE** |
| t005 | Polish logout page + dashboard sample page/sidebar; final review pass | 45m | t003, t004 | **DONE** |
| t006 | Simplify                                                            | 20m | t005       | **DONE** |
| t007 | Test coverage                                                       | 30m | t005       | **DONE** |

## Definition of done

Every page under `dashboard/src/features/auth/pages/` and `dashboard/src/routes/index.tsx` shares one consistent visual language (spacing scale, card treatment, typography) reused from the beancount-dashboard reference (`overview-stat-card.tsx`/`overview-metrics-panel.tsx`, `ledger-layout`); no Ory logo/branding badge is visible on any rendered auth page (`hide_ory_branding: true` set, verified via a real browser pass — no Playwright-visible Ory logo image/link); `yarn typecheck && yarn lint && yarn test && yarn build` all pass.

## Source + Goal linkage

- **Source:** user request 2026-07-06 — "get back to the dashboard. can we preserve the same kind of layout from beancount dashboard https://beancount.io/ledger/open_ledger/example and then remove ory logos from those element components? review page one by one and ensure they all look polished."
- **Goal linkage:** `docs/vision.md`'s human-facing dashboard pillar — the dashboard is bex's Render-style product surface; a polished, on-brand UI (not a generic Ory-branded widget bolted on) is table stakes for anyone actually using it, and directly supports the "one dashboard for every service" pitch already in `AUTH_FEATURES` copy.
- **Expected outcome:** a visual/UX pass across every dashboard page — auth flows (login/register/forgot-password/settings/logout) and the sample dashboard page — reviewed one by one, each matching the reference layout's density and polish, with Ory's own branding removed.
- **Why now:** the dashboard just went live end-to-end (w5/m1: scaffold + Kratos wiring; latest commit deployed it to `dashboard.bex.co`) — before any more features layer on top, this is the cheapest point to fix the visual rough edges (Ory's default branding, inconsistent spacing between the hand-built shell and Ory-rendered flow cards) rather than doing it later across more surface area.
