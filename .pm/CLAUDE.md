# .pm/CLAUDE.md

Internal PM board — see [README.md](README.md). `/pm` is the only skill that writes here; its conventions live in [.claude/skills/pm/SKILL.md](../.claude/skills/pm/SKILL.md).

## Done items move to `done/` folders

When work is completed, do not leave it in place — move it:

- **Task done** (`wN/mN/tNNN.md`): set frontmatter `status: done`, mark its row `— **DONE**` in the milestone `README.md` and update the `**Status:**` line, then **move the file to `wN/mN/done/tNNN.md`**.
- **Milestone done** (no open tasks left in `wN/mN/`): **move the whole milestone directory to `wN/done/mN/`** (`mv` the directory, then `rmdir` the empty original — **never leave a tombstone, stub, or redirect README at the old path**; a done milestone's original path must simply not exist) and check its box (`- [x]`) in the workstream `README.md`.
- **Inbox note done** (`wN/NNN.md`): **move it to `wN/done/NNN.md`** (e.g. `w1/done/001.md`).

Keep status in sync across all three places it lives: the workstream README checkbox, the milestone README `**Status:**` line + task-table `— DONE` markers, and each task's `status:` frontmatter. When scanning the board for open work, skip `done/` directories. If a `/goal` (or other tooling) references a milestone by its pre-move path, that path is _expected_ to disappear on completion — do not recreate it.
