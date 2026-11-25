---
name: pm
metadata:
  internal: "true"
description: >-
  Inspect and maintain the repository's .pm workstreams, inbox notes, milestones, tasks, and routine lists. Use when the user asks for PM board status or requests a supported board mutation such as creating, promoting, adding, or completing work, or scheduling a routine.


allowed-tools: Read, Write, Edit, Bash(ls:*), Bash(find:*), Bash(cat:*), Bash(mv:*), Bash(git rm:*), Bash(git fetch:*), Bash(git ls-tree:*), Bash(npx prettier*:*), Bash(python3 scripts/pm-board-validate.py:*), Bash(bash scripts/docs-links-validate.sh:*), Bash(python3 scripts/loopx-guard.py routine-due:*), Bash(python3 scripts/pm-roll-closed.py:*)
---

# Task: Arrange the `.pm` board

`/pm` is the only skill that files and schedules board items: workstreams, notes, milestones, tasks and routines. A `/loopx` worker also lands an item's outcome (done, `blocked/`, or delete) under these same conventions ([`../loopx/worker.md`](../loopx/worker.md#3-land-the-outcome)). No other skill writes to `.pm/`. `/pm` arranges milestones and tasks under the conventions below. `/pm-brainstorm` proposes; `/pm` materializes. This file is the **canonical** definition of the board conventions — hierarchy, sizing rule, quality gate, standing closing tasks, templates. `/pm-brainstorm` reads it at runtime and must not restate or diverge from it. Parse the subcommand from `$ARGUMENTS` (default = `status`).

## The `.pm` hierarchy

| Level | Path | Meaning | Effort |
| --- | --- | --- | --- |
| Workstream | `wN/` (`w1`, `w2`, `w3`, …) | a general-purpose worker queue; `README.md` + inbox notes | — |
| Inbox note | `wN/NNN.md` (`w1/005.md`) | one idea or a **sub-hour** unit of work, plain markdown | ≤ ~1h |
| Milestone | `wN/mN/` (`m1`, `m2`, …) | a shippable chunk: `README.md` + task files | **> ~1h**, multiple tasks |
| Task | `wN/mN/tNNN.md` | a single unit | tens of minutes |
| Routine list | `wN/routine/README.md` | scheduled commands `/loopx <wN>` runs when the queue is drained | — |

## Rules (enforce every time)

