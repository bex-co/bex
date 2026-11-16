# w10 · m376 — Absolute times read in the dashboard's language, and a custom range states its window

**Worker:** worker10 **Goal:** in the zh dashboard every absolute date and time reads in Chinese while en stays byte-for-byte unchanged, the log clock follows the dashboard language instead of the browser's, a custom Logs or Metrics range shows the window it covers, and the Pre-Deploy Command has one Chinese name. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The shared date formatters follow the dashboard language (absorbs w4/600) — **DONE** | 1h | — |
| [t002](done/t002.md) | Server and client format dates in the same language, so a zh cold load hydrates cleanly — **DONE** | 30m | t001 |
| [t003](done/t003.md) | A custom Logs or Metrics range shows the window it covers (absorbs w4/619) — **DONE** | 25m | t001 |
| [t004](done/t004.md) | The Pre-Deploy Command has one Chinese name (absorbs w4/617) — **DONE** | 15m | — |
| [t005](done/t005.md) | A test keeps hard-coded locales out of date formatting — **DONE** | 20m | t001 |
| [t006](done/t006.md) | Render parity — **DONE** | 20m | t002, t003, t004, t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 40m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 3h55m**.

## Definition of done

On production, after the deploy that carries this milestone:

1. With `?lang=zh`, a deploy detail page's `创建于`, its status timeline, the log clock, Billing → Charges' dates and the Events tab's tooltips read in Chinese (`2026年…月…日 …`).
2. With `?lang=en` the same places read exactly as before (`November 11, 2026 at 11:06 PM`, `11:07:28 PM`).
3. A zh cold load of a service page logs no hydration error.
4. A custom Logs range of 3:39:00–3:39:20 PM on a service, and a custom range on a Key Value Logs tab and on Metrics, show that window in the trigger, in en and in zh.
5. The zh Settings and Usage pages name the Pre-Deploy Command only "预部署命令".
6. `yarn lint` and `yarn test` pass, including the locale guard.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w10` (2026-11-12/13), filed by `/pmx them all to w10` on 2026-11-13, re-checked at filing against `origin/main` `3c6f11dde`, item 5. Absorbs [w4/600](../../w4/done/600.md), [w4/619](../../w4/done/619.md) and [w4/617](../../w4/done/617.md) (`/qa-find-bugs w4` rounds 231-251, 2026-11-12/13).
- **Render:** n/a for the language (Render has no Chinese dashboard); the en output stays as it is.
- **Parity scope, stated plainly:** this is bex's own i18n consistency, not a Render-parity gap. It was kept in the filing because the user selected every proposed item (`/pmx them all to w10`); the brainstorm's follow-up noted it carries little parity value.
- **Goal linkage:** the dashboard is bex's primary product surface (ADR008); the zh locale shipped in w4/573 and w4/587.
- **Expected outcome:** a zh reader sees no English month names or AM/PM, and a custom range says which window it covers.
- **Why now:** dates are the last English left in the zh dashboard, and w4/619's label needs the locale-aware formatter, so both land together.
- **Render parity:** included as a consistency check only (UI changes; en must stay unchanged).

## Findings

**2026-11-14 (`/loopx w10`).** Every formatter now takes `i18n.language`, the instance SSR renders with, and every absolute time still renders only after hydration, so the SSR markup holds no formatted time. The ESLint guard found nine more locale-less call sites than w4/600 listed (chart tooltips and markers, cron runs, skill versions, Usage month labels, a disk snapshot, four time-zone probes); all now go through `src/common/lib/format`. `periodLabel(…).split(" ")[0]` (the Usage month) only ever worked in English; `periodMonthLabel` replaces it. w21/054 is a separate language-bootstrap case and is not claimed fixed.

**2026-11-16 (`/pm unblock`, verified evidence).** Affects w10/m376/t002 and t009. The release gate is clear: production runs pin `284b83e1f540` (deploy.yml on `284b83e1f` succeeded 2026-11-16T21:11Z; pin commit `02026e53f`), and `git merge-base --is-ancestor 8842ddc56 284b83e1f540` holds, so the deploy carries m376's last code commit. No decision was needed. Next: replay DoD 1–5 on dashboard.bex.co as the QA user (zh/en cold loads with the console checked, custom Logs/Key Value/Metrics range labels, 预部署命令), then close or re-park on that evidence. Remaining gate: none.

**2026-11-16 (live DoD replay, QA user, pin `284b83e1f540`).** All six DoD items hold on dashboard.bex.co. (1) `?lang=zh` on deploy `dep-dbth5tp31crs73ajgof0` (hello-go): `创建于: 2026年11月16日 上午6:28`, timeline `2026年11月16日 上午6:28`, log clock `上午06:29:38`; Billing invoices `2026年10月`; Events tooltips `2026年11月16日 上午6:30`. (2) `?lang=en`: `November 16, 2026 at 6:28 AM`, `06:29:38 AM`, `October 2026`, Events `November 16, 2026 at 6:30 AM`. (3) Cold loads of the service deploy, Events, Settings, Billing, Logs, Metrics and Key Value pages in zh (by `?lang=zh` and by the `i18nextLng=zh` cookie alone) logged no console error or warning. (4) A custom 3:39:00–3:39:20 PM window on the service Logs, the Key Value Logs tab and Metrics shows `Nov 15, 3:39:00 PM – 3:39:20 PM` in en and `11/15 下午3:39:00 – 下午3:39:20` in zh. (5) zh Settings and Billing name only `预部署命令`. (6) `yarn lint`/`yarn test` gate deploy.yml, which passed on `284b83e1f`. Closed.
