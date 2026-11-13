# w10 · m376 — Absolute times read in the dashboard's language, and a custom range states its window

**Worker:** worker10 **Goal:** in the zh dashboard every absolute date and time reads in Chinese while en stays byte-for-byte unchanged, the log clock follows the dashboard language instead of the browser's, a custom Logs or Metrics range shows the window it covers, and the Pre-Deploy Command has one Chinese name. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The shared date formatters follow the dashboard language (absorbs w4/600) | 1h | — |
| [t002](t002.md) | Server and client format dates in the same language, so a zh cold load hydrates cleanly | 30m | t001 |
| [t003](t003.md) | A custom Logs or Metrics range shows the window it covers (absorbs w4/619) | 25m | t001 |
| [t004](t004.md) | The Pre-Deploy Command has one Chinese name (absorbs w4/617) | 15m | — |
| [t005](t005.md) | A test keeps hard-coded locales out of date formatting | 20m | t001 |
| [t006](t006.md) | Render parity | 20m | t002, t003, t004, t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 40m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

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

None yet.