- **Respect the anti-goals.** Read `.pm/DO_NOT_DO.md` before proposing or materializing work. Do not create milestones/tasks that conflict with it.
- **The board is public.** Anyone can read `bex-co/bex`, every `.pm/` file (`done/` and `blocked/` included) and `docs/`, and git keeps a line after it is edited ([`.pm/AGENTS.md`](../../../.pm/AGENTS.md#the-board-is-public)).
  - Name only bex's own fixture workspaces and services: those in `scripts/lib/first_party_workspaces.txt`, plus the disposable QA and drill workspaces a run creates and deletes, each marked `(disposable)` on the line that names it.
  - Hash other tenants' ids, service or app names, emails and metering with `scripts/lib/public_log.py`, or leave them out. Neutral words work too ("a Scale customer workspace", "customer workspace 3 of 11"). The full detail stays in your own local notes.
  - `scripts/ship-gates.sh` warns about an added `tea-` id that is neither listed nor marked. The warning never stops a ship.
- **Workers are general-purpose.** Every `wN` may take work anywhere in the bex project. Workstream placement records scheduling and history, not a permanent mission, specialty, component boundary, or ownership claim. Do not assign themes or missions to workstreams, and do not use prior milestones to reject otherwise appropriate work.
- **Sizing rule.** A milestone must be **> ~1 hour of work across more than one task**. If a chunk is ≤ ~1h (tens of minutes, a task or two), do **NOT** create an `mN/` directory — record it as a loose inbox note `wN/NNN.md`. Tasks take tens of minutes; milestones take hours.
- **A gate that clears by itself has one checkable form.** A parked item's gate lives in its workstream line's `**BLOCKED (…)**`. When it waits for production to run some commits, it names them as ``pin must carry `<sha>`[, `<sha>`…]``: commits on `main`, full or abbreviated (for "the second deploy after X", name the pin commit of the first). When it waits for a time, it says `date gate: YYYY-MM-DD` or `date gate: YYYY-MM-DDTHH:MMZ` (a bare date means 00:00Z). A gate can carry both, and free text around them says what the worker does next. `python3 scripts/pm-board-validate.py` refuses a BLOCKED summary that waits for a production pin, deploy or roll, or names a date gate, without its form, or whose pin gate names no commit in the repository. `python3 scripts/loopx-guard.py board <wN>` lists the item under `blocked.ready` once the newest `chore(deploy): pin platform images to <sha>` on `origin/main` contains every named commit and the date has passed, and `/loopx` resumes it with no board change (`scripts/pm_gates.py` reads both forms). Gates that need a person (a decision, a credential, hardware) stay free text. When the pin part of such a gate already holds, record it as a fact ("production already carries `<sha>`") instead.
- **IDs must match the path.** A task's `id: wN/mN/tNNN` frontmatter must equal the directory it lives in. Never create a milestone dir whose path disagrees with the IDs inside it; if you find existing drift, flag and repair it.
- **Keep status in sync** across all three places it lives: the workstream `README.md` milestone checkbox, the milestone `README.md` `**Status:**` line + the `— DONE` marker in the task table, and each task's `status:` frontmatter.
- **Numbering:** next free zero-padded 3-digit for inbox notes (`NNN`, per workstream) and tasks (`tNNN`, per milestone); next free `wN` / `mN` (milestone numbers are global across workstreams, `done/` and `blocked/` included). Every worker has its own checkout, so the local tree alone hands two workers the same number (`m348` is already used by three workstreams). Run `git fetch origin main` first and take the highest number across the local tree **and** `git ls-tree -r --name-only origin/main -- .pm` (for `NNN` and `tNNN`, within the same workstream or milestone); don't reuse a number. `python3 scripts/pm-board-validate.py` fails a change that adds a milestone number another workstream uses.
- **Moves rewrite inbound links.** Any move (`done`, a milestone closeout, `blocked/`, `unblock`) leaves nothing pointing at the old path: search `docs/`, every `AGENTS.md` and `README.md`, `.pm/*.md`, every `.pm/*/README.md` and every `.pm/*/done/closed-*.md` for the old relative path and rewrite each link to the new one. A file that is gone for good (purged evidence, a removed item) is written as plain text with its id, never as a link; report any link that can be neither rewritten nor made plain text. `bash scripts/docs-links-validate.sh` must exit 0 afterwards (it runs in CI, `.github/workflows/references.yml`), and it reads each workstream's own `.pm/wN/README.md` and its `done/closed-*.md` as well as `docs/`, every `AGENTS.md`/`README.md` and `.pm/*.md`.
- **A workstream README holds open work; its closed lines roll into `done/`.** A closed `- [x]` line stays in the README when it is checked. Once the README passes 384 KiB, roll it with `python3 scripts/pm-roll-closed.py <wN>`. This keeps it clear of the markdown size guard, which warns at 512 KiB and refuses at 768 KiB (`scripts/repo-artifact-guard.sh`, which names the command when it warns about a workstream README). The script moves every top-level `- [x]` line, with the indented lines under it, into new `wN/done/closed-K.md` files, each under 256 KiB. Each file keeps the README sections and order the lines had, and the script rewrites their relative links for `done/` and lists each file under the README's `## Closed items`. Open and parked lines, headings and prose stay, so `scripts/loopx-guard.py board`, `scripts/pm_gates.py` and `scripts/pm-board-validate.py` read the README as before. Nothing is appended to a `closed-K.md` later: the next roll writes the next K (counting `origin/main`). Ship the roll as its own `docs(pm): roll <wN>'s closed lines into done/` commit. If it conflicts on rebase, take main's README, delete the new `closed-K.md` files and run the script again. Never raise the size limit or exempt a file from it instead.
- Use `worker: worker1` unless the workstream README names another worker.
- **Milestones must be meaningful.** Every milestone must include direct project-goal linkage, an observable expected outcome, and why this work matters now (dependency/risk/sequence rationale).
- **Every board item briefly explains why.** Milestones carry it in the milestone `README.md` (`## Source + Goal linkage`). Single tasks — inbox notes `wN/NNN.md` — carry it in the note itself: a one-line `Why: …` directly under the title (a sentence on the motivation/payoff is enough).
- **Every milestone ends with standing closing tasks**, appended after the implementation tasks whenever a milestone is materialized:

  1. **Render parity** — _only when the milestone is feature development or a fix that touches a user/tenant-facing surface._ Check that the change is consistent across every surface it exposes: REST, GraphQL, and MCP in `lego/backend/` (same fields, semantics, error shapes — bex-api is meant to be Render-compatible, see `docs/ADR006-bex-api.md`) and the `dashboard/` UI (`dashboard/AGENTS.md`). Compare against the equivalent render.com behavior/API and flag any drift as follow-up work rather than silently diverging. For cloud coding-agent surfaces (agent sessions, `/agents`, sandboxes, PR delivery; `docs/ADR047-cloud-coding-agent-sessions.md`), the reference product is **Devin** instead of render.com, and the same check applies against Devin's observable behavior. Omit this task only for milestones with no REST/GraphQL/MCP/UI surface change (pure infra, operator-internal mechanism, docs, etc.) — note why it was omitted in the milestone's `## Source + Goal linkage`.
  2. **Simplify** — run `/simplify` over the code this milestone changed (reuse / simplification / efficiency; behavior-preserving).
  3. **Test coverage** — add meaningful tests for the behavior this milestone shipped. Tests must assert real behavior and failure modes; never game coverage with trivial, tautological, or snapshot-everything tests.
  4. **Closeout** — the final task, added last. When the milestone's other tasks are all complete **and its definition of done is actually met**, close the milestone: set every remaining task's `status: done`, move each `tNNN.md` to `wN/mN/done/`, mark every row `— **DONE**` and set `**Status:** done` in the milestone `README.md`, move the whole `wN/mN/` directory to `wN/done/mN/`, and check `- [x]` in the workstream `README.md`, then rewrite the links that pointed at the old paths (see **Moves rewrite inbound links**). Completing this task _is_ the move — running `/pm done <wN/mN/tNNN>` on it last triggers the milestone move (the `done` subcommand's step 4). Do **not** run it until the DoD holds: a milestone lands in `done/` when its observable end state is real, not merely when the code is written.

  Each `depends_on` the last implementation task(s) (Simplify and Test coverage depend on Render parity when it's present; Closeout depends on Test coverage) and all count toward the `(N tasks)` total. `add-task` inserts new work **before** these (before Closeout) and updates their `depends_on`.

- After editing any `.md`, run `npx prettier@3.4.2 --write "**/*.md"` (repo rule).

## Subcommands

### `status` (default)

Read the tree (`find .pm -type f -name '*.md'`, skipping `done/`) and `.pm/DO_NOT_DO.md`. Print, per open workstream: its milestones with `**Status:**`, and the **next actionable task** per milestone — the first non-done task whose `depends_on` are all satisfied. Also list open inbox notes, and the rows of the workstream's routine list when it has one. Then run a lightweight validation pass and flag:

- items conflicting with `.pm/DO_NOT_DO.md`,
- milestones missing `## Source + Goal linkage`,
- milestones whose definition of done is vague/non-testable,
- inbox notes missing a `Why:` line.

Touch no files.

### `new workstream`

Create the next free `wN/` with a role-neutral `README.md` from the workstream template below.

### `add <wN> <idea…>`

Create the next free inbox note `wN/NNN.md` with the idea as plain terse markdown (no frontmatter), with a one-line `Why: …` directly under the title. This is the default home for **sub-hour** work.

### `promote <wN/NNN>` / `new milestone <wN> <title>`

Apply the **sizing rule first.**

- If the work is **> ~1h and splits into more than one task**: create `wN/mN/` with `README.md` (milestone template) + one `tNNN.md` per task (task template) **+ the standing closing tasks (Render parity when it's feature dev/a fix touching REST/GraphQL/MCP/UI, then Simplify, then Test coverage, then Closeout)**, add the `- [ ] **mN** — …` line to the workstream `README.md`, and fill `## Source + Goal linkage` with source + goal linkage + expected outcome + why-now rationale (note there why Render parity was included or omitted).
- If it is **≤ ~1h**: do NOT create a milestone. Keep/append it as an inbox note `wN/NNN.md` (with its one-line `Why: …`) and tell the user why (too small for a milestone).

### `add-task <wN/mN> <title>`

Create the next `tNNN.md` from the task template and add its row to the milestone `README.md` table **before the standing closing tasks**, updating their `depends_on` to include it. Update the `(N tasks)` count in the workstream README.

### `done <wN/mN/tNNN>`

1. Set the task's frontmatter `status: done`.
2. In the milestone `README.md`: mark the row `— **DONE**` and update the `**Status:**` line (e.g. `todo (t001 done)`).
3. **Move** the file to `wN/mN/done/tNNN.md`.
4. If no open tasks remain in the milestone, **move the whole milestone** to `wN/done/mN/` and check its box (`- [x]`) in the workstream `README.md`.
5. **Rewrite inbound links** to every path this moved (the task file, and the milestone directory when it moved), as **Moves rewrite inbound links** says, then run `bash scripts/docs-links-validate.sh`.

Show the intended moves before mutating if the user passed `DRY_RUN=1`.

### `unblock <wN/mN|wN/NNN> <decision or verified finding>`

Record an unblock outcome on an existing item; `/unblock` supplies the conversation and evidence. Accept the item's logical ID or its current `blocked/` path. This subcommand also records partial decisions that do **not** yet clear every gate.

If current-code evidence proves the item obsolete, duplicate or superseded, apply a terminal disposition instead of reopening it. Cite the checked revision, concrete evidence and surviving item when applicable. Items containing findings, decision history or completed task records are archived to `done/` with an explicit superseded/invalid verdict; synchronize the normal done indexes without claiming the original implementation or live acceptance passed. Only items with no unique history may be deleted with their README entry removed. This cleanup needs no repeated user decision; uncertainty alone never establishes obsolescence.

1. Re-read the item and affected READMEs immediately before writing; other sessions may have updated or moved them. Locate the existing item rather than recreating it. If both blocked and active copies exist, inspect and reconcile their history before moving anything; never overwrite one with the other.
2. Append a dated decision record to the milestone README or inbox note: affected IDs, source (explicit user decision, existing rule, or verified evidence), chosen behavior and scope, rationale (including the reference-product basis: how render.com does it for hosting, or Devin for coding agents), remaining gates with owner and release condition, and the worker's next steps and observable acceptance checks. Preserve prior decisions and evidence; do not attribute a recommendation or silence to the user. Apply a shared decision to each affected item explicitly. Do not store secrets.
3. Update affected task objectives, steps, dependencies and acceptance criteria to reflect the outcome. Preserve completed tasks and their `done/` files. A scope change needs an explicit basis; never remove an unmet DoD merely to clear a blocker. Verify gates before moving: a plan to grant access is not working access. If any necessary gate remains, keep the item in `blocked/` and update the workstream's BLOCKED summary with the precise remaining action.
4. Once the remaining work's gates are verified clear, move the whole milestone `wN/blocked/mN/` → `wN/mN/`, or note `wN/blocked/NNN.md` → `wN/NNN.md`, preserving IDs, numbering and history. Leave no stub at the old path. Keep the workstream checkbox unchecked, replace the current BLOCKED summary with the next action, and update its link. For milestones, set the README status to `todo` (noting completed tasks) and reset only the now-unblocked tasks' blocked statuses to `todo`; keep DONE markers and completed frontmatter unchanged. Repair affected links in the moved documents and board indexes, and rewrite the inbound links to the old path (see **Moves rewrite inbound links**).
5. A deployed version is not proof that its live DoD passed: ready-for-QA work returns as pending verification. If the original end state is already verified, use the normal done conventions instead of reopening or marking unimplemented work done. An already-active item gets only genuinely new decision information; a repeated invocation must not duplicate the record or create a second item.

Run the repository Markdown formatter and verify that file locations, README links/statuses, task frontmatter and dependencies agree. With `DRY_RUN=1`, show the intended record, status changes and moves without writing. Implementation and verification belong to `/loopx <wN>` after the board is ready; this subcommand does not start a worker or grant commit/push permission.

### `routine <wN> add <name> <every> [always|changed] [paths] -- <command>` / `routine <wN> remove <name>` / `routine <wN> budget <N>`

Edit the workstream's routine list, `wN/routine/README.md` (template below). `/loopx <wN>` runs the listed routines only when the queue is drained. `scripts/loopx-guard.py` schedules them: each row runs on its own interval, and new board items always go first. The list holds configuration only: it is not an item and never moves to `done/`. Run history stays out of the board, in `.git/loopx/`.

- `add`: everything after `--` is the `<command>`, stored verbatim. It can be any command a `/loopx` worker can carry out: a slash command with all of its arguments (`/rt-remove-dead-code go`, `/qa-find-bugs-cli w4`, `/simplify`), a shell command, or a task in plain words. Nothing is appended at run time, so spell out every argument (a QA skill needs its target workstream to file into this one).
  - A row is a standing request to run the command, and to ship what it produces, every interval while `/loopx <wN>` runs. Add one only when the user asks for it, and write limits such as `DRY_RUN=1` or "report only" into the command itself.
  - Refuse unless `<name>` is a unique lowercase slug (letters, digits, dashes; it keys the run state), `<every>` is an interval like `30m`, `24h`, `3d`, or `1w`, and the command contains no `|` (wrap a pipeline in a script). When the command is a slash command for a repository skill, confirm `.claude/skills/<skill>/SKILL.md` exists.
  - `when` defaults to `always` (run every interval). `changed` is an opt-in token saver: run only after one of the comma-separated repository-relative `paths` changed (any path when none are given), and double the interval after a run that ships nothing.
  - Pair a short interval with a matching `budget`, or the default of 4 runs/day caps it. Create the file from the template if it does not exist (git does not track an empty `routine/` directory).
- `remove`: delete the row with that name. Keep the file even when the table is empty: an empty table means no routines.
- `budget`: set the `Budget: N runs/day` line (default 4 when absent).
- **Verify before returning:** run `python3 scripts/loopx-guard.py routine-due <wN>` and confirm the reply lists no `problems` for the rows you touched. It reads `origin/main`, so it shows the change only after it ships; before that, check the rows by hand against the template.

## Templates

### Workstream `README.md`

```markdown
# wN — project workstream (<worker>)

**Worker:** <worker>. This is a general-purpose bex workstream. It may take work anywhere in the project; the milestones below are scheduled work and historical records, not a permanent purpose, specialty, or ownership boundary.

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

## Source + Goal linkage

- **Source:** <pointer to the inbox note / brainstorm / docs this came from>
- **Goal linkage:** <which project goal / pillar this advances>
- **Expected outcome:** <observable impact after shipping>
- **Why now:** <dependency / risk / sequence rationale>
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

Plain terse markdown, no frontmatter — one idea or a sub-hour unit of work. Opens with, directly under the title:

```markdown
Why: <one sentence — motivation / payoff>
```

### Routine list `wN/routine/README.md`

```markdown
# wN routines

Routines `/loopx wN` runs on schedule whenever the queue is drained; new board items always go first. A command is run verbatim: a slash command with its arguments, a shell command, or a task in plain words. `when: always` (the default) runs every interval; `when: changed` also waits for one of its `paths` (any path when empty) to change and backs off after empty runs. Edit with `/pm routine`.

Budget: 4 runs/day

| name         | command                   | every | when    | paths |
| ------------ | ------------------------- | ----- | ------- | ----- |
| go-dead-code | `/rt-remove-dead-code go` | 24h   | changed | lego  |
```

## Arguments

$ARGUMENTS
