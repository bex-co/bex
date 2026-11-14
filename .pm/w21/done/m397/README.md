# w21 · m397 — Docs and board references stay valid: a link gate, repaired links, and no new duplicate milestone numbers

**Worker:** worker21 **Goal:** the Markdown links in `docs/`, the `AGENTS.md` files and READMEs resolve and stay resolving, `/pm` moves rewrite the links that point at what they move, and a change cannot add a milestone number another workstream already uses. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Add a docs link gate script and its test — **DONE** | 60m | — |
| [t002](done/t002.md) | Repair the broken links by cause — **DONE** | 75m | t001 |
| [t003](done/t003.md) | Make `/pm` rewrite inbound links on moves and number from `origin/main` — **DONE** | 45m | — |
| [t004](done/t004.md) | Add a board validator for new duplicate numbers and open-task id drift — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Run both gates in CI, including for `.pm`-only changes — **DONE** | 30m | t001, t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

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

## Findings

### Link gate (t001, t002)

`scripts/docs-links-validate.sh` reads `docs/**/*.md`, every `AGENTS.md` and `README.md`, and `.pm/*.md` at the top level (not the `wN/` history or workstream READMEs, not the trees `.prettierignore` lists, which it reads at run time) and fails a relative link, image or reference definition whose target is missing, has the wrong case, or lies outside the repository. Fenced code, inline code, external URLs, absolute paths and anchors are not checked. On the tree it found 72 broken links (71 at filing, plus one from m396's own move), repaired by cause with no file recreated and no stub left:

| Cause | Links | Repair |
| --- | --- | --- |
| `.pm` item moved to `done/` or `blocked/` | 33 | the new relative path |
| evidence purged on 2026-11-04 | 22 | plain text: the link's words, then `(wN/mN evidence, purged 2026-11-04)`; the conclusion each line already states is untouched |
| `docs/ADR027-sso.md` linking the old names `auth.md`, `render-parity.md`, `sandboxes.md`, `github-integration.md` | 14 | `ADR012-auth.md`, `ADR018-render-parity.md`, `ADR014-sandboxes.md`, `ADR026-github-integration.md` |
| `render-webhook-vocabulary-2026-08-17.json`, renamed to `…-2026-11-02.json` by `4d4f37c84` (its `renderDashboardCapturedAt` still says 2026-08-17) | 3 | the renamed file |

### Board validator (t004)

`scripts/pm-board-validate.py` fails a milestone directory a change adds under a number another workstream uses, and an open or blocked task whose `id:` differs from its path. On the tree it found four blocked tasks with no frontmatter at all (`w3/m174/t007`, `w3/m175/t005`, `w3/m244/t008`, `w3/m248/t001`): the loopx park commits `029ed4ae9`, `5d684f694` and `670993de2` appended a gate note to a task path whose file was already gone, leaving only the note. Each was restored from the parent of its park commit with the note kept after it.

### Simplify (t006)

Applied: the board check compares milestone _directories_ against the merge base, not added files (the first version flagged any task added to a milestone whose number is already duplicated, `m348` and `m336` among them, and any move to another workstream); `.prettierignore` read at run time instead of copied; inline-code pairing in linear time (a regex took 61 s on 2,001 unclosed backticks); backtick fences with an info string, closing fences, targets with parentheses, links around images, footnotes, CRLF, targets outside the repository and targets of the wrong case; quoted, commented and BOM-prefixed `id:` lines; the duplicate prose in the skill and `.pm/AGENTS.md` stated once; the push trigger lost its path filter, since a document link breaks as readily when a script or fixture it names is deleted.

Declined: a shared shell test-assert library (two copies of a 20-line helper are cheaper than touching the ADR registry test); the duplicated `paths` list (GitHub workflow YAML has no shared anchors for it); a shallow checkout (a push run needs the commit before it, and a PR run with no merge base skips the check silently); parsing links split across lines (none in the tree); running the unit tests once per class (5 s). Filed: [078](../../078.md), the workstream READMEs the gate does not yet read (130 dead links in seven of them).

### Test coverage (t007)

| Piece | Fails when reverted |
| --- | --- |
| a missing target, image, reference definition, angle-bracket or encoded target | `docs-links-validate.test.sh` red cases, each naming `file:line -> target` |
| fenced code, inline code, external, absolute and anchor links ignored | the "ignored" fixture; a fence closes only on a fence at least as long |
| the fence opener and closer rules | with the opener allowing backticks in its info string, or the closer an info string, two cases fail |
| targets outside the repository, and of the wrong case | with either check removed, its case fails |
| linear-time span pairing | with the backtracking regex the dense-line case runs past 30 s |
| the `.prettierignore` trees | listed trees pass, an unlisted one is red |
| a new duplicate milestone number, committed, staged or untracked | `test_pm_board_validate.py`; with every directory counted as added, three cases fail, as does the real board |
| a historical duplicate stays workable, and a re-home is not a duplicate | with the deleted places not subtracted, the re-home case fails |
| the `id:` rule, quoted, commented and BOM-prefixed ids | with the plain pattern, the quoting case fails |
| the workflow's trigger and runner rules | `scripts/github-actions-validate.sh` and `scripts/ci_test_inputs.py` |

### Definition of done

1. `bash scripts/docs-links-validate.sh` exits 0 on the tree (268 files); its self-test proves a missing target fails and fenced code is ignored.
2. The `/pm` skill states that moves rewrite inbound links and that numbering reads `origin/main`; `bash scripts/skill-layout-validate.sh` passes.
3. `python3 scripts/pm-board-validate.py` fails a newly added duplicate `mN` and an open task whose `id:` differs from its path, and passes on the tree.
4. `.github/workflows/references.yml` runs both, and the board's self-test, on a push to main and on a pull request touching Markdown, `.pm/**`, `docs/**` or the scripts; `scripts/github-actions-validate.sh` and `scripts/ci_test_inputs.py` pass.
