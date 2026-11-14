# w21 · m396 — Form checks and server rules share one corpus: the Go validator and the dashboard validator must agree on every input

**Worker:** worker21 **Goal:** one corpus per validator family drives both a Go test against the real server validator and a dashboard test against the client validator, so a form that refuses a valid input or accepts an invalid one fails CI instead of surfacing as a QA finding or an English toast. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Inventory the mirrored validators and choose the first families | 45m | — |
| [t002](t002.md) | Define the corpus format and assert it against the Go validators | 60m | t001 |
| [t003](t003.md) | Assert the dashboard's validators against the same corpus | 60m | t002 |
| [t004](t004.md) | Resolve the divergences the corpus finds | 90m | t003 |
| [t005](t005.md) | Register the corpus as a test input of both suites | 30m | t002, t003 |
| [t006](t006.md) | Render parity | 20m | t004, t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 20m | t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

## Definition of done

1. At least eight families have a corpus under `lego/backend/testdata/validation/` (or the location t001 settles), with boundary inputs, and a Go test asserts the real server validator on every entry.
2. A dashboard test asserts each client validator against the same files: a deliberately stricter and a deliberately looser client both fail it (self-test); a client may be more lenient only on entries marked `serverOnly`.
3. Every divergence found is fixed, marked `serverOnly` with a reason, or filed as a follow-up note.
4. `python3 scripts/ci_test_inputs.py` passes, a corpus-only change is in the `paths` of both workflows, and `yarn lint`, `yarn test` and `cd lego/backend && go test ./...` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21` (2026-11-14), filed by `/pmx them all to w21` on 2026-11-14, re-checked at filing against `origin/main` `cc671cbfd`, item 3. Thirteen w21 notes (015, 023, 033–036, 038, 043, 045, 047, 048, 050, 053) are one defect found field by field: 38 dashboard files name a backend path in a comment while 4 tests read Go source, and those compare vocabulary constants only. The repo's rule for two-sided duplication is a drift-guard test (`rt-unify-duplicates`).
- **Goal linkage:** Render parity first in the dashboard (ADR008), and the root `AGENTS.md` rule that the dashboard consumes the API and a hand-mirrored rule needs a guard.
- **Expected outcome:** Validation drift between the forms and bex-api fails a test; new forms add a corpus entry rather than a hand-copied regex.
- **Why now:** The defect is still being found (w4/642 is the latest), each costs a QA round, and the validators are pure functions on both sides, so one corpus is cheap now and costly to retrofit after more forms copy the rules.
- **Render parity task:** included (t006). The milestone edits dashboard form rules that mirror REST, GraphQL and MCP validators; the check compares each pinned rule against Render so the corpus does not harden a divergence.

## Open decisions

Recorded as the proposal's recommendation, not a user decision. A rule where bex's server is stricter than Render's, with no entry in `docs/ADR018-render-parity.md`, stands only with the user's sign-off. The recommendation, from ADR008 Render parity first, is to conform the server to Render in a follow-up and then have the corpus pin the conformed rule; until then the corpus pins the current behavior and t006 files the follow-up. The corpus never tightens a rule.

## Findings

(t001 fills in the pairing table.)
