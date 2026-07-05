---
description: Arrange the .pm board — create/promote workstreams, milestones, tasks; mark done; show status
argument-hint: "[status | new workstream <title> | add <wN> <idea> | promote <wN/NNN> | new milestone <wN> <title> | add-task <wN/mN> <title> | done <wN/mN/tNNN>]"
allowed-tools: Read, Write, Edit, Bash(ls:*), Bash(find:*), Bash(cat:*)
---

# Task: Arrange the `.pm` board

`/pm` is the **only** command that writes to `.pm/`. It arranges milestones and tasks under the conventions below. `/pm-brainstorm` proposes; `/pm` materializes. Parse the subcommand from `$ARGUMENTS` (default = `status`).

## The `.pm` hierarchy

| Level | Path | Meaning | Effort |
| --- | --- | --- | --- |
| Workstream | `wN/` (`w1`, `w2`, `w3`, …) | a themed track / "worker" roadmap; `README.md` + inbox notes | — |
| Inbox note | `wN/NNN.md` (`w1/005.md`) | one idea or a **sub-hour** unit of work, plain markdown | ≤ ~1h |
| Milestone | `wN/mN/` (`m1`, `m2`, …) | a shippable chunk: `README.md` + task files | **> ~1h**, multiple tasks |
| Task | `wN/mN/tNNN.md` | a single unit | tens of minutes |

## Rules (enforce every time)

- **Sizing rule.** A milestone must be **> ~1 hour of work across more than one task**. If a chunk is ≤ ~1h (tens of minutes, a task or two), do **NOT** create an `mN/` directory — record it as a loose inbox note `wN/NNN.md`. Tasks take tens of minutes; milestones take hours.
- **IDs must match the path.** A task's `id: wN/mN/tNNN` frontmatter must equal the directory it lives in. Never create a milestone dir whose path disagrees with the IDs inside it (the existing `w2/m1`-holds-`w1/m1`-IDs drift is the anti-example — if you touch it, flag/repair, don't copy it).
- **Keep status in sync** across all three places it lives: the workstream `README.md` milestone checkbox, the milestone `README.md` `**Status:**` line + the `— DONE` marker in the task table, and each task's `status:` frontmatter.
- **Numbering:** next free zero-padded 3-digit for inbox notes (`NNN`) and tasks (`tNNN`); next free `wN` / `mN`. Scan the tree first; don't reuse a number.
- Use `worker: worker1` unless the workstream README names another worker.
- After editing any `.md`, run `npx prettier@3.4.2 --write "**/*.md"` (repo rule).

## Subcommands

### `status` (default)

Read the tree (`find .pm -type f -name '*.md'`, skipping `done/`). Print, per open workstream: its milestones with `**Status:**`, and the **next actionable task** per milestone — the first non-done task whose `depends_on` are all satisfied. Also list open inbox notes. Touch no files.

### `new workstream <title>`

Create the next free `wN/` with `README.md` from the workstream template below.

### `add <wN> <idea…>`

Create the next free inbox note `wN/NNN.md` with the idea as plain terse markdown (no frontmatter). This is the default home for **sub-hour** work.

### `promote <wN/NNN>` / `new milestone <wN> <title>`

Apply the **sizing rule first.**

- If the work is **> ~1h and splits into more than one task**: create `wN/mN/` with `README.md` (milestone template) + one `tNNN.md` per task (task template), add the `- [ ] **mN** — …` line to the workstream `README.md`, and cite the source (`## Source` → the inbox note or brainstorm).
- If it is **≤ ~1h**: do NOT create a milestone. Keep/append it as an inbox note `wN/NNN.md` and tell the user why (too small for a milestone).

### `add-task <wN/mN> <title>`

Create the next `tNNN.md` from the task template and add its row to the milestone `README.md` table. Update the `(N tasks)` count in the workstream README.

### `done <wN/mN/tNNN>`

1. Set the task's frontmatter `status: done`.
2. In the milestone `README.md`: mark the row `— **DONE**` and update the `**Status:**` line (e.g. `todo (t001 done)`).
3. **Move** the file to `wN/mN/done/tNNN.md`.
4. If no open tasks remain in the milestone, **move the whole milestone** to `wN/done/mN/` and check its box (`- [x]`) in the workstream `README.md`.

Show the intended moves before mutating if the user passed `DRY_RUN=1`.

## Templates

### Workstream `README.md`

```markdown
# wN — <roadmap title> (<worker>)

**Worker:** <worker> <one-line provenance / ordering rationale>

## Milestones

- [ ] **mN** — <title> (<N> tasks) ← from <source>
```

### Milestone `README.md`

```markdown
# wN · mN — <name>

**Worker:** <worker> **Goal:** <what shipping this achieves> **Status:** todo

## Tasks (in order)

| id   | title   | est | depends_on |
| ---- | ------- | --- | ---------- |
| t001 | <title> | 30m | —          |

## Definition of done

<observable, testable end state>

## Source

<pointer to the inbox note / brainstorm / docs this came from>
```

### Task `tNNN.md`

```markdown
---
id: wN/mN/tNNN
title: <title>
worker: <worker>
status: todo
estimate: 30m
depends_on: [wN/mN/tMMM]
---

## Objective

<one paragraph>

## Context

- <concrete paths / k8s resources / facts>

## Steps

1. <step>

## Files

- <paths to touch>

## Acceptance criteria

- [ ] <testable check>

## Out of scope

- <deferred adjacent work>
```

### Inbox note `wN/NNN.md`

Plain terse markdown, no frontmatter — one idea or a sub-hour unit of work.

## Arguments

$ARGUMENTS
