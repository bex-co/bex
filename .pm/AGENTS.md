# .pm/AGENTS.md

Internal PM board — see [README.md](README.md). `/pm` files and schedules work here, and `/loopx` workers land each item's outcome (done, `blocked/`, delete); no other skill writes here. The conventions live in [.claude/skills/pm/SKILL.md](../.claude/skills/pm/SKILL.md).

## Evidence lives outside git

The board is markdown and small text (user decision 2026-11-04, w6/100 D10). Never commit build artifacts or bulk evidence here: no file over 1 MiB and no `whl`/`gz`/`tar`/`zip`/`bin`/image/`pdf`/database file under `.pm/`. Raw run output of any size is evidence too: no `*evidence*` directory, no `json`/`jsonl`/`log`/`stdout`/`stderr` or `*evidence*.txt` file anywhere under `.pm/` (second history rewrite, 2026-11-04). Keep it in a local `evidence/` directory (gitignored) or upload it, and write the conclusion, the sha256 and the link into the item's markdown. `scripts/repo-artifact-guard.sh` rejects violations at ship time and in CI.

## The board is public

`bex-co/bex` is a public repository. Anyone can read every file under `.pm/`, `done/` and `blocked/` included, and every file under `docs/`. Git keeps a line after it is edited. The rule [ADR019](../docs/ADR019-infra-credentials.md) §5 sets for production job logs applies to the board and docs too:

- **Name only bex's own fixtures:** the workspaces in [`scripts/lib/first_party_workspaces.txt`](../scripts/lib/first_party_workspaces.txt) and their services, plus the disposable QA and drill workspaces a run creates and deletes. Mark each disposable one `(disposable)` on the line that names it.
- **Hash or leave out everything about other tenants:** their workspace, service, database and deploy ids, their service, app and host names, their members' emails, and their metering, billing and plan state. `scripts/lib/public_log.py` turns ids and names into keyed hashes that match an operator's local run. Otherwise write neutral words, such as "a Scale customer workspace" or "customer workspace 3 of 11".
- **Keep the full detail in your own local notes**, outside the repository.
- `scripts/ship-gates.sh` warns about a `tea-` id a push adds under `.pm/` or `docs/` that the list does not hold and its line does not mark `(disposable)` (`scripts/workspace_id_guard.py`). The warning never stops the ship, so the rule still rests on whoever writes the line.

## Done items move to `done/` folders

When work is completed, do not leave it in place — move it:

- **Task done** (`wN/mN/tNNN.md`): set frontmatter `status: done`, mark its row `— **DONE**` in the milestone `README.md` and update the `**Status:**` line, then **move the file to `wN/mN/done/tNNN.md`**.
- **Milestone done** (no open tasks left in `wN/mN/`): **move the whole milestone directory to `wN/done/mN/`** (`mv` the directory, then `rmdir` the empty original — **never leave a tombstone, stub, or redirect README at the old path**; a done milestone's original path must simply not exist) and check its box (`- [x]`) in the workstream `README.md`.
- **Inbox note done** (`wN/NNN.md`): **move it to `wN/done/NNN.md`** (e.g. `w1/done/001.md`).

**A move rewrites the links that point at it, and numbering reads `origin/main`.** Both rules are spelled out in the `/pm` skill (`.claude/skills/pm/SKILL.md`, **Numbering** and **Moves rewrite inbound links**). Two scripts enforce what they can in CI (`.github/workflows/references.yml`): `bash scripts/docs-links-validate.sh` fails a link in `docs/`, an `AGENTS.md` or `README.md`, `.pm/*.md`, a workstream's `.pm/wN/README.md` or its rolled `done/closed-K.md` left pointing at a path that no longer exists, and `python3 scripts/pm-board-validate.py` fails a change that adds a milestone number another workstream already uses, an open task whose `id:` differs from its path, or a BLOCKED summary that waits for a production deploy or a date without its checkable form (``pin must carry `<sha>` ``, `date gate: YYYY-MM-DD`; the `/pm` skill's **A gate that clears by itself has one checkable form**).

**A workstream README holds open work; its closed lines roll into `done/`.** Once `wN/README.md` passes 384 KiB, `python3 scripts/pm-roll-closed.py wN` moves its `- [x]` lines into new `wN/done/closed-K.md` files, each under 256 KiB and never appended to, and lists them under the README's `## Closed items`. This keeps the README under the markdown size guard's 512 KiB warning (`scripts/repo-artifact-guard.sh`). w4 rolled first, on 2026-11-24 (w10/058). The `/pm` skill has the details, under **A workstream README holds open work**.

Keep status in sync across all three places it lives: the workstream README checkbox, the milestone README `**Status:**` line + task-table `— DONE` markers, and each task's `status:` frontmatter. When scanning the board for open work, skip `done/` directories. If a `/goal` (or other tooling) references a milestone by its pre-move path, that path is _expected_ to disappear on completion — do not recreate it.

## Routine lists and loop state

`wN/routine/README.md` lists the commands `/loopx wN` runs on a schedule once the queue is drained (format and the `/pm routine` subcommand: `.claude/skills/pm/SKILL.md`). It is configuration, not an item: it never moves to `done/`, and only the user schedules a row. Run history, each loop's mark and the listener's state live in `.git/loopx/`, never on the board. `scripts/loopx-guard.py` writes them; it is also the `Stop` and `SessionEnd` hook in `.claude/settings.json` that keeps a `/loopx` session going while its workstream has ready work (`.claude/skills/loopx/SKILL.md`).
