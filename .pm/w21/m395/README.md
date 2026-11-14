# w21 · m395 — Dashboard accessibility gates: lint, an axe pass over every page fixture, and a contrast test on the colour tokens

**Worker:** worker21 **Goal:** a missing accessible name, a control whose name drops its visible text, or a text and background pair below WCAG AA fails `yarn lint` or `yarn test`, so the class is caught by the suite instead of a QA round. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Wire an accessibility lint into `yarn lint` and triage the baseline — **DONE** | 60m | — |
| [t002](t002.md) | Run axe over every page-level fixture, in English and Chinese | 75m | — |
| [t003](t003.md) | Test the colour tokens' contrast in both themes | 45m | — |
| [t004](t004.md) | Fix what t001–t003 find, absorbing w4/607 and w4/612 | 90m | t001, t002, t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 20m | t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

## Definition of done

1. `yarn lint` runs accessibility rules for JSX; a planted `<img>` with no `alt` fails it. (If t001 finds no maintained linter that runs on ESLint 10, this bullet is replaced by the evidence in `dashboard/AGENTS.md` and bullets 2 and 3 carry the milestone.)
2. `yarn test` fails for an unnamed `dialog`, for a link whose accessible name omits its visible text, and for a token pair below the AA threshold in either theme; each has a self-test that shows it.
3. Every page-level fixture passes the axe pass in English and Chinese, or sits on an exempt table with a reason.
4. w4/607 and w4/612 are fixed (or were already) and are recorded as absorbed; the hard-coded English `aria-label`s in `autoscaling-section.tsx` and `code-block.tsx` read from the i18n catalogs.
5. `yarn lint` and `yarn test` pass, and the added `yarn test` time is recorded against the w21/m337 baseline.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21` (2026-11-14), filed by `/pmx them all to w21` on 2026-11-14, re-checked at filing against `origin/main` `cc671cbfd`, item 2. At filing the board held about 47 notes on accessible names, 26 on `aria-invalid` and 16 on contrast, each found by a live QA round and fixed in one file; `dashboard/eslint.config.js` has no accessibility rule and `axe-core` is not a dependency. Absorbs [w4/607](../../w4/done/607.md) and [w4/612](../../w4/done/612.md).
- **Goal linkage:** A correct, Render-style dashboard (ADR008 Render parity first) and the dashboard's own rules in `dashboard/AGENTS.md`.
- **Expected outcome:** Accessibility defects of this class fail the suite, as hydration mismatches have since w21/m337.
- **Why now:** Each missing name costs a QA round and a one-file fix, and two such notes are open now. The test sweep from w21/m337 already renders every page-level fixture in both languages, so the marginal cost of the pass is low, and a lint choice needs a spike because the usual plugin lags the project's ESLint major.
- **Render parity task:** omitted. This is lint and test infrastructure plus accessibility attributes; no REST, GraphQL or MCP field and no feature changes, and Render documents no accessibility contract to compare. t004's visible-copy changes are accessible names only.

## Open decisions

None needing the user. Which linter t001 picks is a technical choice recorded in `dashboard/AGENTS.md`.
