# w21 · m397 — Docs and board references stay valid: a link gate, repaired links, and no new duplicate milestone numbers

**Worker:** worker21 **Goal:** the Markdown links in `docs/`, the `AGENTS.md` files and READMEs resolve and stay resolving, `/pm` moves rewrite the links that point at what they move, and a change cannot add a milestone number another workstream already uses. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Add a docs link gate script and its test | 60m | — |
| [t002](t002.md) | Repair the broken links by cause | 75m | t001 |
| [t003](t003.md) | Make `/pm` rewrite inbound links on moves and number from `origin/main` | 45m | — |
| [t004](t004.md) | Add a board validator for new duplicate numbers and open-task id drift | 45m | t003 |
| [t005](t005.md) | Run both gates in CI, including for `.pm`-only changes | 30m | t001, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 20m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

1. `bash scripts/docs-links-validate.sh` exits 0 on the tree; its test script shows a missing target fails it and fenced code is ignored.
2. The `/pm` skill states that moves rewrite inbound links and that numbering reads `origin/main`; `bash scripts/skill-layout-validate.sh` passes.
3. `python3 scripts/pm-board-validate.py` fails on a newly added duplicate `mN` and on an open task whose `id:` differs from its path, and passes on the tree.
4. A new CI workflow runs both on changes to `**/*.md`, `.pm/**` and the scripts, and `scripts/github-actions-validate.sh` and `scripts/ci_test_inputs.py` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w21` (2026-11-14), filed by `/pmx them all to w21` on 2026-11-14, re-checked at filing against `origin/main` `cc671cbfd`, item 4. A script over `docs/`, `AGENTS.md` files and READMEs counted 1900 relative links and 71 broken (22 into evidence purged on 2026-11-04, 16 gone, 12 moved by `/pm`, 18 into renamed docs), and 16 of 143 milestone numbers since m250 are used by two or three workstreams.
- **Goal linkage:** The repo treats `docs/` and `AGENTS.md` as the entry points for agents (the AI-native thesis in ADR008) and `.pm` as the work record.
- **Expected outcome:** A reader or agent following a link from an ADR reaches the file; a milestone number names one milestone.
- **Why now:** The 2026-11-04 purge turned 22 links into dead ends at once and every closeout keeps adding more, so the count only grows; the duplicate numbers already make a bare `m348` ambiguous in commit messages and notes.
- **Render parity task:** omitted. Documentation and repository tooling; no REST, GraphQL, MCP or dashboard surface changes.

## Open decisions

None needing the user.
