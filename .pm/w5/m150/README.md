# w5 · m150 — Render templates with free-form resource names or dotted env keys validate

**Worker:** worker5 **Goal:** Render templates whose resource names are free-form or whose env keys are dotted validate and deploy on bex. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | A Blueprint name is a display name mapped onto a generated slug id | 1h30m | — |
| t002 | Accept dotted env keys end to end | 1h15m | — |
| t003 | Corpus and template tests hold the line | 45m | t001, t002 |
| t004 | Render parity | 30m | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

Total: **7 tasks, about 5h15m**.

## Definition of done

- pgadmin, gitpod-openvscode-server, vibe-code-demo-apps, postgresql-missing-trigger-index and elasticsearch validate in the corpus run.
- `knownBlueprintGaps` is empty.

## Source + Goal linkage

- **Source:** fifth `/pm-brainstorm for w5` (2026-10-20), all five items chosen by the user the same day after `2163b27c7` made Render parity the top principle (a divergence stands only with ADR018-recorded user sign-off; never park parity on an optional decision); promoted from w5/169 (w5/m143's corpus run), which was parked on a naming decision that the new principle forbids parking on.
- **Goal linkage:** ADR049 render.yaml parity; ADR008.
- **Expected outcome:** Render's own templates deploy unchanged.
- **Why now:** the principle change makes the parked note workable now, with the stricter naming choice left to w5/m118.
- **Render parity:** included: Blueprint validation, REST and dashboard names.
