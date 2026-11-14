# .pm/AGENTS.md

Internal PM board — see [README.md](README.md). `/pm` is the only skill that writes here; its conventions live in [.claude/skills/pm/SKILL.md](../.claude/skills/pm/SKILL.md).

## Evidence lives outside git

The board is markdown and small text (user decision 2026-11-04, w6/100 D10). Never commit build artifacts or bulk evidence here: no file over 1 MiB and no `whl`/`gz`/`tar`/`zip`/`bin`/image/`pdf`/database file under `.pm/`. Raw run output of any size is evidence too: no `*evidence*` directory, no `json`/`jsonl`/`log`/`stdout`/`stderr` or `*evidence*.txt` file anywhere under `.pm/` (second history rewrite, 2026-11-04). Keep it in a local `evidence/` directory (gitignored) or upload it, and write the conclusion, the sha256 and the link into the item's markdown. `scripts/repo-artifact-guard.sh` rejects violations at ship time and in CI.

## Done items move to `done/` folders

When work is completed, do not leave it in place — move it:

- **Task done** (`wN/mN/tNNN.md`): set frontmatter `status: done`, mark its row `— **DONE**` in the milestone `README.md` and update the `**Status:**` line, then **move the file to `wN/mN/done/tNNN.md`**.
- **Milestone done** (no open tasks left in `wN/mN/`): **move the whole milestone directory to `wN/done/mN/`** (`mv` the directory, then `rmdir` the empty original — **never leave a tombstone, stub, or redirect README at the old path**; a done milestone's original path must simply not exist) and check its box (`- [x]`) in the workstream `README.md`.
- **Inbox note done** (`wN/NNN.md`): **move it to `wN/done/NNN.md`** (e.g. `w1/done/001.md`).

**A move rewrites the links that point at it, and numbering reads `origin/main`.** Both rules are spelled out in the `/pm` skill (`.claude/skills/pm/SKILL.md`, **Numbering** and **Moves rewrite inbound links**). Two scripts enforce what they can in CI (`.github/workflows/references.yml`): `bash scripts/docs-links-validate.sh` fails a link in `docs/`, an `AGENTS.md` or `README.md`, or `.pm/*.md` left pointing at a path that no longer exists, and `python3 scripts/pm-board-validate.py` fails a change that adds a milestone number another workstream already uses, or an open task whose `id:` differs from its path. The workstreams' own `README.md` files are not yet read by the link gate.

Keep status in sync across all three places it lives: the workstream README checkbox, the milestone README `**Status:**` line + task-table `— DONE` markers, and each task's `status:` frontmatter. When scanning the board for open work, skip `done/` directories. If a `/goal` (or other tooling) references a milestone by its pre-move path, that path is _expected_ to disappear on completion — do not recreate it.
